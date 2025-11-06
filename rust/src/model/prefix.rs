use anyhow::Result;
use libsql::Value;

use super::{sqlite::SqlitePool, SuggestModel, Suggestion};

#[derive(Clone, Debug)]
pub struct PrefixModel {
    pool: SqlitePool,
}

impl PrefixModel {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }
}

impl SuggestModel for PrefixModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        if input.trim().is_empty() {
            return Ok(Vec::new());
        }
        log::debug!("PrefixModel::predict invoked with input: {input}");
        let like = format!("{}%", input);
        let sql = r#"
            SELECT command, count
            FROM history
            WHERE command LIKE ?
            ORDER BY count DESC
            LIMIT 20
        "#;

        match self
            .pool
            .query_collect(sql, vec![Value::Text(like)], |row| {
                let command: String = row.get(0)?;
                let count: i64 = row.get(1)?;
                Ok(Suggestion::with_source(command, count as f64, "prefix"))
            }) {
            Ok(rows) => {
                log::debug!("PrefixModel::predict returning {} suggestions", rows.len());
                Ok(rows)
            }
            Err(err) if err.to_string().contains("no such table") => Ok(Vec::new()),
            Err(err) => Err(err),
        }
    }

    fn weight(&self) -> f64 {
        0.8
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn returns_commands_matching_prefix() {
        let pool = SqlitePool::open_memory().unwrap();
        pool.execute(
            "CREATE TABLE history (id INTEGER PRIMARY KEY, command TEXT, count INTEGER);",
            std::iter::empty::<Value>(),
        )
        .unwrap();
        for (cmd, count) in [("git status", 5), ("git commit", 3), ("ls", 10)] {
            pool.execute(
                "INSERT INTO history (command, count) VALUES (?, ?)",
                vec![Value::Text(cmd.to_string()), Value::Integer(count as i64)],
            )
            .unwrap();
        }

        let model = PrefixModel::new(pool);
        let suggestions = model.predict("git").unwrap();
        assert_eq!(suggestions.len(), 2);
        assert_eq!(suggestions[0].text, "git status");
        assert_eq!(suggestions[1].text, "git commit");
    }
}
