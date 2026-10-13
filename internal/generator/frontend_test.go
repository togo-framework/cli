package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

func TestTanStackFrontend(t *testing.T) {
	dir := t.TempDir()
	if TanStackFrontend(dir, "") {
		t.Fatal("an empty project is not TanStack (keeps the Next.js views)")
	}
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "src", "router.tsx"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !TanStackFrontend(dir, "frontend") || TanStackFrontend(dir, "") {
		t.Error("TanStackFrontend should follow the configured frontend dir")
	}
	ops := WithoutViews(ResourcePlan("example.com/app", "sqlite", config.Resource{Name: "Post", Table: "posts"}, &config.Manifest{}))
	for _, op := range ops {
		if isViewTemplate(op.Template) {
			t.Errorf("WithoutViews kept %s", op.Path)
		}
	}
	if len(ops) == 0 {
		t.Error("WithoutViews dropped everything")
	}
}
