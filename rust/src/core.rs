use ahash::AHashSet;
use anyhow::{bail, Context, Result};
use directories::UserDirs;
use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;
use hex::encode;
use libsql::Value;
use once_cell::sync::Lazy;
use ratatui::layout::Rect;
use sha2::{Digest, Sha256};
use log::{info, warn};
use std::cmp::Ordering;
use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};
use tokio::sync::mpsc;
use tokio::task::JoinHandle;

use crate::model::{
    AliasModel, EmbeddingModel, EmbeddingStore, EnsembleBuilder, FreqModel,
    LlamaEmbeddingClient, LlmConfig, LlmModel, PrefixModel, SqlitePool,
    SuggestModel, Suggestion,
};
use crate::model::ensemble::Ensemble;

static MATCHER: Lazy<SkimMatcherV2> = Lazy::new(SkimMatcherV2::default);
/// Run the fuzzy search over one or more history files
pub fn run_search(files: Vec<PathBuf>, query: &str, top: usize, unique: bool) -> Result<()> {
    if files.is_empty() {
        bail!("Please specify at least one --file");
    }

    // Read all lines from the provided files
    let mut lines = Vec::new();
    for path in files {
        lines.extend(read_history_file(&path).with_context(|| format!("reading {:?}", path))?);
    }

    // Optionally remove duplicates
    if unique {
        let mut seen = AHashSet::with_capacity(lines.len());
        lines.retain(|s| seen.insert(s.to_string()));
    }

    let mut builder = EnsembleBuilder::new().with_light_model(FuzzyHistoryModel::new(lines));

    if let Ok(pool) = SqlitePool::open_default() {
        builder = builder
            .with_light_model(PrefixModel::new(pool.clone()))
            .with_light_model(FreqModel::new(pool.clone()))
            .with_light_model(AliasModel::with_sql_store(pool));
    }

    let ensemble = builder.build();

    let suggestions = ensemble.predict(query)?;
    for suggestion in suggestions.into_iter().take(top) {
        println!("{}", suggestion.text);
    }
    Ok(())
}

// ---------------------
// TUI model
// ---------------------
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Main,
    History,
}

#[derive(Clone)]
pub struct HistoryEntry {
    pub cmd: String,
    pub output_lines: Vec<String>,
}

pub struct App {
    // input
    pub input: String,
    pub cursor: usize,

    // suggestions
    pub suggestions: Vec<String>,
    pub selected: usize,
    pub max_suggestions: usize,

    // history
    pub history: Vec<HistoryEntry>,

    // output streaming
    pub output_lines: Vec<String>,
    pub is_running: bool,
    pub last_run_cmd: Option<String>,

    // view state
    pub current_tab: Tab,
    pub selected_history_index: usize,
    pub pinned_output: bool, // middle-left shows output instead of suggestions
    pub recent_runs_area: Option<Rect>, // clickable area cache
    pub main_tab_area: Option<Rect>,
    pub history_tab_area: Option<Rect>,
    pub output_scroll: u16,        // scroll offset for main tab output
    pub history_scroll: u16,       // scroll offset for history tab output

    // corpus (legacy fuzzy matching)
    pub corpus: Vec<String>,

    // ensemble for multi-model suggestions
    pub ensemble: Ensemble,

    // debounce state for suggestion refresh
    pub last_input_time: Option<Instant>,
    pub pending_refresh: bool,

    // async heavy model state
    heavy_model_rx: Option<mpsc::UnboundedReceiver<Vec<Suggestion>>>,
    heavy_model_tx: Option<mpsc::UnboundedSender<Vec<Suggestion>>>,
    heavy_model_tasks: Vec<JoinHandle<()>>,
    pending_heavy_model_query: Option<String>,
}

