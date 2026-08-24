package terminal

import (
	"path/filepath"
	"testing"
)

func TestSelectShellFallsBackToBash(t *testing.T) {
	t.Parallel()

	shell, err := selectShell("/definitely/missing/rrs-shell")
	if err != nil {
		t.Fatalf("selectShell() error = %v", err)
	}
	if filepath.Base(shell.path) != "bash" {
		t.Fatalf("selectShell() path = %q, want Bash", shell.path)
	}
	if len(shell.args) != 1 || shell.args[0] != "-i" {
		t.Fatalf("selectShell() args = %#v, want [-i]", shell.args)
	}
}

func TestSessionEnvironmentOverridesTerminalValues(t *testing.T) {
	t.Parallel()

	environment := sessionEnvironment([]string{"PATH=/bin", "TERM=old"})
	want := map[string]bool{"PATH=/bin": true, "TERM=xterm-256color": true, "COLORTERM=truecolor": true}
	for _, entry := range environment {
		delete(want, entry)
	}
	if len(want) != 0 {
		t.Fatalf("sessionEnvironment() missing entries: %#v", want)
	}
}
