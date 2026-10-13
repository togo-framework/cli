package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/generator"
	"github.com/togo-framework/cli/internal/ui"
)

// RegisterMake adds the make:* family: the core flow (model → controller → view),
// the all-in-one resource shortcut, actions, and single-artifact generators.
func RegisterMake(root *cobra.Command) {
	root.AddCommand(
		makeModelCmd(),
		makeControllerCmd(),
		makeViewCmd(),
		makeResourceCmd(),
		makeActionCmd(),
	)
	// Single-artifact generators (operate on an existing model).
	for verb, short := range map[string]string{
		"make:query":     "Generate the sqlc query file for a model",
		"make:migration": "Generate the Atlas schema for a model",
		"make:graphql":   "Generate the GraphQL schema fragment for a model",
		"make:api":       "Generate the Huma REST handler for a model",
		"make:seeder":    "Generate the seeder for a model",
		"make:factory":   "Generate the faker factory for a model",
		"make:page":      "Generate the Next.js page for a model (Next.js frontend only)",
		"make:test":      "Generate a Go feature test (PHPUnit-style) for a model",
		"make:e2e":       "Generate a Playwright e2e spec (Dusk-style) for a model",
	} {
		verb, short := verb, short
		root.AddCommand(&cobra.Command{
			Use: verb + " <Name>", Short: short, GroupID: groupMake, Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error { return runSingle(cmd, verb, args[0]) },
		})
	}
}

// make:model — the start of the flow. Interactive field prompt when none given.
func makeModelCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "make:model <Name> [field:type ...]",
		Short:   "Create a model: fields (id + timestamps by default), migration, seeder",
		GroupID: groupMake,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, m, opts, err := makeContext(cmd)
			if err != nil {
				return err
			}
			fields, err := resolveFields(args[1:])
			if err != nil {
				return err
			}
			name := generator.Pascal(args[0])
			res := config.Resource{Name: name, Table: generator.TableName(name), Fields: fields}
			if existing := m.Find(name); existing != nil {
				res.Controller = existing.Controller // preserve
			}
			if _, err := m.Upsert(res, opts.Force); err != nil {
				return err
			}
			results, err := generator.Execute(opts, generator.ModelPlan(proj.Module, sqlDriver(proj), res, m))
			if err != nil {
				return err
			}
			ui.Info("Model %s (%s)", res.Name, res.Table)
			generator.Report(results, opts.DryRun)
			if !opts.DryRun {
				if err := m.Save(); err != nil {
					return err
				}
				ui.Success("Created model %s", res.Name)
				ui.Step("next: togo migrate   then: togo make:controller %s", res.Name)
			}
			return nil
		},
	}
}

// make:controller --model=Name — the API layer.
func makeControllerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "make:controller <Name>",
		Short:   "Create a CRUD controller (REST + GraphQL + docs + hooks) for a model",
		GroupID: groupMake,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, m, opts, err := makeContext(cmd)
			if err != nil {
				return err
			}
			name := modelName(cmd, args)
			if name == "" {
				return fmt.Errorf("provide a model: togo make:controller <Name> or --model=<Name>")
			}
			res := m.Find(generator.Pascal(name))
			if res == nil {
				return fmt.Errorf("model %q not found — run `togo make:model %s` first", generator.Pascal(name), name)
			}
			m.SetController(res.Name)
			res.Controller = true
			results, err := generator.Execute(opts, generator.ControllerPlan(proj.Module, *res, m))
			if err != nil {
				return err
			}
			ui.Info("Controller %s", res.Name)
			generator.Report(results, opts.DryRun)
			if !opts.DryRun {
				if err := m.Save(); err != nil {
					return err
				}
				ui.Success("Created controller %s (REST + GraphQL)", res.Name)
				ui.Step("next: togo generate   then: togo make:view %s", res.Name)
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "the model to build the controller for")
	return cmd
}