impl App {
    pub fn new(
        corpus: Vec<String>,
        top: usize,
        db: Option<SqlitePool>,
        enable_embedding: bool,
        embedding_model: Option<PathBuf>,
        enable_llm: bool,
        llm_model: Option<PathBuf>,
    ) -> Result<Self> {
        // Load recent history from database
        let history = if let Some(ref pool) = db {
            load_recent_history(pool, 100).unwrap_or_default()
        } else {
            Vec::new()
        };

        // Build ensemble with all suggestion models
        let mut builder = EnsembleBuilder::new().with_light_model(FuzzyHistoryModel::new(corpus.clone()));

        // Add database-backed models if available
        if enable_embedding {
            if let Some(ref pool) = db {
                builder = builder
                    .with_light_model(PrefixModel::new(pool.clone()))
                    .with_light_model(FreqModel::new(pool.clone()))
                    .with_light_model(AliasModel::with_sql_store(pool.clone()));

                match LlamaEmbeddingClient::from_env_or(embedding_model.clone()) {
                    Ok(client) => {
                        let store = EmbeddingStore::new(pool.clone());
                        let embedding_model = EmbeddingModel::new(store, client);
                        match embedding_model.warm_up() {
                            Ok(_) => {
                                if let Err(err) = embedding_model.learn(&corpus) {
                                    warn!("embedding warmup failed: {err:?}");
                                }
                                builder = builder.with_heavy_model(embedding_model);
                                info!("embedding model enabled via Ollama");
                            }
                            Err(err) => {
                                warn!("skipping embedding model; Ollama embeddings unavailable: {err:?}");
                            }
                        }
                    }
                    Err(err) => {
                        warn!("failed to construct embedding client: {err:?}");
                    }
                }
            }
        }

        // Add LLM model as heavy model if enabled
        if enable_llm {
            if let Some(model_path) = llm_model {
                let llm_config = LlmConfig {
                    model_path,
                    ..Default::default()
                };
                builder = builder.with_heavy_model(LlmModel::new(llm_config));
            } else {
                warn!("--enable-llm specified but --llm-model not provided");
            }
        }

        let ensemble = builder.build();

        // Create channel for async heavy model results
        let (tx, rx) = mpsc::unbounded_channel();

        Ok(Self {
            input: String::new(),
            cursor: 0,
            suggestions: Vec::new(),
            selected: 0,
            max_suggestions: top,
            history,
            output_lines: Vec::new(),
            is_running: false,
            last_run_cmd: None,
            current_tab: Tab::Main,
            selected_history_index: 0,
            pinned_output: false,
            recent_runs_area: None,
            main_tab_area: None,
            history_tab_area: None,
            output_scroll: 0,
            history_scroll: 0,
            corpus,
            ensemble,
            last_input_time: None,
            pending_refresh: false,
            heavy_model_rx: Some(rx),
            heavy_model_tx: Some(tx),
            heavy_model_tasks: Vec::new(),
            pending_heavy_model_query: None,
        })
    }

    pub fn refresh_suggestions(&mut self) {
        if self.input.trim().is_empty() {
            self.suggestions.clear();
            self.selected = 0;
            return;
        }

        let query_owned = self.input.clone();
        let query = query_owned.as_str();

        // Phase 1: Get quick suggestions from light models (non-blocking)
        match self.ensemble.predict_light_models(query) {
            Ok(suggestions) => {
                self.suggestions = suggestions
                    .into_iter()
                    .take(self.max_suggestions)
                    .map(|s| s.text)
                    .collect();
            }
            Err(e) => {
                // Fallback to simple fuzzy matching if ensemble fails
                use log::warn;
                warn!("Light model prediction failed: {}. Falling back to fuzzy matching.", e);

                let mut scored: Vec<(i64, String)> = Vec::new();
                for line in self.corpus.iter() {
                    if let Some(score) = MATCHER.fuzzy_match(line, query) {
                        scored.push((score, line.clone()));
                    }
                }
                scored.sort_by(|a, b| b.0.cmp(&a.0));
                self.suggestions = scored
                    .into_iter()
                    .take(self.max_suggestions)
                    .map(|(_, s)| s)
                    .collect();
            }
        }

        self.selected = self.selected.min(self.suggestions.len().saturating_sub(1));
        self.pending_refresh = false; // Clear pending flag after refresh

        // Phase 2: Spawn background tasks for heavy models (non-blocking)
        self.spawn_heavy_model_tasks(query);
    }

    /// Mark that input has changed, but defer the actual suggestion refresh (debounce)
    pub fn mark_input_changed(&mut self) {
        self.last_input_time = Some(Instant::now());
        self.pending_refresh = true;
    }

