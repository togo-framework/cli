package generator

import (
	"os"
	"path/filepath"
)

// TanStackFrontend reports whether the project's web app is the TanStack Router
// (Vite) frontend rather than Next.js. That frontend renders every resource with
// its generic /admin/<table> route, and the make:view files (web/app/<table>/
// page.tsx, swr hooks under web/lib) are Next.js-only, so they are not written.
func TanStackFrontend(root, webDir string) bool {
	if webDir == "" {
		webDir = "web"
	}
	for _, f := range []string{"src/router.tsx", "vite.config.ts"} {
		if _, err := os.Stat(filepath.Join(root, webDir, f)); err == nil {
			return true
		}
	}
	return false
}

// WithoutViews drops the Next.js view layer (make:view targets) from a plan.
func WithoutViews(ops []FileOp) []FileOp {
	out := ops[:0:0]
	for _, op := range ops {
		if !isViewTemplate(op.Template) {
			out = append(out, op)
		}
	}
	return out
}

func isViewTemplate(key string) bool {
	for _, t := range viewTargets {
		if t.key == key {
			return true
		}
	}
	return false
}
