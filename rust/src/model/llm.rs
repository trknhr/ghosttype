use std::collections::HashSet;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use anyhow::{Context, Result};
use candle_core::{Device, Tensor};
use candle_core::quantized::gguf_file;
use candle_transformers::generation::{LogitsProcessor, Sampling};
use candle_transformers::models::quantized_qwen2::ModelWeights as Qwen2;
use log::{info, warn};
use tokenizers::Tokenizer;

use super::{SuggestModel, Suggestion};

/// Which Qwen2 model variant to use
#[derive(Debug, Clone, Copy)]
pub enum Which {
    W0_5b,
    W1_5b,
    W7b,
}

impl Which {
    pub fn from_str(s: &str) -> Result<Self> {
        match s {
            "0.5b" => Ok(Which::W0_5b),
            "1.5b" => Ok(Which::W1_5b),
            "7b" => Ok(Which::W7b),
            _ => anyhow::bail!("Unknown model variant: {}", s),
        }
    }

    fn repo(&self) -> &'static str {
        match self {
            Which::W0_5b => "unsloth/Qwen2.5-Coder-0.5B-Instruct-GGUF",
            Which::W1_5b => "Qwen/Qwen2-1.5B-Instruct-GGUF",
            Which::W7b => "Qwen/Qwen2-7B-Instruct-GGUF",
        }
    }

    fn filename(&self) -> &'static str {
        match self {
            Which::W0_5b => "Qwen2.5-Coder-0.5B-Instruct-Q4_K_M.gguf",
            Which::W1_5b => "qwen2-1_5b-instruct-q4_0.gguf",
            Which::W7b => "qwen2-7b-instruct-q4_0.gguf",
        }
    }

    fn tokenizer_repo(&self) -> &'static str {
        match self {
            Which::W0_5b => "Qwen/Qwen2.5-Coder-0.5B-Instruct",
            Which::W1_5b => "Qwen/Qwen2-1.5B-Instruct",
            Which::W7b => "Qwen/Qwen2-7B-Instruct",
        }
    }
}

/// Configuration for the LLM model
#[derive(Debug, Clone)]
pub struct LlmConfig {
    pub model_path: Option<PathBuf>,
    pub which: Which,
    pub temperature: f64,
    pub repeat_penalty: f32,
    pub repeat_last_n: usize,
    pub max_new_tokens: usize,
    pub seed: u64,
}

impl Default for LlmConfig {
    fn default() -> Self {
        Self {
            model_path: None,
            which: Which::W0_5b,
            temperature: 0.8,
            repeat_penalty: 1.0,
            repeat_last_n: 32,
            max_new_tokens: 12,
            seed: 299792458,
        }
    }
}

/// Loaded model state (heavy, kept in memory)
struct ModelState {
    model: Qwen2,
    tokenizer: Tokenizer,
    device: Device,
    eos_token: u32,
    config: LlmConfig,
}

impl std::fmt::Debug for ModelState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ModelState")
            .field("device", &self.device)
            .field("eos_token", &self.eos_token)
            .field("config", &self.config)
            .field("model", &"<Qwen2 model>")
            .field("tokenizer", &"<Tokenizer>")
            .finish()
    }
}

impl ModelState {
    fn load(config: &LlmConfig) -> Result<Self> {
        info!("Loading LLM model ({})", match config.which {
            Which::W0_5b => "0.5B",
            Which::W1_5b => "1.5B",
            Which::W7b => "7B",
        });

        let device = Device::Cpu;

        // Get model path (from arg or download from HF)
        let model_path = match &config.model_path {
            Some(path) => path.clone(),
            None => {
                info!("Downloading model from HuggingFace...");
                let api = hf_hub::api::sync::Api::new()?;
                let repo = api.repo(hf_hub::Repo::model(config.which.repo().to_string()));
                repo.get(config.which.filename())?
            }
        };

        // Load GGUF model
        let mut file = std::fs::File::open(&model_path)
            .with_context(|| format!("Failed to open model file: {:?}", model_path))?;

        let content = gguf_file::Content::read(&mut file)
            .map_err(|e| anyhow::anyhow!("Failed to read GGUF: {}", e))?;

        let model = Qwen2::from_gguf(content, &mut file, &device)
            .context("Failed to load Qwen2 model from GGUF")?;

        // Get tokenizer (from arg or download from HF)
        let tokenizer = {
            let api = hf_hub::api::sync::Api::new()?;
            let repo = api.model(config.which.tokenizer_repo().to_string());
            let tokenizer_path = repo.get("tokenizer.json")?;
            Tokenizer::from_file(tokenizer_path)
                .map_err(|e| anyhow::anyhow!("Failed to load tokenizer: {}", e))?
        };

        // Get EOS token
        let eos_token = *tokenizer
            .get_vocab(true)
            .get("<|im_end|>")
            .ok_or_else(|| anyhow::anyhow!("EOS token not found in vocabulary"))?;

        info!("LLM model loaded successfully");

        Ok(Self {
            model,
            tokenizer,
            device,
            eos_token,
            config: config.clone(),
        })
    }

