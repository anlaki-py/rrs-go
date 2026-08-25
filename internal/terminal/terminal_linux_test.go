//go:build linux

package terminal

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/anlaki-py/rrs-go/internal/protocol"
)

func TestLinuxTerminalStartsAndStops(t *testing.T) {
	session, err := Start(StartOptions{Size: protocol.Size{Rows: 24, Cols: 80}})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if session.PID() < 1 {
		t.Fatalf("PID() = %d", session.PID())
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := session.Terminate(stopCtx); err != nil {
		t.Errorf("Terminate() error = %v", err)
	}
	if err := session.Close(); err != nil && err != io.EOF {
		t.Errorf("Close() error = %v", err)
	}
}
