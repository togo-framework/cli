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

// TestSchemaAndQueriesGolden pins the DDL, sqlc queries and Atlas HCL that
// make:resource writes for each database driver. Regenerate with: go test ./internal/generator -run Golden -update
func TestSchemaAndQueriesGolden(t *testing.T) {
	for _, driver := range []string{"sqlite", "pgx", "mysql"} {
		dialect := SQLDialect(driver)
		for _, r := range goldenResources(t) {
			ops := append(ModelPlan("example.com/app", driver, r, &config.Manifest{}), SinglePlan("make:migration", "example.com/app", driver, r)...)
			for _, op := range ops {
				var name string
				switch op.Template {
				case "resource/schema.sql.tmpl":
					name = "schema.sql"
				case "resource/queries.sql.tmpl":
					name = "queries.sql"
				case "resource/table.hcl.tmpl":
					name = "table.hcl"
				default:
					continue
				}
				got, err := render("", op)
				if err != nil {
					t.Fatalf("%s %s: %v", dialect, op.Template, err)
				}
				got = bytes.ReplaceAll(got, []byte("\r"), nil) // CRLF checkouts of the stubs
				golden := filepath.Join("testdata", "golden", dialect, Snake(r.Name)+"."+name)
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

// Atlas only applies an HCL schema named like the one the URL connects to, so
// table.hcl must not hard-code Postgres' "public" for the other drivers.
func TestAtlasTableHasNoPostgresSyntax(t *testing.T) {
	for _, dialect := range []string{DialectSQLite, DialectMySQL} {
		for _, r := range goldenResources(t) {
			out, err := render("", FileOp{Template: "resource/table.hcl.tmpl", Data: ResourceData{Dialect: dialect, Resource: r}})
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"schema.public", "= uuid", "timestamptz", "jsonb", "gen_random_uuid", "now()"} {
				if bytes.Contains(out, []byte(bad)) {
					t.Errorf("%s table.hcl for %s contains %q:\n%s", dialect, r.Name, bad, out)
				}
			}
		}
	}
}

func TestAtlasSchema(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	for dialect, want := range map[string]string{DialectPostgres: "public", DialectSQLite: "main", DialectMySQL: "shop"} {
		if got := AtlasSchema(root, dialect); got != want {
			t.Errorf("AtlasSchema(no schema.hcl, %s) = %q, want %q", dialect, got, want)
		}
	}
	// The project's schema.hcl wins, so a renamed schema carries over to new tables.
	file := filepath.Join(root, AtlasSchemaFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("# the app database\nschema \"shop_prod\" {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AtlasSchema(root, DialectMySQL); got != "shop_prod" {
		t.Errorf("AtlasSchema(schema.hcl) = %q, want shop_prod", got)
	}
	out, err := render(root, FileOp{Template: "resource/table.hcl.tmpl", Data: ResourceData{Dialect: DialectMySQL, Resource: config.Resource{Name: "Post", Table: "posts"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("schema = schema.shop_prod")) {
		t.Errorf("table.hcl does not reference the project's schema:\n%s", out)
	}
}
