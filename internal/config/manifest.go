package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ManifestFile is the resource manifest filename — the source of truth that all
// aggregate registries are regenerated from.
const ManifestFile = "togo.resources.yaml"

const manifestHeader = "Managed by togo. Source of truth for generated registries."

// Manifest is the parsed togo.resources.yaml.
//
// The manifest is hand-editable, so it may carry keys the CLI does not model.
// Load keeps the original YAML node tree and Save merges the (possibly
// modified) resources back into it, so unknown keys, comments, flow styles and
// the resource order all survive a rewrite.
type Manifest struct {
	Resources []Resource `yaml:"resources"`

	path string     `yaml:"-"`
	doc  *yaml.Node `yaml:"-"`
}

// Resource is one generated entity and its fields. Controller is set once a
// controller (API) is generated for it, so registries only wire resources that
// actually have handlers/resolvers.
type Resource struct {
	Name       string  `yaml:"name"`
	Table      string  `yaml:"table"`
	Fields     []Field `yaml:"fields"`
	Controller bool    `yaml:"controller,omitempty"`
	// Seeder optionally overrides whether the resource is wired into the
	// generated seeder registry (see HasSeeder).
	Seeder *bool `yaml:"seeder,omitempty"`

	// noController records an explicit `controller: false` in the manifest,
	// which marks a hand-managed resource (no generated controller or seeder).
	noController bool
}

// Field describes a single column across all generation targets.
type Field struct {
	Name     string   `yaml:"name"`
	Go       string   `yaml:"go"`
	GQL      string   `yaml:"gql"`
	PG       string   `yaml:"pg"`
	Null     bool     `yaml:"null"`
	Enum     []string `yaml:"enum,omitempty"`
	Relation string   `yaml:"relation,omitempty"`
}

// HasSeeder reports whether the resource belongs in the generated seeder
// registry: an explicit `seeder:` key wins; otherwise resources explicitly
// marked `controller: false` are hand-managed and have no generated seeder.
func (r Resource) HasSeeder() bool {
	if r.Seeder != nil {
		return *r.Seeder
	}
	return !r.noController
}

// LoadManifest reads togo.resources.yaml from the project root, returning an
// empty manifest (not an error) when the file does not yet exist.
func LoadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{path: path}, nil
		}
		return nil, err
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	m.path = path
	return m, nil
}

// ParseManifest parses manifest bytes, keeping the node tree for lossless Save.
func ParseManifest(data []byte) (*Manifest, error) {
	m := &Manifest{}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 { // empty file
		return m, nil
	}
	if err := doc.Decode(m); err != nil {
		return nil, err
	}
	m.doc = &doc
	if seq := m.resourcesNode(false); seq != nil {
		for _, rn := range seq.Content {
			if v := mapValue(rn, "controller"); v != nil && v.Kind == yaml.ScalarNode {
				var b bool
				if v.Decode(&b) == nil && !b {
					if r := m.Find(scalarValue(mapValue(rn, "name"))); r != nil {
						r.noController = true
					}
				}
			}
		}
	}
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

// Upsert appends a new resource or (when force is true) replaces an existing
// entry in place. The existing resource order is preserved; new resources are
// appended. It reports whether the resource already existed.
func (m *Manifest) Upsert(r Resource, force bool) (existed bool, err error) {
	if existing := m.Find(r.Name); existing != nil {
		if !force {
			return true, fmt.Errorf("resource %q already in %s (use --force to replace)", r.Name, ManifestFile)
		}
		if r.Seeder == nil {
			r.Seeder = existing.Seeder
		}
		r.noController = existing.noController && !r.Controller
		*existing = r
		return true, nil
	}
	m.Resources = append(m.Resources, r)
	return false, nil
}

// SetController marks a resource as having a controller (API).
func (m *Manifest) SetController(name string) bool {
	if r := m.Find(name); r != nil {
		r.Controller = true
		r.noController = false
		return true
	}
	return false
}

// Sorted returns a name-sorted copy of the resources, for deterministic
// registry output independent of manifest order.
func (m *Manifest) Sorted() []Resource {
	out := append([]Resource(nil), m.Resources...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Save writes the manifest back to disk.
func (m *Manifest) Save() error {
	data, err := m.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o644)
}

// Marshal renders the manifest, merging the in-memory resources into the
// original YAML so keys the CLI does not model are kept intact.
func (m *Manifest) Marshal() ([]byte, error) {
	if m.doc == nil {
		m.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
		m.doc.Content[0].HeadComment = "# " + manifestHeader
	}
	seq := m.resourcesNode(true)
	byName := make(map[string]*yaml.Node, len(seq.Content))
	for _, rn := range seq.Content {
		byName[scalarValue(mapValue(rn, "name"))] = rn
	}
	for _, r := range m.Resources {
		var fresh yaml.Node
		if err := fresh.Encode(r); err != nil {
			return nil, err
		}
		if r.noController && !r.Controller {
			setMapValue(&fresh, "controller", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"})
		}
		if existing, ok := byName[r.Name]; ok {
			mergeResource(existing, &fresh)
			continue
		}
		seq.Content = append(seq.Content, &fresh)
		byName[r.Name] = &fresh
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(m.doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resourcesNode returns the `resources` sequence, optionally creating it.
func (m *Manifest) resourcesNode(create bool) *yaml.Node {
	if m.doc == nil || len(m.doc.Content) == 0 {
		return nil
	}
	root := m.doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	seq := mapValue(root, "resources")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		return seq
	}
	if !create {
		return nil
	}
	seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	setMapValue(root, "resources", seq)
	return seq
}

// mergeResource applies fresh (the encoded struct) onto existing: keys the
// struct emits are updated (only when their value changed, to keep styles and
// comments), keys it does not emit are left untouched. Fields are merged by
// name so per-field keys like enum/relation survive a --force replace.
func mergeResource(existing, fresh *yaml.Node) {
	for i := 0; i+1 < len(fresh.Content); i += 2 {
		key, val := fresh.Content[i].Value, fresh.Content[i+1]
		cur := mapValue(existing, key)
		if key == "fields" && cur != nil && cur.Kind == yaml.SequenceNode && val.Kind == yaml.SequenceNode {
			mergeFields(cur, val)
			continue
		}
		if cur == nil || !nodeEqual(cur, val) {
			setMapValue(existing, key, val)
		}
	}
}

// mergeFields rebuilds the field list in the fresh order, reusing the
// existing node (with any extra keys) for fields that keep their name.
func mergeFields(existing, fresh *yaml.Node) {
	old := make(map[string]*yaml.Node, len(existing.Content))
	for _, fn := range existing.Content {
		old[scalarValue(mapValue(fn, "name"))] = fn
	}
	out := make([]*yaml.Node, 0, len(fresh.Content))
	for _, fn := range fresh.Content {
		if prev, ok := old[scalarValue(mapValue(fn, "name"))]; ok {
			mergeResource(prev, fn)
			out = append(out, prev)
			continue
		}
		out = append(out, fn)
	}
	existing.Content = out
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func setMapValue(n *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			// Keep comments attached to the replaced value.
			val.HeadComment, val.LineComment, val.FootComment = n.Content[i+1].HeadComment, n.Content[i+1].LineComment, n.Content[i+1].FootComment
			n.Content[i+1] = val
			return
		}
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

func scalarValue(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// nodeEqual compares two nodes structurally, ignoring style and comments.
func nodeEqual(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode && (a.Value != b.Value || a.ShortTag() != b.ShortTag()) {
		return false
	}
	for i := range a.Content {
		if !nodeEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}
