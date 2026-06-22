package cmd

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/togo-framework/cli/internal/ui"
)

// watchAndServe runs the API with a file watcher: on any .go change it rebuilds
// and restarts the API process (web, if provided, runs alongside and hot-reloads
// itself via next dev). Blocks until Ctrl-C.
func watchAndServe(root string, api service, web *service) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if web != nil {
		go func() {
			c := exec.CommandContext(ctx, web.bin, web.args...)
			c.Dir, c.Env = web.dir, web.env
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			setProcessGroup(c)
			_ = c.Run()
		}()
	}

	var cur *exec.Cmd
	start := func() {
		c := exec.Command(api.bin, api.args...)
		c.Dir, c.Env = api.dir, api.env
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		setProcessGroup(c)
		if err := c.Start(); err != nil {
			ui.Error("api start: %v", err)
			return
		}
		cur = c
	}
	stop := func() {
		if cur != nil {
			terminate(cur)
			_, _ = cur.Process.Wait()
			cur = nil
		}
	}

	ui.Info("watching %s for changes (Ctrl-C to stop)", root)
	start()
	last := snapshot(root)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stop()
			return nil
		case <-ticker.C:
			if cur := snapshot(root); changed(last, cur) {
				last = cur
				ui.Info("change detected — restarting api")
				stop()
				start()
			}
		}
	}
}

// snapshot maps each .go file under root to its mtime (skipping web/vendor/hidden).
func snapshot(root string) map[string]int64 {
	out := map[string]int64{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			n := d.Name()
			if n == "web" || n == "node_modules" || n == "vendor" || (strings.HasPrefix(n, ".") && n != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") {
			if fi, err := d.Info(); err == nil {
				out[p] = fi.ModTime().UnixNano()
			}
		}
		return nil
	})
	return out
}

func changed(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range b {
		if a[k] != v {
			return true
		}
	}
	return false
}
