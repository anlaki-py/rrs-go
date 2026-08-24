package terminal

import (
	"context"
	"errors"
	"io"

	"github.com/anlaki-py/rrs/internal/protocol"
)

var errUnsupported = errors.New("terminal sessions are unsupported on this platform")

type StartOptions struct {
	Size protocol.Size
}

type Session struct {
	platform platformSession
}

type platformSession interface {
	io.ReadWriteCloser
	PID() int
	Resize(protocol.Size) error
	Terminate(context.Context) error
}

func processDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func Start(options StartOptions) (*Session, error) {
	if err := options.Size.Validate(); err != nil {
		return nil, err
	}
	platform, err := startPlatform(options)
	if err != nil {
		return nil, err
	}
	return &Session{platform: platform}, nil
}

func (s *Session) PID() int {
	return s.platform.PID()
}

func (s *Session) Read(p []byte) (int, error) {
	return s.platform.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	return s.platform.Write(p)
}

func (s *Session) Resize(size protocol.Size) error {
	if err := size.Validate(); err != nil {
		return err
	}
	return s.platform.Resize(size)
}

func (s *Session) Terminate(ctx context.Context) error {
	return s.platform.Terminate(ctx)
}

func (s *Session) Close() error {
	return s.platform.Close()
}
