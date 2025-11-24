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
use ratatui::widgets::{Block, Borders, Clear, List, ListItem, Paragraph};
use ratatui::{Frame, Terminal};
use std::borrow::Cow;
use std::io::Stdout;
use std::path::PathBuf;
use std::time::{Duration, Instant};

pub enum KeyResult {
    Continue,
    Quit,
    RunCommand(String),
}

pub fn run_tui(
    files: Vec<PathBuf>,
    top: usize,
    unique: bool,
    pool: Option<SqlitePool>,
    enable_embedding: bool,
    embedding_model: Option<PathBuf>,
    enable_llm: bool,
    llm_model: Option<PathBuf>,
    initial_input: Option<String>,
) -> Result<(Option<String>, String)> {
    let corpus = core::load_history_lines(files, unique)?;
    let mut app = core::App::new(
        corpus,
        top,
        pool,
        enable_embedding,
        embedding_model,
        enable_llm,
        llm_model,
    )?;

    // Restore any previously retained input
    if let Some(initial_input) = initial_input {
        app.input = initial_input;
        app.cursor = app.input.len();
        if !app.input.trim().is_empty() {
            app.refresh_suggestions();
        }
    }

    enable_raw_mode()?;
    let mut stdout = std::io::stdout();
    stdout.execute(EnterAlternateScreen)?;
    stdout.execute(EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let tick_rate = Duration::from_millis(33);
    let mut last_tick = Instant::now();
    let mut command_to_run: Option<String> = None;

    loop {
        terminal.draw(|f| ui(f, &mut app)).ok();

        let timeout = tick_rate.saturating_sub(last_tick.elapsed());
        let mut should_quit = false;

        if crossterm::event::poll(timeout)? {
            match event::read()? {
                Event::Key(key) if key.kind == KeyEventKind::Press => {
                    let result = handle_key(key.code, key.modifiers, &mut app)?;
                    match result {
                        KeyResult::Quit => should_quit = true,
                        KeyResult::RunCommand(cmd) => {
                            command_to_run = Some(cmd);
                            break;
                        }
                        KeyResult::Continue => {}
                    }
                }
                Event::Mouse(mev) => {
                    handle_mouse(mev, &mut app);
                }
                Event::Resize(_, _) => {}
                _ => {}
            }
        }

        // Check if we should refresh suggestions (debounce)
        if app.should_refresh_suggestions() {
            app.refresh_suggestions();
        }

        // Poll for heavy model results (non-blocking)
        app.poll_heavy_model_results();

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
    let final_input = app.input.clone();
    Ok((command_to_run, final_input))
}

pub fn handle_key(
    code: KeyCode,
    mods: KeyModifiers,
    app: &mut core::App,
) -> Result<KeyResult> {
    match (code, mods) {
        (KeyCode::Char('c'), KeyModifiers::CONTROL) => return Ok(KeyResult::Quit),
        (KeyCode::Esc, _) => return Ok(KeyResult::Quit),

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
                    if app.selected_history_index + 1 < app.history.len() {
                        app.selected_history_index += 1;
                    }
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
                    app.selected_history_index = app.selected_history_index.saturating_sub(1);
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
                app.mark_input_changed(); // Debounced refresh
            }
        }
        (KeyCode::Delete, _) if app.current_tab == core::Tab::Main => {
            if app.cursor < app.input.len() {
                app.input.remove(app.cursor);
                app.output_lines.clear(); // Clear output when typing
                app.output_scroll = 0; // Reset scroll
                app.mark_input_changed(); // Debounced refresh
            }
        }
        (KeyCode::Home, _) if app.current_tab == core::Tab::Main => {
            app.cursor = 0;
        }
        (KeyCode::End, _) if app.current_tab == core::Tab::Main => {
            app.cursor = app.input.len();
        }
        (KeyCode::Char('a'), KeyModifiers::CONTROL) if app.current_tab == core::Tab::Main => {
            app.cursor = app.input.len(); // Move cursor to end (simulates select all)
        }
        (KeyCode::Char('a'), KeyModifiers::SUPER) if app.current_tab == core::Tab::Main => {
            app.cursor = app.input.len(); // Move cursor to end (simulates select all) - Mac Cmd+A
        }
        (KeyCode::Char(c), KeyModifiers::NONE | KeyModifiers::SHIFT) if app.current_tab == core::Tab::Main => {
            app.input.insert(app.cursor, c);
            app.cursor += 1;
            app.output_lines.clear(); // Clear output when typing
            app.output_scroll = 0; // Reset scroll
            app.mark_input_changed(); // Debounced refresh
        }
        (KeyCode::Tab, _) if app.current_tab == core::Tab::Main => {
            if let Some(sel) = app.suggestions.get(app.selected).cloned() {
                app.input = sel;
                app.cursor = app.input.len();
                app.mark_input_changed(); // Debounced refresh
            }
        }

        (KeyCode::Enter, _) if app.current_tab == core::Tab::Main => {
            let to_run = if let Some(sel) = app.suggestions.get(app.selected) {
                sel.clone()
            } else {
                app.input.clone()
            };
            if to_run.trim().is_empty() {
                return Ok(KeyResult::Continue);
            }
            return Ok(KeyResult::RunCommand(to_run));
        }

        _ => {}
    }
    Ok(KeyResult::Continue)
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

fn normalized_command_for_display<'a>(text: &'a str) -> Cow<'a, str> {
    let trimmed_ws = text.trim_end_matches(|c| c == ' ' || c == '\t');
    if trimmed_ws.ends_with('\\') {
        Cow::Owned(trimmed_ws[..trimmed_ws.len().saturating_sub(1)].to_string())
    } else if trimmed_ws.len() != text.len() {
        Cow::Owned(trimmed_ws.to_string())
    } else {
        Cow::Borrowed(text)
    }
}

