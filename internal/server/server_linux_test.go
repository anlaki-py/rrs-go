//go:build linux

package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anlaki-py/rrs-go/internal/protocol"
	"github.com/coder/websocket"
)

func TestLinuxShellSession(t *testing.T) {
	service, address, stop := startTestServer(t, Config{AllowUnauthenticated: true, MaxSessions: 1})
	defer stop()

	connection, _, err := websocket.Dial(context.Background(), "ws://"+address, &websocket.DialOptions{
		Subprotocols: []string{protocol.Subprotocol},
	})
	if err != nil {
		t.Fatalf("dial WebSocket: %v", err)
	}

	marker := "RRS_GO_INTEGRATION_MARKER"
	if err := connection.Write(context.Background(), websocket.MessageBinary, []byte("printf '"+marker+"\\n'\n")); err != nil {
		t.Fatalf("write terminal input: %v", err)
	}
	output := readUntil(t, connection, marker)
	if !strings.Contains(output, marker) {
		t.Fatalf("terminal output = %q", output)
	}

	resize, err := protocol.EncodeResize(protocol.Size{Rows: 37, Cols: 91})
	if err != nil {
		t.Fatalf("encode resize: %v", err)
	}
	if err := connection.Write(context.Background(), websocket.MessageText, resize); err != nil {
		t.Fatalf("write resize: %v", err)
	}
	if err := connection.Write(context.Background(), websocket.MessageBinary, []byte("stty size\n")); err != nil {
		t.Fatalf("request terminal size: %v", err)
	}
	if output := readUntil(t, connection, "37 91"); !strings.Contains(output, "37 91") {
		t.Fatalf("resized terminal output = %q", output)
	}

	connection.CloseNow()
	deadline := time.Now().Add(3 * time.Second)
	for service.activeSessions() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if active := service.activeSessions(); active != 0 {
		t.Fatalf("active sessions after disconnect = %d", active)
	}
}

func readUntil(t *testing.T, connection *websocket.Conn, marker string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output strings.Builder
	for !strings.Contains(output.String(), marker) {
		messageType, message, err := connection.Read(ctx)
		if err != nil {
			t.Fatalf("read terminal output: %v; output = %q", err, output.String())
		}
		if messageType != websocket.MessageBinary {
			t.Fatalf("terminal message type = %v", messageType)
		}
		output.Write(message)
	}
	return output.String()
}
