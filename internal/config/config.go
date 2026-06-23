// Package config resolves where Claude Code keeps its data on this machine
// and where ccmanager keeps its own state.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the resolved filesystem locations and the server port.
type Config struct {
	ConfigRoot  string // CLAUDE_CONFIG_DIR or ~/.claude
	ProjectsDir string // ConfigRoot/projects — where transcripts and auto-memory live
	DataDir     string // ConfigRoot/ccmanager — this app's own state
	TrashDir    string // DataDir/trash
	DBPath      string // DataDir/index.db
	Port        int
}

// Resolve determines all paths. It does not create any directories.
func Resolve(port int) (Config, error) {
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("locating home dir: %w", err)
		}
		root = filepath.Join(home, ".claude")
	}

	data := filepath.Join(root, "ccmanager")
	cfg := Config{
		ConfigRoot:  root,
		ProjectsDir: filepath.Join(root, "projects"),
		DataDir:     data,
		TrashDir:    filepath.Join(data, "trash"),
		DBPath:      filepath.Join(data, "index.db"),
		Port:        port,
	}
	return cfg, nil
}

// EnsureDataDirs creates the app's own directories (DataDir and TrashDir).
func (c Config) EnsureDataDirs() error {
	if err := os.MkdirAll(c.TrashDir, 0o755); err != nil {
		return fmt.Errorf("creating data dirs: %w", err)
	}
	return nil
}
