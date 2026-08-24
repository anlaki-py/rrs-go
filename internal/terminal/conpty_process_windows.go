//go:build windows

package terminal

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (p *windowsPTY) Spawn(path string, args, environment []string, directory string, job windows.Handle) (int, windows.Handle, error) {
	application, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, fmt.Errorf("encode application path: %w", err)
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{path}, args...)))
	if err != nil {
		return 0, 0, fmt.Errorf("encode command line: %w", err)
	}
	directoryPointer, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return 0, 0, fmt.Errorf("encode working directory: %w", err)
	}
	environmentBlock, err := createWindowsEnvironmentBlock(environment)
	if err != nil {
		return 0, 0, err
	}

	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.ProcThreadAttributeList = p.attrs.List()
	process := windows.ProcessInformation{}
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(
		application,
		commandLine,
		nil,
		nil,
		false,
		flags,
		&environmentBlock[0],
		directoryPointer,
		&startup.StartupInfo,
		&process,
	); err != nil {
		return 0, 0, fmt.Errorf("create process: %w", err)
	}
	defer windows.CloseHandle(process.Thread)
	runtime.KeepAlive(environmentBlock)
	runtime.KeepAlive(startup)

	if err := p.Release(); err != nil {
		closeStartedWindowsProcess(process.Process)
		return 0, 0, err
	}
	if job != 0 {
		if err := windows.AssignProcessToJobObject(job, process.Process); err != nil {
			closeStartedWindowsProcess(process.Process)
			return 0, 0, fmt.Errorf("assign process to terminal job: %w", err)
		}
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		closeStartedWindowsProcess(process.Process)
		return 0, 0, fmt.Errorf("resume terminal process: %w", err)
	}
	return int(process.ProcessId), process.Process, nil
}

func closeStartedWindowsProcess(process windows.Handle) {
	_ = windows.TerminateProcess(process, 1)
	_, _ = windows.WaitForSingleObject(process, 2_000)
	_ = windows.CloseHandle(process)
}

func createWindowsEnvironmentBlock(environment []string) ([]uint16, error) {
	entries := slices.Clone(environment)
	if len(entries) == 0 {
		return []uint16{0, 0}, nil
	}
	for _, entry := range entries {
		if strings.IndexByte(entry, 0) >= 0 {
			return nil, errors.New("terminal environment contains a NUL byte")
		}
	}
	slices.SortFunc(entries, func(left, right string) int {
		return strings.Compare(strings.ToUpper(left), strings.ToUpper(right))
	})

	block := make([]uint16, 0)
	for _, entry := range entries {
		block = append(block, utf16.Encode([]rune(entry))...)
		block = append(block, 0)
	}
	return append(block, 0), nil
}
