package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	// Pragmas optimized for microSD on Raspberry Pi 5:
	// - WAL mode: concurrent reads, fewer writes to disk
	// - synchronous=NORMAL: safe with WAL, reduces fsync calls (gentler on SD wear)
	// - cache_size=-20000: 20MB in-memory cache (negative = KB), reduces disk reads
	// - wal_autocheckpoint=1000: checkpoint every 1000 pages (~4MB), less frequent writes
	// - mmap_size=67108864: 64MB memory-mapped I/O, bypasses read syscalls
	// - temp_store=MEMORY: temp tables in RAM, not disk
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-20000&_foreign_keys=ON", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// WAL mode allows concurrent readers, so we can have more than 1 conn
	// but SQLite still serializes writes — 2 conns is the sweet spot
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Additional pragmas that can't be set via DSN
	pragmas := []string{
		"PRAGMA wal_autocheckpoint=1000",  // checkpoint every ~4MB (less frequent SD writes)
		"PRAGMA mmap_size=67108864",       // 64MB mmap (reduces read syscalls)
		"PRAGMA temp_store=MEMORY",        // temp tables in RAM, not on SD
		"PRAGMA page_size=8192",           // 8KB pages (better for SD block alignment)
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			slog.Warn("pragma failed (non-fatal)", "pragma", p, "error", err)
		}
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("database connected",
		"path", path,
		"max_conns", 2,
		"cache_size_kb", 20000,
		"mmap_size_mb", 64,
		"wal_autocheckpoint", 1000,
	)
	return db, nil
}

func runMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS builders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			handle TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			bio TEXT DEFAULT '',
			avatar_url TEXT DEFAULT '',
			website TEXT DEFAULT '',
			github_url TEXT DEFAULT '',
			twitter_url TEXT DEFAULT '',
			invitation_code TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS technologies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			category TEXT DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS builds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			builder_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'building',
			repo_url TEXT DEFAULT '',
			live_url TEXT DEFAULT '',
			cover_image TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (builder_id) REFERENCES builders(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS build_technologies (
			build_id INTEGER NOT NULL,
			technology_id INTEGER NOT NULL,
			PRIMARY KEY (build_id, technology_id),
			FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE,
			FOREIGN KEY (technology_id) REFERENCES technologies(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS build_updates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			build_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS invitation_codes (
			code TEXT PRIMARY KEY,
			used_by INTEGER DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			used_at DATETIME DEFAULT NULL,
			FOREIGN KEY (used_by) REFERENCES builders(id)
		)`,

		// Auth: sessions
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			builder_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (builder_id) REFERENCES builders(id) ON DELETE CASCADE
		)`,

		// Auth: WebAuthn credentials
		`CREATE TABLE IF NOT EXISTS webauthn_credentials (
			id TEXT PRIMARY KEY,
			builder_id INTEGER NOT NULL REFERENCES builders(id) ON DELETE CASCADE,
			credential_id BLOB NOT NULL,
			public_key BLOB NOT NULL,
			attestation_type TEXT,
			transport TEXT,
			sign_count INTEGER DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)`,

		// Auth: GitHub connections
		`CREATE TABLE IF NOT EXISTS github_connections (
			builder_id INTEGER PRIMARY KEY REFERENCES builders(id) ON DELETE CASCADE,
			github_id INTEGER NOT NULL UNIQUE,
			github_username TEXT NOT NULL,
			github_avatar_url TEXT,
			access_token TEXT,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)`,

		// FTS5 search index
		`CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
			entity_type,
			entity_id,
			title,
			body,
			tokenize='porter unicode61'
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_builds_builder_id ON builds(builder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_builds_status ON builds(status)`,
		`CREATE INDEX IF NOT EXISTS idx_builds_updated_at ON builds(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_build_technologies_tech_id ON build_technologies(technology_id)`,
		`CREATE INDEX IF NOT EXISTS idx_build_updates_build_id ON build_updates(build_id)`,
		`CREATE INDEX IF NOT EXISTS idx_builders_handle ON builders(handle)`,
		`CREATE INDEX IF NOT EXISTS idx_technologies_slug ON technologies(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_builder_id ON sessions(builder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_builder ON webauthn_credentials(builder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_github_connections_github_id ON github_connections(github_id)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w\nSQL: %s", err, m)
		}
	}

	// Schema evolution: add columns (idempotent — ignores "duplicate column" errors)
	alterations := []string{
		`ALTER TABLE builds ADD COLUMN what_works TEXT DEFAULT ''`,
		`ALTER TABLE builds ADD COLUMN what_broke TEXT DEFAULT ''`,
		`ALTER TABLE builds ADD COLUMN what_id_change TEXT DEFAULT ''`,
		`ALTER TABLE build_updates ADD COLUMN type TEXT DEFAULT 'milestone'`,
		`ALTER TABLE build_updates ADD COLUMN title TEXT DEFAULT ''`,
	}
	for _, alt := range alterations {
		if _, err := db.Exec(alt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("schema evolution: %w\nSQL: %s", err, alt)
			}
		}
	}

	slog.Info("database migrations completed")
	return nil
}
