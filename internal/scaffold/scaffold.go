// Package scaffold renders a new togo project from the create-togo-app template.
package scaffold

import (
	"bytes"
	"fmt"
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
	App      string
	Module   string
	Dir      string
	Force    bool
	DryRun   bool
	Frontend string // "tanstack" (default) | "nextjs"
	DB       string // "sqlite" (default) | postgres | togo-postgres | supabase | mysql | mongodb
}

// Resolved is Options with defaults applied.
type Resolved struct {
	Options
}

// Resolve fills in default module path, target directory, and frontend.
func (o Options) Resolve() Resolved {
	r := Resolved{Options: o}
	if r.Dir == "" {
		r.Dir = r.App
	}
	if r.Module == "" {
		r.Module = "github.com/" + r.App + "/" + r.App
	}
	if r.Frontend == "" {
		r.Frontend = "tanstack"
	}
	if r.DB == "" {
		r.DB = "sqlite"
	}
	return r
}

// data is the template view model for project files.
type data struct {
	App       string
	AppPascal string
	Module    string
	DB        string // chosen stack id (sqlite | postgres | togo-postgres | supabase | mysql | mongodb)
	DBDriver  string // database/sql driver name (DB_DRIVER): sqlite | pgx | mysql | mongodb
	DBURL     string // DATABASE_URL for the chosen stack
}

// dbConfig maps a database stack to its DB_DRIVER name + DATABASE_URL.
func dbConfig(db, app string) (driver, url string) {
	switch db {
	case "postgres", "togo-postgres", "supabase":
		dbName := app
		if db == "supabase" {
			dbName = "postgres"
		}
		return "pgx", "postgres://postgres:postgres@localhost:5432/" + dbName + "?sslmode=disable"
	case "mysql":
		return "mysql", "root:root@tcp(localhost:3306)/" + app + "?parseTime=true"
	case "mongodb":
		return "mongodb", "mongodb://root:root@localhost:27017/" + app + "?authSource=admin"
	default: // sqlite
		return "sqlite", "file:./togo.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	}
}

// New renders the project template into the target directory and returns the
// number of files written (or that would be written in dry-run).
func New(opts Options) (int, error) {
	r := opts.Resolve()
	driver, url := dbConfig(r.DB, r.App)
	d := data{App: r.App, AppPascal: generator.Pascal(r.App), Module: r.Module, DB: r.DB, DBDriver: driver, DBURL: url}
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
	if err != nil {
		return count, err
	}

	// Render the chosen frontend (tanstack | nextjs) into the project's web/ dir.
	fcount, ferr := renderFrontend(r, d)
	return count + fcount, ferr
}

// renderFrontend renders template/frontend/<name>/ into <Dir>/web/.
func renderFrontend(r Resolved, d data) (int, error) {
	src := createtogoapp.FrontendFS()
	base := createtogoapp.FrontendRoot + "/" + r.Frontend
	if _, err := fs.Stat(src, base); err != nil {
		return 0, fmt.Errorf("unknown frontend %q", r.Frontend)
	}
	count := 0
	err := fs.WalkDir(src, base, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return err
		}
		rel = strings.TrimSuffix(rel, ".tmpl")
		dest := filepath.Join(r.Dir, "web", rel)
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
