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
use std::cmp::Ordering;
use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};

use crate::model::{
    AliasModel, EnsembleBuilder, FreqModel, LlmConfig, LlmModel, LlmWhich, PrefixModel,
    SqlitePool, SuggestModel, Suggestion,
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
}

impl App {
    pub fn new(
        corpus: Vec<String>,
        top: usize,
        db: Option<SqlitePool>,
        enable_llm: bool,
        llm_model: Option<PathBuf>,
        llm_which: String,
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
        if let Some(ref pool) = db {
            builder = builder
                .with_light_model(PrefixModel::new(pool.clone()))
                .with_light_model(FreqModel::new(pool.clone()))
                .with_light_model(AliasModel::with_sql_store(pool.clone()));
        }

        // Add LLM model if enabled
        if enable_llm {
            let which = LlmWhich::from_str(&llm_which).unwrap_or(LlmWhich::W0_5b);
            let llm_config = LlmConfig {
                model_path: llm_model,
                which,
                ..Default::default()
            };
            builder = builder.with_light_model(LlmModel::new(llm_config));
        }

        let ensemble = builder.build();

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
        })
    }

    pub fn refresh_suggestions(&mut self) {
        if self.input.trim().is_empty() {
            self.suggestions.clear();
            self.selected = 0;
            return;
        }

        let query = self.input.as_str();

        // Use ensemble for multi-model suggestions
        match self.ensemble.predict(query) {
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
                warn!("Ensemble prediction failed: {}. Falling back to fuzzy matching.", e);

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