// make:view --model=Name — the frontend layer.
func makeViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "make:view <Name>",
		Short:   "Create Next.js views (page + hook + type) for a model (Next.js frontend only)",
		GroupID: groupMake,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, m, opts, err := makeContext(cmd)
			if err != nil {
				return err
			}
			name := modelName(cmd, args)
			res := m.Find(generator.Pascal(name))
			if res == nil {
				return fmt.Errorf("model %q not found — run `togo make:model %s` first", generator.Pascal(name), name)
			}
			if tanStack(proj) {
				skipViews(res.Table)
				return nil
			}
			results, err := generator.Execute(opts, generator.ViewPlan(proj.Module, *res))
			if err != nil {
				return err
			}
			ui.Info("Views for %s", res.Name)
			generator.Report(results, opts.DryRun)
			if !opts.DryRun {
				ui.Success("Created views for %s", res.Name)
			}
			return nil
		},
	}
	cmd.Flags().String("model", "", "the model to build views for")
	return cmd
}

// make:resource — model + controller + view in one shot.
func makeResourceCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "make:resource <Name> [field:type ...]",
		Short:   "Scaffold a full resource: model + controller + views (the all-in-one shortcut)",
		GroupID: groupMake,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, m, opts, err := makeContext(cmd)
			if err != nil {
				return err
			}
			fields, err := resolveFields(args[1:])
			if err != nil {
				return err
			}
			name := generator.Pascal(args[0])
			res := config.Resource{Name: name, Table: generator.TableName(name), Fields: fields, Controller: true}
			if _, err := m.Upsert(res, opts.Force); err != nil {
				return err
			}
			plan := generator.ResourcePlan(proj.Module, sqlDriver(proj), res, m)
			if tanStack(proj) {
				plan = generator.WithoutViews(plan)
			}
			results, err := generator.Execute(opts, plan)
			if err != nil {
				return err
			}
			ui.Info("Resource %s (%s)", res.Name, res.Table)
			generator.Report(results, opts.DryRun)
			if !opts.DryRun {
				if err := m.Save(); err != nil {
					return err
				}
				ui.Success("Created resource %s", res.Name)
				ui.Step("next: togo generate && togo migrate && togo serve")
				if tanStack(proj) {
					ui.Step("page: /admin/%s (the TanStack admin renders every resource)", res.Table)
				}
			}
			return nil
		},
	}
}

// make:action — a reusable Action (business logic) callable from controllers.
func makeActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "make:action <Name>",
		Short:   "Create an Action (reusable logic dispatched from controllers/events)",
		GroupID: groupMake,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, _, opts, err := makeContext(cmd)
			if err != nil {
				return err
			}
			name := generator.Pascal(args[0])
			data := generator.ResourceData{Module: proj.Module, Resource: config.Resource{Name: name}}
			op := generator.FileOp{
				Path:     "internal/actions/" + generator.Snake(name) + ".go",
				Template: "resource/action.go.tmpl", Data: data, Mode: generator.CreateOnly,
			}
			results, err := generator.Execute(opts, []generator.FileOp{op})
			if err != nil {
				return err
			}
			generator.Report(results, opts.DryRun)
			if !opts.DryRun {
				ui.Success("Created action %s", name)
			}
			return nil
		},
	}
}

func runSingle(cmd *cobra.Command, verb, name string) error {
	proj, m, opts, err := makeContext(cmd)
	if err != nil {
		return err
	}
	res := m.Find(generator.Pascal(name))
	if res == nil {
		return fmt.Errorf("model %q not found — run `togo make:model %s` first", generator.Pascal(name), name)
	}
	plan := generator.SinglePlan(verb, proj.Module, sqlDriver(proj), *res)
	if tanStack(proj) && len(generator.WithoutViews(plan)) < len(plan) {
		skipViews(res.Table)
		return nil
	}
	results, err := generator.Execute(opts, plan)
	if err != nil {
		return err
	}
	generator.Report(results, opts.DryRun)
	return nil
}

