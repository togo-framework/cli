package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

const devVersion = "0.1.0-dev"

// Version metadata, overridable at build time via -ldflags. Builds without
// ldflags (e.g. `go install …@vX.Y.Z`) fall back to the Go build info.
var (
	Version = devVersion
	Commit  = "none"
	Date    = "unknown"
)

var readBuildInfo = debug.ReadBuildInfo

func registerVersion(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "version",
		Short:   "Print the togo CLI version",
		GroupID: groupProject,
		Run: func(cmd *cobra.Command, args []string) {
			info, _ := readBuildInfo()
			v, c, d := buildVersion(Version, Commit, Date, info)
			fmt.Fprintf(cmd.OutOrStdout(), "togo %s (commit %s, built %s, %s)\n", v, c, d, runtime.Version())
		},
	})
}

// buildVersion fills in whatever ldflags left at its default from the build
// info: the module version (without its "v", matching goreleaser), and the
// VCS revision and time when the binary was built from a checkout.
func buildVersion(version, commit, date string, info *debug.BuildInfo) (string, string, string) {
	if info == nil {
		return version, commit, date
	}
	if v := info.Main.Version; version == devVersion && v != "" && v != "(devel)" {
		version = strings.TrimPrefix(v, "v")
	}
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && commit == "none":
			commit = s.Value
		case s.Key == "vcs.time" && date == "unknown":
			date = s.Value
		}
	}
	return version, commit, date
}
