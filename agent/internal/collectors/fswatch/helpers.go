package fswatch

import (
	"os"
	"path/filepath"
	"regexp"
)

var winVar = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// expandPath expands Windows-style %VAR% and Unix-style $VAR references, then
// cleans the path. Unknown variables expand to empty.
func expandPath(p string) string {
	if p == "" {
		return ""
	}
	p = winVar.ReplaceAllStringFunc(p, func(m string) string {
		return os.Getenv(m[1 : len(m)-1])
	})
	p = os.ExpandEnv(p)
	return filepath.Clean(p)
}

func parentDir(p string) string { return filepath.Dir(p) }
