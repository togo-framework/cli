package cmd

import (
	"bytes"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	vcs := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "329abd75f83c2e4ceb852c245439ce248ae8faeb"},
		{Key: "vcs.time", Value: "2026-10-08T11:27:10Z"},
	}
	cases := []struct {
		name                string
		version, commit, dt string
		info                *debug.BuildInfo
		want                [3]string
	}{
		{
			name:    "go install from the module proxy",
			version: devVersion, commit: "none", dt: "unknown",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.2.33"}},
			want: [3]string{"0.2.33", "none", "unknown"},
		},
		{
			name:    "ldflags win over build info",
			version: "0.2.33", commit: "329abd7", dt: "2026-10-08T11:27:41Z",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}, Settings: vcs},
			want: [3]string{"0.2.33", "329abd7", "2026-10-08T11:27:41Z"},
		},
		{
			name:    "local build from a checkout",
			version: devVersion, commit: "none", dt: "unknown",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs},
			want: [3]string{devVersion, "329abd75f83c2e4ceb852c245439ce248ae8faeb", "2026-10-08T11:27:10Z"},
		},
		{
			name:    "no module version",
			version: devVersion, commit: "none", dt: "unknown",
			info: &debug.BuildInfo{},
			want: [3]string{devVersion, "none", "unknown"},
		},
		{
			name:    "no build info",
			version: devVersion, commit: "none", dt: "unknown",
			want: [3]string{devVersion, "none", "unknown"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, cm, d := buildVersion(c.version, c.commit, c.dt, c.info)
			if got := [3]string{v, cm, d}; got != c.want {
				t.Fatalf("buildVersion = %q, want %q", got, c.want)
			}
		})
	}
}

// `togo version` on a `go install …@v0.2.33` binary (no ldflags) reports the
// module version, not the 0.1.0-dev default.
func TestVersionCommandUsesModuleVersion(t *testing.T) {
	orig := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.2.33"}}, true
	}
	t.Cleanup(func() { readBuildInfo = orig })

	versionCmd, _, err := rootCmd.Find([]string{"version"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })
	versionCmd.Run(versionCmd, nil)

	want := "togo 0.2.33 (commit none, built unknown, " + runtime.Version() + ")"
	if got := strings.TrimSpace(out.String()); got != want {
		t.Fatalf("togo version = %q, want %q", got, want)
	}
}
