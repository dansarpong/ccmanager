package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// UpsertSession inserts or replaces a session summary row.
func (s *Store) UpsertSession(se Session) error {
	_, err := s.db.Exec(`
INSERT INTO sessions (
    session_id, file_path, project_path, project_dir, display_name,
    custom_title, ai_title, first_prompt, message_count, file_size,
    last_activity, first_parent_uuid, file_mtime, indexed_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(session_id) DO UPDATE SET
    file_path=excluded.file_path, project_path=excluded.project_path,
    project_dir=excluded.project_dir, display_name=excluded.display_name,
    custom_title=excluded.custom_title, ai_title=excluded.ai_title,
    first_prompt=excluded.first_prompt, message_count=excluded.message_count,
    file_size=excluded.file_size, last_activity=excluded.last_activity,
    first_parent_uuid=excluded.first_parent_uuid, file_mtime=excluded.file_mtime,
    indexed_at=excluded.indexed_at`,
		se.SessionID, se.FilePath, se.ProjectPath, se.ProjectDir, se.DisplayName,
		se.CustomTitle, se.AITitle, se.FirstPrompt, se.MessageCount, se.FileSize,
		se.LastActivity, se.FirstParentUUID, se.FileMtime, se.IndexedAt)
	if err != nil {
		return fmt.Errorf("upserting session %s: %w", se.SessionID, err)
	}
	return nil
}

// ReplaceMessages deletes a session's existing message + FTS rows and inserts
// the supplied ones. Doing both in one transaction keeps the FTS index
// consistent on every re-scan and after a delete.
func (s *Store) ReplaceMessages(sessionID string, msgs []Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin replace messages: %w", err)
	}
	defer tx.Rollback()

	if err := deleteMessageRows(tx, sessionID); err != nil {
		return err
	}

	insMsg, err := tx.Prepare(`INSERT INTO messages (session_id, uuid, parent_uuid, role, ts, seq) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare message insert: %w", err)
	}
	defer insMsg.Close()
	insFts, err := tx.Prepare(`INSERT INTO messages_fts (body, session_id, msg_id) VALUES (?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare fts insert: %w", err)
	}
	defer insFts.Close()

	for _, m := range msgs {
		res, err := insMsg.Exec(m.SessionID, m.UUID, m.ParentUUID, m.Role, m.Timestamp, m.Seq)
		if err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("message last insert id: %w", err)
		}
		if strings.TrimSpace(m.Body) == "" {
			continue
		}
		if _, err := insFts.Exec(m.Body, m.SessionID, id); err != nil {
			return fmt.Errorf("insert fts row: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace messages: %w", err)
	}
	return nil
}

func deleteMessageRows(tx *sql.Tx, sessionID string) error {
	if _, err := tx.Exec(`DELETE FROM messages_fts WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete fts rows: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete message rows: %w", err)
	}
	return nil
}

// DeleteSessionRows removes a session and all its message/FTS rows from the
// index (used after a transcript is moved to trash).
func (s *Store) DeleteSessionRows(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete session: %w", err)
	}
	defer tx.Rollback()
	if err := deleteMessageRows(tx, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("delete session row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete session: %w", err)
	}
	return nil
}

// IndexedFiles returns the mtime and size recorded for each indexed file,
// keyed by file path, so the indexer can skip unchanged files.
func (s *Store) IndexedFiles() (map[string]FileMeta, error) {
	rows, err := s.db.Query(`SELECT file_path, file_mtime, file_size FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("querying indexed files: %w", err)
	}
	defer rows.Close()
	out := make(map[string]FileMeta)
	for rows.Next() {
		var path string
		var m FileMeta
		if err := rows.Scan(&path, &m.Mtime, &m.Size); err != nil {
			return nil, fmt.Errorf("scanning indexed file: %w", err)
		}
		out[path] = m
	}
	return out, rows.Err()
}

// FileMeta is the change-detection key for a transcript file.
type FileMeta struct {
	Mtime int64
	Size  int64
}

