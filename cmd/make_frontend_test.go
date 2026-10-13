package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

// make:resource in a TanStack app wrote a Next.js page (web/app/<table>/page.tsx)
// plus swr hooks the app can't build; the generic /admin/<table> renders it.
// A Next.js app still gets its views.
func TestMakeResourceViewsFollowFrontend(t *testing.T) {
	for _, tc := range []struct {
		name, marker string
		views        bool
	}{
		{"tanstack", "web/src/router.tsx", false},
		{"nextjs", "web/next.config.mjs", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mustWrite(t, filepath.Join(dir, config.ConfigFile), []byte("name: app\nmodule: example.com/app\n"))
			marker := filepath.Join(dir, filepath.FromSlash(tc.marker))
			if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, marker, []byte("// frontend\n"))
			defer func() {
				rootCmd.SetArgs(nil)
				_ = rootCmd.PersistentFlags().Set("config", "")
			}()
			cfg := filepath.Join(dir, config.ConfigFile)
			for _, args := range [][]string{
				{"make:resource", "Post", "title:string", "--config", cfg},
				{"make:view", "Post", "--config", cfg},
				{"make:page", "Post", "--config", cfg},
			} {
				rootCmd.SetArgs(args)
				if err := rootCmd.Execute(); err != nil {
					t.Fatalf("%v: %v", args, err)
				}
			}
			for _, f := range []string{"web/app/posts/page.tsx", "web/lib/hooks/usePosts.ts", "web/lib/api/post.ts"} {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); (err == nil) != tc.views {
					t.Errorf("%s exists = %v, want %v", f, err == nil, tc.views)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "internal", "rest", "post_handler.go")); err != nil {
				t.Errorf("the API layer must still be generated: %v", err)
			}
		})
	}
}
