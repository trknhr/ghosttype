use ahash::AHashSet;
use anyhow::{bail, Context, Result};
use directories::UserDirs;
use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;
use hex::encode;
use libsql::Value;
use log::warn;
use once_cell::sync::Lazy;
use ratatui::layout::Rect;
use sha2::{Digest, Sha256};
use std::cmp::Ordering;
use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{self, Command, Stdio};
use std::sync::mpsc::Sender;
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

use crate::model::{
    AliasModel, EnsembleBuilder, FreqModel, PrefixModel, SqlitePool, SuggestModel, Suggestion,
};

static MATCHER: Lazy<SkimMatcherV2> = Lazy::new(SkimMatcherV2::default);
const SOURCE_TUI: &str = "tui";
const MAX_OUTPUT_LEN: usize = 16 * 1024;
const TRUNC_SUFFIX: &str = "\n…[truncated]";
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
    pub exit_code: Option<i32>,
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

    // corpus
    pub corpus: Vec<String>,

    db: Option<SqlitePool>,
    session_id: String,
}

impl App {
    pub fn new(corpus: Vec<String>, top: usize, db: Option<SqlitePool>) -> Self {
        // Load recent history from database
        let history = if let Some(ref pool) = db {
            load_recent_history(pool, 100).unwrap_or_default()
        } else {
            Vec::new()
        };

        Self {
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
            db,
            session_id: Self::generate_session_id(),
        }
    }

    pub fn refresh_suggestions(&mut self) {
        if self.input.trim().is_empty() {
            self.suggestions.clear();
            self.selected = 0;
            return;
        }
        let query = self.input.as_str();
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
        self.selected = self.selected.min(self.suggestions.len().saturating_sub(1));
    }

    fn generate_session_id() -> String {
        let ts = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis();
        format!("tui-{}-{ts}", process::id())
    }

