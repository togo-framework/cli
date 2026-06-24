package cmd

import "testing"

func TestSplitAssetRef(t *testing.T) {
	cases := []struct {
		in         string
		kind, name string
		ok         bool
	}{
		{"agent:togo-qa", "agent", "togo-qa", true},
		{"skill:resource", "skill", "resource", true},
		{"owner/repo", "", "", false},
		{"claude", "", "", false},
		{"agent:", "", "", false},
		{"plain", "", "", false},
	}
	for _, c := range cases {
		k, n, ok := splitAssetRef(c.in)
		if ok != c.ok || k != c.kind || n != c.name {
			t.Errorf("splitAssetRef(%q) = (%q,%q,%v), want (%q,%q,%v)", c.in, k, n, ok, c.kind, c.name, c.ok)
		}
	}
}

func TestAssetDirs(t *testing.T) {
	if _, dst, ok := assetDirs("agent"); !ok || dst == "" {
		t.Error("agent should be a known kind with a dst dir")
	}
	if _, _, ok := assetDirs("nope"); ok {
		t.Error("nope should be unknown")
	}
}
