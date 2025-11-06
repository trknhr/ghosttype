use std::sync::Arc;

use anyhow::Result;
use libsql::Value;

use crate::model::{sqlite::SqlitePool, SuggestModel, Suggestion};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct AliasEntry {
    pub name: String,
    pub cmd: String,
}

pub trait AliasStore: Send + Sync {
    fn query_aliases(&self, input: &str) -> Result<Vec<AliasEntry>>;
}

#[derive(Clone)]
pub struct AliasModel {
    store: Arc<dyn AliasStore>,
}

impl AliasModel {
    pub fn new(store: Arc<dyn AliasStore>) -> Self {
        Self { store }
    }

    pub fn with_sql_store(pool: SqlitePool) -> Self {
        Self::new(Arc::new(SqlAliasStore::new(pool)))
    }
}

impl std::fmt::Debug for AliasModel {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AliasModel").finish()
    }
}

impl SuggestModel for AliasModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        if input.trim().is_empty() {
            return Ok(Vec::new());
        }
        let entries = self.store.query_aliases(input)?;
        Ok(entries
            .into_iter()
            .map(|entry| Suggestion::with_source(entry.name, 1.0, "alias"))
            .collect())
    }

    fn weight(&self) -> f64 {
        0.8
    }
}

#[derive(Clone, Debug)]
pub struct SqlAliasStore {
    pool: SqlitePool,
}

impl SqlAliasStore {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }
}

impl AliasStore for SqlAliasStore {
    fn query_aliases(&self, input: &str) -> Result<Vec<AliasEntry>> {
        let like = format!("{}%", input);
        let sql = r#"
            SELECT name, cmd
            FROM aliases
            WHERE name LIKE ? OR cmd LIKE ?
            ORDER BY updated_at DESC
            LIMIT 10
        "#;

        match self.pool.query_collect(
            sql,
            vec![Value::Text(like.clone()), Value::Text(like)],
            |row| {
                let name: String = row.get(0)?;
                let cmd: String = row.get(1)?;
                Ok(AliasEntry { name, cmd })
            },
        ) {
            Ok(rows) => Ok(rows),
            Err(err) if err.to_string().contains("no such table") => Ok(Vec::new()),
            Err(err) => Err(err),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Clone, Debug)]
    struct MockStore {
        entries: Vec<AliasEntry>,
        error: Option<String>,
    }

    impl AliasStore for MockStore {
        fn query_aliases(&self, _input: &str) -> Result<Vec<AliasEntry>> {
            if let Some(err) = &self.error {
                Err(anyhow::anyhow!(err.clone()))
            } else {
                Ok(self.entries.clone())
            }
        }
    }

    #[test]
    fn predict_returns_alias_suggestions() {
        let store = MockStore {
            entries: vec![
                AliasEntry {
                    name: "gs".into(),
                    cmd: "git status".into(),
                },
                AliasEntry {
                    name: "ll".into(),
                    cmd: "ls -al".into(),
                },
            ],
            error: None,
        };
        let model = AliasModel::new(Arc::new(store));
        let suggestions = model.predict("g").unwrap();
        assert_eq!(suggestions.len(), 2);
        assert_eq!(suggestions[0].text, "gs");
        assert_eq!(suggestions[0].source.as_deref(), Some("alias"));
    }

    #[test]
    fn sql_store_queries_aliases() {
        let pool = SqlitePool::open_memory().unwrap();
        pool.execute(
            "CREATE TABLE aliases (name TEXT PRIMARY KEY, cmd TEXT NOT NULL, updated_at TEXT DEFAULT CURRENT_TIMESTAMP);",
            std::iter::empty::<Value>(),
        )
        .unwrap();
        for (name, cmd) in [
            ("gcm", "git commit"),
            ("gst", "git status"),
            ("ll", "ls -al"),
        ] {
            pool.execute(
                "INSERT INTO aliases (name, cmd) VALUES (?, ?);",
                vec![Value::Text(name.to_string()), Value::Text(cmd.to_string())],
            )
            .unwrap();
        }

        let store = SqlAliasStore::new(pool.clone());
        let entries = store.query_aliases("g").unwrap();
        assert!(entries.iter().any(|e| e.name == "gcm"));
        assert!(entries
            .iter()
            .all(|e| e.name.starts_with('g') || e.cmd.starts_with('g')));
    }
}
