package generator

import (
	"github.com/togo-framework/cli/internal/config"
)

// ResourceData is the view model for per-resource stubs.
type ResourceData struct {
	Module    string
	Resource  config.Resource
	NeedsTime bool
}

// RegistryData is the view model for regenerated aggregate registries.
type RegistryData struct {
	Module    string
	Resources []config.Resource
}

// target couples a stub key with the output path pattern for one artifact kind.
type target struct {
	key  string // stub key
	path func(snake, plural string) string
	mode OpMode
}

// perResourceTargets are the template-owned, CreateOnly fragments.
var perResourceTargets = []target{
	{"resource/model.go.tmpl", func(s, p string) string { return "internal/models/" + s + ".go" }, CreateOnly},
	{"resource/schema.sql.tmpl", func(s, p string) string { return "internal/db/schema/" + s + ".sql" }, CreateOnly},
	{"resource/queries.sql.tmpl", func(s, p string) string { return "internal/db/queries/" + s + ".sql" }, CreateOnly},
	{"resource/table.hcl.tmpl", func(s, p string) string { return "db/atlas/schema/" + s + ".hcl" }, CreateOnly},
	{"resource/type.graphqls.tmpl", func(s, p string) string { return "internal/graph/schema/" + s + ".graphqls" }, CreateOnly},
	// NOTE: GraphQL resolvers are owned by gqlgen (generated from the .graphqls
	// fragment on `togo generate`), so we intentionally do not emit a resolver here.
	{"resource/rest_handler.go.tmpl", func(s, p string) string { return "internal/rest/" + s + "_handler.go" }, CreateOnly},
	{"resource/resource.go.tmpl", func(s, p string) string { return "internal/resources/" + s + ".go" }, CreateOnly},
	{"resource/seeder.go.tmpl", func(s, p string) string { return "internal/db/seeders/" + s + "_seeder.go" }, CreateOnly},
	{"resource/apitype.ts.tmpl", func(s, p string) string { return "web/lib/api/" + s + ".ts" }, CreateOnly},
	{"resource/page.tsx.tmpl", func(s, p string) string { return "web/app/" + p + "/page.tsx" }, CreateOnly},
	{"resource/hook.ts.tmpl", func(s, p string) string { return "web/lib/hooks/use" + Pascal(p) + ".ts" }, CreateOnly},
}

// singleTargets maps a make:<kind> verb to one perResourceTargets entry.
var singleTargets = map[string]string{
	"make:model":     "resource/model.go.tmpl",
	"make:query":     "resource/queries.sql.tmpl",
	"make:migration": "resource/table.hcl.tmpl",
	"make:graphql":   "resource/type.graphqls.tmpl",
	"make:api":       "resource/rest_handler.go.tmpl",
	"make:seeder":    "resource/seeder.go.tmpl",
	"make:page":      "resource/page.tsx.tmpl",
}

// aggregateTargets are the regenerated, Overwrite registries.
var aggregateTargets = []target{
	{"registry/rest_registry.go.tmpl", func(s, p string) string { return "internal/rest/registry.gen.go" }, Overwrite},
}

// TableName returns the conventional table name (plural snake_case) for a model.
func TableName(name string) string { return inflector.Plural(Snake(name)) }

// ResourcePlan returns the full set of ops for `make:resource`: every per-resource
// fragment plus the regenerated aggregate registries.
func ResourcePlan(module string, r config.Resource, manifest *config.Manifest) []FileOp {
	snake, plural := Snake(r.Name), TableName(r.Name)
	data := ResourceData{Module: module, Resource: r, NeedsTime: NeedsTimeImport(r.Fields)}

	ops := make([]FileOp, 0, len(perResourceTargets)+len(aggregateTargets))
	for _, t := range perResourceTargets {
		ops = append(ops, FileOp{Path: t.path(snake, plural), Template: t.key, Data: data, Mode: t.mode})
	}
	ops = append(ops, aggregatePlan(module, manifest)...)
	return ops
}

// SinglePlan returns the op for a single make:<kind> verb against one resource.
func SinglePlan(verb, module string, r config.Resource) []FileOp {
	key := singleTargets[verb]
	for _, t := range perResourceTargets {
		if t.key == key {
			snake, plural := Snake(r.Name), TableName(r.Name)
			data := ResourceData{Module: module, Resource: r, NeedsTime: NeedsTimeImport(r.Fields)}
			return []FileOp{{Path: t.path(snake, plural), Template: t.key, Data: data, Mode: t.mode}}
		}
	}
	return nil
}

// aggregatePlan regenerates the registries from the whole manifest.
func aggregatePlan(module string, manifest *config.Manifest) []FileOp {
	data := RegistryData{Module: module, Resources: manifest.Resources}
	ops := make([]FileOp, 0, len(aggregateTargets))
	for _, t := range aggregateTargets {
		ops = append(ops, FileOp{Path: t.path("", ""), Template: t.key, Data: data, Mode: t.mode})
	}
	return ops
}
