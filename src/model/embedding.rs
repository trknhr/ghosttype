use std::path::PathBuf;
use std::process::{Command, Stdio};

use anyhow::{bail, Context, Result};
use libsql::Value;
use log::debug;

use super::{sqlite::SqlitePool, SuggestModel, Suggestion};

const DEFAULT_SOURCE: &str = "history";
const HEALTHCHECK_PROMPT: &str = "ghosttype-healthcheck";
const MAX_LEARN_INSERTS: usize = 100;
const SEARCH_TOP_K: usize = 10;
const SEARCH_THRESHOLD: f64 = 0.5;
const LLAMA_EMBED_BIN_ENV: &str = "LLAMA_EMBED_BIN";
const LLAMA_EMBED_MODEL_ENV: &str = "LLAMA_EMBED_MODEL";

#[derive(Clone, Debug)]
pub struct EmbeddingStore {
    pool: SqlitePool,
}

impl EmbeddingStore {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    pub fn exists(&self, source: &str, text: &str) -> Result<bool> {
        let rows = self.pool.query_collect(
            "SELECT COUNT(1) FROM embeddings WHERE source = ?1 AND text = ?2",
            vec![
                Value::Text(source.to_string()),
                Value::Text(text.to_string()),
            ],
            |row| {
                let count: i64 = row.get(0)?;
                Ok(count)
            },
        )?;
        Ok(rows.first().copied().unwrap_or(0) > 0)
    }

    pub fn save(&self, source: &str, text: &str, embedding: &[f32]) -> Result<()> {
        let emb_json = serialize_embedding(embedding);
        self.pool.execute(
            "INSERT INTO embeddings (source, text, emb) VALUES (?1, ?2, vector32(?3))",
            vec![
                Value::Text(source.to_string()),
                Value::Text(text.to_string()),
                Value::Text(emb_json),
            ],
        )
    }

    pub fn search_similar(
        &self,
        embedding: &[f32],
        source: &str,
        top_k: usize,
        threshold: f64,
    ) -> Result<Vec<Suggestion>> {
        let emb_json = serialize_embedding(embedding);
        let sql = r#"
WITH q AS (SELECT vector32(?1) AS v)
SELECT e.text, 1 - vector_distance_cos(e.emb, q.v) AS score
FROM q
JOIN vector_top_k('embeddings_idx', (SELECT v FROM q), ?2) AS t
JOIN embeddings e ON e.rowid = t.id
WHERE e.source = ?3
ORDER BY score DESC;
"#;
        let rows = self.pool.query_collect(
            sql,
            vec![
                Value::Text(emb_json),
                Value::Integer(top_k as i64),
                Value::Text(source.to_string()),
            ],
            |row| {
                let text: String = row.get(0)?;
                let score: f64 = row.get(1)?;
                Ok(Suggestion::with_source(text, score, source))
            },
        )?;
        Ok(rows
            .into_iter()
            .filter(|s| s.score >= threshold)
            .collect())
    }
}

#[derive(Clone, Debug)]
pub struct LlamaEmbeddingClient {
    binary: PathBuf,
    model_path: PathBuf,
}

impl LlamaEmbeddingClient {
    pub fn new(binary: impl Into<PathBuf>, model_path: impl Into<PathBuf>) -> Self {
        Self {
            binary: binary.into(),
            model_path: model_path.into(),
        }
    }

    pub fn from_env_or(model_path: Option<PathBuf>) -> Result<Self> {
        let binary = std::env::var(LLAMA_EMBED_BIN_ENV)
            .map(PathBuf::from)
            .unwrap_or_else(|_| PathBuf::from("llama-embedding"));

        let path = if let Ok(env_path) = std::env::var(LLAMA_EMBED_MODEL_ENV) {
            Some(PathBuf::from(env_path))
        } else {
            model_path
        };

        match path {
            Some(p) => Ok(Self::new(binary, p)),
            None => bail!(
                "LLAMA_EMBED_MODEL env var is not set and no --llm-model path was provided"
            ),
        }
    }

    pub fn embed(&self, text: &str) -> Result<Vec<f32>> {
        // llama-embedding -m ./model.gguf --log-disable -p "text"
        let output = Command::new(&self.binary)
            .arg("-m")
            .arg(&self.model_path)
            .arg("--log-disable")
            .arg("-p")
            .arg(text)
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .output()
            .with_context(|| "running llama-embedding")?;

        if !output.status.success() {
            bail!("llama-embedding exited with status {}", output.status);
        }

        let stdout = String::from_utf8_lossy(&output.stdout);
        parse_embedding_output(&stdout)
    }

    pub fn health_check(&self) -> Result<()> {
        let _ = self.embed(HEALTHCHECK_PROMPT)?;
        Ok(())
    }
}

#[derive(Clone, Debug)]
pub struct EmbeddingModel {
    store: EmbeddingStore,
    client: LlamaEmbeddingClient,
}

impl EmbeddingModel {
    pub fn new(store: EmbeddingStore, client: LlamaEmbeddingClient) -> Self {
        Self { store, client }
    }

    pub fn warm_up(&self) -> Result<()> {
        self.client.health_check()
    }

    pub fn learn(&self, entries: &[String]) -> Result<()> {
        let mut inserted = 0usize;

        for entry in entries {
            if inserted >= MAX_LEARN_INSERTS {
                break;
            }

            let candidate = entry.trim();
            if candidate.is_empty() {
                continue;
            }

            if self.store.exists(DEFAULT_SOURCE, candidate)? {
                continue;
            }

            match self.client.embed(candidate) {
                Ok(embedding) => {
                    if let Err(err) = self.store.save(DEFAULT_SOURCE, candidate, &embedding) {
                        debug!("failed to save embedding: {err:?}");
                    } else {
                        inserted += 1;
                    }
                }
                Err(err) => {
                    debug!("embedding request failed: {err:?}");
                }
            }
        }

        Ok(())
    }
}

impl SuggestModel for EmbeddingModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        if input.trim().is_empty() {
            return Ok(Vec::new());
        }

        let embedding = match self.client.embed(input) {
            Ok(vec) => vec,
            Err(err) => {
                debug!("embedding predict failed: {err:?}");
                return Ok(Vec::new());
            }
        };

        let mut suggestions = self
            .store
            .search_similar(&embedding, DEFAULT_SOURCE, SEARCH_TOP_K, SEARCH_THRESHOLD)?;
        let weight = self.weight();
        for suggestion in &mut suggestions {
            suggestion.score *= weight;
        }
        Ok(suggestions)
    }

    fn weight(&self) -> f64 {
        0.6
    }
}

fn serialize_embedding(vec: &[f32]) -> String {
    let mut out = String::with_capacity(vec.len() * 8 + 2);
    out.push('[');
    for (idx, value) in vec.iter().enumerate() {
        if idx > 0 {
            out.push(',');
        }
        out.push_str(&value.to_string());
    }
    out.push(']');
    out
}

fn parse_embedding_output(raw: &str) -> Result<Vec<f32>> {
    let mut values = Vec::new();

    for token in raw.split_whitespace() {
        let cleaned = token.trim_matches(|c| c == '[' || c == ']' || c == ',');
        if cleaned.is_empty() {
            continue;
        }
        match cleaned.parse::<f32>() {
            Ok(value) => values.push(value),
            Err(_) => {
                // ignore tokens that are not numeric
            }
        }
    }

    if values.is_empty() {
        bail!("llama-embedding returned no embedding values");
    }

    Ok(values)
}
