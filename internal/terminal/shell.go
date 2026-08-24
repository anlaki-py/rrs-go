package terminal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type shellCommand struct {
	path string
	args []string
}

func selectShell(configured string) (shellCommand, error) {
	if runtime.GOOS == "windows" {
		return selectWindowsShell(configured)
	}
	if configured != "" {
		path, err := exec.LookPath(configured)
		if err == nil {
			return shellCommand{path: path, args: []string{"-i"}}, nil
		}
	}

	path, err := exec.LookPath("bash")
	if err != nil {
		return shellCommand{}, fmt.Errorf("find interactive shell: %w", err)
	}
	return shellCommand{path: path, args: []string{"-i"}}, nil
}

func sessionEnvironment(base []string) []string {
	environment := append([]string(nil), base...)
	environment = setEnvironment(environment, "TERM", "xterm-256color")
	return setEnvironment(environment, "COLORTERM", "truecolor")
}

func setEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	for index, entry := range environment {
		if len(entry) >= len(prefix) && strings.EqualFold(entry[:len(prefix)], prefix) {
			environment[index] = prefix + value
			return environment
		}
	}
	return append(environment, prefix+value)
}

func shellName(command shellCommand) string {
	return filepath.Base(command.path)
}
