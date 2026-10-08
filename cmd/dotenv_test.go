package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/togo-framework/cli/internal/config"
)

func TestParseDotenv(t *testing.T) {
	src := "\ufeff# full-line comment\r\n" +
		"APP_ENV=development                 # development | production\r\n" +
		"DB_DRIVER=pgx\r\n" +
		"AUTH_SECRET=                        # REQUIRED in production (>= 32 bytes)\r\n" +
		"CORS_ORIGINS=                       # comma-separated allowed origins\r\n" +
		"EMPTY=\n" +
		"HASH_START=#not-a-value\n" +
		"URL=http://localhost:3000/#frag\n" +
		`DQ="a # b"  # comment` + "\n" +
		`SQ='123:abc'` + "\n" +
		"export EXPORTED=yes\n" +
		"  SPACED  =  v  \n" +
		"\n" +
		"NOEQ\n" +
		"1BAD=x\n" +
		`OPEN="never closed` + "\n" +
		`TRAIL="x" junk` + "\n"
	vars, skipped, err := parseDotenv(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"APP_ENV":      "development",
		"DB_DRIVER":    "pgx",
		"AUTH_SECRET":  "",
		"CORS_ORIGINS": "",
		"EMPTY":        "",
		"HASH_START":   "",
		"URL":          "http://localhost:3000/#frag",
		"DQ":           "a # b",
		"SQ":           "123:abc",
		"EXPORTED":     "yes",
		"SPACED":       "v",
	}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars =\n%q\nwant\n%q", vars, want)
	}
	// NOEQ, 1BAD, OPEN and TRAIL, by 1-based line number.
	if want := []int{14, 15, 16, 17}; !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped lines = %v, want %v", skipped, want)
	}
}

func TestReadDotenvMissing(t *testing.T) {
	vars, skipped, err := readDotenv(t.TempDir())
	if vars != nil || skipped != nil || err != nil {
		t.Fatalf("readDotenv(no .env) = %v, %v, %v; want nil, nil, nil", vars, skipped, err)
	}
}

func TestReadDotenvUnreadable(t *testing.T) {
	const secret = "sentinel-secret-value"
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".env"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readDotenv(dir); err == nil {
			t.Fatal("readDotenv(.env is a directory) succeeded, want an error")
		}
	})
	t.Run("line too long", func(t *testing.T) {
		dir := t.TempDir()
		long := "KEY=" + secret + strings.Repeat("x", 70*1024) + "\n"
		mustWrite(t, filepath.Join(dir, ".env"), []byte(long))
		_, _, err := readDotenv(dir)
		if err == nil {
			t.Fatal("readDotenv(70 KiB line) succeeded, want an error")
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaks .env contents: %v", err)
		}
	})
	t.Run("no permission", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("file modes don't deny reads here")
		}
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, ".env"), []byte("KEY="+secret+"\n"))
		if err := os.Chmod(filepath.Join(dir, ".env"), 0); err != nil {
			t.Fatal(err)
		}
		_, _, err := readDotenv(dir)
		if err == nil {
			t.Fatal("readDotenv(mode 000) succeeded, want an error")
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaks .env contents: %v", err)
		}
	})
}

func TestMergeDotenv(t *testing.T) {
	environ := []string{"PATH=/bin", "DB_DRIVER=sqlite", "EMPTY="}
	isSet := func(k string) bool {
		return k == "PATH" || k == "DB_DRIVER" || k == "EMPTY"
	}
	vars := map[string]string{"DB_DRIVER": "pgx", "EMPTY": "filled", "DATABASE_URL": "postgres://x", "A": "1"}
	env, added, kept := mergeDotenv(environ, vars, isSet)
	want := []string{"PATH=/bin", "DB_DRIVER=sqlite", "EMPTY=", "A=1", "DATABASE_URL=postgres://x"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("env = %q, want %q", env, want)
	}
	if added != 2 || kept != 2 {
		t.Errorf("added, kept = %d, %d; want 2, 2", added, kept)
	}
}