// tanStack reports whether the project's web app is the TanStack frontend, where
// the Next.js view layer (page, hook, API type) does not apply.
func tanStack(proj *config.Project) bool {
	return generator.TanStackFrontend(proj.Root, proj.Frontend.Dir)
}

func skipViews(table string) {
	ui.Info("TanStack frontend: no Next.js views written; the generic admin renders /admin/%s", table)
}

// makeContext loads the project, manifest, and shared options for a make command.
func makeContext(cmd *cobra.Command) (*config.Project, *config.Manifest, generator.Options, error) {
	proj, err := loadProject(cmd)
	if err != nil {
		return nil, nil, generator.Options{}, err
	}
	m, err := config.LoadManifest(proj.Root)
	if err != nil {
		return nil, nil, generator.Options{}, err
	}
	force, _ := cmd.Flags().GetBool("force")
	dry, _ := cmd.Flags().GetBool("dry-run")
	warnSQLCEngine(proj.Root, sqlDriver(proj))
	return proj, m, generator.Options{Root: proj.Root, Force: force, DryRun: dry}, nil
}

// sqlDriver is the driver whose SQL dialect make:* writes: DB_DRIVER, then
// togo.yaml database.driver, then sqlite (the kernel's default). A togo.yaml
// without a driver keeps SQLite output rather than the config's postgres default.
func sqlDriver(proj *config.Project) string {
	if v := os.Getenv("DB_DRIVER"); v != "" {
		return v
	}
	if proj.Database.DriverDefaulted {
		return "sqlite"
	}
	return proj.Database.Driver
}

var sqlcEngine = regexp.MustCompile(`(?m)^\s*-?\s*engine:\s*"?([A-Za-z]+)"?`)

// warnSQLCEngine warns when sqlc.yaml's engine cannot parse the SQL make:*
// writes for driver (projects scaffolded before sqlc.yaml followed the driver).
func warnSQLCEngine(root, driver string) {
	b, err := os.ReadFile(filepath.Join(root, "sqlc.yaml"))
	if err != nil {
		return
	}
	match := sqlcEngine.FindSubmatch(b)
	if match == nil {
		return
	}
	want := map[string]string{
		generator.DialectPostgres: "postgresql",
		generator.DialectMySQL:    "mysql",
		generator.DialectSQLite:   "sqlite",
	}[generator.SQLDialect(driver)]
	if got := string(match[1]); got != want {
		ui.Warn("sqlc.yaml uses engine %q but database driver %q writes %s SQL: set engine: %q in sqlc.yaml (with matching overrides) before `togo generate`", got, driver, generator.SQLDialect(driver), want)
	}
}

func modelName(cmd *cobra.Command, args []string) string {
	if v, _ := cmd.Flags().GetString("model"); v != "" {
		return v
	}
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// resolveFields parses field args, or prompts interactively when none are given.
func resolveFields(args []string) ([]config.Field, error) {
	if len(args) > 0 {
		return generator.ParseFields(args)
	}
	return promptFields()
}

// promptFields interactively collects fields (id + timestamps are always added).
func promptFields() ([]config.Field, error) {
	ui.Info("Define fields (id, created_at, updated_at are added automatically).")
	ui.Step("Enter a blank name to finish. Types: string,text,int,bool,float,uuid,time,json")
	sc := bufio.NewScanner(os.Stdin)
	var specs []string
	for {
		fmt.Print("  field name: ")
		if !sc.Scan() {
			break
		}
		name := strings.TrimSpace(sc.Text())
		if name == "" {
			break
		}
		fmt.Print("  type [string]: ")
		sc.Scan()
		typ := strings.TrimSpace(sc.Text())
		if typ == "" {
			typ = "string"
		}
		fmt.Print("  nullable? [y/N]: ")
		sc.Scan()
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(sc.Text())), "y") {
			typ += ":nullable"
		}
		specs = append(specs, name+":"+typ)
	}
	return generator.ParseFields(specs)
}
