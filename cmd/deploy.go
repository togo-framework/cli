package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

const deployLong = `Fast push-and-build deploy of a togo app to a server.

Reads the deploy: block from togo.yaml (env vars TOGO_DEPLOY_HOST/USER/PATH/SSH_KEY
override). It builds the app locally (frontend + Go binary), rsyncs the artifact to
the server, then runs the configured restart command over ssh — so an update ships
in seconds.

  deploy:
    host: 152.53.136.52
    user: root
    path: /opt/myapp
    restart: systemctl restart myapp
    # …or multiple environments:
    # default: production
    # targets:
    #   production: { host: …, user: …, path: …, restart: … }
    #   staging:    { host: …, user: …, path: …, restart: … }

Examples:
  togo deploy                 # the inline target, or deploy.default
  togo deploy staging         # a named target under deploy.targets
  togo deploy --dry-run       # print the plan without touching the server
  togo deploy --no-build      # ship the existing build
  togo deploy --remote-build  # rsync the source and build on the server`

func registerDeploy(root *cobra.Command) {
	c := &cobra.Command{
		Use:     "deploy [env]",
		Short:   "Fast push-and-build deploy to the connected server (togo.yaml deploy:)",
		Long:    deployLong,
		GroupID: groupInfra,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			env := ""
			if len(args) == 1 {
				env = args[0]
			}
			noBuild, _ := cmd.Flags().GetBool("no-build")
			remoteBuild, _ := cmd.Flags().GetBool("remote-build")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return runDeploy(proj, env, deployOpts{noBuild: noBuild, remoteBuild: remoteBuild, dryRun: dryRun})
		},
	}
	c.Flags().Bool("no-build", false, "skip the build; ship the existing artifact")
	c.Flags().Bool("remote-build", false, "rsync the source and build on the server")
	c.Flags().Bool("dry-run", false, "print the deploy plan without changing anything")
	root.AddCommand(c)
}

type deployOpts struct {
	noBuild, remoteBuild, dryRun bool
}

// resolveTarget selects the deploy target for env and applies env-var overrides + defaults.
func resolveTarget(proj *config.Project, env string) (config.DeployTarget, string, error) {
	d := proj.Deploy
	var t config.DeployTarget
	name := env
	switch {
	case env != "":
		var ok bool
		if t, ok = d.Targets[env]; !ok {
			return t, "", fmt.Errorf("no deploy target %q under deploy.targets in togo.yaml", env)
		}
	case d.Default != "":
		var ok bool
		if t, ok = d.Targets[d.Default]; !ok {
			return t, "", fmt.Errorf("deploy.default = %q but no such target under deploy.targets", d.Default)
		}
		name = d.Default
	case d.Host != "":
		t, name = d.DeployTarget, "(inline)"
	case len(d.Targets) == 1:
		for k, v := range d.Targets {
			t, name = v, k
		}
	default:
		return t, "", fmt.Errorf("no deploy target configured — add a deploy: block to togo.yaml (host, user, path, restart)")
	}

	if v := os.Getenv("TOGO_DEPLOY_HOST"); v != "" {
		t.Host = v
	}
	if v := os.Getenv("TOGO_DEPLOY_USER"); v != "" {
		t.User = v
	}
	if v := os.Getenv("TOGO_DEPLOY_PATH"); v != "" {
		t.Path = v
	}
	if v := os.Getenv("TOGO_DEPLOY_SSH_KEY"); v != "" {
		t.SSHKey = v
	}

	if t.Port == 0 {
		t.Port = 22
	}
	if t.GOOS == "" {
		t.GOOS = "linux"
	}
	if t.GOARCH == "" {
		t.GOARCH = "amd64"
	}
	if t.Binary == "" {
		if t.Binary = proj.Name; t.Binary == "" {
			t.Binary = "app"
		}
	}
	return t, name, nil
}

