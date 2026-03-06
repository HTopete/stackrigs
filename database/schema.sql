-- ============================================================================
-- StackRigs — Schema SQL para SQLite
-- stackrigs.com — Indice abierto y estructurado de lo que los developers
--                 estan construyendo.
-- ============================================================================
-- Generado: 2026-03-06
-- Motor:    SQLite 3.x (con FTS5)
-- Fuente de verdad: internal/database/db.go
-- ============================================================================

-- ---------------------------------------------------------------------------
-- PRAGMAs — ejecutar ANTES de cualquier sentencia DDL
-- ---------------------------------------------------------------------------
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout  = 5000;

-- ---------------------------------------------------------------------------
-- 1. schema_version — tracking de migraciones manuales
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    description TEXT
);

-- Registro inicial
INSERT INTO schema_version (version, description)
VALUES (1, 'Initial schema — synced with db.go');

-- ---------------------------------------------------------------------------
-- 2. builders — perfil del usuario
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS builders (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    handle          TEXT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL,
    bio             TEXT DEFAULT '',
    avatar_url      TEXT DEFAULT '',
    website         TEXT DEFAULT '',
    github_url      TEXT DEFAULT '',
    twitter_url     TEXT DEFAULT '',
    invitation_code TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ---------------------------------------------------------------------------
-- 3. technologies — taxonomia normalizada
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS technologies (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name     TEXT NOT NULL UNIQUE,
    slug     TEXT NOT NULL UNIQUE,
    category TEXT DEFAULT ''
);

-- ---------------------------------------------------------------------------
-- 4. builds — proyecto / build log
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS builds (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    builder_id   INTEGER NOT NULL,
    name         TEXT NOT NULL,
    description  TEXT DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'building',
    repo_url     TEXT DEFAULT '',
    live_url     TEXT DEFAULT '',
    cover_image  TEXT DEFAULT '',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (builder_id) REFERENCES builders(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- 5. build_technologies — tabla puente
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS build_technologies (
    build_id      INTEGER NOT NULL,
    technology_id INTEGER NOT NULL,
    PRIMARY KEY (build_id, technology_id),
    FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE,
    FOREIGN KEY (technology_id) REFERENCES technologies(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- 6. build_updates — changelog incremental
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS build_updates (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    build_id   INTEGER NOT NULL,
    content    TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- 7. invitation_codes — sistema de invitaciones
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS invitation_codes (
    code       TEXT PRIMARY KEY,
    used_by    INTEGER DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    used_at    DATETIME DEFAULT NULL,
    FOREIGN KEY (used_by) REFERENCES builders(id)
);

-- ---------------------------------------------------------------------------
-- 8. sessions — autenticacion
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    builder_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (builder_id) REFERENCES builders(id) ON DELETE CASCADE
);

-- ---------------------------------------------------------------------------
-- 9. webauthn_credentials — credenciales de passkeys
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id               TEXT PRIMARY KEY,
    builder_id       INTEGER NOT NULL REFERENCES builders(id) ON DELETE CASCADE,
    credential_id    BLOB NOT NULL,
    public_key       BLOB NOT NULL,
    attestation_type TEXT,
    transport        TEXT,
    sign_count       INTEGER DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- 10. github_connections — conexiones OAuth de GitHub
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS github_connections (
    builder_id        INTEGER PRIMARY KEY REFERENCES builders(id) ON DELETE CASCADE,
    github_id         INTEGER NOT NULL UNIQUE,
    github_username   TEXT NOT NULL,
    github_avatar_url TEXT,
    access_token      TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- ---------------------------------------------------------------------------
-- 11. search_index — FTS5 full-text search
-- ---------------------------------------------------------------------------
CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
    entity_type,
    entity_id,
    title,
    body,
    tokenize='porter unicode61'
);

-- ---------------------------------------------------------------------------
-- 12. build_views — contador de vistas anonimo (feature planificada)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS build_views (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    build_id  INTEGER NOT NULL REFERENCES builds(id) ON DELETE CASCADE,
    viewed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    referrer  TEXT
);

-- ---------------------------------------------------------------------------
-- 13. bookmarks — bookmarks privados (feature planificada)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookmarks (
    builder_id INTEGER NOT NULL REFERENCES builders(id) ON DELETE CASCADE,
    build_id   INTEGER NOT NULL REFERENCES builds(id)   ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    PRIMARY KEY (builder_id, build_id)
);

-- ===========================================================================
-- INDICES (coinciden exactamente con db.go)
-- ===========================================================================

-- builds
CREATE INDEX IF NOT EXISTS idx_builds_builder_id ON builds(builder_id);
CREATE INDEX IF NOT EXISTS idx_builds_status ON builds(status);
CREATE INDEX IF NOT EXISTS idx_builds_updated_at ON builds(updated_at);

-- build_technologies
CREATE INDEX IF NOT EXISTS idx_build_technologies_tech_id ON build_technologies(technology_id);

-- build_updates
CREATE INDEX IF NOT EXISTS idx_build_updates_build_id ON build_updates(build_id);

-- builders
CREATE INDEX IF NOT EXISTS idx_builders_handle ON builders(handle);

-- technologies
CREATE INDEX IF NOT EXISTS idx_technologies_slug ON technologies(slug);

-- sessions
CREATE INDEX IF NOT EXISTS idx_sessions_builder_id ON sessions(builder_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- webauthn_credentials
CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_builder ON webauthn_credentials(builder_id);

-- github_connections
CREATE INDEX IF NOT EXISTS idx_github_connections_github_id ON github_connections(github_id);

-- build_views (feature planificada)
CREATE INDEX IF NOT EXISTS idx_build_views_build_id ON build_views(build_id, viewed_at DESC);

-- bookmarks (feature planificada)
CREATE INDEX IF NOT EXISTS idx_bookmarks_build_id ON bookmarks(build_id);