// Windows environment names are case-insensitive: a shell `db_driver` is the
// same variable as DB_DRIVER in .env, so .env must not add a second one.
func TestLocalAppEnvWindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), []byte("TOGO_T10_CASE=from-dotenv\n"))
	t.Setenv("togo_t10_case", "from-shell")
	env, err := localAppEnv(false, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range env {
		if strings.EqualFold(kv, "TOGO_T10_CASE=from-dotenv") {
			t.Fatalf("shell variable overridden by .env: %q", kv)
		}
	}
}

func TestLocalAppEnv(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), []byte("TOGO_T10_A=dotenv\nTOGO_T10_B=dotenv\n"))
	t.Setenv("TOGO_T10_B", "shell")
	t.Setenv("APP_ENV", "")
	os.Unsetenv("APP_ENV")

	env, err := localAppEnv(false, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := envValue(env, "TOGO_T10_A"); got != "dotenv" {
		t.Errorf("TOGO_T10_A = %q, want dotenv (from .env)", got)
	}
	if got := envValue(env, "TOGO_T10_B"); got != "shell" {
		t.Errorf("TOGO_T10_B = %q, want shell (shell wins)", got)
	}

	t.Run("--no-env-file", func(t *testing.T) {
		env, err := localAppEnv(true, root)
		if err != nil {
			t.Fatal(err)
		}
		if got := envValue(env, "TOGO_T10_A"); got != "" {
			t.Errorf("TOGO_T10_A = %q, want unset", got)
		}
	})
	t.Run("APP_ENV=production", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		env, err := localAppEnv(false, root)
		if err != nil {
			t.Fatal(err)
		}
		if got := envValue(env, "TOGO_T10_A"); got != "" {
			t.Errorf("TOGO_T10_A = %q, want unset in production", got)
		}
	})
	t.Run("unreadable .env stops the command", func(t *testing.T) {
		bad := t.TempDir()
		if err := os.Mkdir(filepath.Join(bad, ".env"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := localAppEnv(false, bad); err == nil {
			t.Fatal("want an error for an unreadable .env")
		}
	})
}

// fakeApp writes a togo project whose cmd/migrate and cmd/seed record the
// variables they see into $TOGO_T10_OUT.
func fakeApp(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "togo.yaml"), []byte("name: demo\nmodule: example.com/demo\n"))
	mustWrite(t, filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.21\n"))
	prog := []byte(`package main

import (
	"fmt"
	"os"
)

func main() {
	_, secretSet := os.LookupEnv("AUTH_SECRET")
	out := fmt.Sprintf("DB_DRIVER=%s\nSECRET_SET=%v\n", os.Getenv("DB_DRIVER"), secretSet)
	if err := os.WriteFile(os.Getenv("TOGO_T10_OUT"), []byte(out), 0o600); err != nil {
		panic(err)
	}
}
`)
	for _, name := range []string{"migrate", "seed"} {
		if err := os.MkdirAll(filepath.Join(root, "cmd", name), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(root, "cmd", name, "main.go"), prog)
	}
}

// runCLI runs `togo args...` and returns everything it printed.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() { _, err := io.Copy(&buf, r); done <- err }()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		_ = rootCmd.PersistentFlags().Set("config", "")
		for _, name := range []string{"migrate", "seed", "dev", "serve"} {
			c, _, _ := rootCmd.Find([]string{name})
			c.Flags().VisitAll(func(f *pflag.Flag) {
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			})
		}
	})
	rootCmd.SetArgs(args)
	runErr := rootCmd.Execute()
	os.Stdout, os.Stderr = stdout, stderr
	w.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.Close()
	return buf.String(), runErr
}