fn lines_from_display_text(target: &str) -> Vec<Line<'static>> {
    target
        .split('\n')
        .map(|segment| {
            if segment.trim().is_empty() {
                Line::from(" ")
            } else {
                Line::from(segment.to_string())
            }
        })
        .collect()
}

fn format_command_lines_for_display(text: &str) -> Vec<Line<'static>> {
    let normalized = normalized_command_for_display(text);
    lines_from_display_text(normalized.as_ref())
}

fn input_lines_with_cursor(text: &str, cursor: usize) -> Vec<Line<'static>> {
    let highlight = Style::default().add_modifier(Modifier::REVERSED);

    if text.is_empty() {
        return vec![Line::from(" ")];
    }

    let mut lines = Vec::new();
    let mut offset = 0usize;

    for raw_line in text.split('\n') {
        let line_len = raw_line.len();
        let cursor_in_line = cursor >= offset && cursor < offset + line_len;
        let mut spans: Vec<Span<'static>> = Vec::new();

        if cursor_in_line {
            let col = cursor - offset;
            let (left, rest) = raw_line.split_at(col);
            if !left.is_empty() {
                spans.push(Span::raw(left.to_string()));
            }

            let mut rest_chars = rest.chars();
            if let Some(cursor_char) = rest_chars.next() {
                let cursor_str = cursor_char.to_string();
                spans.push(Span::styled(cursor_str, highlight));
                let remaining = rest_chars.as_str();
                if !remaining.is_empty() {
                    spans.push(Span::raw(remaining.to_string()));
                }
            }
        } else if raw_line.is_empty() {
            spans.push(Span::raw(" ".to_string()));
        } else {
            spans.push(Span::raw(raw_line.to_string()));
        }

        lines.push(Line::from(spans));
        offset += line_len + 1; // include newline
    }

    lines
}