    pub(crate) fn persist_last_run(&self, exit_code: i32) {
        let Some(pool) = self.db.as_ref() else {
            return;
        };
        let Some(cmd) = self.last_run_cmd.as_ref() else {
            return;
        };
        if let Err(err) =
            persist_history_entry(pool, cmd, &self.output_lines, exit_code, &self.session_id)
        {
            warn!("failed to persist history entry: {err:?}");
        }
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

pub enum ExecMsg {
    Line(String),
    Done(i32),
}

pub fn spawn_command(cmdline: String, tx: Sender<ExecMsg>) {
    thread::spawn(move || {
        let mut child = match Command::new("/bin/sh")
            .arg("-lc")
            .arg(&cmdline)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
        {
            Ok(c) => c,
            Err(e) => {
                let _ = tx.send(ExecMsg::Line(format!("spawn error: {e}")));
                let _ = tx.send(ExecMsg::Done(127));
                return;
            }
        };

        let stdout = child.stdout.take();
        let stderr = child.stderr.take();

        let tx1 = tx.clone();
        let t_out = thread::spawn(move || stream_reader(stdout, tx1));
        let tx2 = tx.clone();
        let t_err = thread::spawn(move || stream_reader(stderr, tx2));

        let status = child.wait().unwrap_or_default();
        let _ = t_out.join();
        let _ = t_err.join();
        let code = status.code().unwrap_or(-1);
        let _ = tx.send(ExecMsg::Done(code));
    });
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

fn stream_reader<R: Read + Send + 'static>(mut reader: Option<R>, tx: Sender<ExecMsg>) {
    use std::io::{BufRead, BufReader};
    if let Some(r) = reader.take() {
        let br = BufReader::new(r);
        for line in br.lines() {
            match line {
                Ok(l) => {
                    let _ = tx.send(ExecMsg::Line(l));
                }
                Err(e) => {
                    let _ = tx.send(ExecMsg::Line(format!("read error: {e}")));
                    break;
                }
            }
        }
    }
}

fn persist_history_entry(
    pool: &SqlitePool,
    command: &str,
    output_lines: &[String],
    exit_code: i32,
    session_id: &str,
) -> Result<()> {
    let trimmed = command.trim();
    if trimmed.is_empty() {
        return Ok(());
    }

    let hash = hash_command(trimmed);
    let output = truncate_output(&format_output(output_lines, exit_code));

    pool.execute(
        r#"
        INSERT INTO history (command, hash, count, source, session_id, output)
        VALUES (?1, ?2, 1, ?3, ?4, ?5)
        ON CONFLICT(hash) DO UPDATE SET
            count = count + 1,
            source = excluded.source,
            session_id = excluded.session_id,
            output = excluded.output;
    "#,
        vec![
            Value::Text(trimmed.to_string()),
            Value::Text(hash),
            Value::Text(SOURCE_TUI.to_string()),
            Value::Text(session_id.to_string()),
            Value::Text(output),
        ],
    )?;

    Ok(())
}

fn load_recent_history(pool: &SqlitePool, limit: usize) -> Result<Vec<HistoryEntry>> {
    pool.query_collect(
        "SELECT command, output FROM history ORDER BY id DESC LIMIT ?1",
        vec![Value::Integer(limit as i64)],
        |row| {
            let command: String = row.get(0)?;
            let output_str: String = row.get(1).unwrap_or_default();

            // Parse output to extract lines and exit code
            let mut output_lines = Vec::new();
            let mut exit_code = None;

            for line in output_str.lines() {
                if line.starts_with("(exit code: ") && line.ends_with(")") {
                    // Extract exit code from the last line
                    if let Some(code_str) = line.strip_prefix("(exit code: ").and_then(|s| s.strip_suffix(")")) {
                        exit_code = code_str.parse::<i32>().ok();
                    }
                } else {
                    output_lines.push(line.to_string());
                }
            }

            Ok(HistoryEntry {
                cmd: command,
                exit_code,
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

fn format_output(output_lines: &[String], exit_code: i32) -> String {
    let mut buf = output_lines.join("\n");
    if !buf.is_empty() {
        buf.push('\n');
    }
    buf.push_str(&format!("(exit code: {exit_code})"));
    buf
}

fn truncate_output(output: &str) -> String {
    if output.len() <= MAX_OUTPUT_LEN {
        return output.to_string();
    }

    let limit = MAX_OUTPUT_LEN.saturating_sub(TRUNC_SUFFIX.len());
    if limit == 0 {
        return TRUNC_SUFFIX.to_string();
    }

    let mut end = limit;
    while end > 0 && !output.is_char_boundary(end) {
        end -= 1;
    }

    if end == 0 {
        return TRUNC_SUFFIX.to_string();
    }

    let mut truncated = output[..end].to_string();
    truncated.push_str(TRUNC_SUFFIX);
    truncated
}

#[cfg(test)]
mod tests {
    use super::*;
    use libsql::Value;

    fn setup_history_table(pool: &SqlitePool) {
        pool.execute(
            "CREATE TABLE history (
                id INTEGER PRIMARY KEY,
                command TEXT NOT NULL,
                hash TEXT NOT NULL UNIQUE,
                count INTEGER NOT NULL DEFAULT 1,
                source TEXT,
                session_id TEXT,
                output TEXT
            );",
            Vec::<Value>::new(),
        )
        .unwrap();
    }

    #[test]
    fn persist_history_inserts_and_updates() {
        let pool = SqlitePool::open_memory().unwrap();
        setup_history_table(&pool);

        let lines = vec!["hello".to_string()];
        persist_history_entry(&pool, "echo hello", &lines, 0, "session-1").unwrap();

        let rows = pool
            .query_collect(
                "SELECT command, count, output FROM history",
                Vec::<Value>::new(),
                |row| {
                    let command: String = row.get(0)?;
                    let count: i64 = row.get(1)?;
                    let output: String = row.get(2)?;
                    Ok((command, count, output))
                },
            )
            .unwrap();
        assert_eq!(rows.len(), 1);
        let (command, count, output) = &rows[0];
        assert_eq!(command, "echo hello");
        assert_eq!(*count, 1);
        assert!(output.contains("hello"));
        assert!(output.contains("(exit code: 0)"));

        let lines2 = vec!["bye".to_string()];
        persist_history_entry(&pool, "echo hello", &lines2, 1, "session-1").unwrap();

        let rows = pool
            .query_collect(
                "SELECT count, output FROM history",
                Vec::<Value>::new(),
                |row| {
                    let count: i64 = row.get(0)?;
                    let output: String = row.get(1)?;
                    Ok((count, output))
                },
            )
            .unwrap();
        let (count, output) = &rows[0];
        assert_eq!(*count, 2);
        assert!(output.contains("bye"));
        assert!(output.contains("(exit code: 1)"));
        assert!(!output.contains("hello"));
    }

    #[test]
    fn truncate_output_limits_size() {
        let pool = SqlitePool::open_memory().unwrap();
        setup_history_table(&pool);

        let long_line = "x".repeat(MAX_OUTPUT_LEN + 100);
        let lines = vec![long_line];
        persist_history_entry(&pool, "echo long", &lines, 0, "session-2").unwrap();

        let rows = pool
            .query_collect("SELECT output FROM history", Vec::<Value>::new(), |row| {
                let output: String = row.get(0)?;
                Ok(output)
            })
            .unwrap();
        let output = rows.first().unwrap();
        assert!(output.len() <= MAX_OUTPUT_LEN);
        assert!(output.contains("…[truncated]"));
    }
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