    fn generate_suggestions(&mut self, prefix: &str, n: usize) -> Result<Vec<String>> {
        // Format FIM prompt
        let prompt_str = format!("<|fim_prefix|>{}<|fim_suffix|><|fim_middle|>", prefix);

        // Tokenize
        let tokens = self
            .tokenizer
            .encode(prompt_str, true)
            .map_err(|e| anyhow::anyhow!("Tokenization failed: {}", e))?;
        let prefix_tokens = tokens.get_ids();

        // Sampling config
        let sampling = if self.config.temperature <= 0.0 {
            Sampling::ArgMax
        } else {
            Sampling::All {
                temperature: self.config.temperature,
            }
        };

        let mut out = Vec::new();
        let mut seen = HashSet::new();
        let mut seed_base = self.config.seed;

        let max_attempts = n * 2; // Allow more attempts to get n unique results
        let mut attempts = 0;

        while out.len() < n && attempts < max_attempts {
            attempts += 1;

            match self.generate_single_line(prefix_tokens, sampling.clone(), seed_base) {
                Ok(cand) => {
                    seed_base = seed_base.wrapping_add(1);

                    // Skip empty results
                    if cand.is_empty() {
                        continue;
                    }

                    // Normalize and deduplicate
                    let cand = cand.trim().to_string();
                    if !cand.is_empty() && seen.insert(cand.clone()) {
                        out.push(cand);
                    }
                }
                Err(e) => {
                    warn!("LLM generation attempt {} failed: {}", attempts, e);
                    seed_base = seed_base.wrapping_add(1);
                }
            }
        }

        Ok(out)
    }

    fn generate_single_line(
        &mut self,
        prefix_tokens: &[u32],
        sampling: Sampling,
        seed: u64,
    ) -> Result<String> {
        let mut lp = LogitsProcessor::from_sampling(seed, sampling);

        // All tokens for repeat penalty
        let mut all_tokens: Vec<u32> = prefix_tokens.to_vec();

        // Warm up: forward through prompt without sampling
        let mut last_logits = None;
        for (pos, tk) in prefix_tokens.iter().enumerate() {
            let input = Tensor::new(&[*tk], &self.device)?.unsqueeze(0)?;
            let logits = self.model.forward(&input, pos)?.squeeze(0)?;
            last_logits = Some(logits);
        }

        // Sample first token from prompt's final logits
        let mut next_token = {
            let logits = last_logits.ok_or_else(|| anyhow::anyhow!("No logits from prompt"))?;
            lp.sample(&logits)?
        };
        all_tokens.push(next_token);

        // Generate until newline, EOS, or max tokens
        let mut generated = String::new();
        for i in 0..self.config.max_new_tokens {
            if next_token == self.eos_token {
                break;
            }

            // Decode token
            if let Some(piece) = self.tokenizer.decode(&[next_token], false).ok() {
                // Stop at first newline
                if let Some(nl) = piece.find('\n') {
                    generated.push_str(&piece[..nl]);
                    break;
                } else {
                    generated.push_str(&piece);
                }
            }

            // Generate next token
            let input = Tensor::new(&[next_token], &self.device)?.unsqueeze(0)?;
            let mut logits = self
                .model
                .forward(&input, prefix_tokens.len() + i)?
                .squeeze(0)?;

            // Apply repeat penalty
            if self.config.repeat_penalty != 1.0 {
                let start_at = all_tokens.len().saturating_sub(self.config.repeat_last_n);
                logits = candle_transformers::utils::apply_repeat_penalty(
                    &logits,
                    self.config.repeat_penalty,
                    &all_tokens[start_at..],
                )?;
            }

            next_token = lp.sample(&logits)?;
            all_tokens.push(next_token);
        }

        Ok(generated.trim().to_string())
    }
}

/// LLM-based suggestion model
#[derive(Debug)]
pub struct LlmModel {
    config: Arc<LlmConfig>,
    state: Arc<Mutex<Option<ModelState>>>,
}

impl LlmModel {
    /// Create a new LLM model with the given configuration
    pub fn new(config: LlmConfig) -> Self {
        Self {
            config: Arc::new(config),
            state: Arc::new(Mutex::new(None)),
        }
    }

    /// Create with default configuration
    pub fn with_defaults() -> Self {
        Self::new(LlmConfig::default())
    }

    /// Create with custom model path and variant
    pub fn with_model(model_path: Option<PathBuf>, which: Which) -> Self {
        Self::new(LlmConfig {
            model_path,
            which,
            ..Default::default()
        })
    }
}

impl SuggestModel for LlmModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        // Skip if input is too short
        if input.trim().len() < 2 {
            return Ok(Vec::new());
        }

        // Lazy load model on first call
        let mut state_lock = self.state.lock().unwrap();

        if state_lock.is_none() {
            match ModelState::load(&self.config) {
                Ok(state) => {
                    *state_lock = Some(state);
                }
                Err(e) => {
                    warn!("Failed to load LLM model: {}. LLM suggestions disabled.", e);
                    return Ok(Vec::new());
                }
            }
        }

        let state = match state_lock.as_mut() {
            Some(s) => s,
            None => return Ok(Vec::new()),
        };

        // Generate 5 suggestions
        match state.generate_suggestions(input, 5) {
            Ok(results) => Ok(results
                .into_iter()
                .map(|cmd| Suggestion::with_source(cmd, 1.0, "llm"))
                .collect()),
            Err(e) => {
                warn!("LLM generation failed: {}", e);
                Ok(Vec::new())
            }
        }
    }

    fn weight(&self) -> f64 {
        0.4
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_which_from_str() {
        assert!(matches!(Which::from_str("0.5b"), Ok(Which::W0_5b)));
        assert!(matches!(Which::from_str("1.5b"), Ok(Which::W1_5b)));
        assert!(matches!(Which::from_str("7b"), Ok(Which::W7b)));
        assert!(Which::from_str("invalid").is_err());
    }
}
