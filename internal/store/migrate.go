package store

import "fmt"

// migrations are applied in order. To change the schema, append a new entry;
// never edit an existing one. The index in this slice is the version number.
var migrations = []string{
	// 0: initial schema.
	`
CREATE TABLE sessions (
    session_id        TEXT PRIMARY KEY,
    file_path         TEXT NOT NULL,
    project_path      TEXT,            -- cwd recorded inside the transcript
    project_dir       TEXT,            -- the projects/<encoded> folder name
    display_name      TEXT,
    custom_title      TEXT,
    ai_title          TEXT,
    first_prompt      TEXT,
    message_count     INTEGER,
    file_size         INTEGER,
    last_activity     TEXT,            -- ISO-8601
    first_parent_uuid TEXT,           -- links a resumed/branched session to its parent
    file_mtime        INTEGER,         -- unix seconds, for change detection
    indexed_at        TEXT
);

CREATE TABLE messages (
    id          INTEGER PRIMARY KEY,
    session_id  TEXT NOT NULL,
    uuid        TEXT,
    parent_uuid TEXT,
    role        TEXT,
    ts          TEXT,
    seq         INTEGER
);
CREATE INDEX idx_messages_session ON messages(session_id);
CREATE INDEX idx_messages_uuid ON messages(uuid);

-- Contentless FTS5: we store the text only in the index and keep our own
-- row id (msg_id) so we can join back to messages/sessions.
CREATE VIRTUAL TABLE messages_fts USING fts5(
    body,
    session_id UNINDEXED,
    msg_id     UNINDEXED
);

CREATE TABLE trash (
    id            INTEGER PRIMARY KEY,
    kind          TEXT,    -- "session" | "memory-backup" | "memory-delete"
    original_path TEXT,
    trash_path    TEXT,
    session_id    TEXT,
    deleted_at    TEXT,
    purge_after   TEXT,    -- deleted_at + 30 days, ISO-8601
    note          TEXT
);

CREATE TABLE advisor_cache (
    id          INTEGER PRIMARY KEY,
    scope_key   TEXT,
    kind        TEXT,
    input_hash  TEXT,
    result_json TEXT,
    created_at  TEXT
);
`,
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("creating schema_version: %w", err)
	}

	var current int
	row := s.db.QueryRow(`SELECT version FROM schema_version`)
	switch err := row.Scan(&current); err {
	case nil:
		// have a version
	default:
		// No row yet: start before the first migration.
		current = -1
		if _, err := s.db.Exec(`INSERT INTO schema_version(version) VALUES(-1)`); err != nil {
			return fmt.Errorf("seeding schema_version: %w", err)
		}
	}

	for v := current + 1; v < len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", v, err)
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`UPDATE schema_version SET version = ?`, v); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", v, err)
		}
	}
	return nil
}
