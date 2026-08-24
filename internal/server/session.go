package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/anlaki-py/rrs/internal/protocol"
	"github.com/anlaki-py/rrs/internal/terminal"
	"github.com/coder/websocket"
)

const terminalStopTimeout = 2 * time.Second

type terminalSession interface {
	io.ReadWriteCloser
	Resize(protocol.Size) error
	Terminate(context.Context) error
}

type terminalStarter func(terminal.StartOptions) (terminalSession, error)

func startTerminal(options terminal.StartOptions) (terminalSession, error) {
	return terminal.Start(options)
}

func (s *Server) runSession(serverCtx context.Context, connection *websocket.Conn) error {
	session, err := s.startTerm(terminal.StartOptions{
		Size: protocol.Size{Rows: 24, Cols: 80},
	})
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "Unable to start terminal")
		return fmt.Errorf("start terminal: %w", err)
	}

	ctx, cancel := context.WithCancel(serverCtx)
	results := make(chan error, 2)
	go func() { results <- copyWebSocketToTerminal(ctx, connection, session) }()
	go func() { results <- copyTerminalToWebSocket(ctx, connection, session) }()

	firstErr := <-results
	cancel()
	connection.CloseNow()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), terminalStopTimeout)
	terminateErr := session.Terminate(stopCtx)
	stopCancel()
	closeErr := session.Close()
	secondErr := <-results

	return errors.Join(normalizeSessionError(firstErr), normalizeSessionError(secondErr), terminateErr, closeErr)
}

func copyWebSocketToTerminal(ctx context.Context, connection *websocket.Conn, session terminalSession) error {
	for {
		messageType, message, err := connection.Read(ctx)
		if err != nil {
			return err
		}
		switch messageType {
		case websocket.MessageBinary:
			if _, err := io.Copy(session, bytes.NewReader(message)); err != nil {
				return fmt.Errorf("write terminal input: %w", err)
			}
		case websocket.MessageText:
			size, err := protocol.ParseResize(message)
			if err != nil {
				_ = connection.Close(websocket.StatusInvalidFramePayloadData, "Invalid resize message")
				return err
			}
			if err := session.Resize(size); err != nil {
				return err
			}
		default:
			return errors.New("unsupported WebSocket message type")
		}
	}
}

func copyTerminalToWebSocket(ctx context.Context, connection *websocket.Conn, session terminalSession) error {
	buffer := make([]byte, 32*1024)
	for {
		count, err := session.Read(buffer)
		if count > 0 {
			if writeErr := connection.Write(ctx, websocket.MessageBinary, buffer[:count]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				_ = connection.Close(websocket.StatusNormalClosure, "Terminal exited")
				return nil
			}
			return fmt.Errorf("read terminal output: %w", err)
		}
	}
}

func normalizeSessionError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return nil
	}
	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return nil
	}
	return err
}

func expectedSessionError(err error) bool {
	return err == nil || errors.Is(err, context.Canceled)
}
