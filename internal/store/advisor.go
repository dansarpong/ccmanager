package store

import (
	"database/sql"
	"fmt"
)

// GetAdvisorCache returns a cached LLM result for a given input hash and kind,
// or ok=false if there is no cached entry.
func (s *Store) GetAdvisorCache(inputHash, kind string) (resultJSON string, ok bool, err error) {
	row := s.db.QueryRow(`SELECT result_json FROM advisor_cache WHERE input_hash = ? AND kind = ? LIMIT 1`, inputHash, kind)
	switch scanErr := row.Scan(&resultJSON); scanErr {
	case nil:
		return resultJSON, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("reading advisor cache: %w", scanErr)
	}
}

// PutAdvisorCache stores an LLM result keyed by input hash and kind, replacing
// any prior entry for that key.
func (s *Store) PutAdvisorCache(scopeKey, kind, inputHash, resultJSON, createdAt string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin advisor cache write: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM advisor_cache WHERE input_hash = ? AND kind = ?`, inputHash, kind); err != nil {
		return fmt.Errorf("clearing old advisor cache: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO advisor_cache (scope_key, kind, input_hash, result_json, created_at)
VALUES (?,?,?,?,?)`, scopeKey, kind, inputHash, resultJSON, createdAt); err != nil {
		return fmt.Errorf("writing advisor cache: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit advisor cache: %w", err)
	}
	return nil
}
