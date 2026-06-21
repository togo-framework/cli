// Package scaffold renders a new togo project from the create-togo-app template.
package scaffold

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	createtogoapp "github.com/togo-framework/create-togo-app/template"
	"github.com/togo-framework/cli/internal/generator"
)

// Options configures project scaffolding.
type Options struct {
	App    string
	Module string
	Dir    string
	Force  bool
	DryRun bool
}

// Resolved is Options with defaults applied.
type Resolved struct {
	Options
}

// Resolve fills in default module path and target directory.
func (o Options) Resolve() Resolved {
	r := Resolved{Options: o}
	if r.Dir == "" {
		r.Dir = r.App
	}
	if r.Module == "" {
		r.Module = "github.com/" + r.App + "/" + r.App
	}
	return r
}

// data is the template view model for project files.
type data struct {
	App       string
	AppPascal string
	Module    string
}

// New renders the project template into the target directory and returns the
// number of files written (or that would be written in dry-run).
func New(opts Options) (int, error) {
	r := opts.Resolve()
	d := data{App: r.App, AppPascal: generator.Pascal(r.App), Module: r.Module}
	src := createtogoapp.FS()

	count := 0
	err := fs.WalkDir(src, createtogoapp.Root, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(createtogoapp.Root, p)
		if err != nil {
			return err
		}
		rel = strings.TrimSuffix(rel, ".tmpl")
		dest := filepath.Join(r.Dir, rel)

		if !r.Force && fileExists(dest) {
			return nil
		}

		raw, err := src.ReadFile(p)
		if err != nil {
			return err
		}
		out, err := renderProjectFile(p, raw, d)
		if err != nil {
			return err
		}
		count++
		if r.DryRun {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, out, 0o644)
	})
	return count, err
}

func renderProjectFile(path string, raw []byte, d data) ([]byte, error) {
	if !strings.HasSuffix(path, ".tmpl") {
		return raw, nil // copy verbatim (binary/asset files)
	}
	tmpl, err := template.New(filepath.Base(path)).Funcs(generator.FuncMap()).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
