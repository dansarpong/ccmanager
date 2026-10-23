package server

import (
	"database/sql"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dansarpong/ccmanager/internal/sessions"
	"github.com/dansarpong/ccmanager/internal/store"
)

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sort := r.URL.Query().Get("sort")
	project := r.URL.Query().Get("project")
	status := r.URL.Query().Get("status")

	list, err := s.sessionList(sort, project, status)
	if err != nil {
		s.fail(w, "listing sessions", err)
		return
	}
	data := map[string]any{
		"Title":    "Sessions — ccmanager",
		"Sessions": list,
		"Sort":     sort,
		"Project":  project,
		"Status":   status,
		"Filtered": project != "" || status != "",
		"Active":   "sessions",
	}
	s.page(w, r, "sessions.html", data)
}

// sessionList lists sessions with their derived status, optionally keeping
// only one status.
func (s *Server) sessionList(sort, project, status string) ([]sessionView, error) {
	list, err := s.store.ListSessions(sort, project)
	if err != nil {
		return nil, err
	}
	views := sessionViews(list, time.Now())
	if status == "" {
		return views, nil
	}
	kept := views[:0]
	for _, v := range views {
		if v.Status == status {
			kept = append(kept, v)
		}
	}
	return kept, nil
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	se, err := s.store.GetSession(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.fail(w, "getting session", err)
		return
	}

	chain := sessions.ResumeChain{}
	parentID, err := s.store.ParentSessionID(se.SessionID, se.FirstParentUUID)
	if err != nil {
		s.fail(w, "finding parent session", err)
		return
	}
	if parentID != "" {
		if parent, err := s.store.GetSession(parentID); err == nil {
			chain.Parent = &parent
		}
	}
	children, err := s.store.ChildSessions(se.SessionID)
	if err != nil {
		s.fail(w, "finding child sessions", err)
		return
	}
	chain.Children = children

	page, err := sessions.ReadTranscript(se.FilePath, 0, 50)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// The transcript was deleted on disk after it was indexed. Prune
			// the stale row so it stops showing in the list, then 404.
			_ = s.store.DeleteSessionRows(se.SessionID)
			http.NotFound(w, r)
			return
		}
		s.fail(w, "reading transcript", err)
		return
	}

	data := map[string]any{
		"Title":   se.DisplayName + " — ccmanager",
		"Session": newSessionView(se, time.Now()),
		"Chain":   chain,
		"Page":    page,
		"Active":  "sessions",
	}
	s.page(w, r, "session_detail.html", data)
}

func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	se, err := s.store.GetSession(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.fail(w, "getting session", err)
		return
	}
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	page, err := sessions.ReadTranscript(se.FilePath, offset, 50)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_ = s.store.DeleteSessionRows(se.SessionID)
			http.NotFound(w, r)
			return
		}
		s.fail(w, "reading transcript", err)
		return
	}
	data := map[string]any{"Session": se, "Page": page}
	s.render(w, "_transcript.html", data)
}

// searchHit pairs a raw hit with a snippet whose <mark> highlights are kept as
// HTML while the surrounding transcript text is escaped.
type searchHit struct {
	store.SearchHit
	SnippetHTML template.HTML
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var hits []searchHit
	var searchErr string
	if q != "" {
		raw, err := s.store.SearchFTS(ftsQuery(q), 100)
		if err != nil {
			// FTS5 syntax errors are user-facing, not server failures.
			searchErr = err.Error()
		}
		for _, h := range raw {
			hits = append(hits, searchHit{SearchHit: h, SnippetHTML: highlightSnippet(h.Snippet)})
		}
	}
	data := map[string]any{"Query": q, "Hits": hits, "Error": searchErr}
	s.render(w, "_search_results.html", data)
}

// highlightSnippet escapes the FTS5 snippet text, then unescapes only the
// <mark> markers we asked snippet() to insert, so transcript content cannot
// inject HTML but the highlights still render.
func highlightSnippet(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	escaped = strings.ReplaceAll(escaped, "&lt;mark&gt;", "<mark>")
	escaped = strings.ReplaceAll(escaped, "&lt;/mark&gt;", "</mark>")
	return template.HTML(escaped)
}

// handleRefresh re-indexes and returns the session list. The current filters
// arrive in the POST body (hx-include), so they are read with FormValue.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if _, err := s.reindex(); err != nil {
		s.fail(w, "refreshing index", err)
		return
	}
	project, status := r.FormValue("project"), r.FormValue("status")
	list, err := s.sessionList(r.FormValue("sort"), project, status)
	if err != nil {
		s.fail(w, "listing sessions", err)
		return
	}
	s.render(w, "session-list", map[string]any{
		"Sessions": list,
		"Filtered": project != "" || status != "",
	})
}

// handlePalette returns only the palette's grouped results for a query.
func (s *Server) handlePalette(w http.ResponseWriter, r *http.Request) {
	s.render(w, "palette-results", s.palette(strings.TrimSpace(r.URL.Query().Get("q"))))
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// ftsQuery turns a raw user query into a safe FTS5 MATCH expression by quoting
// each whitespace-separated term as a phrase. This avoids FTS5 syntax errors
// from characters like "-" or ":" in the user's text while still matching all
// terms (implicit AND).
func ftsQuery(raw string) string {
	fields := strings.Fields(raw)
	for i, f := range fields {
		fields[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
	}
	return strings.Join(fields, " ")
}