// ListSessions returns session summaries ordered by the given sort key.
// sortKey is one of "activity" (default), "size", "count". projectFilter, if
// non-empty, restricts to a single project_path.
func (s *Store) ListSessions(sortKey, projectFilter string) ([]Session, error) {
	order := "last_activity DESC"
	switch sortKey {
	case "size":
		order = "file_size DESC"
	case "count":
		order = "message_count DESC"
	}
	q := `SELECT session_id, file_path, project_path, project_dir, display_name,
                 custom_title, ai_title, first_prompt, message_count, file_size,
                 last_activity, first_parent_uuid, file_mtime, indexed_at
          FROM sessions`
	var args []any
	if projectFilter != "" {
		q += ` WHERE project_path = ?`
		args = append(args, projectFilter)
	}
	q += ` ORDER BY ` + order
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

// GetSession returns one session by id, or sql.ErrNoRows if absent.
func (s *Store) GetSession(id string) (Session, error) {
	row := s.db.QueryRow(`SELECT session_id, file_path, project_path, project_dir, display_name,
                 custom_title, ai_title, first_prompt, message_count, file_size,
                 last_activity, first_parent_uuid, file_mtime, indexed_at
          FROM sessions WHERE session_id = ?`, id)
	var se Session
	err := row.Scan(&se.SessionID, &se.FilePath, &se.ProjectPath, &se.ProjectDir, &se.DisplayName,
		&se.CustomTitle, &se.AITitle, &se.FirstPrompt, &se.MessageCount, &se.FileSize,
		&se.LastActivity, &se.FirstParentUUID, &se.FileMtime, &se.IndexedAt)
	if err != nil {
		return Session{}, fmt.Errorf("getting session %s: %w", id, err)
	}
	return se, nil
}

// ChildSessions returns sessions that resumed/branched from a uuid owned by
// the given session (i.e. their first_parent_uuid belongs to this session).
func (s *Store) ChildSessions(sessionID string) ([]Session, error) {
	rows, err := s.db.Query(`
SELECT c.session_id, c.file_path, c.project_path, c.project_dir, c.display_name,
       c.custom_title, c.ai_title, c.first_prompt, c.message_count, c.file_size,
       c.last_activity, c.first_parent_uuid, c.file_mtime, c.indexed_at
FROM sessions c
JOIN messages m ON m.uuid = c.first_parent_uuid
WHERE m.session_id = ? AND c.session_id <> ?`, sessionID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying child sessions: %w", err)
	}
	defer rows.Close()
	return scanSessions(rows)
}

// ParentSessionID returns the session that owns this session's
// first_parent_uuid, or "" if it has no cross-session parent.
func (s *Store) ParentSessionID(sessionID, firstParentUUID string) (string, error) {
	if firstParentUUID == "" {
		return "", nil
	}
	row := s.db.QueryRow(`SELECT session_id FROM messages WHERE uuid = ? AND session_id <> ? LIMIT 1`, firstParentUUID, sessionID)
	var owner string
	switch err := row.Scan(&owner); err {
	case nil:
		return owner, nil
	case sql.ErrNoRows:
		return "", nil
	default:
		return "", fmt.Errorf("finding parent session: %w", err)
	}
}

// DistinctProjects returns every distinct project_path in the index, sorted.
func (s *Store) DistinctProjects() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT project_path FROM sessions WHERE project_path <> '' ORDER BY project_path`)
	if err != nil {
		return nil, fmt.Errorf("querying distinct projects: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MostRecentlyActiveID returns the session_id with the newest last_activity,
// or "" if the index is empty.
func (s *Store) MostRecentlyActiveID() (string, error) {
	row := s.db.QueryRow(`SELECT session_id FROM sessions ORDER BY last_activity DESC LIMIT 1`)
	var id string
	switch err := row.Scan(&id); err {
	case nil, sql.ErrNoRows:
		return id, nil
	default:
		return "", fmt.Errorf("finding most recent session: %w", err)
	}
}

// SearchFTS runs a full-text query and returns hits with highlighted snippets.
func (s *Store) SearchFTS(query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
SELECT f.session_id,
       COALESCE(se.display_name, ''),
       COALESCE(se.project_path, ''),
       snippet(messages_fts, 0, '<mark>', '</mark>', ' … ', 12)
FROM messages_fts f
LEFT JOIN sessions se ON se.session_id = f.session_id
WHERE messages_fts MATCH ?
ORDER BY rank
LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("running fts query: %w", err)
	}
	defer rows.Close()
	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.SessionID, &h.DisplayName, &h.ProjectPath, &h.Snippet); err != nil {
			return nil, fmt.Errorf("scanning hit: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

func scanSessions(rows *sql.Rows) ([]Session, error) {
	var out []Session
	for rows.Next() {
		var se Session
		if err := rows.Scan(&se.SessionID, &se.FilePath, &se.ProjectPath, &se.ProjectDir, &se.DisplayName,
			&se.CustomTitle, &se.AITitle, &se.FirstPrompt, &se.MessageCount, &se.FileSize,
			&se.LastActivity, &se.FirstParentUUID, &se.FileMtime, &se.IndexedAt); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		out = append(out, se)
	}
	return out, rows.Err()
}
