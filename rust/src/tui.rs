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
                        output_lines: app.output_lines.clone(),
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

        // Tab switching: Ctrl+Tab to toggle between tabs
        (KeyCode::Tab, KeyModifiers::CONTROL) => {
            app.current_tab = match app.current_tab {
                core::Tab::Main => core::Tab::History,
                core::Tab::History => core::Tab::Main,
            };
        }

        (KeyCode::Up, _) => {
            match app.current_tab {
                core::Tab::Main => {
                    app.selected = app.selected.saturating_sub(1);
                }
                core::Tab::History => {
                    app.selected_history_index = app.selected_history_index.saturating_sub(1);
                    app.history_scroll = 0; // Reset scroll when changing selection
                }
            }
        }
        (KeyCode::Down, _) => {
            match app.current_tab {
                core::Tab::Main => {
                    if app.selected + 1 < app.suggestions.len() {
                        app.selected += 1;
                    }
                }
                core::Tab::History => {
                    if app.selected_history_index + 1 < app.history.len() {
                        app.selected_history_index += 1;
                    }
                    app.history_scroll = 0; // Reset scroll when changing selection
                }
            }
        }
        (KeyCode::PageUp, _) => {
            match app.current_tab {
                core::Tab::Main => {
                    // If showing output, scroll it; otherwise navigate suggestions
                    if app.is_running || !app.output_lines.is_empty() {
                        app.output_scroll = app.output_scroll.saturating_sub(10);
                    } else {
                        app.selected = app.selected.saturating_sub(5);
                    }
                }
                core::Tab::History => {
                    app.history_scroll = app.history_scroll.saturating_sub(10);
                }
            }
        }
        (KeyCode::PageDown, _) => {
            match app.current_tab {
                core::Tab::Main => {
                    // If showing output, scroll it; otherwise navigate suggestions
                    if app.is_running || !app.output_lines.is_empty() {
                        app.output_scroll = app.output_scroll.saturating_add(10);
                    } else {
                        app.selected = (app.selected + 5).min(app.suggestions.len().saturating_sub(1));
                    }
                }
                core::Tab::History => {
                    app.history_scroll = app.history_scroll.saturating_add(10);
                }
            }
        }

        (KeyCode::Left, _) if app.current_tab == core::Tab::Main => {
            app.cursor = app.cursor.saturating_sub(1);
        }
        (KeyCode::Right, _) if app.current_tab == core::Tab::Main => {
            app.cursor = (app.cursor + 1).min(app.input.len());
        }
        (KeyCode::Backspace, _) if app.current_tab == core::Tab::Main => {
            if app.cursor > 0 {
                app.input.remove(app.cursor - 1);
                app.cursor -= 1;
                app.output_lines.clear(); // Clear output when typing
                app.output_scroll = 0; // Reset scroll
                app.refresh_suggestions();
            }
        }
        (KeyCode::Delete, _) if app.current_tab == core::Tab::Main => {
            if app.cursor < app.input.len() {
                app.input.remove(app.cursor);
                app.output_lines.clear(); // Clear output when typing
                app.output_scroll = 0; // Reset scroll
                app.refresh_suggestions();
            }
        }
        (KeyCode::Home, _) if app.current_tab == core::Tab::Main => {
            app.cursor = 0;
        }
        (KeyCode::End, _) if app.current_tab == core::Tab::Main => {
            app.cursor = app.input.len();
        }
        (KeyCode::Char(c), KeyModifiers::NONE | KeyModifiers::SHIFT) if app.current_tab == core::Tab::Main => {
            app.input.insert(app.cursor, c);
            app.cursor += 1;
            app.output_lines.clear(); // Clear output when typing
            app.output_scroll = 0; // Reset scroll
            app.refresh_suggestions();
        }
        (KeyCode::Tab, _) if app.current_tab == core::Tab::Main => {
            if let Some(sel) = app.suggestions.get(app.selected).cloned() {
                app.input = sel;
                app.cursor = app.input.len();
                app.refresh_suggestions();
            }
        }

        (KeyCode::Enter, _) if app.current_tab == core::Tab::Main => {
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
            app.output_scroll = 0; // Reset scroll for new command
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
        // Check if clicking on Main tab
        if let Some(area) = app.main_tab_area {
            if point_in_rect(mev.column, mev.row, area) {
                app.current_tab = core::Tab::Main;
                return;
            }
        }

        // Check if clicking on History tab
        if let Some(area) = app.history_tab_area {
            if point_in_rect(mev.column, mev.row, area) {
                app.current_tab = core::Tab::History;
                return;
            }
        }

        // Check if clicking on recent runs area (legacy functionality)
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
    match app.current_tab {
        core::Tab::Main => ui_main_tab(f, app),
        core::Tab::History => ui_history_tab(f, app),
    }
}

