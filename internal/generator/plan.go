package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/togo-framework/cli/internal/templates"
	"github.com/togo-framework/cli/internal/ui"
)

// OpMode controls how a rendered file reconciles against disk.
type OpMode int

const (
	// CreateOnly writes the file only if it does not exist (skip unless --force).
	// Used for per-resource, hand-editable artifacts.
	CreateOnly OpMode = iota
	// Overwrite always rewrites the file. Used for fully-generated aggregate
	// registries that carry a DO NOT EDIT banner.
	Overwrite
)

// FileOp is one planned file write.
type FileOp struct {
	// Path is relative to the project root.
	Path string
	// Template is the stub key (e.g. "resource/model.go.tmpl").
	Template string
	// Data is passed to the template.
	Data any
	Mode OpMode
}

// Options controls a generator run.
type Options struct {
	Root   string
	Force  bool
	DryRun bool
}

// Result captures the outcome of one op for reporting.
type Result struct {
	Path   string
	Status string // CREATE | OVERWRITE | FORCE | SKIP
}

// Execute renders every op in memory first (so a template error writes nothing),
// then reconciles to disk honoring CreateOnly/Overwrite, --force and --dry-run.
func Execute(opts Options, ops []FileOp) ([]Result, error) {
	type rendered struct {
		op    FileOp
		bytes []byte
	}
	all := make([]rendered, 0, len(ops))

	// Phase 1: render everything.
	for _, op := range ops {
		out, err := render(opts.Root, op)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", op.Template, err)
		}
		all = append(all, rendered{op: op, bytes: out})
	}

	// Phase 2: reconcile.
	results := make([]Result, 0, len(all))
	for _, r := range all {
		abs := filepath.Join(opts.Root, r.op.Path)
		exists := fileExists(abs)

		status, write := decide(r.op.Mode, exists, opts.Force, abs, r.bytes)
		results = append(results, Result{Path: r.op.Path, Status: status})

		if !write || opts.DryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return results, err
		}
		if err := os.WriteFile(abs, r.bytes, 0o644); err != nil {
			return results, err
		}
	}
	return results, nil
}

// decide computes the status label and whether to write for one op.
func decide(mode OpMode, exists, force bool, abs string, content []byte) (status string, write bool) {
	switch mode {
	case Overwrite:
		if exists && bytesEqualFile(abs, content) {
			return "SKIP", false // identical, no-op
		}
		if exists {
			return "OVERWRITE", true
		}
		return "CREATE", true
	default: // CreateOnly
		if !exists {
			return "CREATE", true
		}
		if force {
			return "FORCE", true
		}
		return "SKIP", false
	}
}

// render loads a stub, applies the FuncMap, formats Go output, and returns bytes.
func render(root string, op FileOp) ([]byte, error) {
	raw, err := templates.Read(root, op.Template)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(filepath.Base(op.Template)).Funcs(FuncMap()).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, op.Data); err != nil {
		return nil, err
	}
	out := buf.Bytes()

	if strings.HasSuffix(op.Path, ".go") {
		formatted, ferr := format.Source(out)
		if ferr != nil {
			return nil, fmt.Errorf("generated invalid Go for %s: %w", op.Path, ferr)
		}
		out = formatted
	}
	return out, nil
}

// Report prints a per-op summary table.
func Report(results []Result, dryRun bool) {
	for _, r := range results {
		ui.Step("%s  %s", ui.Label(r.Status), r.Path)
	}
	if dryRun {
		ui.Warn("dry-run: no files were written")
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func bytesEqualFile(p string, content []byte) bool {
	existing, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return bytes.Equal(existing, content)
}
