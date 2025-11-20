use std::collections::HashSet;
use std::path::PathBuf;
use std::process::{Command, Stdio};

use anyhow::{bail, Result};
use log::{info, warn};

use super::{SuggestModel, Suggestion};

/// Configuration for LLM model using external llama-cli
#[derive(Debug, Clone)]
pub struct LlmConfig {
    pub model_path: PathBuf,
    pub temperature: f64,
    pub max_tokens: usize,
    pub seed: u64,
}

impl Default for LlmConfig {
    fn default() -> Self {
        Self {
            model_path: PathBuf::new(), // Will be set by CLI args
            temperature: 0.05,
            max_tokens: 3,
            seed: 299792458,
        }
    }
}

/// LLM-based suggestion model using external llama-cli command
#[derive(Debug)]
pub struct LlmModel {
    config: LlmConfig,
    llama_cli_available: bool,
}

impl LlmModel {
    /// Create a new LLM model with the given configuration
    pub fn new(config: LlmConfig) -> Self {
        let available = check_llama_cli_available();

        if !available {
            warn!("llama-cli not found. LLM suggestions will be disabled.");
            warn!("Install llama.cpp: brew install llama.cpp");
        } else {
            info!("llama-cli found. LLM suggestions enabled with model: {:?}", config.model_path);
        }

        Self {
            config,
            llama_cli_available: available,
        }
    }

    /// Call llama-cli once to generate a single suggestion
    fn call_llama_cli(&self, input: &str, seed: u64) -> Result<String> {
        // Format few-shot prompt with examples
        let prompt = format!(
            "git s→status\ndocker p→ps\nnpm i→install\n{}→",
            input
        );

        let output = Command::new("llama-cli")
            .arg("-m")
            .arg(&self.config.model_path)
            .arg("-p")
            .arg(&prompt)
            .arg("-n")
            .arg(self.config.max_tokens.to_string())
            .arg("--temp")
            .arg(self.config.temperature.to_string())
            .arg("--top-k")
            .arg("1")
            .arg("--seed")
            .arg(seed.to_string())
            .arg("--no-display-prompt")
            .arg("--reverse-prompt")
            .arg("\n")
            .arg("-no-cnv") // Disable conversation mode
            .stderr(Stdio::null()) // Suppress stderr output
            .output()?;

        if !output.status.success() {
            bail!("llama-cli exited with status: {}", output.status);
        }

        let text = String::from_utf8_lossy(&output.stdout);
        Ok(parse_llama_output(&text))
    }
}

impl SuggestModel for LlmModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        // Skip if llama-cli is not available
        if !self.llama_cli_available {
            return Ok(Vec::new());
        }

        // Skip if input is too short
        if input.trim().len() < 2 {
            return Ok(Vec::new());
        }

        let mut suggestions = Vec::new();
        let mut seen = HashSet::new();

        // Try up to 10 times to get 5 unique suggestions
        for i in 0..10 {
            if suggestions.len() >= 5 {
                break;
            }

            match self.call_llama_cli(input, self.config.seed + i) {
                Ok(text) => {
                    let trimmed = text.trim().to_string();

                    // Skip empty results
                    if trimmed.is_empty() {
                        continue;
                    }

                    // Add only unique suggestions
                    if seen.insert(trimmed.clone()) {
                        suggestions.push(Suggestion::with_source(trimmed, 1.0, "llm"));
                    }
                }
                Err(e) => {
                    warn!("llama-cli call {} failed: {}", i, e);
                }
            }
        }

        Ok(suggestions)
    }

    fn weight(&self) -> f64 {
        0.4
    }
}

/// Check if llama-cli command is available
fn check_llama_cli_available() -> bool {
    Command::new("llama-cli")
        .arg("--help")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

/// Parse llama-cli output to extract only the generated text
/// Removes any metadata, timing info, or prompts
fn parse_llama_output(text: &str) -> String {
    // llama-cli with --simple-io should give clean output,
    // but we still need to handle potential artifacts

    let lines: Vec<&str> = text.lines().collect();

    if lines.is_empty() {
        return String::new();
    }

    // Find the first non-empty line that looks like actual output
    // Skip lines that are obviously metadata or prompts
    for line in &lines {
        let trimmed = line.trim();

        // Skip empty lines
        if trimmed.is_empty() {
            continue;
        }

        // Skip lines that look like metadata
        if trimmed.starts_with("llama_")
            || trimmed.starts_with("sampling")
            || trimmed.contains("ms / ")
            || trimmed.contains("tok/s")
            || trimmed.starts_with("Log") {
            continue;
        }

        // Return the first line that looks like actual generated text
        return trimmed.to_string();
    }

    // If we didn't find anything good, return the first non-empty line
    lines.iter()
        .map(|l| l.trim())
        .find(|l| !l.is_empty())
        .unwrap_or("")
        .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_llama_output() {
        let output = "status\n";
        assert_eq!(parse_llama_output(output), "status");

        let output_with_metadata = "llama_model_loader: loaded meta data\nstatus\n";
        assert_eq!(parse_llama_output(output_with_metadata), "status");

        let empty = "";
        assert_eq!(parse_llama_output(empty), "");
    }

    #[test]
    fn test_check_llama_cli() {
        // This will fail in environments without llama-cli, which is OK
        let available = check_llama_cli_available();
        println!("llama-cli available: {}", available);
    }
}
