package generator

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

func TestSQLDialect(t *testing.T) {
	for driver, want := range map[string]string{
		"pgx": DialectPostgres, "postgres": DialectPostgres, "PostgreSQL": DialectPostgres, "supabase": DialectPostgres,
		"mysql": DialectMySQL, "mariadb": DialectMySQL,
		"sqlite": DialectSQLite, "sqlite3": DialectSQLite, "": DialectSQLite, "mongodb": DialectSQLite,
	} {
		if got := SQLDialect(driver); got != want {
			t.Errorf("SQLDialect(%q) = %q, want %q", driver, got, want)
		}
	}
}

// goldenResources are rendered for every dialect: the README's Post, and one
// field of every type (nullable and not) so each type mapping is pinned.
func goldenResources(t *testing.T) []config.Resource {
	t.Helper()
	post, err := ParseFields([]string{"title:string", "body:text"})
	if err != nil {
		t.Fatal(err)
	}
	all, err := ParseFields([]string{
		"name:string", "bio:text?", "views:int", "stock:int32?", "active:bool", "score:float?",
		"price:decimal", "owner_id:uuid?", "published_at:time?", "birthday:date", "meta:json?",
	})
	if err != nil {
		t.Fatal(err)
	}
	return []config.Resource{
		{Name: "Post", Table: "posts", Fields: post},
		{Name: "Gadget", Table: "gadgets", Fields: all},
		{Name: "Tag", Table: "tags"},
	}
}

// TestSchemaAndQueriesGolden pins the DDL and sqlc queries that make:resource
// writes for each database driver. Regenerate with: go test ./internal/generator -run Golden -update
func TestSchemaAndQueriesGolden(t *testing.T) {
	for _, driver := range []string{"sqlite", "pgx", "mysql"} {
		dialect := SQLDialect(driver)
		for _, r := range goldenResources(t) {
			for _, op := range ModelPlan("example.com/app", driver, r, &config.Manifest{}) {
				var kind string
				switch op.Template {
				case "resource/schema.sql.tmpl":
					kind = "schema"
				case "resource/queries.sql.tmpl":
					kind = "queries"
				default:
					continue
				}
				got, err := render("", op)
				if err != nil {
					t.Fatalf("%s %s: %v", dialect, op.Template, err)
				}
				golden := filepath.Join("testdata", "golden", dialect, Snake(r.Name)+"."+kind+".sql")
				if *update {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, got, 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("missing golden (run with -update): %v", err)
				}
				want = bytes.ReplaceAll(want, []byte("\r"), nil) // CRLF checkouts
				if !bytes.Equal(got, want) {
					t.Errorf("%s does not match:\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
				}
			}
		}
	}
}

// The Postgres DDL must not carry any SQLite-only syntax (the original bug).
func TestPostgresSchemaHasNoSQLiteSyntax(t *testing.T) {
	for _, r := range goldenResources(t) {
		out, err := render("", FileOp{Template: "resource/schema.sql.tmpl", Data: ResourceData{Dialect: DialectPostgres, Resource: r}})
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"randomblob", "datetime", "hex(", " real", "?"} {
			if bytes.Contains(out, []byte(bad)) {
				t.Errorf("postgres schema for %s contains %q:\n%s", r.Name, bad, out)
			}
		}
	}
}
