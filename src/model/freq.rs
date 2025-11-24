use anyhow::Result;
use libsql::Value;

use super::{sqlite::SqlitePool, SuggestModel, Suggestion};

#[derive(Clone, Debug)]
pub struct FreqModel {
    pool: SqlitePool,
}

impl FreqModel {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }
}

impl SuggestModel for FreqModel {
    fn predict(&self, input: &str) -> Result<Vec<Suggestion>> {
        if input.trim().is_empty() {
            return Ok(Vec::new());
        }

        let sql = r#"
            SELECT h.command, h.count
            FROM history_fts f
            JOIN history h ON f.rowid = h.id
            WHERE f.command MATCH ? || '*'
            ORDER BY h.count DESC
            LIMIT 20
        "#;

        match self
            .pool
            .query_collect(sql, vec![Value::Text(input.to_string())], |row| {
                let command: String = row.get(0)?;
                let count: i64 = row.get(1)?;
                Ok(Suggestion::with_source(command, count as f64, "freq"))
            }) {
            Ok(rows) => Ok(rows),
            Err(err) if err.to_string().contains("no such table") => Ok(Vec::new()),
            Err(err) => Err(err),
        }
    }

    fn weight(&self) -> f64 {
        0.5
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn returns_ranked_matches_from_fts() {
        let pool = SqlitePool::open_memory().unwrap();
        pool.execute(
            "CREATE TABLE history (id INTEGER PRIMARY KEY, command TEXT, count INTEGER);",
            std::iter::empty::<Value>(),
        )
        .unwrap();
        pool.execute(
            "CREATE VIRTUAL TABLE history_fts USING fts5(command);",
            std::iter::empty::<Value>(),
        )
        .unwrap();

        let entries = [
            (1_i64, "git status", 8_i64),
            (2, "git commit", 5),
            (3, "ls", 12),
        ];
        for (id, cmd, count) in entries {
            pool.execute(
                "INSERT INTO history (id, command, count) VALUES (?, ?, ?);",
                vec![
                    Value::Integer(id),
                    Value::Text(cmd.to_string()),
                    Value::Integer(count),
                ],
            )
            .unwrap();
            pool.execute(
                "INSERT INTO history_fts (rowid, command) VALUES (?, ?);",
                vec![Value::Integer(id), Value::Text(cmd.to_string())],
            )
            .unwrap();
        }

        let model = FreqModel::new(pool);
        let suggestions = model.predict("git").unwrap();
        assert_eq!(suggestions.len(), 2);
        assert_eq!(suggestions[0].text, "git status");
        assert_eq!(suggestions[0].score, 8.0);
        assert_eq!(suggestions[1].text, "git commit");
        assert_eq!(suggestions[1].score, 5.0);
    }
}
