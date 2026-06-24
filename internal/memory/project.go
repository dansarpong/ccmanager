package memory

import (
	"path/filepath"
	"strings"
)

// findProjectRoot returns the directory Claude Code treats as the project root:
// the nearest ancestor containing a .git entry, walking up from workdir. If no
// .git is found, the working directory itself is the root (matching Claude
// Code's "outside a git repo, the project root is used" behaviour).
func findProjectRoot(workdir string) string {
	cur := workdir
	for {
		if fileOrDirExists(filepath.Join(cur, ".git")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return workdir
		}
		cur = parent
	}
}

// isExternal reports whether target lies outside the project subtree rooted at
// projectRoot. Such imports are approval-gated by Claude Code.
func isExternal(projectRoot, target string) bool {
	rel, err := filepath.Rel(projectRoot, target)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// matchesAny reports whether absPath matches any of the glob patterns. Patterns
// are matched against the absolute path with filepath.Match per path segment;
// a "**" segment matches across directory boundaries.
func matchesAny(absPath string, patterns []string) bool {
	for _, p := range patterns {
		if globMatch(p, absPath) {
			return true
		}
	}
	return false
}

// globMatch supports "**" (any number of path segments) in addition to the
// single-segment wildcards filepath.Match handles. It is used for both
// claudeMdExcludes and rule paths: matching.
func globMatch(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Collapse consecutive ** and match the remainder against any suffix.
			rest := pat[1:]
			if len(rest) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if matchSegments(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		ok, err := filepath.Match(pat[0], name[0])
		if err != nil || !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}
