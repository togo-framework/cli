package generator

import (
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
