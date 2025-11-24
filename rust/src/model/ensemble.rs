use std::cmp::Ordering;
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use anyhow::Result;

use super::{SuggestModel, Suggestion};

pub type SharedModel = Arc<dyn SuggestModel>;

#[derive(Default)]
pub struct EnsembleBuilder {
    light_models: Vec<SharedModel>,
    heavy_models: Vec<SharedModel>,
}

impl EnsembleBuilder {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_light_model<M>(mut self, model: M) -> Self
    where
        M: SuggestModel + 'static,
    {
        self.light_models.push(Arc::new(model));
        self
    }

    pub fn with_heavy_model<M>(mut self, model: M) -> Self
    where
        M: SuggestModel + 'static,
    {
        self.heavy_models.push(Arc::new(model));
        self
    }

    pub fn build(self) -> Ensemble {
        Ensemble::new(self.light_models, self.heavy_models)
    }
}

pub struct Ensemble {
    light_models: RwLock<Vec<SharedModel>>,
    heavy_models: RwLock<Vec<SharedModel>>,
}

impl Ensemble {
    pub fn new(light_models: Vec<SharedModel>, heavy_models: Vec<SharedModel>) -> Self {
        Self {
            light_models: RwLock::new(light_models),
            heavy_models: RwLock::new(heavy_models),
        }
    }

    /// Legacy method: predicts using all models (both light and heavy)
    /// This blocks on heavy models, so should be avoided in favor of predict_light_models()
    pub fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        let light = self.light_models.read().expect("ensemble lock poisoned");
        let heavy = self.heavy_models.read().expect("ensemble lock poisoned");
        let all_models = light.iter().chain(heavy.iter()).cloned();
        Self::aggregate_predictions(all_models, input)
    }

    /// Predict using only light (fast, synchronous) models
    /// Returns immediately without blocking on heavy models
    pub fn predict_light_models(&self, input: &str) -> Result<Vec<Suggestion>> {
        let models = self.light_models.read().expect("ensemble lock poisoned");
        Self::aggregate_predictions(models.iter().cloned(), input)
    }

    /// Get clones of heavy models for async execution
    pub fn get_heavy_models(&self) -> Vec<SharedModel> {
        let models = self.heavy_models.read().expect("ensemble lock poisoned");
        models.clone()
    }

    fn aggregate_predictions<I>(models: I, input: &str) -> Result<Vec<Suggestion>>
    where
        I: IntoIterator<Item = SharedModel>,
    {
        let mut score_map: HashMap<String, (f64, Option<String>)> = HashMap::new();

        for model in models {
            let suggestions = model.predict(input)?;
            let weight = model.weight();

            for suggestion in suggestions {
                let entry = score_map
                    .entry(suggestion.text.clone())
                    .or_insert((0.0, None));
                entry.0 += suggestion.score * weight;
                if entry.1.is_none() {
                    entry.1 = suggestion.source.clone();
                }
            }
        }

        let mut ranked: Vec<Suggestion> = score_map
            .into_iter()
            .map(|(text, (score, source))| Suggestion {
                text,
                score,
                source,
            })
            .collect();

        ranked.sort_by(|a, b| b.score.partial_cmp(&a.score).unwrap_or(Ordering::Equal));

        Ok(ranked)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Debug)]
    struct StaticModel {
        suggestions: Vec<Suggestion>,
        weight: f64,
    }

    impl StaticModel {
        fn new(weight: f64, suggestions: Vec<Suggestion>) -> Self {
            Self {
                suggestions,
                weight,
            }
        }
    }

    impl SuggestModel for StaticModel {
        fn predict(&self, _input: &str) -> Result<Vec<Suggestion>> {
            Ok(self.suggestions.clone())
        }

        fn weight(&self) -> f64 {
            self.weight
        }
    }

    #[test]
    fn aggregates_scores_across_models() {
        let first = Arc::new(StaticModel::new(
            1.0,
            vec![Suggestion::with_source("git status", 2.0, "freq")],
        )) as SharedModel;
        let second = Arc::new(StaticModel::new(
            0.5,
            vec![Suggestion::with_source("git status", 4.0, "llm")],
        )) as SharedModel;

        // Test with both light and heavy models
        let ensemble = Ensemble::new(vec![first.clone()], vec![second.clone()]);

        let result = ensemble.predict("git").unwrap();

        assert_eq!(result.len(), 1);
        assert_eq!(result[0].text, "git status");
        assert!((result[0].score - 4.0).abs() < f64::EPSILON);
    }

    #[test]
    fn light_models_only() {
        let first = Arc::new(StaticModel::new(
            1.0,
            vec![Suggestion::with_source("git status", 2.0, "freq")],
        )) as SharedModel;
        let second = Arc::new(StaticModel::new(
            0.5,
            vec![Suggestion::with_source("git commit", 3.0, "prefix")],
        )) as SharedModel;

        let ensemble = Ensemble::new(vec![first, second], vec![]);

        let result = ensemble.predict_light_models("git").unwrap();

        assert_eq!(result.len(), 2);
    }
}
