use std::path::Path;
#[cfg(test)]
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};

use anyhow::{Context, Result};
use directories::BaseDirs;
use libsql::{params::Params, Builder, Connection, Database, Row, Value};
use tokio::runtime::{Builder as RuntimeBuilder, Runtime};

#[derive(Clone)]
pub struct SqlitePool {
    #[allow(dead_code)]
    db: Arc<Database>,
    conn: Arc<Mutex<Connection>>,
    runtime: Arc<Runtime>,
}

impl std::fmt::Debug for SqlitePool {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SqlitePool").finish()
    }
}

impl SqlitePool {
    fn new(db: Database, runtime: Arc<Runtime>, apply_migrations: bool) -> Result<Self> {
        let conn = db
            .connect()
            .context("creating initial connection for sqlite pool")?;

        if apply_migrations {
            run_migrations(runtime.as_ref(), &conn)?;
        }

        Ok(Self {
            db: Arc::new(db),
            conn: Arc::new(Mutex::new(conn)),
            runtime,
        })
    }

    pub fn open_path(path: impl AsRef<Path>) -> Result<Self> {
        let path = path.as_ref();
        let path_str = path.to_string_lossy().to_string();
        let runtime = Arc::new(
            RuntimeBuilder::new_current_thread()
                .enable_all()
                .build()
                .context("creating runtime for libsql builder")?,
        );
        let db = runtime
            .block_on(Builder::new_local(&path_str).build())
            .with_context(|| format!("opening libsql database at {}", path_str))?;
        Self::new(db, runtime, true)
    }

    pub fn open_default() -> Result<Self> {
        let cache_dir = BaseDirs::new()
            .context("resolving cache directory for libsql")?
            .cache_dir()
            .to_path_buf();
        let db_path = cache_dir.join("ghosttype").join("ghosttype.db");
        if let Some(parent) = db_path.parent() {
            std::fs::create_dir_all(parent).with_context(|| {
                format!("creating parent directories for {}", db_path.display())
            })?;
        }
        Self::open_path(&db_path)
    }

    #[cfg(test)]
    pub fn open_memory() -> Result<Self> {
        static MEMORY_DB_COUNTER: AtomicUsize = AtomicUsize::new(0);
        let id = MEMORY_DB_COUNTER.fetch_add(1, Ordering::Relaxed);
        let uri = format!("file:ghosttype-memory-{id}?mode=memory&cache=shared");
        let runtime = Arc::new(
            RuntimeBuilder::new_current_thread()
                .enable_all()
                .build()
                .context("creating runtime for in-memory libsql")?,
        );
        let db = runtime
            .block_on(Builder::new_local(&uri).build())
            .context("opening in-memory libsql database")?;
        Self::new(db, runtime, false)
    }

    pub fn query_collect<T, I, F>(&self, sql: &str, params: I, mut map: F) -> Result<Vec<T>>
    where
        I: IntoIterator<Item = Value>,
        F: FnMut(Row) -> Result<T>,
    {
        let conn = self.conn.lock().expect("sqlite connection poisoned");
        let params = Params::Positional(params.into_iter().collect());
        let mut rows = self
            .runtime
            .block_on(conn.query(sql, params))
            .context("running libsql query")?;
        let mut out = Vec::new();
        while let Some(row) = self
            .runtime
            .block_on(rows.next())
            .context("fetching libsql row")?
        {
            out.push(map(row)?);
        }
        Ok(out)
    }

    pub fn execute<I>(&self, sql: &str, params: I) -> Result<()>
    where
        I: IntoIterator<Item = Value>,
    {
        let conn = self.conn.lock().expect("sqlite connection poisoned");
        let params = Params::Positional(params.into_iter().collect());
        self.runtime
            .block_on(conn.execute(sql, params))
            .context("executing libsql statement")?;
        Ok(())
    }
}

fn run_migrations(runtime: &Runtime, conn: &Connection) -> Result<()> {
    const SCHEMA_STATEMENTS: &[&str] = &[
        r#"CREATE TABLE IF NOT EXISTS history (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            command     TEXT NOT NULL,
            hash        TEXT NOT NULL UNIQUE,
            count       INTEGER NOT NULL DEFAULT 1,
            source      TEXT DEFAULT 'shell',
            session_id  TEXT DEFAULT '',
            output      TEXT DEFAULT '',
            created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );"#,
        "CREATE INDEX IF NOT EXISTS idx_history_command_prefix ON history(command);",
        r#"CREATE VIRTUAL TABLE IF NOT EXISTS history_fts USING fts5(
            command, content='history', content_rowid='id'
        );"#,
        r#"CREATE TRIGGER IF NOT EXISTS history_ai AFTER INSERT ON history BEGIN
            INSERT INTO history_fts(rowid, command) VALUES (new.id, new.command);
        END;"#,
        r#"CREATE TRIGGER IF NOT EXISTS history_ad AFTER DELETE ON history BEGIN
            INSERT INTO history_fts(history_fts, rowid, command) VALUES ('delete', old.id, old.command);
        END;"#,
        r#"CREATE TRIGGER IF NOT EXISTS history_au AFTER UPDATE ON history BEGIN
            INSERT INTO history_fts(history_fts, rowid, command) VALUES ('delete', old.id, old.command);
            INSERT INTO history_fts(rowid, command) VALUES (new.id, new.command);
        END;"#,
        "CREATE INDEX IF NOT EXISTS idx_history_hash ON history(hash);",
        r#"CREATE TABLE IF NOT EXISTS aliases (
            name TEXT PRIMARY KEY,
            cmd TEXT NOT NULL,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );"#,
        r#"CREATE TABLE IF NOT EXISTS meta (
            key TEXT PRIMARY KEY,
            path TEXT NOT NULL,
            mtime INTEGER NOT NULL
        );"#,
        r#"CREATE TABLE IF NOT EXISTS embeddings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            source TEXT NOT NULL,
            text TEXT NOT NULL,
            emb F32_BLOB(768),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );"#,
        "CREATE INDEX IF NOT EXISTS embeddings_idx ON embeddings(libsql_vector_idx(emb));",
    ];

    for sql in SCHEMA_STATEMENTS {
        runtime
            .block_on(conn.execute(sql, Params::Positional(Vec::<Value>::new())))
            .with_context(|| {
                format!(
                    "running migration statement: {}",
                    sql.lines().next().unwrap_or(sql)
                )
            })?;
    }

    if let Err(err) = runtime.block_on(conn.execute(
        "ALTER TABLE history ADD COLUMN output TEXT DEFAULT '';",
        Params::Positional(Vec::<Value>::new()),
    )) {
        let msg = err.to_string();
        if !msg.contains("duplicate column name") {
            let result: std::result::Result<(), libsql::Error> = Err(err);
            result.context("adding output column to history")?;
        }
    }

    Ok(())
}
