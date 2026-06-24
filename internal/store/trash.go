package store

import (
	"database/sql"
	"fmt"
)

// TrashEntry is one row of the trash manifest.
type TrashEntry struct {
	ID           int64
	Kind         string // "session" | "memory-backup" | "memory-delete"
	OriginalPath string
	TrashPath    string
	SessionID    string
	DeletedAt    string // ISO-8601
	PurgeAfter   string // ISO-8601 (deleted_at + 30 days)
	Note         string
}

// InsertTrash records a trashed file and returns its new id.
func (s *Store) InsertTrash(e TrashEntry) (int64, error) {
	res, err := s.db.Exec(`
INSERT INTO trash (kind, original_path, trash_path, session_id, deleted_at, purge_after, note)
VALUES (?,?,?,?,?,?,?)`,
		e.Kind, e.OriginalPath, e.TrashPath, e.SessionID, e.DeletedAt, e.PurgeAfter, e.Note)
	if err != nil {
		return 0, fmt.Errorf("inserting trash entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("trash last insert id: %w", err)
	}
	return id, nil
}

// GetTrash returns a single trash entry by id.
func (s *Store) GetTrash(id int64) (TrashEntry, error) {
	row := s.db.QueryRow(`
SELECT id, kind, original_path, trash_path, session_id, deleted_at, purge_after, note
FROM trash WHERE id = ?`, id)
	return scanTrash(row)
}

// ListTrash returns all trash entries, newest first.
func (s *Store) ListTrash() ([]TrashEntry, error) {
	rows, err := s.db.Query(`
SELECT id, kind, original_path, trash_path, session_id, deleted_at, purge_after, note
FROM trash ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing trash: %w", err)
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		e, err := scanTrashRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExpiredTrash returns entries whose purge_after is on or before nowISO.
func (s *Store) ExpiredTrash(nowISO string) ([]TrashEntry, error) {
	rows, err := s.db.Query(`
SELECT id, kind, original_path, trash_path, session_id, deleted_at, purge_after, note
FROM trash WHERE purge_after <= ? ORDER BY purge_after`, nowISO)
	if err != nil {
		return nil, fmt.Errorf("querying expired trash: %w", err)
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		e, err := scanTrashRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteTrash removes a trash manifest row by id (used after restore or purge).
func (s *Store) DeleteTrash(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM trash WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting trash row %d: %w", id, err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrash(r rowScanner) (TrashEntry, error) {
	var e TrashEntry
	err := r.Scan(&e.ID, &e.Kind, &e.OriginalPath, &e.TrashPath, &e.SessionID, &e.DeletedAt, &e.PurgeAfter, &e.Note)
	if err != nil {
		if err == sql.ErrNoRows {
			return TrashEntry{}, err
		}
		return TrashEntry{}, fmt.Errorf("scanning trash entry: %w", err)
	}
	return e, nil
}

func scanTrashRows(rows *sql.Rows) (TrashEntry, error) {
	var e TrashEntry
	if err := rows.Scan(&e.ID, &e.Kind, &e.OriginalPath, &e.TrashPath, &e.SessionID, &e.DeletedAt, &e.PurgeAfter, &e.Note); err != nil {
		return TrashEntry{}, fmt.Errorf("scanning trash row: %w", err)
	}
	return e, nil
}