fn ui_main_tab(f: &mut Frame, app: &mut core::App) {
    let size = f.size();

    let vchunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1), // tab bar
            Constraint::Length(3), // input
            Constraint::Min(10),   // content area (suggestions or output)
        ])
        .split(size);

    draw_tabs(f, vchunks[0], app);
    draw_input(f, vchunks[1], app);

    // Show output if running or has output lines, otherwise show suggestions
    if app.is_running || !app.output_lines.is_empty() {
        draw_output(f, vchunks[2], app);
    } else {
        draw_suggestions(f, vchunks[2], app);
    }
}

fn ui_history_tab(f: &mut Frame, app: &mut core::App) {
    let size = f.size();

    let vchunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1), // tab bar
            Constraint::Min(10),   // history browser
        ])
        .split(size);

    draw_tabs(f, vchunks[0], app);

    let h_chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage(40), // left: command list
            Constraint::Percentage(60), // right: output
        ])
        .split(vchunks[1]);

    draw_history_list(f, h_chunks[0], app);
    draw_history_output(f, h_chunks[1], app);
}

fn draw_tabs(f: &mut Frame, area: Rect, app: &mut core::App) {
    // Calculate tab widths
    let tab_width = 12u16;
    let spacing = 1u16;

    // Main tab area
    let main_tab_rect = Rect {
        x: area.x,
        y: area.y,
        width: tab_width,
        height: area.height,
    };

    // History tab area
    let history_tab_rect = Rect {
        x: area.x + tab_width + spacing,
        y: area.y,
        width: tab_width,
        height: area.height,
    };

    // Store tab areas for click detection
    app.main_tab_area = Some(main_tab_rect);
    app.history_tab_area = Some(history_tab_rect);

    // Draw Main tab
    let main_style = if app.current_tab == core::Tab::Main {
        Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
    } else {
        Style::default()
    };
    let main_tab = Paragraph::new(" Main ")
        .style(main_style);
    f.render_widget(main_tab, main_tab_rect);

    // Draw History tab
    let history_style = if app.current_tab == core::Tab::History {
        Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
    } else {
        Style::default()
    };
    let history_tab = Paragraph::new(" History ")
        .style(history_style);
    f.render_widget(history_tab, history_tab_rect);

    // Draw remaining space
    let remaining_rect = Rect {
        x: area.x + (tab_width + spacing) * 2,
        y: area.y,
        width: area.width.saturating_sub((tab_width + spacing) * 2),
        height: area.height,
    };
    let remaining = Paragraph::new("");
    f.render_widget(remaining, remaining_rect);
}

fn draw_input(f: &mut Frame, area: Rect, app: &core::App) {
    let title = "ghosttype ▸ input  (Enter: run  Tab: accept  Ctrl+Tab: switch  PgUp/PgDn: scroll  Ctrl-C/ESC: quit)";
    let block = Block::default()
        .title(title)
        .borders(Borders::ALL);
    let text = vec![Line::from(app.input.as_str())];
    let p = Paragraph::new(text).block(block);
    f.render_widget(p, area);
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

fn draw_history_list(f: &mut Frame, area: Rect, app: &core::App) {
    let items: Vec<ListItem> = app
        .history
        .iter()
        .rev()
        .enumerate()
        .map(|(display_idx, h)| {
            let actual_idx = app.history.len().saturating_sub(1).saturating_sub(display_idx);
            let label = if let Some(code) = h.exit_code {
                format!("[{}] {}", code, h.cmd)
            } else {
                format!("[?] {}", h.cmd)
            };
            let style = if actual_idx == app.selected_history_index {
                Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
            } else {
                Style::default()
            };
            ListItem::new(Line::from(Span::styled(label, style)))
        })
        .collect();
    let list = List::new(items).block(Block::default().title("Recent Commands").borders(Borders::ALL));
    f.render_widget(list, area);
}

fn draw_history_output(f: &mut Frame, area: Rect, app: &core::App) {
    let (title, text) = if app.history.is_empty() {
        ("Output".to_string(), vec![Line::from("(no history yet)")])
    } else if let Some(entry) = app.history.get(app.selected_history_index) {
        let title = format!(
            "Output — {} [exit code: {}]",
            entry.cmd,
            entry.exit_code.map(|c| c.to_string()).unwrap_or_else(|| "?".to_string())
        );
        let text = if entry.output_lines.is_empty() {
            vec![Line::from("(no output)")]
        } else {
            entry.output_lines.iter().map(|l| Line::from(l.as_str())).collect()
        };
        (title, text)
    } else {
        ("Output".to_string(), vec![Line::from("(select a command)")])
    };

    let p = Paragraph::new(text)
        .block(Block::default().title(title).borders(Borders::ALL))
        .scroll((app.history_scroll, 0));
    f.render_widget(p, area);
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
    let p = Paragraph::new(text)
        .block(Block::default().title(title).borders(Borders::ALL))
        .scroll((app.output_scroll, 0));
    f.render_widget(p, area);
}