// Regression for #10: `togo migrate` / `togo seed` didn't pass the project .env
// to the app, so the kernel fell back to DB_DRIVER=sqlite.
func TestMigrateSeedLoadProjectDotenv(t *testing.T) {
	if !goAvailable() {
		t.Skip("go toolchain not on PATH")
	}
	const secret = "sentinel-secret-value"
	parent := t.TempDir()
	root := filepath.Join(parent, "app")
	fakeApp(t, root)
	// A parent-directory .env must never be read.
	mustWrite(t, filepath.Join(parent, ".env"), []byte("DB_DRIVER=from-parent\nAUTH_SECRET=parent\n"))
	mustWrite(t, filepath.Join(root, ".env"), []byte("DB_DRIVER=pgx   # from .env.example\nAUTH_SECRET="+secret+"\n"))
	t.Setenv("DB_DRIVER", "")
	os.Unsetenv("DB_DRIVER")
	t.Setenv("AUTH_SECRET", "")
	os.Unsetenv("AUTH_SECRET")
	t.Setenv("APP_ENV", "")
	os.Unsetenv("APP_ENV")
	sub := filepath.Join(root, "cmd")
	t.Chdir(sub) // a subdirectory resolves to the project root's .env

	for _, name := range []string{"migrate", "seed"} {
		t.Run(name, func(t *testing.T) {
			outFile := filepath.Join(t.TempDir(), "out")
			t.Setenv("TOGO_T10_OUT", outFile)

			printed, err := runCLI(t, name)
			if err != nil {
				t.Fatalf("togo %s: %v\n%s", name, err, printed)
			}
			if got := readFile(t, outFile); got != "DB_DRIVER=pgx\nSECRET_SET=true\n" {
				t.Errorf("app saw:\n%s\nwant DB_DRIVER=pgx from the project .env", got)
			}
			if strings.Contains(printed, secret) || strings.Contains(printed, "AUTH_SECRET") {
				t.Errorf("output leaks .env contents:\n%s", printed)
			}

			t.Setenv("DB_DRIVER", "shell")
			if printed, err := runCLI(t, name); err != nil {
				t.Fatalf("togo %s: %v\n%s", name, err, printed)
			}
			if got := readFile(t, outFile); !strings.HasPrefix(got, "DB_DRIVER=shell\n") {
				t.Errorf("app saw:\n%s\nwant the shell DB_DRIVER to win", got)
			}
			os.Unsetenv("DB_DRIVER")

			if printed, err := runCLI(t, name, "--no-env-file"); err != nil {
				t.Fatalf("togo %s --no-env-file: %v\n%s", name, err, printed)
			}
			if got := readFile(t, outFile); got != "DB_DRIVER=\nSECRET_SET=false\n" {
				t.Errorf("app saw:\n%s\nwant no .env values with --no-env-file", got)
			}
		})
	}
}

// `togo dev` gives the API the project .env; `togo serve` keeps the plain
// process environment (owner decision on #10: serve is unchanged).
func TestDevLoadsDotenvServeDoesNot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "togo.yaml"), []byte("name: demo\nmodule: example.com/demo\n"))
	mustWrite(t, filepath.Join(root, ".env"), []byte("TOGO_T10_DEV=pgx\n"))
	t.Setenv("APP_ENV", "")
	os.Unsetenv("APP_ENV")
	t.Chdir(root)

	var got devOptions
	orig := runDevFn
	runDevFn = func(_ *config.Project, opts devOptions) error { got = opts; return nil }
	t.Cleanup(func() { runDevFn = orig })

	if out, err := runCLI(t, "dev", "--api-only"); err != nil {
		t.Fatalf("togo dev: %v\n%s", err, out)
	}
	if v := envValue(got.apiEnv, "TOGO_T10_DEV"); v != "pgx" {
		t.Errorf("dev api env TOGO_T10_DEV = %q, want pgx", v)
	}

	got = devOptions{}
	if out, err := runCLI(t, "serve", "--api-only"); err != nil {
		t.Fatalf("togo serve: %v\n%s", err, out)
	}
	if got.apiEnv != nil {
		t.Errorf("serve api env = %d vars, want nil (plain process environment)", len(got.apiEnv))
	}
	if env := apiEnv(got); envValue(env, "TOGO_T10_DEV") != "" {
		t.Error("serve passed a .env value to the API")
	}
}

func envValue(env []string, key string) string {
	v := ""
	for _, kv := range env {
		if k, val, ok := strings.Cut(kv, "="); ok && k == key {
			v = val
		}
	}
	return v
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
