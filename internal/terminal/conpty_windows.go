//go:build windows

package terminal

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsPTY struct {
	api        *conptyAPI
	handle     windows.Handle
	input      windows.Handle
	output     windows.Handle
	attrs      *windows.ProcThreadAttributeListContainer
	release    sync.Once
	releaseErr error
	close      sync.Once
	closeErr   error
}

func newWindowsPTY(width, height int) (_ *windowsPTY, returnErr error) {
	api, err := loadConPTYAPI()
	if err != nil {
		return nil, err
	}
	pty := &windowsPTY{api: api}
	defer func() {
		if returnErr != nil {
			_ = pty.Close()
		}
	}()

	var inputRead, outputWrite windows.Handle
	if err := windows.CreatePipe(&inputRead, &pty.input, nil, 0); err != nil {
		return nil, fmt.Errorf("create pseudo console input pipe: %w", err)
	}
	defer windows.CloseHandle(inputRead)
	if err := windows.CreatePipe(&pty.output, &outputWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create pseudo console output pipe: %w", err)
	}
	defer windows.CloseHandle(outputWrite)

	size := windows.Coord{X: int16(width), Y: int16(height)}
	packedSize := *(*uint32)(unsafe.Pointer(&size))
	if err := callHRESULT(api.create, uintptr(packedSize), uintptr(inputRead), uintptr(outputWrite), 0, uintptr(unsafe.Pointer(&pty.handle))); err != nil {
		return nil, fmt.Errorf("create pseudo console: %w", err)
	}

	pty.attrs, err = windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, fmt.Errorf("allocate pseudo console process attributes: %w", err)
	}
	if err := updatePseudoConsoleAttribute(pty.attrs.List(), pty.handle); err != nil {
		return nil, fmt.Errorf("set pseudo console process attribute: %w", err)
	}
	return pty, nil
}

var updateProcThreadAttribute = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")

func updatePseudoConsoleAttribute(list *windows.ProcThreadAttributeList, handle windows.Handle) error {
	result, _, callErr := updateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(list)),
		0,
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(handle),
		unsafe.Sizeof(handle),
		0,
		0,
	)
	if result == 0 {
		return callErr
	}
	return nil
}

func (p *windowsPTY) Release() error {
	p.release.Do(func() {
		p.releaseErr = callHRESULT(p.api.release, uintptr(p.handle))
		if p.releaseErr != nil {
			p.releaseErr = fmt.Errorf("release pseudo console reference: %w", p.releaseErr)
		}
	})
	return p.releaseErr
}

func (p *windowsPTY) Read(buffer []byte) (int, error) {
	var count uint32
	err := windows.ReadFile(p.output, buffer, &count, nil)
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_NO_DATA) {
		err = io.EOF
	}
	return int(count), err
}

func (p *windowsPTY) Write(buffer []byte) (int, error) {
	var count uint32
	err := windows.WriteFile(p.input, buffer, &count, nil)
	return int(count), err
}

func (p *windowsPTY) Resize(width, height int) error {
	size := windows.Coord{X: int16(width), Y: int16(height)}
	packedSize := *(*uint32)(unsafe.Pointer(&size))
	if err := callHRESULT(p.api.resize, uintptr(p.handle), uintptr(packedSize)); err != nil {
		return fmt.Errorf("resize pseudo console: %w", err)
	}
	return nil
}

func (p *windowsPTY) Close() error {
	p.close.Do(func() {
		if p.attrs != nil {
			p.attrs.Delete()
		}
		if p.handle != 0 {
			p.api.close.Call(uintptr(p.handle))
		}
		p.closeErr = errors.Join(
			closeWindowsHandle(p.input),
			closeWindowsHandle(p.output),
		)
	})
	return p.closeErr
}

func closeWindowsHandle(handle windows.Handle) error {
	if handle == 0 || handle == windows.InvalidHandle {
		return nil
	}
	return windows.CloseHandle(handle)
}
