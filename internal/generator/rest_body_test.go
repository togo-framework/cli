package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

// REST bodies follow the GraphQL input rules: required means "key present"
// (Huma's default for a non-pointer field, so 0/false/"" are accepted) and a
// nullable field may be omitted. validate:"required" rejected zero values.
func TestRestHandlerBodyTags(t *testing.T) {
	fields, err := ParseFields([]string{"name:string", "views:int", "active:bool", "score:float?", "owner_id:uuid?"})
	if err != nil {
		t.Fatal(err)
	}
	r := config.Resource{Name: "Gadget", Table: "gadgets", Fields: fields}
	out, err := render("", FileOp{Path: "internal/rest/gadget_handler.go", Template: "resource/rest_handler.go.tmpl", Data: ResourceData{Module: "example.com/app", Resource: r}})
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	if strings.Contains(src, `validate:"required"`) {
		t.Errorf(`handler still uses validate:"required", which rejects 0/false:%s`, src)
	}
	for _, f := range []string{`Name\s+string`, `Views\s+int64`, `Active\s+bool`} {
		re := regexp.MustCompile(f + `\s+` + "`json:\"[a-z_]+\"`")
		if n := len(re.FindAllString(src, -1)); n != 2 { // create + update bodies
			t.Errorf("want %q as a required (key present) body field twice, found %d", f, n)
		}
	}
	for _, f := range []string{`Score\s+\*float64`, `OwnerID\s+\*string`} {
		re := regexp.MustCompile(f + `\s+` + "`json:\"[a-z_]+\" required:\"false\"`")
		if n := len(re.FindAllString(src, -1)); n != 2 {
			t.Errorf("want nullable %q to be optional in both bodies, found %d", f, n)
		}
	}
	for _, want := range []string{
		"errors.Is(err, orm.ErrInvalidInput)",
		`huma.Error422UnprocessableEntity("invalid field value", err)`,
		`huma.Error404NotFound("gadget not found")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("handler is missing %q", want)
		}
	}
}

// An id the database can't parse is "no row" in GraphQL too, and a bad body
// value is a BAD_USER_INPUT error rather than an internal one.
func TestResolverMapsInvalidInput(t *testing.T) {
	r := config.Resource{Name: "Gadget", Table: "gadgets"}
	out, err := render("", FileOp{Path: "internal/graph/resolvers/gadget.resolver.go", Template: "resource/resolver.go.tmpl", Data: ResourceData{Module: "example.com/app", Resource: r}})
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	if strings.Count(src, "errors.Is(err, orm.ErrInvalidInput)") != 3 {
		t.Errorf("want query, create and delete to handle orm.ErrInvalidInput:\n%s", src)
	}
	if !strings.Contains(src, `"BAD_USER_INPUT"`) {
		t.Error("create should report a bad value as BAD_USER_INPUT")
	}
}
