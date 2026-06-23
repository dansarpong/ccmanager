package server

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticEmbed embed.FS

// staticFS is the static subtree served under /static/.
var staticFS = mustSub(staticEmbed, "static")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(fmt.Sprintf("subFS %s: %v", dir, err))
	}
	return sub
}

// render executes a named template into a buffer first, so a template error
// surfaces as a 500 instead of a half-written response.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template %s: %v", name, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (s *Server) fail(w http.ResponseWriter, msg string, err error) {
	http.Error(w, fmt.Sprintf("%s: %v", msg, err), http.StatusInternalServerError)
}
