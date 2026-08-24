//go:build windows

package terminal

import (
	"fmt"
	"os/exec"
)

func selectWindowsShell(configured string) (shellCommand, error) {
	if configured != "" {
		if path, err := exec.LookPath(configured); err == nil {
			return shellCommand{path: path}, nil
		}
	}
	for _, candidate := range []string{"powershell.exe", "pwsh.exe"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return shellCommand{path: path, args: []string{"-NoLogo"}}, nil
		}
	}
	return shellCommand{}, fmt.Errorf("find interactive shell: neither pwsh.exe nor powershell.exe is available")
}
