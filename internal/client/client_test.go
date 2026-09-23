package client

import (
	"strings"
	"testing"
)

func TestRemoteTerminalModesUseAlternateScreenWithoutForcedMouse(t *testing.T) {
	t.Parallel()

	if !strings.Contains(enterRemoteTerminal, "?1049h") {
		t.Errorf("enterRemoteTerminal does not enable %s", "?1049h")
	}
	for _, mode := range []string{"?1000h", "?1002h", "?1003h", "?1006h"} {
		if strings.Contains(enterRemoteTerminal, mode) {
			t.Errorf("enterRemoteTerminal must not force %s", mode)
		}
	}
	for _, mode := range []string{"?1049l", "?1000l", "?1002l", "?1003l", "?1006l"} {
		if !strings.Contains(leaveRemoteTerminal, mode) {
			t.Errorf("leaveRemoteTerminal does not disable %s", mode)
		}
	}
}