    /// Check if enough time has passed since last input to refresh suggestions
    /// Debounce delay: 300ms
    pub fn should_refresh_suggestions(&self) -> bool {
        const DEBOUNCE_MS: u64 = 300;

        if !self.pending_refresh {
            return false;
        }

        if let Some(last_time) = self.last_input_time {
            last_time.elapsed() >= Duration::from_millis(DEBOUNCE_MS)
        } else {
            false
        }
    }

    /// Spawn background tasks for heavy model predictions
    /// Cancels any previous tasks and spawns new ones
    fn spawn_heavy_model_tasks(&mut self, query: &str) {
        // Cancel all previous heavy model tasks
        for handle in self.heavy_model_tasks.drain(..) {
            handle.abort();
        }

        // Get heavy models from ensemble
        let heavy_models = self.ensemble.get_heavy_models();

        if heavy_models.is_empty() {
            return; // No heavy models to run
        }

        // Save current query to detect if it changes
        self.pending_heavy_model_query = Some(query.to_string());

        let query = query.to_string();
        let tx = match &self.heavy_model_tx {
            Some(tx) => tx.clone(),
            None => return,
        };

        // Spawn a task for each heavy model
        for model in heavy_models {
            let query = query.clone();
            let tx = tx.clone();

            let handle = tokio::spawn(async move {
                // Run heavy model prediction in blocking task (subprocess calls)
                let result = tokio::task::spawn_blocking(move || {
                    model.predict(&query)
                }).await;

                // Send results through channel
                if let Ok(Ok(suggestions)) = result {
                    let _ = tx.send(suggestions);
                }
            });

            self.heavy_model_tasks.push(handle);
        }
    }

    /// Poll for heavy model results without blocking
    /// Merges results into current suggestions if they arrive
    pub fn poll_heavy_model_results(&mut self) {
        let mut pending_results = Vec::new();
        {
            let rx = match &mut self.heavy_model_rx {
                Some(rx) => rx,
                None => return,
            };

            // Non-blocking check for results
            while let Ok(heavy_suggestions) = rx.try_recv() {
                pending_results.push(heavy_suggestions);
            }
        }

        for heavy_suggestions in pending_results {
            self.merge_heavy_model_suggestions(heavy_suggestions);
        }
    }

    /// Merge heavy model suggestions into current suggestion list
    /// Uses the same scoring logic as ensemble aggregation
    fn merge_heavy_model_suggestions(&mut self, heavy_suggestions: Vec<Suggestion>) {
        use std::collections::HashMap;

        // Build a map of existing suggestions with their positions
        let mut score_map: HashMap<String, f64> = HashMap::new();

        for (idx, text) in self.suggestions.iter().enumerate() {
            // Higher positt on = lower score in the list
            let position_score = (self.suggestions.len() - idx) as f64;
            score_map.insert(text.clone(), position_score);
        }

        // Add heavy model suggestions with their scores
        for suggestion in heavy_suggestions {
            let entry = score_map.entry(suggestion.text).or_insert(0.0);
            *entry += suggestion.score;
        }

        // Re-rank all suggestions
        let mut ranked: Vec<(String, f64)> = score_map.into_iter().collect();
        ranked.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(Ordering::Equal));

        // Update suggestions list
        self.suggestions = ranked
            .into_iter()
            .take(self.max_suggestions)
            .map(|(text, _)| text)
            .collect();

        // Adjust selection to stay in bounds
        self.selected = self.selected.min(self.suggestions.len().saturating_sub(1));
    }

}

pub fn load_history_lines(files: Vec<PathBuf>, unique: bool) -> Result<Vec<String>> {
    let mut paths = files;

    if paths.is_empty() {
        if let Some(ud) = UserDirs::new() {
            let home = ud.home_dir();
            let candidates = [".zsh_history", ".bash_history", ".fish_history"];
            for c in candidates {
                paths.push(home.join(c));
            }
        }
    }

    if paths.is_empty() {
        bail!("No history files provided and HOME not found");
    }

    let mut lines: Vec<String> = Vec::new();
    for p in paths {
        if p.exists() {
            lines.extend(read_history_file(&p).with_context(|| format!("reading {p:?}"))?);
        }
    }

    if unique {
        let mut seen = AHashSet::with_capacity(lines.len());
        lines.retain(|s| seen.insert(s.to_owned()));
    }
    Ok(lines)
}

