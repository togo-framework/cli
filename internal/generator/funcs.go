package generator

import (
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/gertd/go-pluralize"
)

var inflector = pluralize.NewClient()

// FuncMap is the shared template helper set available to every stub.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"pascal":   Pascal,
		"camel":    Camel,
		"snake":    Snake,
		"kebab":    Kebab,
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"plural":   inflector.Plural,
		"singular": inflector.Singular,
		"title":    strings.Title, //nolint:staticcheck // adequate for ASCII identifiers
		"year":     func() int { return clockYear() },
		"inc":       func(i int) int { return i + 1 },
		"add":       func(a, b int) int { return a + b },
		"atlasType":  atlasType,
		"tsType":     tsType,
		"sample":     SampleValue,
		"sqliteType": SQLiteType,
		"fakerFor":   FakerFor,
	}
}

// tsType maps a Go type to its TypeScript equivalent for the frontend client.
func tsType(goType string) string {
	switch goType {
	case "int", "int32", "int64", "float64":
		return "number"
	case "bool":
		return "boolean"
	default: // string, time.Time, uuid, etc.
		return "string"
	}
}

// atlasType maps a Postgres type to an Atlas HCL type expression. Known scalar
// types render as bare tokens; anything else falls back to sql("…").
func atlasType(pg string) string {
	switch pg {
	case "text", "uuid", "boolean", "integer", "bigint", "numeric", "date", "jsonb", "timestamptz":
		return pg
	default:
		return `sql("` + pg + `")`
	}
}

// clockYear is overridable in tests; Date.Now is unavailable in some sandboxes
// but the CLI runs in a real shell so time.Now is fine here.
var clockYear = func() int { return time.Now().Year() }

// words splits an identifier on camelCase, snake_case, kebab-case and spaces.
func words(s string) []string {
	var out []string
	var cur strings.Builder
	var prev rune
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i, r := range s {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			flush()
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(prev) || unicode.IsDigit(prev)):
			flush()
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
		prev = r
	}
	flush()
	return out
}

// Pascal renders an identifier as PascalCase (e.g. "blog post" -> "BlogPost").
func Pascal(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		b.WriteString(capitalize(w))
	}
	return b.String()
}

// Camel renders an identifier as camelCase.
func Camel(s string) string {
	p := Pascal(s)
	if p == "" {
		return p
	}
	r := []rune(p)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// Snake renders an identifier as snake_case.
func Snake(s string) string {
	parts := words(s)
	for i, w := range parts {
		parts[i] = strings.ToLower(w)
	}
	return strings.Join(parts, "_")
}

// Kebab renders an identifier as kebab-case.
func Kebab(s string) string {
	return strings.ReplaceAll(Snake(s), "_", "-")
}

func capitalize(w string) string {
	if w == "" {
		return w
	}
	// Preserve common all-caps initialisms.
	switch strings.ToUpper(w) {
	case "ID", "URL", "API", "HTTP", "JSON", "SQL", "UUID":
		return strings.ToUpper(w)
	}
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])
	for i := 1; i < len(r); i++ {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}
