//go:build windows

package console

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/anlaki-py/rrs-go/internal/protocol"
	"golang.org/x/sys/windows"
)

const windowsInputPollInterval = 50 * time.Millisecond

type windowsState struct {
	input                 windows.Handle
	output                windows.Handle
	inputMode, outputMode uint32
}

func enterRaw(input *os.File) (platformState, error) {
	inputHandle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return nil, fmt.Errorf("enter raw mode: get input handle: %w", err)
	}
	outputHandle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return nil, fmt.Errorf("enter raw mode: get output handle: %w", err)
	}
	var inputMode, outputMode uint32
	if err := windows.GetConsoleMode(inputHandle, &inputMode); err != nil {
		return nil, fmt.Errorf("enter raw mode: read input mode: %w", err)
	}
	if err := windows.GetConsoleMode(outputHandle, &outputMode); err != nil {
		return nil, fmt.Errorf("enter raw mode: read output mode: %w", err)
	}
	newInputMode := inputMode | windows.ENABLE_VIRTUAL_TERMINAL_INPUT | windows.ENABLE_EXTENDED_FLAGS
	newInputMode &^= windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_QUICK_EDIT_MODE
	newOutputMode := outputMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
	if err := windows.SetConsoleMode(inputHandle, newInputMode); err != nil {
		return nil, fmt.Errorf("enter raw mode: set input mode: %w", err)
	}
	if err := windows.SetConsoleMode(outputHandle, newOutputMode); err != nil {
		_ = windows.SetConsoleMode(inputHandle, inputMode)
		return nil, fmt.Errorf("enter raw mode: set output mode: %w", err)
	}
	return &windowsState{input: inputHandle, output: outputHandle, inputMode: inputMode, outputMode: outputMode}, nil
}

func (s *windowsState) restore(_ *os.File) error {
	var inputErr, outputErr error
	if err := windows.SetConsoleMode(s.input, s.inputMode); err != nil {
		inputErr = fmt.Errorf("restore input mode: %w", err)
	}
	if err := windows.SetConsoleMode(s.output, s.outputMode); err != nil {
		outputErr = fmt.Errorf("restore output mode: %w", err)
	}
	return errors.Join(inputErr, outputErr)
}

func terminalSize(output *os.File) (protocol.Size, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(output.Fd()), &info); err != nil {
		return protocol.Size{}, fmt.Errorf("read terminal size: %w", err)
	}
	size := protocol.Size{Rows: uint16(info.Window.Bottom - info.Window.Top + 1), Cols: uint16(info.Window.Right - info.Window.Left + 1)}
	if err := size.Validate(); err != nil {
		return protocol.Size{}, fmt.Errorf("read terminal size: %w", err)
	}
	return size, nil
}

func readInput(ctx context.Context, input *os.File, buffer []byte) (int, error) {
	if err := context.Cause(ctx); err != nil {
		return 0, err
	}
	type inputResult struct {
		count int
		err   error
	}
	threadReady := make(chan windows.Handle, 1)
	result := make(chan inputResult, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		thread, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
		if err != nil {
			threadReady <- 0
			result <- inputResult{err: fmt.Errorf("open terminal input thread: %w", err)}
			return
		}
		threadReady <- thread
		count, readErr := input.Read(buffer)
		result <- inputResult{count: count, err: readErr}
	}()
	thread := <-threadReady
	if thread != 0 {
		defer windows.CloseHandle(thread)
	}
	select {
	case read := <-result:
		return read.count, read.err
	case <-ctx.Done():
	}

	ticker := time.NewTicker(windowsInputPollInterval)
	defer ticker.Stop()
	for {
		if thread != 0 {
			_ = cancelSynchronousIO(thread)
		}
		select {
		case <-result:
			return 0, context.Cause(ctx)
		case <-ticker.C:
		}
	}
}

var cancelSynchronousIOProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

func cancelSynchronousIO(thread windows.Handle) error {
	result, _, callErr := cancelSynchronousIOProc.Call(uintptr(thread))
	if result == 0 {
		return callErr
	}
	return nil
}

func resizeEvents(ctx context.Context, output *os.File) <-chan protocol.Size {
	events := make(chan protocol.Size, 1)
	go func() {
		defer close(events)
		ticker := time.NewTicker(windowsInputPollInterval)
		defer ticker.Stop()
		var previous protocol.Size
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				size, err := terminalSize(output)
				if err != nil || size == previous {
					continue
				}
				previous = size
				select {
				case events <- size:
				default:
				}
			}
		}
	}()
	return events
}
