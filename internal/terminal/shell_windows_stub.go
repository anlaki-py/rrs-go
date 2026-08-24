//go:build !windows

package terminal

import "errors"

func selectWindowsShell(string) (shellCommand, error) {
	return shellCommand{}, errors.New("Windows shell selection is unavailable on this platform")
}
