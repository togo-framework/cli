package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/togo-framework/cli/internal/ui"
)

// The project .env is read only from the project root (the folder holding
// togo.yaml), never from a parent directory. Values are never printed: output
// and errors carry only counts, line numbers and the file path.

// readDotenv parses <root>/.env. A missing file returns nil, nil, nil; any other
// failure to read it (permissions, a directory, an over-long line) is an error.
func readDotenv(root string) (map[string]string, []int, error) {
	path := filepath.Join(root, ".env")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()
	vars, skipped, err := parseDotenv(f)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return vars, skipped, nil
}

// parseDotenv parses KEY=VALUE lines. It accepts a UTF-8 BOM, CRLF endings,
// `export ` prefixes, # comments (full-line, or after whitespace in an unquoted
// value) and single- or double-quoted values, taken literally. There is no
// escape processing or ${VAR} expansion. It returns the 1-based numbers of
// lines it couldn't parse.
func parseDotenv(r io.Reader) (map[string]string, []int, error) {
	vars := map[string]string{}
	var skipped []int
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if n == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := parseDotenvLine(line)
		if !ok {
			skipped = append(skipped, n)
			continue
		}
		vars[k] = v
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	return vars, skipped, nil
}

func parseDotenvLine(line string) (key, value string, ok bool) {
	line = strings.TrimPrefix(line, "export ")
	key, raw, found := strings.Cut(line, "=")
	key = strings.TrimSpace(key)
	if !found || !validEnvKey(key) {
		return "", "", false
	}
	raw = strings.TrimSpace(raw)
	if raw != "" && (raw[0] == '"' || raw[0] == '\'') {
		end := strings.IndexByte(raw[1:], raw[0])
		if end < 0 {
			return "", "", false
		}
		rest := strings.TrimSpace(raw[end+2:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", "", false
		}
		return key, raw[1 : end+1], true
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t') {
			raw = raw[:i]
			break
		}
	}
	return key, strings.TrimSpace(raw), true
}

func validEnvKey(k string) bool {
	if k == "" || (k[0] >= '0' && k[0] <= '9') {
		return false
	}
	for _, c := range k {
		if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// mergeDotenv appends the .env variables the shell doesn't already define
// (even as empty) to environ. isSet is os.LookupEnv in production, which is
// case-insensitive on Windows.
func mergeDotenv(environ []string, vars map[string]string, isSet func(string) bool) (env []string, added, kept int) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env = append([]string(nil), environ...)
	for _, k := range keys {
		if isSet(k) {
			kept++
			continue
		}
		env = append(env, k+"="+vars[k])
		added++
	}
	return env, added, kept
}

// localAppEnv is the environment for app processes started by `togo migrate`,
// `togo seed` and `togo dev`: the shell environment plus the project .env, the
// shell winning. It is the plain shell environment with --no-env-file or when
// the shell sets APP_ENV=production.
func localAppEnv(noEnvFile bool, root string) ([]string, error) {
	if noEnvFile {
		return os.Environ(), nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if fileExists(filepath.Join(root, ".env")) {
			ui.Info("APP_ENV=production: not loading .env")
		}
		return os.Environ(), nil
	}
	vars, skipped, err := readDotenv(root)
	if err != nil {
		return nil, fmt.Errorf("%w (fix or remove it, or pass --no-env-file)", err)
	}
	for _, n := range skipped {
		ui.Warn("ignoring .env line %d: not a valid KEY=VALUE", n)
	}
	if vars == nil {
		return os.Environ(), nil
	}
	env, added, kept := mergeDotenv(os.Environ(), vars, func(k string) bool {
		_, ok := os.LookupEnv(k)
		return ok
	})
	ui.Info("loaded %d variables from .env (%d kept from the shell)", added, kept)
	return env, nil
}
