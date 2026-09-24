package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A realistic togo.yaml: non-alphabetical keys, 2-space indent, blank lines, comments.
const togoYAML = `# togo project config
name: wasla
module: github.com/wasla-platform/wasla

database:
  driver: pgx # dialect-aware
  url: ${DATABASE_URL}

api:
  graphql: /graphql
  rest: /api
  docs: /docs

plugins:
  - github.com/togo-framework/data
  # installed by hand

frontend:
  dir: web
`

// Regression: `togo install` must only append to plugins:, not re-marshal the
// whole file (which sorted keys, switched to 4-space indent and dropped blank lines).
func TestAddPluginPreservesLayout(t *testing.T) {
	out, changed, err := AddPlugin([]byte(togoYAML), "github.com/togo-framework/auth")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	want := strings.Replace(togoYAML, "  - github.com/togo-framework/data\n",
		"  - github.com/togo-framework/data\n  - github.com/togo-framework/auth\n", 1)
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	var p Project
	if err := yaml.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Plugins) != 2 || p.Plugins[1] != "github.com/togo-framework/auth" {
		t.Errorf("plugins = %v", p.Plugins)
	}

	again, changed, err := AddPlugin(out, "github.com/togo-framework/auth")
	if err != nil || changed || string(again) != string(out) {
		t.Errorf("re-adding an installed plugin must be a no-op (changed=%v err=%v)", changed, err)
	}
}

func TestAddPluginLayouts(t *testing.T) {
	const pkg = "github.com/togo-framework/auth"
	cases := map[string]struct{ in, want string }{
		"missing key": {
			"name: app\n\napi:\n  rest: /api\n",
			"name: app\n\napi:\n  rest: /api\n\nplugins:\n  - " + pkg + "\n",
		},
		"empty flow": {
			"name: app\nplugins: [] # none yet\napi:\n  rest: /api\n",
			"name: app\nplugins: [" + pkg + "] # none yet\napi:\n  rest: /api\n",
		},
		"flow with items": {
			"name: app\nplugins: [\"a/b\"]\n",
			"name: app\nplugins: [\"a/b\", " + pkg + "]\n",
		},
		"null value": {
			"name: app\nplugins:\n\nfrontend:\n  dir: web\n",
			"name: app\nplugins:\n  - " + pkg + "\n\nfrontend:\n  dir: web\n",
		},
		"quoted block items": {
			"plugins:\n    - \"a/b\"\nname: app\n",
			"plugins:\n    - \"a/b\"\n    - \"" + pkg + "\"\nname: app\n",
		},
		"crlf": {
			"name: app\r\n\r\nplugins:\r\n  - a/b\r\n",
			"name: app\r\n\r\nplugins:\r\n  - a/b\r\n  - " + pkg + "\r\n",
		},
		"empty file": {"", "plugins:\n  - " + pkg + "\n"},
	}
	for name, c := range cases {
		out, changed, err := AddPlugin([]byte(c.in), pkg)
		if err != nil || !changed {
			t.Errorf("%s: changed=%v err=%v", name, changed, err)
			continue
		}
		if string(out) != c.want {
			t.Errorf("%s:\n got %q\nwant %q", name, out, c.want)
		}
	}
}
