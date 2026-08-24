package client

import (
	"strings"
	"testing"
)

func TestRemoteTerminalModesUseAlternateScreenAndSGRMouse(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"?1049h", "?1000h", "?1002h", "?1003h", "?1006h"} {
		if !strings.Contains(enterRemoteTerminal, mode) {
			t.Errorf("enterRemoteTerminal does not enable %s", mode)
		}
	}
	for _, mode := range []string{"?1049l", "?1000l", "?1002l", "?1003l", "?1006l"} {
		if !strings.Contains(leaveRemoteTerminal, mode) {
			t.Errorf("leaveRemoteTerminal does not disable %s", mode)
		}
	}
}
