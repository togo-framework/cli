package cmd

import (
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

func TestResolveTarget_InlineDefaults(t *testing.T) {
	proj := &config.Project{Name: "blog", Deploy: config.Deploy{
		DeployTarget: config.DeployTarget{Host: "1.2.3.4", User: "root", Path: "/opt/blog"},
	}}
	tgt, name, err := resolveTarget(proj, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "(inline)" || tgt.Host != "1.2.3.4" {
		t.Fatalf("inline target not resolved: name=%s tgt=%+v", name, tgt)
	}
	if tgt.Port != 22 || tgt.GOOS != "linux" || tgt.GOARCH != "amd64" || tgt.Binary != "blog" {
		t.Fatalf("defaults not applied: %+v", tgt)
	}
}

func TestResolveTarget_DefaultEnv(t *testing.T) {
	proj := &config.Project{Deploy: config.Deploy{
		Default: "prod",
		Targets: map[string]config.DeployTarget{
			"prod":    {Host: "h", User: "u", Path: "p"},
			"staging": {Host: "h2", User: "u", Path: "p"},
		},
	}}
	tgt, name, err := resolveTarget(proj, "")
	if err != nil || name != "prod" || tgt.Host != "h" {
		t.Fatalf("default env not resolved: name=%s tgt=%+v err=%v", name, tgt, err)
	}
	if _, _, err := resolveTarget(proj, "staging"); err != nil {
		t.Fatalf("named env staging should resolve: %v", err)
	}
	if _, _, err := resolveTarget(proj, "nope"); err == nil {
		t.Fatal("unknown env should error")
	}
}

func TestResolveTarget_None(t *testing.T) {
	if _, _, err := resolveTarget(&config.Project{}, ""); err == nil {
		t.Fatal("expected an error when no deploy target is configured")
	}
}
