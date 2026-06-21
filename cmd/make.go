package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/generator"
	"github.com/togo-framework/cli/internal/ui"
)

// makeSpec describes one make:<kind> command in the family.
type makeSpec struct {
	name  string
	short string
}

// makeRegistry is the single source of truth for the make:* family. make:resource
// is the flagship (emits every fragment + aggregates); the rest regenerate one
// fragment for a resource already present in togo.resources.yaml.
var makeRegistry = []makeSpec{
	{"make:resource", "Scaffold a full resource: model, sqlc, Atlas, GraphQL, REST, seeder, Next.js page"},
	{"make:model", "Generate the Go model struct for a resource"},
	{"make:query", "Generate the sqlc query file for a resource"},
	{"make:migration", "Generate the Atlas schema (desired state) for a resource"},
	{"make:graphql", "Generate the GraphQL schema fragment for a resource"},
	{"make:api", "Generate the Huma REST handler for a resource"},
	{"make:seeder", "Generate the seeder for a resource"},
	{"make:page", "Generate the Next.js page for a resource"},
}

// RegisterMake adds the whole make:* family from the registry table.
func RegisterMake(root *cobra.Command) {
	for _, s := range makeRegistry {
		s := s
		cmd := &cobra.Command{
			Use:     s.name + " <Name> [field:type ...]",
			Short:   s.short,
			GroupID: groupMake,
			Args:    cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runMake(cmd, s.name, args)
			},
		}
		root.AddCommand(cmd)
	}
}

func runMake(cmd *cobra.Command, verb string, args []string) error {
	proj, err := loadProject(cmd)
	if err != nil {
		return err
	}
	opts, err := optsFor(cmd, proj.Root)
	if err != nil {
		return err
	}

	name := args[0]
	manifest, err := config.LoadManifest(proj.Root)
	if err != nil {
		return err
	}

	if verb == "make:resource" {
		return makeResource(proj, manifest, name, args[1:], opts)
	}
	return makeSingle(proj, manifest, verb, name, opts)
}

// makeResource upserts the resource into the manifest, then renders all fragments
// and regenerates the aggregate registries.
func makeResource(proj *config.Project, manifest *config.Manifest, name string, fieldArgs []string, opts generator.Options) error {
	fields, err := generator.ParseFields(fieldArgs)
	if err != nil {
		return err
	}
	res := config.Resource{
		Name:   generator.Pascal(name),
		Table:  generator.TableName(name),
		Fields: fields,
	}
	existed, err := manifest.Upsert(res, opts.Force)
	if err != nil {
		return err
	}

	plan := generator.ResourcePlan(proj.Module, res, manifest)
	results, err := generator.Execute(opts, plan)
	if err != nil {
		return err
	}
	ui.Info("Resource %s (%s)", res.Name, res.Table)
	generator.Report(results, opts.DryRun)

	if !opts.DryRun {
		if err := manifest.Save(); err != nil {
			return err
		}
		printResourceNextSteps(res, existed)
	}
	return nil
}

// makeSingle regenerates one fragment for an existing manifest resource.
func makeSingle(proj *config.Project, manifest *config.Manifest, verb, name string, opts generator.Options) error {
	res := manifest.Find(generator.Pascal(name))
	if res == nil {
		return fmt.Errorf("resource %q not found in %s — run `togo make:resource %s <field:type...>` first",
			generator.Pascal(name), config.ManifestFile, name)
	}
	plan := generator.SinglePlan(verb, proj.Module, *res)
	if plan == nil {
		return fmt.Errorf("%s is not yet implemented", verb)
	}
	results, err := generator.Execute(opts, plan)
	if err != nil {
		return err
	}
	generator.Report(results, opts.DryRun)
	return nil
}

func printResourceNextSteps(res config.Resource, existed bool) {
	verb := "Created"
	if existed {
		verb = "Updated"
	}
	ui.Success("%s resource %s", verb, res.Name)
	ui.Step("next: togo generate   # run sqlc + gqlgen + atlas + openapi")
	ui.Step("then: togo migrate && togo serve")
}

// optsFor builds generator.Options from the shared persistent flags.
func optsFor(cmd *cobra.Command, root string) (generator.Options, error) {
	force, _ := cmd.Flags().GetBool("force")
	dry, _ := cmd.Flags().GetBool("dry-run")
	return generator.Options{Root: root, Force: force, DryRun: dry}, nil
}
