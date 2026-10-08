package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pseudoVersion matches Go pseudo-versions (vX.Y.Z-[0.]yyyymmddhhmmss-abcdef123456),
// which point at an untagged commit.
var pseudoVersion = regexp.MustCompile(`-(?:0\.)?\d{14}-[0-9a-f]{12}$`)

// A released CLI must build from tagged ToGo modules only: no pseudo-versions
// (untagged commits) and no replace directives (local or forked sources).
func TestReleaseGoModUsesTaggedDependencies(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if problems := releaseGoModProblems(string(src)); len(problems) > 0 {
		t.Fatalf("go.mod is not releasable:\n  %s", strings.Join(problems, "\n  "))
	}
}

func TestReleaseGoModProblems(t *testing.T) {
	cases := []struct {
		name  string
		gomod string
		want  int
	}{
		{"tagged", "module m\n\nrequire github.com/togo-framework/create-togo-app v0.2.0\n", 0},
		{"pseudo-version in block", "module m\n\nrequire (\n\tgithub.com/togo-framework/create-togo-app v0.1.1-0.20260703123537-ab566b255c51\n)\n", 1},
		{"v0.0.0 pseudo-version", "module m\n\nrequire github.com/togo-framework/auth v0.0.0-20260703123537-ab566b255c51 // indirect\n", 1},
		{"third-party pseudo-version is allowed", "module m\n\nrequire golang.org/x/exp v0.0.0-20260703123537-ab566b255c51\n", 0},
		{"single replace", "module m\n\nreplace github.com/togo-framework/create-togo-app => ../create-togo-app\n", 1},
		{"replace block", "module m\n\nreplace (\n\tgithub.com/a/b => ../b\n\tgithub.com/c/d v1.0.0 => github.com/e/d v1.0.1\n)\n", 2},
		{"prerelease tag is not a pseudo-version", "module m\n\nrequire github.com/togo-framework/auth v0.10.0-rc.1\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := releaseGoModProblems(c.gomod); len(got) != c.want {
				t.Fatalf("got %d problems %v, want %d", len(got), got, c.want)
			}
		})
	}
}

// releaseGoModProblems lists replace directives and ToGo pseudo-version requires.
func releaseGoModProblems(gomod string) []string {
	var problems []string
	block := ""
	for _, raw := range strings.Split(gomod, "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case line == "":
			continue
		case line == ")":
			block = ""
			continue
		case strings.HasSuffix(line, "("):
			block = strings.TrimSpace(strings.TrimSuffix(line, "("))
			continue
		}
		directive, rest := block, line
		if block == "" {
			directive, rest, _ = strings.Cut(line, " ")
		}
		fields := strings.Fields(rest)
		switch directive {
		case "replace":
			problems = append(problems, "replace "+rest)
		case "require":
			if len(fields) >= 2 && strings.HasPrefix(fields[0], "github.com/togo-framework/") && pseudoVersion.MatchString(fields[1]) {
				problems = append(problems, "pseudo-version "+fields[0]+" "+fields[1])
			}
		}
	}
	return problems
}
