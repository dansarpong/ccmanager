package server

import (
	"net/http"
	"runtime"

	"ccmanager/internal/advisor"
	"ccmanager/internal/memory"
)

// memoryInput builds the pure engine's Input from the app config. The managed
// policy path defaults to the Linux location and is simply absent on other
// platforms or when the file does not exist.
func (s *Server) memoryInput() memory.Input {
	managed := ""
	if runtime.GOOS == "linux" {
		managed = "/etc/claude-code/CLAUDE.md"
	}
	return memory.Input{
		ConfigRoot:  s.cfg.ConfigRoot,
		ProjectsDir: s.cfg.ProjectsDir,
		ManagedPath: managed,
	}
}

// resolveDir returns the working directory to resolve memory for: the ?dir=
// query if present, otherwise the most-recently-active session's project path.
func (s *Server) resolveDir(r *http.Request) string {
	if dir := r.URL.Query().Get("dir"); dir != "" {
		return dir
	}
	id, err := s.store.MostRecentlyActiveID()
	if err != nil || id == "" {
		return ""
	}
	se, err := s.store.GetSession(id)
	if err != nil {
		return ""
	}
	return se.ProjectPath
}

func (s *Server) handleMemoryResolve(w http.ResponseWriter, r *http.Request) {
	dir := s.resolveDir(r)
	projects, err := s.store.DistinctProjects()
	if err != nil {
		s.fail(w, "listing projects", err)
		return
	}

	data := map[string]any{
		"Title":    "Memory — ccmanager",
		"Active":   "memory",
		"Dir":      dir,
		"Projects": projects,
	}

	if dir != "" {
		stack, err := memory.Resolve(s.memoryInput(), dir)
		if err != nil {
			s.fail(w, "resolving memory", err)
			return
		}
		data["Stack"] = stack
		data["Findings"] = advisor.Lint(stack)
	}
	s.render(w, "memory_resolve.html", data)
}

func (s *Server) handleMemoryLint(w http.ResponseWriter, r *http.Request) {
	dir := s.resolveDir(r)
	if dir == "" {
		s.render(w, "_lint.html", map[string]any{"Findings": nil})
		return
	}
	stack, err := memory.Resolve(s.memoryInput(), dir)
	if err != nil {
		s.fail(w, "resolving memory", err)
		return
	}
	s.render(w, "_lint.html", map[string]any{"Findings": advisor.Lint(stack)})
}

func (s *Server) handleMemoryInventory(w http.ResponseWriter, r *http.Request) {
	items, err := scanInventory(s.cfg.ConfigRoot, s.cfg.ProjectsDir)
	if err != nil {
		s.fail(w, "scanning memory inventory", err)
		return
	}
	data := map[string]any{
		"Title":  "Memory inventory — ccmanager",
		"Active": "memory",
		"Items":  items,
	}
	s.render(w, "memory_inventory.html", data)
}
