// Package toolchain ensures the external tools togo needs — Go, Node/npm, sqlc,
// atlas — are present, installing any that are missing into ~/.togo/toolchain and
// putting them on PATH for the current process (and child exec.Commands). Each
// Ensure* is a fast no-op when the tool is already available.
package toolchain

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func home() string { h, _ := os.UserHomeDir(); return h }
func root() string { return filepath.Join(home(), ".togo", "toolchain") }

func has(bin string) bool { _, err := exec.LookPath(bin); return err == nil }
func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func logf(format string, a ...any) { fmt.Fprintf(os.Stderr, "\033[36m→\033[0m "+format+"\n", a...) }

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func prependPath(d string) {
	cur := os.Getenv("PATH")
	for _, p := range filepath.SplitList(cur) {
		if p == d {
			return
		}
	}
	os.Setenv("PATH", d+string(os.PathListSeparator)+cur)
}

// persist appends an env line to ~/.togo/env so future shells can source it.
func persist(line string) {
	p := filepath.Join(home(), ".togo", "env")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if b, _ := os.ReadFile(p); strings.Contains(string(b), line) {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

// EnsureGo makes sure `go` is on PATH, installing the latest stable toolchain
// into ~/.togo/toolchain/go when absent.
func EnsureGo() error {
	if has("go") {
		addGopathBin()
		return nil
	}
	goBin := filepath.Join(root(), "go", "bin")
	if fileExists(filepath.Join(goBin, exe("go"))) {
		prependPath(goBin)
		os.Setenv("GOROOT", filepath.Join(root(), "go"))
		addGopathBin()
		return nil
	}
	ver, err := latestGo()
	if err != nil {
		return fmt.Errorf("resolve latest Go: %w (install manually: https://go.dev/dl)", err)
	}
	logf("installing Go %s (not found on PATH)…", ver)
	url := fmt.Sprintf("https://go.dev/dl/%s.%s-%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	if err := downloadAndExtract(url, root(), 0); err != nil {
		return fmt.Errorf("install Go: %w (install manually: https://go.dev/dl)", err)
	}
	prependPath(goBin)
	os.Setenv("GOROOT", filepath.Join(root(), "go"))
	persist(fmt.Sprintf(`export GOROOT="%s"`, filepath.Join(root(), "go")))
	persist(fmt.Sprintf(`export PATH="%s:$PATH"`, goBin))
	addGopathBin()
	logf("Go %s installed to %s", ver, filepath.Join(root(), "go"))
	return nil
}

// addGopathBin puts $(go env GOPATH)/bin on PATH so go-installed tools (sqlc,
// atlas) resolve. Falls back to ~/go/bin if `go env` is unavailable.
func addGopathBin() {
	gp := ""
	if has("go") {
		if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
			gp = strings.TrimSpace(string(out))
		}
	}
	if gp == "" {
		gp = filepath.Join(home(), "go")
	}
	b := filepath.Join(gp, "bin")
	prependPath(b)
	persist(fmt.Sprintf(`export PATH="%s:$PATH"`, b))
}

func latestGo() (string, error) {
	var list []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := getJSON("https://go.dev/dl/?mode=json&include=stable", &list); err != nil {
		return "", err
	}
	for _, v := range list {
		if v.Stable {
			return v.Version, nil
		}
	}
	if len(list) > 0 {
		return list[0].Version, nil
	}
	return "", fmt.Errorf("no stable Go version found")
}

// EnsureNode makes sure node+npm are on PATH, installing the latest LTS into
// ~/.togo/toolchain/node when absent.
func EnsureNode() error {
	if has("node") && has("npm") {
		return nil
	}
	nodeBin := filepath.Join(root(), "node", "bin")
	if fileExists(filepath.Join(nodeBin, exe("node"))) {
		prependPath(nodeBin)
		return nil
	}
	osName := map[string]string{"darwin": "darwin", "linux": "linux"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if osName == "" || arch == "" {
		return fmt.Errorf("unsupported platform %s/%s for auto Node install; install manually: https://nodejs.org", runtime.GOOS, runtime.GOARCH)
	}
	ver, err := latestNodeLTS()
	if err != nil {
		return fmt.Errorf("resolve latest Node LTS: %w (install manually: https://nodejs.org)", err)
	}
	logf("installing Node %s (not found on PATH)…", ver)
	// Node ships .tar.gz for every platform, so stdlib gzip+tar suffices.
	url := fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-%s-%s.tar.gz", ver, ver, osName, arch)
	if err := downloadAndExtract(url, filepath.Join(root(), "node"), 1); err != nil {
		return fmt.Errorf("install Node: %w (install manually: https://nodejs.org)", err)
	}
	prependPath(nodeBin)
	persist(fmt.Sprintf(`export PATH="%s:$PATH"`, nodeBin))
	logf("Node %s installed to %s", ver, filepath.Join(root(), "node"))
	return nil
}

func latestNodeLTS() (string, error) {
	var list []struct {
		Version string          `json:"version"`
		LTS     json.RawMessage `json:"lts"`
	}
	if err := getJSON("https://nodejs.org/dist/index.json", &list); err != nil {
		return "", err
	}
	// index.json is newest-first; the first entry with lts != false is the latest LTS.
	for _, v := range list {
		if len(v.LTS) > 0 && string(v.LTS) != "false" {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("no Node LTS found")
}

// EnsureSqlc / EnsureAtlas install via `go install` when missing.
func EnsureSqlc() error  { return goInstall("sqlc", "github.com/sqlc-dev/sqlc/cmd/sqlc@latest") }
func EnsureAtlas() error { return goInstall("atlas", "ariga.io/atlas/cmd/atlas@latest") }

func goInstall(bin, pkg string) error {
	if has(bin) {
		return nil
	}
	if err := EnsureGo(); err != nil {
		return err
	}
	if has(bin) { // EnsureGo may have surfaced an existing install on PATH
		return nil
	}
	logf("installing %s (go install %s)…", bin, pkg)
	c := exec.Command("go", "install", pkg)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return fmt.Errorf("install %s: %w", bin, err)
	}
	addGopathBin()
	if !has(bin) {
		return fmt.Errorf("%s installed but not on PATH; add $(go env GOPATH)/bin to your PATH", bin)
	}
	return nil
}

func getJSON(url string, v any) error {
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// downloadAndExtract downloads a .tar.gz from url and extracts it into dest,
// stripping `strip` leading path components from each entry.
func downloadAndExtract(url, dest string, strip int) error {
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	clean := filepath.Clean(dest)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := h.Name
		if strip > 0 {
			parts := strings.SplitN(name, "/", strip+1)
			if len(parts) <= strip {
				continue
			}
			name = parts[strip]
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if target != clean && !strings.HasPrefix(target, clean+string(os.PathSeparator)) {
			continue // path traversal guard
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Remove(target)
			_ = os.Symlink(h.Linkname, target)
		}
	}
	return nil
}

// Tool is the presence/version of a prerequisite, for `togo doctor`.
type Tool struct {
	Name, Bin, Version string
	OK                 bool
}

func versionOf(bin string, args ...string) string {
	if !has(bin) {
		return ""
	}
	out, _ := exec.Command(bin, args...).Output()
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

// Status reports the presence + version of each prerequisite.
func Status() []Tool {
	return []Tool{
		{"Go", "go", versionOf("go", "version"), has("go")},
		{"Node", "node", versionOf("node", "-v"), has("node")},
		{"npm", "npm", versionOf("npm", "-v"), has("npm")},
		{"sqlc", "sqlc", versionOf("sqlc", "version"), has("sqlc")},
		{"atlas", "atlas", versionOf("atlas", "version"), has("atlas")},
	}
}

// EnsureAll installs every missing prerequisite (used by `togo doctor`).
func EnsureAll() error {
	for _, fn := range []func() error{EnsureGo, EnsureNode, EnsureSqlc, EnsureAtlas} {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}
