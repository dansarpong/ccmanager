package memory

import (
	"path/filepath"
	"strings"
)

// EncodeProjectDir mirrors Claude Code's encoding of a working directory into a
// projects/<name> folder: each path separator becomes a dash. This is used
// ONLY to locate the auto-memory directory; the working directory itself always
// comes from the transcript's cwd, never from decoding a folder name.
//
// Verified against real folders on this machine, e.g.
//
//	/home/dansarpong/ARMS/arms-infrastructure-as-code
//	  -> -home-dansarpong-ARMS-arms-infrastructure-as-code
//	/home/dansarpong/Personal/others/claude-tests/uigen
//	  -> -home-dansarpong-Personal-others-claude-tests-uigen
//
// Existing dashes in path segments are left untouched. Whether Claude also
// remaps '.' or '_' could not be confirmed from on-disk data, so they are left
// as-is; if a project path contains them and the encoded dir is not found,
// auto-memory simply shows as absent (the resolved CLAUDE.md stack is
// unaffected).
func EncodeProjectDir(dir string) string {
	clean := filepath.ToSlash(filepath.Clean(dir))
	return strings.ReplaceAll(clean, "/", "-")
}
