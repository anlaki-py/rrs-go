//go:build linux

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/anlaki-py/rrs/internal/protocol"
	"github.com/creack/pty"
)

const gracefulTermination = 750 * time.Millisecond

type linuxSession struct {
	pty           *os.File
	command       *exec.Cmd
	waitDone      chan struct{}
	closeOnce     sync.Once
	closeErr      error
	terminateOnce sync.Once
}

func startPlatform(options StartOptions) (platformSession, error) {
	shell, err := selectShell(os.Getenv("SHELL"))
	if err != nil {
		return nil, err
	}

	command := exec.Command(shell.path, shell.args...)
	command.Env = sessionEnvironment(os.Environ())

	ptyFile, err := pty.StartWithSize(command, &pty.Winsize{
		Rows: options.Size.Rows,
		Cols: options.Size.Cols,
	})
	if err != nil {
		return nil, fmt.Errorf("start %s terminal: %w", shellName(shell), err)
	}

	session := &linuxSession{
		pty:      ptyFile,
		command:  command,
		waitDone: make(chan struct{}),
	}
	go func() {
		_ = command.Wait()
		close(session.waitDone)
	}()
	return session, nil
}

func (s *linuxSession) PID() int {
	return s.command.Process.Pid
}

func (s *linuxSession) Read(p []byte) (int, error) {
	n, err := s.pty.Read(p)
	if errors.Is(err, syscall.EIO) {
		err = io.EOF
	}
	return n, err
}

func (s *linuxSession) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

func (s *linuxSession) Resize(size protocol.Size) error {
	if err := pty.Setsize(s.pty, &pty.Winsize{Rows: size.Rows, Cols: size.Cols}); err != nil {
		return fmt.Errorf("resize terminal: %w", err)
	}
	return nil
}

func (s *linuxSession) Terminate(ctx context.Context) error {
	s.terminateOnce.Do(func() {
		if processDone(s.waitDone) {
			return
		}

		_ = syscall.Kill(-s.PID(), syscall.SIGTERM)
		timer := time.NewTimer(gracefulTermination)
		defer timer.Stop()
		select {
		case <-s.waitDone:
			return
		case <-timer.C:
		case <-ctx.Done():
		}
		_ = syscall.Kill(-s.PID(), syscall.SIGKILL)
	})

	select {
	case <-s.waitDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("terminate terminal: %w", context.Cause(ctx))
	}
}

func (s *linuxSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.pty.Close()
	})
	return s.closeErr
}

func processDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