fn cursor_line_col(display_text: &str, cursor: usize) -> (u16, u16) {
    let mut line: u16 = 0;
    let mut col: u16 = 0;

    let target = cursor.min(display_text.len());
    for (idx, ch) in display_text.char_indices() {
        if idx >= target {
            break;
        }
        if ch == '\n' {
            line = line.saturating_add(1);
            col = 0;
        } else {
            col = col.saturating_add(1);
        }
    }

    (line, col)
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

    // Clear the screen when rendering this tab
    f.render_widget(Clear, size);

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

    // Clear the screen when rendering this tab
    f.render_widget(Clear, size);

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
    let text = input_lines_with_cursor(app.input.as_str(), app.cursor);
    let p = Paragraph::new(text).block(block);
    f.render_widget(p, area);

    if area.width > 2 && area.height > 2 {
        let (line, col) = cursor_line_col(app.input.as_str(), app.cursor);
        let inner_width = area.width - 2;
        let inner_height = area.height - 2;
        let clamped_col = col.min(inner_width.saturating_sub(1));
        let clamped_line = line.min(inner_height.saturating_sub(1));
        f.set_cursor(area.x + 1 + clamped_col, area.y + 1 + clamped_line);
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
            ListItem::new(format_command_lines_for_display(s)).style(style)
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
            let style = if actual_idx == app.selected_history_index {
                Style::default().add_modifier(Modifier::BOLD | Modifier::REVERSED)
            } else {
                Style::default()
            };
            ListItem::new(format_command_lines_for_display(&h.cmd)).style(style)
        })
        .collect();
    let list = List::new(items).block(Block::default().title("Recent Commands").borders(Borders::ALL));
    f.render_widget(list, area);
}

fn draw_history_output(f: &mut Frame, area: Rect, app: &core::App) {
    let (title, text) = if app.history.is_empty() {
        ("Output".to_string(), vec![Line::from("(no history yet)")])
    } else if let Some(entry) = app.history.get(app.selected_history_index) {
        let title = format!("Output — {}", entry.cmd);
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

fn execute_in_terminal(command: &str) -> Result<()> {
    use std::process::Command;

    println!("\n$ {}\n", command);

    // Use the user's shell from $SHELL, fallback to /bin/sh
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string());

    Command::new(shell)
        .arg("-lc")
        .arg(command)
        .status()?;

    Ok(())
}

fn wait_for_enter() -> Result<()> {
    use std::io::{self, Write};

    print!("\nPress Enter to return to ghosttype...");
    io::stdout().flush()?;

    let mut input = String::new();
    io::stdin().read_line(&mut input)?;

    Ok(())
}

pub fn run_tui_loop(
    files: Vec<PathBuf>,
    top: usize,
    unique: bool,
    enable_embedding: bool,
    embedding_model: Option<PathBuf>,
    enable_llm: bool,
    llm_model: Option<PathBuf>,
) -> Result<()> {
    // Create tokio runtime for async heavy model tasks
    let runtime = tokio::runtime::Runtime::new()?;
    let _enter = runtime.enter(); // Enter runtime context for entire session

    // Generate session ID for this TUI session
    let session_id = format!("tui-{}-{}", std::process::id(), std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_millis());

    // Open database pool once for the entire session
    let pool = match SqlitePool::open_default() {
        Ok(p) => Some(p),
        Err(err) => {
            warn!("failed to open sqlite history store: {err:?}");
            None
        }
    };

    // Import shell history files into database on startup
    if let Some(ref p) = pool {
        if let Err(e) = core::import_shell_history_to_db(p, &files) {
            warn!("failed to import shell history: {e:?}");
        }
    }

    let mut retained_input: Option<String> = None;

    loop {
        let (run_result, latest_input) = run_tui(
            files.clone(),
            top,
            unique,
            pool.clone(),
            enable_embedding,
            embedding_model.clone(),
            enable_llm,
            llm_model.clone(),
            retained_input.take(),
        )?;

        retained_input = Some(latest_input.clone());

        match run_result {
            Some(command) => {
                execute_in_terminal(&command)?;

                // Save command to database
                if let Some(ref p) = pool {
                    if let Err(e) = core::persist_command_to_history(p, &command, &session_id) {
                        warn!("failed to save command to history: {e:?}");
                    }
                }

                wait_for_enter()?;
                // Loop continues, TUI restarts
            }
            None => {
                // User quit with Ctrl-C or ESC
                break;
            }
        }
    }
    Ok(())
}