func runDeploy(proj *config.Project, env string, o deployOpts) error {
	t, name, err := resolveTarget(proj, env)
	if err != nil {
		return err
	}
	if t.Host == "" || t.User == "" || t.Path == "" {
		return fmt.Errorf("deploy target %s is missing host/user/path — set them in togo.yaml deploy:", name)
	}
	remote := fmt.Sprintf("%s@%s:%s", t.User, t.Host, t.Path)
	remoteBuild := o.remoteBuild || t.RemoteBuild
	binPath := filepath.Join(proj.Root, ".togo", "deploy", t.Binary)
	artifact := binPath
	if t.Artifact != "" {
		if artifact = t.Artifact; !filepath.IsAbs(artifact) {
			artifact = filepath.Join(proj.Root, t.Artifact)
		}
	}

	ui.Step("Deploy → %s  (target: %s)", remote, name)
	if o.dryRun {
		ui.Info("plan (dry-run):")
		ui.Step("1. build   : %s", buildDesc(t, o))
		if remoteBuild {
			ui.Step("2. ship    : rsync ./ → %s, then build on server", remote)
		} else {
			ui.Step("2. ship    : rsync %s → %s", relTo(proj.Root, artifact), remote)
		}
		ui.Step("3. restart : ssh %s@%s %q", t.User, t.Host, t.Restart)
		return nil
	}

	if remoteBuild {
		if err := rsyncTo(proj.Root+string(os.PathSeparator), remote, t); err != nil {
			return err
		}
		build := t.Build
		if build == "" {
			build = fmt.Sprintf("go build -o %s .", t.Binary)
		}
		ui.Step("remote build: %s", build)
		if err := sshRun(t, fmt.Sprintf("cd %s && %s", t.Path, build)); err != nil {
			return err
		}
	} else {
		if !o.noBuild {
			if err := deployBuild(proj, t, binPath); err != nil {
				return err
			}
		}
		if err := rsyncTo(artifact, remote, t); err != nil {
			return err
		}
	}

	if t.Restart != "" {
		ui.Step("restart: %s", t.Restart)
		if err := sshRun(t, t.Restart); err != nil {
			return err
		}
	} else {
		ui.Warn("no deploy.restart configured — skipped restart")
	}
	ui.Success("Deployed %s → %s", proj.Name, t.Host)
	return nil
}

func deployBuild(proj *config.Project, t config.DeployTarget, binPath string) error {
	if t.Build != "" {
		ui.Step("build: %s", t.Build)
		return runShell(proj.Root, t.Build)
	}
	if dir := frontendDir(proj); dir != "" {
		ui.Step("build frontend: npm run build (%s)", relTo(proj.Root, dir))
		if err := runIn(dir, "npm", "run", "build"); err != nil {
			ui.Warn("frontend build failed (%v) — continuing with the binary", err)
		}
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found on PATH (needed to build the binary)")
	}
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return err
	}
	ui.Step("build binary: GOOS=%s GOARCH=%s go build -o %s", t.GOOS, t.GOARCH, relTo(proj.Root, binPath))
	c := exec.Command("go", "build", "-o", binPath, ".")
	c.Dir = proj.Root
	c.Env = append(os.Environ(), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "CGO_ENABLED=0")
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

// sshArgs builds the -p/-i flags shared by ssh and the rsync -e transport.
func sshArgs(t config.DeployTarget) []string {
	var a []string
	if t.Port != 0 && t.Port != 22 {
		a = append(a, "-p", fmt.Sprint(t.Port))
	}
	if t.SSHKey != "" {
		a = append(a, "-i", expandHome(t.SSHKey))
	}
	return a
}

func sshRun(t config.DeployTarget, remoteCmd string) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("ssh not found on PATH")
	}
	args := append(sshArgs(t), fmt.Sprintf("%s@%s", t.User, t.Host), remoteCmd)
	c := exec.Command("ssh", args...)
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}

func rsyncTo(src, remote string, t config.DeployTarget) error {
	if _, err := exec.LookPath("rsync"); err == nil {
		transport := "ssh"
		if extra := sshArgs(t); len(extra) > 0 {
			transport = "ssh " + strings.Join(extra, " ")
		}
		ui.Step("rsync %s → %s", src, remote)
		c := exec.Command("rsync", "-az", "-e", transport, src, remote)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}
	if _, err := exec.LookPath("scp"); err != nil {
		return fmt.Errorf("neither rsync nor scp found on PATH")
	}
	ui.Warn("rsync not found — falling back to scp")
	var args []string
	if t.Port != 0 && t.Port != 22 {
		args = append(args, "-P", fmt.Sprint(t.Port))
	}
	if t.SSHKey != "" {
		args = append(args, "-i", expandHome(t.SSHKey))
	}
	args = append(args, "-r", src, remote)
	c := exec.Command("scp", args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func frontendDir(proj *config.Project) string {
	cands := []string{}
	if proj.Frontend.Dir != "" {
		cands = append(cands, proj.Frontend.Dir)
	}
	cands = append(cands, "web", "frontend")
	for _, d := range cands {
		full := d
		if !filepath.IsAbs(full) {
			full = filepath.Join(proj.Root, d)
		}
		if _, err := os.Stat(filepath.Join(full, "package.json")); err == nil {
			return full
		}
	}
	return ""
}

func runIn(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func runShell(dir, cmdline string) error {
	c := exec.Command("sh", "-c", cmdline)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func relTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

func buildDesc(t config.DeployTarget, o deployOpts) string {
	switch {
	case o.noBuild:
		return "(skipped: --no-build)"
	case o.remoteBuild || t.RemoteBuild:
		return "on the server (remote_build)"
	case t.Build != "":
		return t.Build
	default:
		return fmt.Sprintf("GOOS=%s GOARCH=%s go build (+ frontend if present)", t.GOOS, t.GOARCH)
	}
}
