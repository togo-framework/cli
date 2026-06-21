package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/togo-framework/cli/internal/ui"
)

// service is one long-running dev process (e.g. the API or the web server).
type service struct {
	name string // label prefix, e.g. "api" / "web"
	bin  string
	args []string
	dir  string
	env  []string
}

// runServices starts every service concurrently, prefixes their output, and
// shuts them all down together on Ctrl-C or when any one exits.
func runServices(services []service) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	cmds := make([]*exec.Cmd, 0, len(services))
	var wg sync.WaitGroup
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	for _, s := range services {
		c := exec.Command(s.bin, s.args...)
		c.Dir = s.dir
		c.Env = s.env
		setProcessGroup(c)

		stdout, err := c.StdoutPipe()
		if err != nil {
			return err
		}
		c.Stderr = c.Stdout // merge streams for simpler prefixing

		if err := c.Start(); err != nil {
			return fmt.Errorf("start %s: %w", s.name, err)
		}
		cmds = append(cmds, c)
		ui.Success("%s started", s.name)

		wg.Add(1)
		go func(name string, r io.Reader) {
			defer wg.Done()
			prefixStream(name, r)
		}(s.name, stdout)

		go func(name string, c *exec.Cmd) {
			_ = c.Wait()
			ui.Warn("%s exited", name)
			closeDone() // one service dying tears the rest down
		}(s.name, c)
	}

	// Wait for a signal or for any service to exit.
	select {
	case <-sigCh:
		ui.Warn("shutting down…")
	case <-done:
	}

	for _, c := range cmds {
		terminate(c)
	}
	wg.Wait()
	return nil
}

// prefixStream writes each line of r with a colored [name] prefix.
func prefixStream(name string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	tag := ui.Tag(name)
	for sc.Scan() {
		fmt.Printf("%s %s\n", tag, sc.Text())
	}
}
