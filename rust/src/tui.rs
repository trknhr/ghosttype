use crate::core;
use crate::model::SqlitePool;
use anyhow::Result;
use crossterm::event::{
    self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode, KeyEventKind, KeyModifiers,
    MouseEvent, MouseEventKind,
};
use crossterm::terminal::{
    disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen,
};
use crossterm::ExecutableCommand;
use log::warn;
use ratatui::backend::CrosstermBackend;
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, List, ListItem, Paragraph};
use ratatui::{Frame, Terminal};
use std::io::Stdout;
use std::path::PathBuf;
use std::sync::mpsc::{self, Receiver, Sender};
use std::time::{Duration, Instant};

pub fn run_tui(files: Vec<PathBuf>, top: usize, unique: bool) -> Result<()> {
    let corpus = core::load_history_lines(files, unique)?;
    let pool = match SqlitePool::open_default() {
        Ok(p) => Some(p),
        Err(err) => {
            warn!("failed to open sqlite history store: {err:?}");
            None
        }
    };
    let mut app = core::App::new(corpus, top, pool);

    enable_raw_mode()?;
    let mut stdout = std::io::stdout();
    stdout.execute(EnterAlternateScreen)?;
    stdout.execute(EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let (tx, rx): (Sender<core::ExecMsg>, Receiver<core::ExecMsg>) = mpsc::channel();

    let tick_rate = Duration::from_millis(33);
    let mut last_tick = Instant::now();

    loop {
        terminal.draw(|f| ui(f, &mut app)).ok();

        let timeout = tick_rate.saturating_sub(last_tick.elapsed());
        let mut should_quit = false;

        if crossterm::event::poll(timeout)? {
            match event::read()? {
                Event::Key(key) if key.kind == KeyEventKind::Press => {
                    should_quit = handle_key(key.code, key.modifiers, &mut app, &tx)?;
                }
                Event::Mouse(mev) => {
                    handle_mouse(mev, &mut app);
                }
                Event::Resize(_, _) => {}
                _ => {}
            }
        }

        while let Ok(msg) = rx.try_recv() {
            match msg {
                core::ExecMsg::Line(l) => app.output_lines.push(l),
                core::ExecMsg::Done(code) => {
                    app.is_running = false;
                    app.persist_last_run(code);
                    app.history.push(core::HistoryEntry {
                        cmd: app.last_run_cmd.clone().unwrap_or_default(),
                        exit_code: Some(code),
                    });
                }
            }
        }

        if should_quit {
            break;
        }
        if last_tick.elapsed() >= tick_rate {
            last_tick = Instant::now();
        }
    }

    let mut stdout: Stdout = std::io::stdout();
    disable_raw_mode()?;
    stdout.execute(LeaveAlternateScreen)?;
    stdout.execute(DisableMouseCapture)?;
    Ok(())
}

pub fn handle_key(
    code: KeyCode,
    mods: KeyModifiers,
    app: &mut core::App,
    tx: &Sender<core::ExecMsg>,
) -> Result<bool> {
    match (code, mods) {
        (KeyCode::Char('c'), KeyModifiers::CONTROL) => return Ok(true),
        (KeyCode::Esc, _) => return Ok(true),

        (KeyCode::Up, _) => {
            app.selected = app.selected.saturating_sub(1);
        }
        (KeyCode::Down, _) => {
            if app.selected + 1 < app.suggestions.len() {
                app.selected += 1;
            }
        }
        (KeyCode::PageUp, _) => {
            app.selected = app.selected.saturating_sub(5);
        }
        (KeyCode::PageDown, _) => {
            app.selected = (app.selected + 5).min(app.suggestions.len().saturating_sub(1));
        }

        (KeyCode::Left, _) => {
            app.cursor = app.cursor.saturating_sub(1);
        }
        (KeyCode::Right, _) => {
            app.cursor = (app.cursor + 1).min(app.input.len());
        }
        (KeyCode::Backspace, _) => {
            if app.cursor > 0 {
                app.input.remove(app.cursor - 1);
                app.cursor -= 1;
                app.refresh_suggestions();
            }
        }
        (KeyCode::Delete, _) => {
            if app.cursor < app.input.len() {
                app.input.remove(app.cursor);
                app.refresh_suggestions();
            }
        }
        (KeyCode::Home, _) => {
            app.cursor = 0;
        }
        (KeyCode::End, _) => {
            app.cursor = app.input.len();
        }
        (KeyCode::Char(c), KeyModifiers::NONE | KeyModifiers::SHIFT) => {
            app.input.insert(app.cursor, c);
            app.cursor += 1;
            app.refresh_suggestions();
        }
        (KeyCode::Tab, _) => {
            if let Some(sel) = app.suggestions.get(app.selected).cloned() {
                app.input = sel;
                app.cursor = app.input.len();
                app.refresh_suggestions();
            }
        }

        (KeyCode::Enter, _) => {
            if app.is_running {
                return Ok(false);
            }
            let to_run = if let Some(sel) = app.suggestions.get(app.selected) {
                sel.clone()
            } else {
                app.input.clone()
            };
            if to_run.trim().is_empty() {
                return Ok(false);
            }
            app.output_lines.clear();
            app.is_running = true;
            app.last_run_cmd = Some(to_run.clone());
            core::spawn_command(to_run, tx.clone());
        }

        _ => {}
    }
    Ok(false)
}

fn handle_mouse(mev: MouseEvent, app: &mut core::App) {
    if let MouseEventKind::Down(_) = mev.kind {
        if let Some(area) = app.recent_runs_area {
            if point_in_rect(mev.column, mev.row, area) {
                app.pinned_output = !app.pinned_output;
            }
        }
    }
}

fn point_in_rect(x: u16, y: u16, r: Rect) -> bool {
    x >= r.x && x < r.x + r.width && y >= r.y && y < r.y + r.height
}

// ---------------------
// Rendering
// ---------------------
fn ui(f: &mut Frame, app: &mut core::App) {
    let size = f.size();

    // Show bottom output only when not pinned AND there is something to show (running or has lines)
    let show_bottom_output = !app.pinned_output && (app.is_running || !app.output_lines.is_empty());

    let vchunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3), // input
            Constraint::Min(10),   // middle row (suggestions or pinned output)
            if show_bottom_output {
                Constraint::Min(5)
            } else {
                Constraint::Length(0)
            }, // bottom row (conditionally hidden)
        ])
        .split(size);

    draw_input(f, vchunks[0], app);

    let mid_chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage(60), // left: suggestions or output
            Constraint::Percentage(40), // right: recent runs
        ])
        .split(vchunks[1]);

    let mid_left = mid_chunks[0];
    let runs_area = mid_chunks[1];
    app.recent_runs_area = Some(runs_area);

    draw_mid_left(f, mid_left, app);
    draw_recent_runs(f, runs_area, app);
    if show_bottom_output {
        draw_output(f, vchunks[2], app);
    }
}

