//go:build windows

package terminal

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anlaki-py/rrs/internal/protocol"
)

func TestWindowsTerminalStartsReadsResizesAndStops(t *testing.T) {
	session, err := Start(StartOptions{Size: protocol.Size{Rows: 24, Cols: 80}})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer session.Close()

	if _, err := session.Write([]byte("echo RRS_WINDOWS_OK\r\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	output := make([]byte, 32*1024)
	deadline := time.Now().Add(5 * time.Second)
	var received strings.Builder
	for !strings.Contains(received.String(), "RRS_WINDOWS_OK") {
		if time.Now().After(deadline) {
			t.Fatalf("terminal output did not contain marker: %q", received.String())
		}
		count, err := session.Read(output)
		if count > 0 {
			received.Write(output[:count])
		}
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Read() error = %v", err)
		}
	}

	if err := session.Resize(protocol.Size{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := session.Terminate(ctx); err != nil {
		t.Fatalf("Terminate() error = %v", err)
	}
}
