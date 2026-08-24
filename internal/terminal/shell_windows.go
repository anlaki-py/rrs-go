//go:build windows

package terminal

import (
	"fmt"
	"os"
	"os/exec"
)

func selectWindowsShell(configured string) (shellCommand, error) {
	if configured != "" {
		if path, err := exec.LookPath(configured); err == nil {
			return shellCommand{path: path, args: []string{}}, nil
		}
	}
	configured = os.Getenv("COMSPEC")
	if configured == "" {
		configured = "cmd.exe"
	}
	path, err := exec.LookPath(configured)
	if err != nil {
		return shellCommand{}, fmt.Errorf("find interactive shell: %w", err)
	}
	return shellCommand{path: path, args: []string{}}, nil
}