fn draw_input(f: &mut Frame, area: Rect, app: &core::App) {
    let block = Block::default()
        .title("ghosttype ▸ input  (Enter: run  Tab: accept  o: pin output  Ctrl-C/ESC: quit)")
        .borders(Borders::ALL);
    let text = vec![Line::from(app.input.as_str())];
    let p = Paragraph::new(text).block(block);
    f.render_widget(p, area);
}

fn draw_mid_left(f: &mut Frame, area: Rect, app: &core::App) {
    if app.pinned_output {
        draw_output(f, area, app);
    } else {
        draw_suggestions(f, area, app);
    }
}

fn draw_suggestions(f: &mut Frame, area: Rect, app: &core::App) {
    let items: Vec<ListItem> = app
        .suggestions
        .iter()
        .enumerate()
        .map(|(i, s)| {
            let style = if i == app.selected {
                Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
            } else {
                Style::default()
            };
            ListItem::new(Line::from(Span::styled(s.clone(), style)))
        })
        .collect();

    let list = List::new(items).block(Block::default().title("suggestions").borders(Borders::ALL));
    f.render_widget(list, area);
}

fn draw_recent_runs(f: &mut Frame, area: Rect, app: &core::App) {
    let title = if app.pinned_output {
        "recent runs — click to show suggestions"
    } else {
        "recent runs — click to pin output here"
    };

    let items: Vec<ListItem> = app
        .history
        .iter()
        .rev()
        .map(|h| {
            let label = if let Some(code) = h.exit_code {
                format!("[{}] {}", code, h.cmd)
            } else {
                format!("[?] {}", h.cmd)
            };
            ListItem::new(Line::from(label))
        })
        .collect();
    let list = List::new(items).block(Block::default().title(title).borders(Borders::ALL));
    f.render_widget(list, area);
}

fn draw_output(f: &mut Frame, area: Rect, app: &core::App) {
    let title = if let Some(cmd) = &app.last_run_cmd {
        format!(
            "output — {}{}",
            cmd,
            if app.is_running { " (running)" } else { "" }
        )
    } else {
        "output".to_string()
    };
    let text: Vec<Line> = if app.output_lines.is_empty() {
        vec![Line::from("(no output yet)")]
    } else {
        app.output_lines
            .iter()
            .map(|l| Line::from(l.as_str()))
            .collect()
    };
    let p = Paragraph::new(text).block(Block::default().title(title).borders(Borders::ALL));
    f.render_widget(p, area);
}
