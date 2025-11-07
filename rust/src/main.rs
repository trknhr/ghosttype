mod core;
mod model;
mod tui;
use anyhow::Result;
use clap::{Parser, Subcommand};
use env_logger::Env;
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
    let _ = env_logger::Builder::from_env(Env::default().default_filter_or("info")).try_init();
    let cli = Cli::parse();
    match cli.cmd {
        Some(Cmd::Tui { files, top, unique }) => tui::run_tui_loop(files, top, unique),
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
