// Package templates embeds the per-artifact generator stubs and resolves them,
// preferring per-project overrides published to ./.togo/stubs (artisan parity).
package templates

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:files
var embedded embed.FS

// StubDir is the project-local override directory checked before the embedded FS.
const StubDir = ".togo/stubs"

// Read returns the raw stub bytes for key (e.g. "resource/model.go.tmpl"),
// preferring root/.togo/stubs/<key> when present, otherwise the embedded copy.
func Read(root, key string) ([]byte, error) {
	if root != "" {
		override := filepath.Join(root, StubDir, key)
		if data, err := os.ReadFile(override); err == nil {
			return data, nil
		}
	}
	return embedded.ReadFile("files/" + key)
}

// Keys lists every embedded stub path (relative to files/), used by stub:publish.
func Keys() ([]string, error) {
	var keys []string
	err := fs.WalkDir(embedded, "files", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("files", p)
		if err != nil {
			return err
		}
		keys = append(keys, rel)
		return nil
	})
	return keys, err
}
