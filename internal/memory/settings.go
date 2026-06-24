package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// settings is the subset of a Claude Code settings file the engine reads.
type settings struct {
	ClaudeMdExcludes    []string `json:"claudeMdExcludes"`
	AutoMemoryDirectory string   `json:"autoMemoryDirectory"`
}

// loadExcludes reads claudeMdExcludes from the settings layers ccmanager
// supports and merges them (arrays merge across layers, per the docs). v1 reads
// the user-level settings and the project's local settings; it does not read
// managed-policy settings (documented in the README).
func loadExcludes(configRoot, workdir string) []string {
	var out []string
	for _, path := range settingsPaths(configRoot, workdir) {
		if s, ok := readSettings(path); ok {
			out = append(out, s.ClaudeMdExcludes...)
		}
	}
	return out
}

// autoMemoryDir returns an overridden auto-memory directory from settings, or
// "" if none is set. A leading ~/ is expanded against the user's home dir.
func autoMemoryDir(configRoot, workdir string) string {
	for _, path := range settingsPaths(configRoot, workdir) {
		if s, ok := readSettings(path); ok && s.AutoMemoryDirectory != "" {
			return expandHome(s.AutoMemoryDirectory)
		}
	}
	return ""
}

// settingsPaths lists the settings files to consult, broadest first so later
// (more specific) layers can extend the earlier ones.
func settingsPaths(configRoot, workdir string) []string {
	var paths []string
	if configRoot != "" {
		paths = append(paths, filepath.Join(configRoot, "settings.json"))
	}
	paths = append(paths,
		filepath.Join(workdir, ".claude", "settings.json"),
		filepath.Join(workdir, ".claude", "settings.local.json"),
	)
	return paths
}

func readSettings(path string) (settings, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return settings{}, false
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return settings{}, false
	}
	return s, true
}

func expandHome(p string) string {
	if p == "~" || len(p) >= 2 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
