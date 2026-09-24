package generator

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

func customerResource() config.Resource {
	fields, _ := ParseFields([]string{"phone:string", "name:string", "default_address:text:nullable"})
	return config.Resource{Name: "Customer", Table: "customers", Fields: fields, Controller: true}
}

// Regression for MH-476: the TS API type must use the same (snake_case) JSON
// names as the Go transformer's json tags, not camelCase.
func TestAPITypeMatchesGoJSONTags(t *testing.T) {
	r := customerResource()
	data := ResourceData{Module: "example.com/app", Resource: r}
	ts, err := render("", FileOp{Path: "web/lib/api/customer.ts", Template: "resource/apitype.ts.tmpl", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	ts = bytes.ReplaceAll(ts, []byte("\r"), nil) // CRLF checkouts
	goSrc, err := render("", FileOp{Path: "internal/resources/customer.go", Template: "resource/transform.go.tmpl", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ts), "  default_address?: string;\n") {
		t.Errorf("TS type should use default_address:\n%s", ts)
	}
	if strings.Contains(string(ts), "defaultAddress") {
		t.Errorf("TS type must not camelCase fields:\n%s", ts)
	}
	for _, m := range regexp.MustCompile("json:\"([a-z_]+)\"").FindAllStringSubmatch(string(goSrc), -1) {
		if !regexp.MustCompile(`(?m)^  ` + m[1] + `\??: `).Match(ts) {
			t.Errorf("Go json tag %q has no matching TS property:\n%s", m[1], ts)
		}
	}
}

func TestSeederRegistrySkipsHandManagedResources(t *testing.T) {
	m, err := config.ParseManifest([]byte(`resources:
    - name: Order
      table: orders
      fields: []
      controller: true
    - name: DispatchOffer
      table: dispatch_offers
      fields: []
      controller: false
    - name: Draft
      table: drafts
      fields: []
`))
	if err != nil {
		t.Fatal(err)
	}
	seeders, err := render("", seederAggregate("example.com/app", m))
	if err != nil {
		t.Fatal(err)
	}
	rest, err := render("", restAggregate("example.com/app", m))
	if err != nil {
		t.Fatal(err)
	}
	s, r := string(seeders), string(rest)
	if strings.Contains(s, "SeedDispatchOffer") || !strings.Contains(s, "SeedOrder") || !strings.Contains(s, "SeedDraft") {
		t.Errorf("seeder registry:\n%s", s)
	}
	if strings.Contains(r, "RegisterDispatchOfferRoutes") || strings.Contains(r, "RegisterDraftRoutes") || !strings.Contains(r, "RegisterOrderRoutes") {
		t.Errorf("rest registry:\n%s", r)
	}
}
