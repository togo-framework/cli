package generator

import (
	"fmt"
	"strings"

	"github.com/togo-framework/cli/internal/config"
)

// typeMap maps a togo field type token to its Go, GraphQL (base, no !), and
// Postgres representations.
var typeMap = map[string]struct {
	Go  string
	GQL string
	PG  string
}{
	"string":   {"string", "String", "text"},
	"text":     {"string", "String", "text"},
	"int":      {"int64", "Int", "bigint"},
	"int32":    {"int32", "Int", "integer"},
	"int64":    {"int64", "Int", "bigint"},
	"bool":     {"bool", "Boolean", "boolean"},
	"boolean":  {"bool", "Boolean", "boolean"},
	"float":    {"float64", "Float", "double precision"},
	"float64":  {"float64", "Float", "double precision"},
	"decimal":  {"string", "String", "numeric"},
	"uuid":     {"string", "ID", "uuid"},
	"time":     {"time.Time", "String", "timestamptz"},
	"datetime": {"time.Time", "String", "timestamptz"},
	"date":     {"time.Time", "String", "date"},
	"json":     {"string", "String", "jsonb"},
}

// ParseFields turns CLI args like "title:string body:text? count:int" into
// config.Field values. A trailing "?" marks the field nullable.
func ParseFields(args []string) ([]config.Field, error) {
	fields := make([]config.Field, 0, len(args))
	for _, a := range args {
		// Accept name:type, name:type? (quote in shells), or name:type:nullable.
		parts := strings.SplitN(a, ":", 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid field %q (expected name:type, e.g. title:string)", a)
		}
		name, typ := parts[0], parts[1]
		null := false
		if strings.HasSuffix(typ, "?") {
			null = true
			typ = strings.TrimSuffix(typ, "?")
		}
		if len(parts) == 3 {
			switch strings.ToLower(parts[2]) {
			case "nullable", "null", "?":
				null = true
			default:
				return nil, fmt.Errorf("unknown modifier %q for %q (use 'nullable')", parts[2], name)
			}
		}
		m, known := typeMap[strings.ToLower(typ)]
		if !known {
			return nil, fmt.Errorf("unknown field type %q for %q (try: string,text,int,bool,float,uuid,time,json)", typ, name)
		}
		gql := m.GQL
		if !null {
			gql += "!"
		}
		fields = append(fields, config.Field{
			Name: Snake(name),
			Go:   m.Go,
			GQL:  gql,
			PG:   m.PG,
			Null: null,
		})
	}
	return fields, nil
}

// NeedsTimeImport reports whether any field uses time.Time, so the model
// template can conditionally import the time package.
func NeedsTimeImport(fields []config.Field) bool {
	for _, f := range fields {
		if f.Go == "time.Time" {
			return true
		}
	}
	return false
}