pub fn read_history_file(path: &Path) -> Result<Vec<String>> {
    let mut file = File::open(path).with_context(|| format!("opening {path:?}"))?;
    let mut buf = Vec::new();
    file.read_to_end(&mut buf)?;
    Ok(String::from_utf8_lossy(&buf)
        .lines()
        .map(|l| l.to_owned())
        .collect())
}

fn load_recent_history(pool: &SqlitePool, limit: usize) -> Result<Vec<HistoryEntry>> {
    pool.query_collect(
        "SELECT command, output FROM command_executions ORDER BY executed_at DESC LIMIT ?1",
        vec![Value::Integer(limit as i64)],
        |row| {
            let command: String = row.get(0)?;
            let output_str: String = row.get(1).unwrap_or_default();

            let output_lines: Vec<String> = output_str
                .lines()
                .map(|s| s.to_string())
                .collect();

            Ok(HistoryEntry {
                cmd: command,
                output_lines,
            })
        },
    )
}

fn hash_command(command: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(command.as_bytes());
    encode(hasher.finalize())
}

pub fn persist_command_to_history(pool: &SqlitePool, command: &str, session_id: &str) -> Result<()> {
    let trimmed = command.trim();
    if trimmed.is_empty() {
        return Ok(());
    }

    let hash = hash_command(trimmed);

    // Update history table (for frequency counting)
    pool.execute(
        r#"
        INSERT INTO history (command, hash, count, source, output)
        VALUES (?1, ?2, 1, 'tui', '')
        ON CONFLICT(hash) DO UPDATE SET
            count = count + 1,
            source = 'tui',
            created_at = CURRENT_TIMESTAMP;
    "#,
        vec![
            Value::Text(trimmed.to_string()),
            Value::Text(hash),
        ],
    )?;

    // Insert into command_executions (for full history with output)
    pool.execute(
        r#"
        INSERT INTO command_executions (command, output, session_id, executed_at)
        VALUES (?1, '', ?2, CURRENT_TIMESTAMP);
    "#,
        vec![
            Value::Text(trimmed.to_string()),
            Value::Text(session_id.to_string()),
        ],
    )?;

    Ok(())
}

pub fn import_shell_history_to_db(pool: &SqlitePool, files: &[PathBuf]) -> Result<()> {
    let lines = load_history_lines(files.to_vec(), true)?; // unique=true to avoid duplicates in memory

    for command in lines {
        let trimmed = command.trim();
        if trimmed.is_empty() {
            continue;
        }

        let hash = hash_command(trimmed);

        // Insert into history table with source='shell'
        // On conflict, just increment count (don't change source from 'tui' to 'shell')
        pool.execute(
            r#"
            INSERT INTO history (command, hash, count, source, output)
            VALUES (?1, ?2, 1, 'shell', '')
            ON CONFLICT(hash) DO UPDATE SET
                count = count + 1,
                created_at = CURRENT_TIMESTAMP;
        "#,
            vec![
                Value::Text(trimmed.to_string()),
                Value::Text(hash),
            ],
        ).ok(); // Ignore errors for individual commands
    }

    Ok(())
}

#[derive(Debug)]
struct FuzzyHistoryModel {
    corpus: Vec<String>,
}

impl FuzzyHistoryModel {
    fn new(corpus: Vec<String>) -> Self {
        Self { corpus }
    }
}

impl SuggestModel for FuzzyHistoryModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        if input.trim().is_empty() {
            return Ok(Vec::new());
        }

        let mut scored: Vec<(f64, String)> = Vec::new();
        for line in &self.corpus {
            if let Some(score) = MATCHER.fuzzy_match(line, input) {
                scored.push((score as f64, line.clone()));
            }
        }

        scored.sort_by(|a, b| b.0.partial_cmp(&a.0).unwrap_or(Ordering::Equal));

        Ok(scored
            .into_iter()
            .map(|(score, text)| Suggestion::with_source(text, score, "history"))
            .collect())
    }

    fn weight(&self) -> f64 {
        1.0
    }
}
