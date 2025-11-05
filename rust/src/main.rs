mod core;
mod tui;
use anyhow::Result;
use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Parser, Debug)]
#[command(
    name = "ghosttype",
    version,
    about = "History + AI + Embedding suggestions (TUI prototype)"
)]
struct Cli {
    #[command(subcommand)]
    cmd: Option<Cmd>,
}

#[derive(Subcommand, Debug)]
enum Cmd {
    /// Launch interactive TUI
    Tui {
        /// History files to load (semicolon separated)
        #[arg(short = 'f', long = "file", num_args = 0.., value_delimiter = ';')]
        files: Vec<PathBuf>,

        /// Max suggestions to show
        #[arg(short = 'n', long = "top", default_value_t = 20)]
        top: usize,

        /// Remove duplicate lines
        #[arg(long, default_value_t = true)]
        unique: bool,
    },

    /// Non-TUI fuzzy search (existing behavior)
    Search {
        #[arg(short = 'f', long = "file", num_args = 1.., value_delimiter = ';')]
        files: Vec<PathBuf>,
        #[arg(short, long)]
        query: String,
        #[arg(short = 'n', long = "top", default_value_t = 20)]
        top: usize,
        #[arg(long, default_value_t = true)]
        unique: bool,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();
    match cli.cmd {
        Some(Cmd::Tui { files, top, unique }) => tui::run_tui(files, top, unique),
        Some(Cmd::Search {
            files,
            query,
            top,
            unique,
        }) => core::run_search(files, &query, top, unique),
        None => {
            eprintln!(
                "Try: ghosttype tui
  or: ghosttype search --file ~/.zsh_history --query \"git st\"
"
            );
            Ok(())
        }
    }
}

// ---------------------
// Non-TUI search
// fn run_search(files: Vec<PathBuf>, query: &str, top: usize, unique: bool) -> Result<()> {
//     if files.is_empty() {
//         bail!("Please specify at least one --file");
//     }

//     // Read all lines from the provided files
//     let mut lines = Vec::new();
//     for path in files {
//         lines.extend(core::read_history_file(&path).with_context(|| format!("reading {:?}", path))?);
//     }

//     // Optionally remove duplicates
//     if unique {
//         let mut seen = AHashSet::with_capacity(lines.len());
//         lines.retain(|s| seen.insert(s.to_string()));
//     }

//     // Perform fuzzy matching
//     let matcher = SkimMatcherV2::default();
//     let mut scored = Vec::with_capacity(lines.len());
//     for line in lines {
//         if let Some(score) = matcher.fuzzy_match(&line, query) {
//             scored.push((score, line));
//         }
//     }

//     // Sort and display results
//     scored.sort_by(|a, b| b.0.cmp(&a.0));
//     for (_, line) in scored.into_iter().take(top) {
//         println!("{line}");
//     }
//     Ok(())
// }

// // ---------------------
// // TUI model
// // ---------------------
// #[derive(Clone)]
// struct HistoryEntry {
//     cmd: String,
//     exit_code: Option<i32>,
//     at: SystemTime,
// }

// struct App {
//     // input
//     input: String,
//     cursor: usize,

//     // suggestions
//     suggestions: Vec<String>,
//     selected: usize,
//     max_suggestions: usize,

//     // history
//     history: Vec<HistoryEntry>,

//     // output streaming
//     output_lines: Vec<String>,
//     is_running: bool,
//     last_run_cmd: Option<String>,

//     // view state
//     pinned_output: bool,            // middle-left shows output instead of suggestions
//     recent_runs_area: Option<Rect>, // clickable area cache

//     // corpus
//     corpus: Vec<String>,
// }

// impl App {
//     fn new(corpus: Vec<String>, top: usize) -> Self {
//         Self {
//             input: String::new(),
//             cursor: 0,
//             suggestions: Vec::new(),
//             selected: 0,
//             max_suggestions: top,
//             history: Vec::new(),
//             output_lines: Vec::new(),
//             is_running: false,
//             last_run_cmd: None,
//             pinned_output: false,
//             recent_runs_area: None,
//             corpus,
//         }
//     }

//     fn refresh_suggestions(&mut self) {
//         if self.input.trim().is_empty() {
//             self.suggestions.clear();
//             self.selected = 0;
//             return;
//         }
//         let query = self.input.as_str();
//         let mut scored: Vec<(i64, String)> = Vec::new();
//         for line in self.corpus.iter() {
//             if let Some(score) = MATCHER.fuzzy_match(line, query) {
//                 scored.push((score, line.clone()));
//             }
//         }
//         scored.sort_by(|a, b| b.0.cmp(&a.0));
//         self.suggestions = scored.into_iter().take(self.max_suggestions).map(|(_, s)| s).collect();
//         self.selected = self.selected.min(self.suggestions.len().saturating_sub(1));
//     }
// }

// ---------------------
// Exec thread
// ---------------------
// fn stream_reader<R: Read + Send + 'static>(mut reader: Option<R>, tx: Sender<core::ExecMsg>) {
//     use std::io::{BufRead, BufReader};
//     if let Some(r) = reader.take() {
//         let br = BufReader::new(r);
//         for line in br.lines() {
//             match line {
//                 Ok(l) => { let _ = tx.send(core::ExecMsg::Line(l)); }
//                 Err(e) => { let _ = tx.send(core::ExecMsg::Line(format!("read error: {e}"))); break; }
//             }
//         }
//     }
// }

// ---------------------
// TUI loop
// ---------------------
// fn run_tui(files: Vec<PathBuf>, top: usize, unique: bool) -> Result<()> {
//     let corpus = core::load_history_lines(files, unique)?;
//     let mut app = App::new(corpus, top);

//     enable_raw_mode()?;
//     let mut stdout = std::io::stdout();
//     stdout.execute(EnterAlternateScreen)?;
//     stdout.execute(EnableMouseCapture)?;
//     let backend = CrosstermBackend::new(stdout);
//     let mut terminal = Terminal::new(backend)?;

//     let (tx, rx): (Sender<core::ExecMsg>, Receiver<core::ExecMsg>) = mpsc::channel();

//     let tick_rate = Duration::from_millis(33);
//     let mut last_tick = Instant::now();

//     loop {
//         terminal.draw(|f| ui(f, &mut app)).ok();

//         let timeout = tick_rate.saturating_sub(last_tick.elapsed());
//         let mut should_quit = false;

//         if crossterm::event::poll(timeout)? {
//             match event::read()? {
//                 Event::Key(key) if key.kind == KeyEventKind::Press => {
//                     should_quit = handle_key(key.code, key.modifiers, &mut app, &tx)?;
//                 }
//                 Event::Mouse(mev) => {
//                     handle_mouse(mev, &mut app);
//                 }
//                 Event::Resize(_, _) => {}
//                 _ => {}
//             }
//         }

//         while let Ok(msg) = rx.try_recv() {
//             match msg {
//                 core::ExecMsg::Line(l) => app.output_lines.push(l),
//                 core::ExecMsg::Done(code) => {
//                     app.is_running = false;
//                     app.history.push(HistoryEntry { cmd: app.last_run_cmd.clone().unwrap_or_default(), exit_code: Some(code), at: SystemTime::now() });
//                 }
//             }
//         }

//         if should_quit { break; }
//         if last_tick.elapsed() >= tick_rate { last_tick = Instant::now(); }
//     }

//     let mut stdout: Stdout = std::io::stdout();
//     disable_raw_mode()?;
//     stdout.execute(LeaveAlternateScreen)?;
//     stdout.execute(DisableMouseCapture)?;
//     Ok(())
// }

// fn handle_key(code: KeyCode, mods: KeyModifiers, app: &mut App, tx: &Sender<core::ExecMsg>) -> Result<bool> {
//     match (code, mods) {
//         (KeyCode::Char('c'), KeyModifiers::CONTROL) => return Ok(true),
//         (KeyCode::Esc, _) => return Ok(true),

//         (KeyCode::Char('o'), _) => { app.pinned_output = !app.pinned_output; },

//         (KeyCode::Up, _) => { app.selected = app.selected.saturating_sub(1); },
//         (KeyCode::Down, _) => { if app.selected + 1 < app.suggestions.len() { app.selected += 1; } },
//         (KeyCode::PageUp, _) => { app.selected = app.selected.saturating_sub(5); },
//         (KeyCode::PageDown, _) => { app.selected = (app.selected + 5).min(app.suggestions.len().saturating_sub(1)); },

//         (KeyCode::Left, _) => { app.cursor = app.cursor.saturating_sub(1); },
//         (KeyCode::Right, _) => { app.cursor = (app.cursor + 1).min(app.input.len()); },
//         (KeyCode::Backspace, _) => {
//             if app.cursor > 0 { app.input.remove(app.cursor - 1); app.cursor -= 1; app.refresh_suggestions(); }
//         },
//         (KeyCode::Delete, _) => {
//             if app.cursor < app.input.len() { app.input.remove(app.cursor); app.refresh_suggestions(); }
//         },
//         (KeyCode::Home, _) => { app.cursor = 0; },
//         (KeyCode::End, _) => { app.cursor = app.input.len(); },
//         (KeyCode::Char(c), KeyModifiers::NONE | KeyModifiers::SHIFT) => {
//             app.input.insert(app.cursor, c);
//             app.cursor += 1;
//             app.refresh_suggestions();
//         },
//         (KeyCode::Tab, _) => {
//             if let Some(sel) = app.suggestions.get(app.selected).cloned() { app.input = sel; app.cursor = app.input.len(); app.refresh_suggestions(); }
//         },

//         (KeyCode::Enter, _) => {
//             if app.is_running { return Ok(false); }
//             let to_run = if let Some(sel) = app.suggestions.get(app.selected) { sel.clone() } else { app.input.clone() };
//             if to_run.trim().is_empty() { return Ok(false); }
//             app.output_lines.clear();
//             app.is_running = true;
//             app.last_run_cmd = Some(to_run.clone());
//             core::spawn_command(to_run, tx.clone());
//         },

//         _ => {}
//     }
//     Ok(false)
// }

// fn handle_mouse(mev: MouseEvent, app: &mut App) {
//     if let MouseEventKind::Down(_) = mev.kind {
//         if let Some(area) = app.recent_runs_area {
//             if point_in_rect(mev.column, mev.row, area) {
//                 app.pinned_output = !app.pinned_output;
//             }
//         }
//     }
// }

// fn point_in_rect(x: u16, y: u16, r: Rect) -> bool {
//     x >= r.x && x < r.x + r.width && y >= r.y && y < r.y + r.height
// }

// // ---------------------
// // Rendering
// // ---------------------
// fn ui(f: &mut Frame, app: &mut App) {
//     let size = f.size();

//     // Show bottom output only when not pinned AND there is something to show (running or has lines)
//     let show_bottom_output = !app.pinned_output && (app.is_running || !app.output_lines.is_empty());

//     let vchunks = Layout::default()
//         .direction(Direction::Vertical)
//         .constraints([
//             Constraint::Length(3),                 // input
//             Constraint::Min(10),                   // middle row (suggestions or pinned output)
//             if show_bottom_output { Constraint::Min(5) } else { Constraint::Length(0) }, // bottom row (conditionally hidden)
//         ])
//         .split(size);

//     draw_input(f, vchunks[0], app);

//     let mid_chunks = Layout::default()
//         .direction(Direction::Horizontal)
//         .constraints([
//             Constraint::Percentage(60), // left: suggestions or output
//             Constraint::Percentage(40), // right: recent runs
//         ])
//         .split(vchunks[1]);

//     let mid_left = mid_chunks[0];
//     let runs_area = mid_chunks[1];
//     app.recent_runs_area = Some(runs_area);

//     draw_mid_left(f, mid_left, app);
//     draw_recent_runs(f, runs_area, app);
//     if show_bottom_output {
//         draw_output(f, vchunks[2], app);
//     }
// }

// fn draw_input(f: &mut Frame, area: Rect, app: &App) {
//     let block = Block::default().title("ghosttype ▸ input  (Enter: run  Tab: accept  o: pin output  Ctrl-C/ESC: quit)").borders(Borders::ALL);
//     let text = vec![Line::from(app.input.as_str())];
//     let p = Paragraph::new(text).block(block);
//     f.render_widget(p, area);
// }

// fn draw_mid_left(f: &mut Frame, area: Rect, app: &App) {
//     if app.pinned_output {
//         draw_output(f, area, app);
//     } else {
//         draw_suggestions(f, area, app);
//     }
// }

// fn draw_suggestions(f: &mut Frame, area: Rect, app: &App) {
//     let items: Vec<ListItem> = app
//         .suggestions
//         .iter()
//         .enumerate()
//         .map(|(i, s)| {
//             let style = if i == app.selected {
//                 Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
//             } else { Style::default() };
//             ListItem::new(Line::from(Span::styled(s.clone(), style)))
//         })
//         .collect();

//     let list = List::new(items)
//         .block(Block::default().title("suggestions").borders(Borders::ALL));
//     f.render_widget(list, area);
// }

// fn draw_recent_runs(f: &mut Frame, area: Rect, app: &App) {
//     let title = if app.pinned_output { "recent runs — click to show suggestions" } else { "recent runs — click to pin output here" };

//     let items: Vec<ListItem> = app.history.iter().rev().map(|h| {
//         let label = if let Some(code) = h.exit_code { format!("[{}] {}", code, h.cmd) } else { format!("[?] {}", h.cmd) };
//         ListItem::new(Line::from(label))
//     }).collect();
//     let list = List::new(items).block(Block::default().title(title).borders(Borders::ALL));
//     f.render_widget(list, area);
// }

// fn draw_output(f: &mut Frame, area: Rect, app: &App) {
//     let title = if let Some(cmd) = &app.last_run_cmd { format!("output — {}{}", cmd, if app.is_running { " (running)" } else { "" }) } else { "output".to_string() };
//     let text: Vec<Line> = if app.output_lines.is_empty() {
//         vec![Line::from("(no output yet)")]
//     } else {
//         app.output_lines.iter().map(|l| Line::from(l.as_str())).collect()
//     };
//     let p = Paragraph::new(text).block(Block::default().title(title).borders(Borders::ALL));
//     f.render_widget(p, area);
// }
