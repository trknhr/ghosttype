use ahash::AHashSet;
use anyhow::{bail, Context, Result};
use directories::UserDirs;
use fuzzy_matcher::skim::SkimMatcherV2;
use fuzzy_matcher::FuzzyMatcher;
use once_cell::sync::Lazy;
use ratatui::layout::Rect;
use std::fs::File;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::mpsc::Sender;
use std::thread;

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

    // Perform fuzzy matching
    let matcher = SkimMatcherV2::default();
    let mut scored = Vec::with_capacity(lines.len());
    for line in lines {
        if let Some(score) = matcher.fuzzy_match(&line, query) {
            scored.push((score, line));
        }
    }

    // Sort and display results
    scored.sort_by(|a, b| b.0.cmp(&a.0));
    for (_, line) in scored.into_iter().take(top) {
        println!("{line}");
    }
    Ok(())
}

// ---------------------
// TUI model
// ---------------------
#[derive(Clone)]
pub struct HistoryEntry {
    pub cmd: String,
    pub exit_code: Option<i32>,
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
    pub pinned_output: bool, // middle-left shows output instead of suggestions
    pub recent_runs_area: Option<Rect>, // clickable area cache

    // corpus
    pub corpus: Vec<String>,
}

impl App {
    pub fn new(corpus: Vec<String>, top: usize) -> Self {
        Self {
            input: String::new(),
            cursor: 0,
            suggestions: Vec::new(),
            selected: 0,
            max_suggestions: top,
            history: Vec::new(),
            output_lines: Vec::new(),
            is_running: false,
            last_run_cmd: None,
            pinned_output: false,
            recent_runs_area: None,
            corpus,
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
