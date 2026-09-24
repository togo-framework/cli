package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddPlugin appends pkg to the top-level `plugins:` list of a togo.yaml
// document and returns the new bytes. It locates the list through the YAML
// node tree but edits the text surgically, so key order, indentation, blank
// lines and comments elsewhere in the file are left exactly as they were.
// changed is false when pkg is already listed.
func AddPlugin(data []byte, pkg string) (out []byte, changed bool, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, err
	}
	nl := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	join := func(ls []string) []byte { return []byte(strings.Join(ls, nl)) }

	var root *yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root != nil && root.Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("%s: top level is not a mapping", ConfigFile)
	}

	var key, seq *yaml.Node
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "plugins" {
				key, seq = root.Content[i], root.Content[i+1]
				break
			}
		}
	}

	// No plugins key: append a new block at the end of the file.
	if key == nil {
		body := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		block := []string{"plugins:", "  - " + pkg, ""}
		if body != "" {
			block = append([]string{body, ""}, block...)
		}
		return join(block), true, nil
	}

	if seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if item.Value == pkg {
				return data, false, nil
			}
		}
	}

	keyLine := key.Line - 1
	keyIndent := lines[keyLine][:key.Column-1]
	comment := ""
	for _, c := range []string{key.LineComment, seq.LineComment} {
		if c != "" {
			comment = " " + c
			break
		}
	}

	switch {
	case seq.Kind == yaml.SequenceNode && seq.Style&yaml.FlowStyle == 0 && len(seq.Content) > 0 &&
		seq.Content[len(seq.Content)-1].Kind == yaml.ScalarNode:
		// Block list: insert after the last item using the same "  - " prefix.
		last := seq.Content[len(seq.Content)-1]
		prefix := lines[last.Line-1][:last.Column-1]
		ins := []string{prefix + quoteLike(last, pkg)}
		at := last.Line // index of the line after the last item
		lines = append(lines[:at], append(ins, lines[at:]...)...)
	case seq.Kind == yaml.SequenceNode && seq.Line == key.Line:
		// Single-line flow list (`plugins: [a, b]` or `plugins: []`).
		items := make([]string, 0, len(seq.Content)+1)
		for _, it := range seq.Content {
			items = append(items, quoteLike(it, it.Value))
		}
		items = append(items, pkg)
		lines[keyLine] = keyIndent + "plugins: [" + strings.Join(items, ", ") + "]" + comment
	case seq.Kind == yaml.ScalarNode && seq.Tag == "!!null" && seq.Line == key.Line:
		// Empty value (`plugins:` or `plugins: ~`): turn it into a block list.
		ins := []string{keyIndent + "plugins:" + comment, keyIndent + "  - " + pkg}
		lines = append(lines[:keyLine], append(ins, lines[keyLine+1:]...)...)
	default:
		return nil, false, fmt.Errorf("%s: unsupported `plugins:` layout; add %s manually", ConfigFile, pkg)
	}
	return join(lines), true, nil
}

// quoteLike renders v with the same quoting style as the reference scalar.
func quoteLike(ref *yaml.Node, v string) string {
	switch {
	case ref.Style&yaml.DoubleQuotedStyle != 0:
		return `"` + v + `"`
	case ref.Style&yaml.SingleQuotedStyle != 0:
		return "'" + v + "'"
	}
	return v
}
