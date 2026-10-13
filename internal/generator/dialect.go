package generator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/togo-framework/cli/internal/config"
)

// SQL dialects the resource stubs can emit. The names follow sqlc's engines
// loosely; the database/sql driver names that map onto them are in SQLDialect.
const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
)

// SQLDialect maps a togo.yaml database.driver (the database/sql driver name, or
// the stack id) to the SQL dialect the schema and query stubs are written in.
// Unknown or empty drivers fall back to SQLite, the `togo new` default.
func SQLDialect(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "pgx", "postgres", "postgresql", "pg", "supabase", "togo-postgres":
		return DialectPostgres
	case "mysql", "mariadb":
		return DialectMySQL
	default:
		return DialectSQLite
	}
}

// ColumnType maps a field to its column type in the given dialect.
func ColumnType(dialect string, f config.Field) string {
	switch dialect {
	case DialectPostgres:
		return f.PG
	case DialectMySQL:
		return MySQLType(f)
	default:
		return SQLiteType(f)
	}
}

// MySQLType maps a field to its MySQL column type. Postgres types are the
// source of truth in the manifest, so the mapping goes from f.PG.
func MySQLType(f config.Field) string {
	switch f.PG {
	case "bigint":
		return "bigint"
	case "integer":
		return "int"
	case "boolean":
		return "boolean"
	case "double precision":
		return "double"
	case "numeric":
		return "decimal(20,6)"
	case "uuid":
		return "char(36)"
	case "timestamptz":
		return "datetime(6)"
	case "date":
		return "date"
	case "jsonb":
		return "json"
	default:
		return "text"
	}
}

// AtlasSchemaFile declares the Atlas schema every table.hcl fragment belongs to.
const AtlasSchemaFile = "db/atlas/schema/schema.hcl"

var atlasSchemaRe = regexp.MustCompile(`(?m)^\s*schema\s+"([^"]+)"`)

// AtlasSchema is the schema table.hcl fragments reference. Atlas needs the HCL
// schema to be the one the URL connects to: "public" on Postgres, "main" on
// SQLite and the database name on MySQL. The project's schema.hcl is the source
// of truth, so a renamed schema there carries over to new tables; without one,
// it falls back to the dialect's default (the app directory name for MySQL,
// which `togo new` uses as the database name).
func AtlasSchema(root, dialect string) string {
	if root != "" {
		if b, err := os.ReadFile(filepath.Join(root, AtlasSchemaFile)); err == nil {
			if m := atlasSchemaRe.FindSubmatch(b); m != nil {
				return string(m[1])
			}
		}
	}
	return DefaultAtlasSchema(root, dialect)
}

// DefaultAtlasSchema is the schema Atlas connects to for a fresh project.
func DefaultAtlasSchema(root, dialect string) string {
	switch dialect {
	case DialectPostgres:
		return "public"
	case DialectMySQL:
		if abs, err := filepath.Abs(root); err == nil && root != "" {
			return filepath.Base(abs)
		}
		return "app"
	default:
		return "main"
	}
}

// AtlasColumnType renders a field's column type as an Atlas HCL expression for
// the dialect. Bare tokens are used where Atlas has one; parameterised types
// (char(36), decimal(20,6), datetime(6)) go through sql("…").
func AtlasColumnType(dialect string, f config.Field) string {
	switch dialect {
	case DialectPostgres:
		return atlasType(f.PG)
	case DialectMySQL:
		t := MySQLType(f)
		if strings.Contains(t, "(") {
			return `sql("` + t + `")`
		}
		return t
	default:
		return SQLiteType(f)
	}
}
