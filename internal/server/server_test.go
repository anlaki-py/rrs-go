package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/anlaki-py/rrs-go/internal/protocol"
	"github.com/anlaki-py/rrs-go/internal/terminal"
	"github.com/coder/websocket"
)

func TestNewValidatesAuthenticationAndLimits(t *testing.T) {
	t.Parallel()

	tests := []Config{
		{MaxSessions: 1},
		{Token: "secret", AllowUnauthenticated: true, MaxSessions: 1},
		{AllowUnauthenticated: true, MaxSessions: 0},
	}
	for _, config := range tests {
		if _, err := New(config, nil); err == nil {
			t.Errorf("New(%#v) error = nil", config)
		}
	}
}

func TestServerHealthAndAuthentication(t *testing.T) {
	t.Parallel()

	service, address, stop := startTestServer(t, Config{Token: "secret", MaxSessions: 2})
	defer stop()
	_ = service

	response, err := http.Get("http://" + address + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatalf("read /healthz: %v", readErr)
	}
	if response.StatusCode != http.StatusOK || string(body) != "OK\n" {
		t.Fatalf("GET /healthz = %d %q", response.StatusCode, body)
	}

	_, response, err = websocket.Dial(context.Background(), "ws://"+address, &websocket.DialOptions{
		Subprotocols: []string{protocol.Subprotocol},
	})
	if err == nil {
		t.Fatal("unauthenticated WebSocket succeeded")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %#v, error = %v", response, err)
	}
}

func TestServerRequiresSubprotocol(t *testing.T) {
	t.Parallel()

	_, address, stop := startTestServer(t, Config{AllowUnauthenticated: true, MaxSessions: 1})
	defer stop()

	_, response, err := websocket.Dial(context.Background(), "ws://"+address, nil)
	if err == nil {
		t.Fatal("unversioned WebSocket succeeded")
	}
	if response == nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unversioned status = %#v, error = %v", response, err)
	}
}

func TestMalformedControlMessageNeverReachesTerminal(t *testing.T) {
	t.Parallel()

	fake := newFakeTerminal()
	service, address, stop := startTestServerWithStarter(
		t,
		Config{AllowUnauthenticated: true, MaxSessions: 1},
		func(terminal.StartOptions) (terminalSession, error) { return fake, nil },
	)
	defer stop()
	_ = service

	connection, _, err := websocket.Dial(context.Background(), "ws://"+address, &websocket.DialOptions{
		Subprotocols: []string{protocol.Subprotocol},
	})
	if err != nil {
		t.Fatalf("dial WebSocket: %v", err)
	}
	defer connection.CloseNow()
	if err := connection.Write(context.Background(), websocket.MessageText, []byte("not JSON")); err != nil {
		t.Fatalf("write malformed message: %v", err)
	}

	readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err = connection.Read(readCtx)
	if status := websocket.CloseStatus(err); status != websocket.StatusInvalidFramePayloadData {
		t.Fatalf("close status = %v, error = %v", status, err)
	}
	if fake.written.String() != "" {
		t.Fatalf("terminal received malformed control message %q", fake.written.String())
	}
}

func startTestServer(t *testing.T, config Config) (*Server, string, func()) {
	t.Helper()
	return startTestServerWithStarter(t, config, startTerminal)
}

func startTestServerWithStarter(t *testing.T, config Config, starter terminalStarter) (*Server, string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	service, err := New(config, slog.New(slog.DiscardHandler))
	if err != nil {
		listener.Close()
		t.Fatalf("New() error = %v", err)
	}
	service.startTerm = starter
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- service.Serve(ctx, listener) }()

	stop := func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Errorf("Serve() error = %v", err)
			}
		case <-time.After(6 * time.Second):
			t.Error("Serve() did not stop")
		}
	}
	return service, listener.Addr().String(), stop
}

type fakeTerminal struct {
	written   bytes.Buffer
	closed    chan struct{}
	closeOnce sync.Once
}

func newFakeTerminal() *fakeTerminal {
	return &fakeTerminal{closed: make(chan struct{})}
}

func (f *fakeTerminal) Read([]byte) (int, error) {
	<-f.closed
	return 0, io.EOF
}

func (f *fakeTerminal) Write(message []byte) (int, error) {
	return f.written.Write(message)
}

func (f *fakeTerminal) Resize(protocol.Size) error {
	return nil
}

func (f *fakeTerminal) Terminate(context.Context) error {
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeTerminal) Close() error {
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}
