package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fixture is a real-world manifest (wasla) with enums, relations, explicit
// `controller: false` resources and a non-alphabetical resource order.
func fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "wasla.resources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func names(rs []Resource) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func TestManifestRoundTripIsLossless(t *testing.T) {
	src := fixture(t)
	m, err := ParseManifest(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("unchanged manifest was rewritten:\n%s", out)
	}
}

// Regression for MH-475: make:resource must not drop enum/relation/controller:false
// keys or reorder resources when it rewrites the manifest.
func TestManifestUpsertPreservesKeysAndOrder(t *testing.T) {
	src := fixture(t)
	before, err := ParseManifest(src)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := names(before.Resources)

	m, _ := ParseManifest(src)
	// Same shape make:resource builds: `Customer phone:string name:string default_address:text:nullable --force`.
	customer := Resource{Name: "Customer", Table: "customers", Controller: true, Fields: []Field{
		{Name: "phone", Go: "string", GQL: "String!", PG: "text"},
		{Name: "name", Go: "string", GQL: "String!", PG: "text"},
		{Name: "default_address", Go: "string", GQL: "String", PG: "text", Null: true},
	}}
	if existed, err := m.Upsert(customer, true); err != nil || !existed {
		t.Fatalf("Upsert(Customer) existed=%v err=%v", existed, err)
	}
	if _, err := m.Upsert(Resource{Name: "Coupon", Table: "coupons", Controller: true,
		Fields: []Field{{Name: "code", Go: "string", GQL: "String!", PG: "text"}}}, false); err != nil {
		t.Fatal(err)
	}
	out, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseManifest(out)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := names(after.Resources), append(wantOrder, "Coupon"); !reflect.DeepEqual(got, want) {
		t.Fatalf("resource order changed:\n got %v\nwant %v", got, want)
	}
	for _, key := range []string{"enum:", "relation:", "controller: false"} {
		if got, want := strings.Count(string(out), key), strings.Count(string(src), key); got != want {
			t.Errorf("%q occurrences: got %d, want %d", key, got, want)
		}
	}
	for _, b := range before.Resources {
		if b.Name == "Customer" {
			continue
		}
		a := after.Find(b.Name)
		if !reflect.DeepEqual(a.Fields, b.Fields) || a.Controller != b.Controller || a.HasSeeder() != b.HasSeeder() {
			t.Errorf("resource %s changed:\nbefore %+v\nafter  %+v", b.Name, b, *a)
		}
	}
	if d := after.Find("Driver").Fields; d[2].Enum == nil || d[3].Relation != "zones" {
		t.Errorf("Driver enum/relation lost: %+v", d)
	}
	if c := after.Find("Customer"); len(c.Fields) != 3 || c.Fields[2].Name != "default_address" || !c.Fields[2].Null {
		t.Errorf("Customer not replaced: %+v", c)
	}
	// Untouched resources render byte-identically: only Customer's notes field
	// was removed and Coupon appended.
	if !strings.Contains(string(out), "          enum: [available, offered, delivering, offline, suspended]\n") {
		t.Error("flow-style enum was not preserved")
	}
}

func TestManifestPreservesUnknownKeys(t *testing.T) {
	src := []byte("# Managed by togo. Source of truth for generated registries.\n" +
		"resources:\n" +
		"    - name: Post\n" +
		"      table: posts\n" +
		"      soft_deletes: true # custom\n" +
		"      fields:\n" +
		"        - name: title\n" +
		"          go: string\n" +
		"          gql: String!\n" +
		"          pg: text\n" +
		"          \"null\": false\n" +
		"          index: unique\n" +
		"      controller: false\n")
	m, err := ParseManifest(src)
	if err != nil {
		t.Fatal(err)
	}
	// make:model-style force replace keeps field-level and resource-level extras.
	if _, err := m.Upsert(Resource{Name: "Post", Table: "posts", Fields: []Field{
		{Name: "title", Go: "string", GQL: "String!", PG: "text"},
		{Name: "body", Go: "string", GQL: "String", PG: "text", Null: true},
	}}, true); err != nil {
		t.Fatal(err)
	}
	if m.Find("Post").HasSeeder() {
		t.Error("controller: false resource must stay out of the seeder registry")
	}
	out, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"soft_deletes: true # custom", "index: unique", "controller: false", "- name: body"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestManifestNewFile(t *testing.T) {
	dir := t.TempDir()
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Upsert(Resource{Name: "Post", Table: "posts", Controller: true}, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ManifestFile))
	if !strings.HasPrefix(string(data), "# Managed by togo.") || !strings.Contains(string(data), "- name: Post") {
		t.Fatalf("unexpected new manifest:\n%s", data)
	}
	back, err := LoadManifest(dir)
	if err != nil || back.Find("Post") == nil || !back.Find("Post").HasSeeder() {
		t.Fatalf("reload failed: %v %+v", err, back)
	}
}

func TestHasSeeder(t *testing.T) {
	m, err := ParseManifest(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"Customer": true, "Order": true, "Zone": true,
		"DispatchOffer": false, "LedgerAccount": false, "LedgerTransaction": false, "LedgerEntry": false,
	} {
		if got := m.Find(name).HasSeeder(); got != want {
			t.Errorf("%s.HasSeeder() = %v, want %v", name, got, want)
		}
	}
	f := false
	r := Resource{Name: "X", Controller: true, Seeder: &f}
	if r.HasSeeder() {
		t.Error("explicit seeder: false must win")
	}
}
