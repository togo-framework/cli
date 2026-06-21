package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ManifestFile is the resource manifest filename — the source of truth that all
// aggregate registries are regenerated from.
const ManifestFile = "togo.resources.yaml"

// Manifest is the parsed togo.resources.yaml.
type Manifest struct {
	Resources []Resource `yaml:"resources"`

	path string `yaml:"-"`
}

// Resource is one generated entity and its fields.
type Resource struct {
	Name   string  `yaml:"name"`
	Table  string  `yaml:"table"`
	Fields []Field `yaml:"fields"`
}

// Field describes a single column across all generation targets.
type Field struct {
	Name string `yaml:"name"`
	Go   string `yaml:"go"`
	GQL  string `yaml:"gql"`
	PG   string `yaml:"pg"`
	Null bool   `yaml:"null"`
}

// LoadManifest reads togo.resources.yaml from the project root, returning an
// empty manifest (not an error) when the file does not yet exist.
func LoadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, ManifestFile)
	m := &Manifest{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, err
	}
	m.path = path
	return m, nil
}

// Find returns the resource with the given name (case-sensitive) or nil.
func (m *Manifest) Find(name string) *Resource {
	for i := range m.Resources {
		if m.Resources[i].Name == name {
			return &m.Resources[i]
		}
	}
	return nil
}

// Upsert inserts or (when force is true) replaces a resource entry, keeping the
// list sorted by name for deterministic registry output. It reports whether the
// resource already existed.
func (m *Manifest) Upsert(r Resource, force bool) (existed bool, err error) {
	if existing := m.Find(r.Name); existing != nil {
		if !force {
			return true, fmt.Errorf("resource %q already in %s (use --force to replace)", r.Name, ManifestFile)
		}
		*existing = r
		m.sort()
		return true, nil
	}
	m.Resources = append(m.Resources, r)
	m.sort()
	return false, nil
}

func (m *Manifest) sort() {
	sort.Slice(m.Resources, func(i, j int) bool { return m.Resources[i].Name < m.Resources[j].Name })
}

// Save writes the manifest back to disk.
func (m *Manifest) Save() error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	header := []byte("# Managed by togo. Source of truth for generated registries.\n")
	return os.WriteFile(m.path, append(header, data...), 0o644)
}
