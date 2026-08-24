//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/anlaki-py/rrs/internal/protocol"
	"golang.org/x/sys/windows"
)

type windowsSession struct {
	pty           *windowsPTY
	process       windows.Handle
	job           windows.Handle
	pid           int
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

	pty, err := newWindowsPTY(int(options.Size.Cols), int(options.Size.Rows))
	if err != nil {
		return nil, fmt.Errorf("create %s terminal: %w", shellName(shell), err)
	}
	job, err := createWindowsJob()
	if err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("create terminal job: %w", err)
	}

	directory := os.Getenv("USERPROFILE")
	if directory == "" {
		directory, err = os.Getwd()
		if err != nil {
			_ = windows.CloseHandle(job)
			_ = pty.Close()
			return nil, fmt.Errorf("determine terminal working directory: %w", err)
		}
	}
	pid, process, err := pty.Spawn(shell.path, shell.args, sessionEnvironment(os.Environ()), directory, job)
	if err != nil {
		_ = windows.CloseHandle(job)
		_ = pty.Close()
		return nil, fmt.Errorf("start %s terminal: %w", shellName(shell), err)
	}

	session := &windowsSession{
		pty:      pty,
		process:  windows.Handle(process),
		job:      job,
		pid:      pid,
		waitDone: make(chan struct{}),
	}
	go session.wait()
	return session, nil
}

func createWindowsJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func (s *windowsSession) wait() {
	_, _ = windows.WaitForSingleObject(s.process, windows.INFINITE)
	close(s.waitDone)
}

func (s *windowsSession) PID() int { return s.pid }

func (s *windowsSession) Read(p []byte) (int, error) {
	return s.pty.Read(p)
}

func (s *windowsSession) Write(p []byte) (int, error) { return s.pty.Write(p) }

func (s *windowsSession) Resize(size protocol.Size) error {
	if err := s.pty.Resize(int(size.Cols), int(size.Rows)); err != nil {
		return fmt.Errorf("resize terminal: %w", err)
	}
	return nil
}

func (s *windowsSession) Terminate(ctx context.Context) error {
	s.terminateOnce.Do(func() {
		if processDone(s.waitDone) {
			return
		}
		_ = windows.TerminateJobObject(s.job, 1)
	})
	select {
	case <-s.waitDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("terminate terminal: %w", context.Cause(ctx))
	}
}

func (s *windowsSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = errors.Join(
			windows.CloseHandle(s.job),
			s.pty.Close(),
			windows.CloseHandle(s.process),
		)
	})
	return s.closeErr
}
