// Package config loads and represents a togo project's configuration
// (togo.yaml) and the resource manifest (togo.resources.yaml) that drives
// code generation.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by Load when no togo.yaml can be located.
var ErrNotFound = errors.New("togo.yaml not found")

// ConfigFile is the canonical project config filename.
const ConfigFile = "togo.yaml"

// Project mirrors togo.yaml. Unknown keys are ignored so the schema can grow.
type Project struct {
	Name     string         `yaml:"name"`
	Module   string         `yaml:"module"`
	Database Database       `yaml:"database"`
	API      API            `yaml:"api"`
	Auth     Auth           `yaml:"auth"`
	Plugins  []string       `yaml:"plugins"`
	Frontend Frontend       `yaml:"frontend"`
	Deploy   Deploy         `yaml:"deploy"`
	Extra    map[string]any `yaml:",inline"`

	// Root is the absolute directory containing togo.yaml (not serialized).
	Root string `yaml:"-"`
}

// Database describes the default connection. URL supports ${ENV} interpolation
// resolved at runtime by the framework, so the CLI stores it verbatim.
type Database struct {
	Driver string `yaml:"driver"`
	URL    string `yaml:"url"`
}

// API holds the mount points for the generated interfaces.
type API struct {
	GraphQL string `yaml:"graphql"`
	REST    string `yaml:"rest"`
	Docs    string `yaml:"docs"`
}

// Auth selects the authentication provider (supabase by default).
type Auth struct {
	Provider string `yaml:"provider"`
}

// Frontend configures the Next.js app location.
type Frontend struct {
	Dir string `yaml:"dir"`
}

// Deploy configures `togo deploy` — fast push-and-build deploy to a server.
// It accepts a single inline target and/or named environments under `targets`.
//
//	deploy:
//	  host: 152.53.136.52      # single inline target
//	  user: root
//	  path: /opt/myapp
//	  restart: systemctl restart myapp
//	  # …or multiple environments:
//	  default: production
//	  targets:
//	    production: { host: …, user: …, path: …, restart: … }
//	    staging:    { host: …, user: …, path: …, restart: … }
type Deploy struct {
	DeployTarget `yaml:",inline"`
	Default      string                  `yaml:"default"`
	Targets      map[string]DeployTarget `yaml:"targets"`
}

// DeployTarget is one server/environment for `togo deploy`. Env vars
// TOGO_DEPLOY_HOST/USER/PATH/SSH_KEY override the resolved values at deploy time.
type DeployTarget struct {
	// Provider selects the deploy backend. Empty / "ssh" / "vps" = the built-in
	// rsync+ssh push-and-build. Any other value (docker, kubernetes, terraform,
	// gcp, aws, digitalocean, …) routes through the `togo-framework/deploy` plugin
	// and its matching `deploy-<provider>` driver (install with `togo install`).
	Provider    string         `yaml:"provider"`
	Host        string         `yaml:"host"`
	User        string         `yaml:"user"`
	Path        string         `yaml:"path"`
	Port        int            `yaml:"port"`
	SSHKey      string         `yaml:"ssh_key"`
	Build       string         `yaml:"build"`        // local build command; default = frontend build + `go build`
	Artifact    string         `yaml:"artifact"`     // file/dir to ship; default = the built binary
	Restart     string         `yaml:"restart"`      // remote command run after upload
	RemoteBuild bool           `yaml:"remote_build"` // rsync source and build on the server
	GOOS        string         `yaml:"goos"`         // target OS for the binary (default linux)
	GOARCH      string         `yaml:"goarch"`       // target arch (default amd64)
	Binary      string         `yaml:"binary"`       // output binary name (default = project name)
	Image       string         `yaml:"image"`        // container image ref (docker/kubernetes providers)
	Domain      string         `yaml:"domain"`       // public domain (cloud/proxy providers)
	Region      string         `yaml:"region"`       // cloud region
	Options     map[string]any `yaml:"options"`      // provider-specific knobs passed through to the driver
}

// Load finds and parses togo.yaml. If path is empty it searches from the current
// working directory upward to the filesystem root.
func Load(path string) (*Project, error) {
	if path == "" {
		found, err := find()
		if err != nil {
			return nil, err
		}
		path = found
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var p Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p.Root = filepath.Dir(abs)
	p.applyDefaults()
	return &p, nil
}

func (p *Project) applyDefaults() {
	if p.Database.Driver == "" {
		p.Database.Driver = "postgres"
	}
	if p.API.GraphQL == "" {
		p.API.GraphQL = "/graphql"
	}
	if p.API.REST == "" {
		p.API.REST = "/api"
	}
	if p.API.Docs == "" {
		p.API.Docs = "/docs"
	}
	if p.Auth.Provider == "" {
		p.Auth.Provider = "supabase"
	}
	if p.Frontend.Dir == "" {
		p.Frontend.Dir = "web"
	}
}

// find walks up from cwd looking for togo.yaml.
func find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}
