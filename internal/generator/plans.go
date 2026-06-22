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

// modelTargets are the data layer (make:model): schema, queries, migration, seeder.
// Models themselves come from sqlc (db.<Name>).
var modelTargets = []target{
	{"resource/schema.sql.tmpl", func(s, p string) string { return "internal/db/schema/" + s + ".sql" }, CreateOnly},
	{"resource/queries.sql.tmpl", func(s, p string) string { return "internal/db/queries/" + s + ".sql" }, CreateOnly},
	{"resource/model_repo.go.tmpl", func(s, p string) string { return "internal/models/" + s + ".go" }, CreateOnly},
	{"resource/seeder.go.tmpl", func(s, p string) string { return "internal/db/seeders/" + s + "_seeder.go" }, CreateOnly},
}

// controllerTargets are the API layer (make:controller): GraphQL schema+resolver,
// REST handler, the customizable response transformer, and a test.
var controllerTargets = []target{
	{"resource/type.graphqls.tmpl", func(s, p string) string { return "internal/graph/schema/" + s + ".graphqls" }, CreateOnly},
	{"resource/resolver.go.tmpl", func(s, p string) string { return "internal/graph/resolvers/" + s + ".resolver.go" }, CreateOnly},
	{"resource/transform.go.tmpl", func(s, p string) string { return "internal/resources/" + s + ".go" }, CreateOnly},
	{"resource/rest_handler.go.tmpl", func(s, p string) string { return "internal/rest/" + s + "_handler.go" }, CreateOnly},
	{"resource/test.go.tmpl", func(s, p string) string { return "internal/rest/" + s + "_test.go" }, CreateOnly},
}

// viewTargets are the frontend layer (make:view): API type, page, data hook.
var viewTargets = []target{
	{"resource/apitype.ts.tmpl", func(s, p string) string { return "web/lib/api/" + s + ".ts" }, CreateOnly},
	{"resource/page.tsx.tmpl", func(s, p string) string { return "web/app/" + p + "/page.tsx" }, CreateOnly},
	{"resource/hook.ts.tmpl", func(s, p string) string { return "web/lib/hooks/use" + Pascal(p) + ".ts" }, CreateOnly},
}

// singleTargets maps a make:<kind> verb to one target across the groups.
var singleTargets = map[string]string{
	"make:query":     "resource/queries.sql.tmpl",
	"make:migration": "resource/table.hcl.tmpl",
	"make:graphql":   "resource/type.graphqls.tmpl",
	"make:api":       "resource/rest_handler.go.tmpl",
	"make:seeder":    "resource/seeder.go.tmpl",
	"make:page":      "resource/page.tsx.tmpl",
}

var allSingleTargets = append(append(append([]target{}, modelTargets...), controllerTargets...), viewTargets...)

// TableName returns the conventional table name (plural snake_case) for a model.
func TableName(name string) string { return inflector.Plural(Snake(name)) }

func planFor(targets []target, module string, r config.Resource) []FileOp {
	snake, plural := Snake(r.Name), TableName(r.Name)
	data := ResourceData{Module: module, Resource: r, NeedsTime: NeedsTimeImport(r.Fields)}
	ops := make([]FileOp, 0, len(targets))
	for _, t := range targets {
		ops = append(ops, FileOp{Path: t.path(snake, plural), Template: t.key, Data: data, Mode: t.mode})
	}
	return ops
}

// ModelPlan: data layer + regenerated seeder registry.
func ModelPlan(module string, r config.Resource, m *config.Manifest) []FileOp {
	return append(planFor(modelTargets, module, r), seederAggregate(module, m))
}

// ControllerPlan: API layer + regenerated REST registry (controller-filtered).
func ControllerPlan(module string, r config.Resource, m *config.Manifest) []FileOp {
	return append(planFor(controllerTargets, module, r), restAggregate(module, m))
}

// ViewPlan: frontend layer.
func ViewPlan(module string, r config.Resource) []FileOp {
	return planFor(viewTargets, module, r)
}

// ResourcePlan: model + controller + view (the all-in-one shortcut).
func ResourcePlan(module string, r config.Resource, m *config.Manifest) []FileOp {
	ops := planFor(modelTargets, module, r)
	ops = append(ops, planFor(controllerTargets, module, r)...)
	ops = append(ops, planFor(viewTargets, module, r)...)
	ops = append(ops, restAggregate(module, m), seederAggregate(module, m))
	return ops
}

// SinglePlan returns the op for a single make:<kind> verb against one resource.
func SinglePlan(verb, module string, r config.Resource) []FileOp {
	key := singleTargets[verb]
	for _, t := range allSingleTargets {
		if t.key == key {
			return planFor([]target{t}, module, r)
		}
	}
	return nil
}

func restAggregate(module string, m *config.Manifest) FileOp {
	return FileOp{Path: "internal/rest/registry.gen.go", Template: "registry/rest_registry.go.tmpl",
		Data: RegistryData{Module: module, Resources: m.Resources}, Mode: Overwrite}
}

func seederAggregate(module string, m *config.Manifest) FileOp {
	return FileOp{Path: "internal/db/seeders/registry.gen.go", Template: "registry/seeder_registry.go.tmpl",
		Data: RegistryData{Module: module, Resources: m.Resources}, Mode: Overwrite}
}
