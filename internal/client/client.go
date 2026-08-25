package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/anlaki-py/rrs-go/internal/console"
	"github.com/anlaki-py/rrs-go/internal/protocol"
	"github.com/coder/websocket"
)

const openTimeout = 15 * time.Second

const (
	enterRemoteTerminal = "\x1b[?1049h\x1b[H\x1b[2J" +
		"\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h"
	leaveRemoteTerminal = "\x1b[?1006l\x1b[?1003l\x1b[?1002l\x1b[?1000l" +
		"\x1b[?1049l"
)

type Config struct {
	URL            string
	Token          string
	Insecure       bool
	AllowPlaintext bool
}

func Run(ctx context.Context, config Config, input, output *os.File) (returnErr error) {
	address, err := normalizeURL(config.URL)
	if err != nil {
		return err
	}
	if address.Scheme == "ws" && !loopbackHost(address.Hostname()) && !config.AllowPlaintext {
		return errors.New("remote plaintext WebSocket requires --allow-plaintext")
	}

	connection, err := dial(ctx, address.String(), config)
	if err != nil {
		return err
	}
	defer connection.CloseNow()
	connection.SetReadLimit(protocol.MaxMessageSize)

	state, err := console.EnterRaw(input)
	if err != nil {
		return err
	}
	defer func() {
		if _, leaveErr := output.WriteString(leaveRemoteTerminal); leaveErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("restore local terminal screen: %w", leaveErr))
		}
		if restoreErr := console.Restore(input, state); restoreErr != nil {
			returnErr = errors.Join(returnErr, restoreErr)
		}
		_, _ = output.WriteString("\r\n")
	}()
	if _, err := output.WriteString(enterRemoteTerminal); err != nil {
		return fmt.Errorf("prepare local terminal screen: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := sendSize(sessionCtx, connection, output); err != nil {
		return err
	}

	type pumpResult struct {
		input bool
		err   error
	}
	results := make(chan pumpResult, 3)
	go func() { results <- pumpResult{input: true, err: copyInput(sessionCtx, connection, input)} }()
	go func() { results <- pumpResult{err: copyOutput(sessionCtx, connection, output)} }()
	go func() {
		results <- pumpResult{err: sendResizeEvents(sessionCtx, connection, console.ResizeEvents(sessionCtx, output))}
	}()

	var first pumpResult
	firstReceived := false
	select {
	case first = <-results:
		firstReceived = true
	case <-ctx.Done():
		first.err = context.Cause(ctx)
	}
	if first.input && first.err == nil {
		first.err = connection.Close(websocket.StatusNormalClosure, "Input closed")
	}
	cancel()
	connection.CloseNow()
	if firstReceived {
		second := <-results
		third := <-results
		return errors.Join(
			normalizeCloseError(first.err),
			normalizeCloseError(second.err),
			normalizeCloseError(third.err),
		)
	}
	firstPump := <-results
	secondPump := <-results
	thirdPump := <-results
	return errors.Join(
		normalizeCloseError(first.err),
		normalizeCloseError(firstPump.err),
		normalizeCloseError(secondPump.err),
		normalizeCloseError(thirdPump.err),
	)
}

func dial(ctx context.Context, address string, config Config) (*websocket.Conn, error) {
	requestHeader := make(http.Header)
	if config.Token != "" {
		requestHeader.Set("Authorization", "Bearer "+config.Token)
	}

	httpClient := &http.Client{}
	if config.Insecure {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // User explicitly requested --insecure.
		httpClient.Transport = transport
	}

	openCtx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	connection, response, err := websocket.Dial(openCtx, address, &websocket.DialOptions{
		HTTPClient:   httpClient,
		HTTPHeader:   requestHeader,
		Subprotocols: []string{protocol.Subprotocol},
	})
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("open WebSocket: HTTP %d: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("open WebSocket: %w", err)
	}
	if connection.Subprotocol() != protocol.Subprotocol {
		connection.CloseNow()
		return nil, errors.New("server did not negotiate rrs.v1")
	}
	return connection, nil
}

func copyInput(ctx context.Context, connection *websocket.Conn, input *os.File) error {
	buffer := make([]byte, 32*1024)
	for {
		count, err := console.Read(ctx, input, buffer)
		if count > 0 {
			if writeErr := connection.Write(ctx, websocket.MessageBinary, buffer[:count]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read terminal input: %w", err)
		}
	}
}

func copyOutput(ctx context.Context, connection *websocket.Conn, output io.Writer) error {
	for {
		messageType, message, err := connection.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != websocket.MessageBinary {
			_ = connection.Close(websocket.StatusUnsupportedData, "Expected terminal output")
			return errors.New("server sent a non-binary terminal message")
		}
		if _, err := io.Copy(output, bytes.NewReader(message)); err != nil {
			return fmt.Errorf("write terminal output: %w", err)
		}
	}
}

func sendResizeEvents(ctx context.Context, connection *websocket.Conn, events <-chan protocol.Size) error {
	for size := range events {
		message, err := protocol.EncodeResize(size)
		if err != nil {
			continue
		}
		if err := connection.Write(ctx, websocket.MessageText, message); err != nil {
			return err
		}
	}
	return nil
}

func sendSize(ctx context.Context, connection *websocket.Conn, output *os.File) error {
	size, err := console.Size(output)
	if err != nil {
		size = protocol.Size{Rows: 24, Cols: 80}
	}
	message, err := protocol.EncodeResize(size)
	if err != nil {
		return err
	}
	if err := connection.Write(ctx, websocket.MessageText, message); err != nil {
		return fmt.Errorf("send terminal size: %w", err)
	}
	return nil
}

func normalizeCloseError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return nil
	}
	return err
}
