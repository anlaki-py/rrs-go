//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/anlaki-py/rrs/internal/protocol"
	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
)

const windowsGracefulTermination = 750 * time.Millisecond

type windowsSession struct {
	pty           *conpty.ConPty
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

	pty, err := conpty.New(int(options.Size.Cols), int(options.Size.Rows), 0)
	if err != nil {
		return nil, fmt.Errorf("create %s terminal: %w", shellName(shell), err)
	}
	job, err := createWindowsJob()
	if err != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("create terminal job: %w", err)
	}

	pid, process, err := pty.Spawn(shell.path, shell.args, &syscall.ProcAttr{
		Dir: os.Getenv("USERPROFILE"),
		Env: sessionEnvironment(os.Environ()),
	})
	if err != nil {
		_ = windows.CloseHandle(job)
		_ = pty.Close()
		return nil, fmt.Errorf("start %s terminal: %w", shellName(shell), err)
	}
	if err := windows.AssignProcessToJobObject(job, windows.Handle(process)); err != nil {
		_ = windows.TerminateProcess(windows.Handle(process), 1)
		_ = windows.CloseHandle(windows.Handle(process))
		_ = windows.CloseHandle(job)
		_ = pty.Close()
		return nil, fmt.Errorf("assign %s terminal to job: %w", shellName(shell), err)
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
	n, err := s.pty.Read(p)
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) {
		return n, io.EOF
	}
	return n, err
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
		timer := time.NewTimer(windowsGracefulTermination)
		defer timer.Stop()
		select {
		case <-s.waitDone:
		case <-timer.C:
		case <-ctx.Done():
		}
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
			s.pty.Close(),
			windows.CloseHandle(s.process),
			windows.CloseHandle(s.job),
		)
	})
	return s.closeErr
}
