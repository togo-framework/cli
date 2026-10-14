package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// withStdin swaps os.Stdin for f during the test.
func withStdin(t *testing.T, f *os.File) {
	t.Helper()
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old })
}

// A redirected stdin (/dev/null, NUL, a file or a pipe) is not a terminal, so
// `togo new` must not prompt or start docker on its own (#4).
func TestIsInteractive_RedirectedStdin(t *testing.T) {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()

	regular := filepath.Join(t.TempDir(), "in.txt")
	if err := os.WriteFile(regular, []byte("y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(regular)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()

	for name, f := range map[string]*os.File{
		os.DevNull:     devnull,
		"regular file": file,
		"pipe":         pr,
	} {
		withStdin(t, f)
		if isInteractive() {
			t.Errorf("isInteractive() = true for stdin %s; want false", name)
		}
	}
}
