use std::fmt::Debug;

use anyhow::Result;

#[derive(Clone, Debug, PartialEq)]
pub struct Suggestion {
    pub text: String,
    pub source: Option<String>,
    pub score: f64,
}

impl Suggestion {
    pub fn with_source<T: Into<String>, S: Into<String>>(text: T, score: f64, source: S) -> Self {
        Self {
            text: text.into(),
            source: Some(source.into()),
            score,
        }
    }
}

pub trait SuggestModel: Send + Sync + Debug {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>>;
    fn weight(&self) -> f64 {
        1.0
    }
}
