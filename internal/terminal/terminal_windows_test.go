//go:build windows

package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/anlaki-py/rrs-go/internal/protocol"
	"golang.org/x/sys/windows"
)

const windowsIntegrationTimeout = 10 * time.Second

func TestWindowsTerminalStartsReadsResizesAndStops(t *testing.T) {
	session, err := Start(StartOptions{Size: protocol.Size{Rows: 24, Cols: 80}})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer session.Close()

	if _, err := session.Write([]byte("Write-Output RRS_WINDOWS_OK\r\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	received := readUntil(t, session, "RRS_WINDOWS_OK", windowsIntegrationTimeout)
	if !strings.Contains(received, "RRS_WINDOWS_OK") {
		t.Fatalf("terminal output did not contain marker: %q", received)
	}

	if err := session.Resize(protocol.Size{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := session.Terminate(stopCtx); err != nil {
		t.Fatalf("Terminate() error = %v", err)
	}
}

func TestWindowsShellExitClosesTerminalOutput(t *testing.T) {
	session, err := Start(StartOptions{Size: protocol.Size{Rows: 24, Cols: 80}})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer session.Close()

	if _, err := session.Write([]byte("exit\r\n")); err != nil {
		t.Fatalf("Write(exit) error = %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, session)
		result <- readErr
	}()
	select {
	case readErr := <-result:
		if readErr != nil {
			t.Fatalf("terminal output ended with error: %v", readErr)
		}
	case <-time.After(windowsIntegrationTimeout):
		t.Fatal("terminal output did not close after PowerShell exited")
	}
}

func TestWindowsTerminationStopsDescendantProcesses(t *testing.T) {
	session, err := Start(StartOptions{Size: protocol.Size{Rows: 24, Cols: 120}})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer session.Close()

	command := "$p = Start-Process powershell.exe -ArgumentList @('-NoLogo','-NoProfile','-Command','Start-Sleep -Seconds 60') -PassThru; Write-Output ('RRS_CHILD_PID=' + $p.Id)\r\n"
	if _, err := session.Write([]byte(command)); err != nil {
		t.Fatalf("Write(Start-Process) error = %v", err)
	}
	match := readUntilPattern(t, session, regexp.MustCompile(`RRS_CHILD_PID=(\d+)`), windowsIntegrationTimeout)
	childPID, err := strconv.ParseUint(match[1], 10, 32)
	if err != nil {
		t.Fatalf("parse child PID %q: %v", match[1], err)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(childPID))
	if err != nil {
		t.Fatalf("OpenProcess(%d) error = %v", childPID, err)
	}
	defer windows.CloseHandle(child)

	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := session.Terminate(stopCtx); err != nil {
		t.Fatalf("Terminate() error = %v", err)
	}
	status, err := windows.WaitForSingleObject(child, 2_000)
	if err != nil {
		t.Fatalf("WaitForSingleObject(child) error = %v", err)
	}
	if status != windows.WAIT_OBJECT_0 {
		t.Fatalf("child process %d remained alive, wait status %#x", childPID, status)
	}
}

func TestBundledConPTYTranslatesRemoteMouseInput(t *testing.T) {
	t.Setenv("RRS_WINDOWS_MOUSE_HELPER", "1")
	pty, err := newWindowsPTY(80, 24)
	if err != nil {
		t.Fatalf("newWindowsPTY() error = %v", err)
	}
	defer pty.Close()

	directory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	_, process, err := pty.Spawn(
		os.Args[0],
		[]string{"-test.run=^TestWindowsMouseInputHelper$"},
		os.Environ(),
		directory,
		0,
	)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	defer windows.CloseHandle(process)
	defer windows.TerminateProcess(process, 1)

	ready := readUntil(t, pty, "RRS_MOUSE_READY", windowsIntegrationTimeout)
	if !strings.Contains(ready, "RRS_MOUSE_READY") {
		t.Fatalf("mouse helper exited before becoming ready: %q", ready)
	}
	if _, err := pty.Write([]byte("\x1b[<0;10;5M\x1b[<0;10;5m\x1b[<35;12;6M\x1b[<64;10;5M")); err != nil {
		t.Fatalf("Write(mouse reports) error = %v", err)
	}
	output := readUntil(t, pty, "RRS_MOUSE 9 4 8388608 4", windowsIntegrationTimeout)
	for _, marker := range []string{
		"RRS_MOUSE 9 4 1 0",
		"RRS_MOUSE 9 4 0 0",
		"RRS_MOUSE 11 5 0 1",
		"RRS_MOUSE 9 4 8388608 4",
	} {
		if !strings.Contains(output, marker) {
			t.Fatalf("mouse output missing %q: %q", marker, output)
		}
	}
}

func TestWindowsMouseInputHelper(t *testing.T) {
	if os.Getenv("RRS_WINDOWS_MOUSE_HELPER") != "1" {
		return
	}
	input, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		t.Fatal(err)
	}
	var mode uint32
	if err := windows.GetConsoleMode(input, &mode); err != nil {
		t.Fatal(err)
	}
	mode |= windows.ENABLE_MOUSE_INPUT | windows.ENABLE_EXTENDED_FLAGS
	mode &^= windows.ENABLE_QUICK_EDIT_MODE | windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT
	if err := windows.SetConsoleMode(input, mode); err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprint(os.Stdout, "\x1b[?1000h\x1b[?1003h\x1b[?1006hRRS_MOUSE_READY\n")

	mouseEvents := 0
	for mouseEvents < 4 {
		var record windowsInputRecord
		if err := readWindowsConsoleInput(input, &record); err != nil {
			t.Fatal(err)
		}
		if record.EventType != windows.MOUSE_EVENT {
			continue
		}
		mouse := (*windowsMouseEventRecord)(unsafe.Pointer(&record.Event[0]))
		_, _ = fmt.Fprintf(
			os.Stdout,
			"RRS_MOUSE %d %d %d %d\n",
			mouse.Position.X,
			mouse.Position.Y,
			mouse.ButtonState,
			mouse.EventFlags,
		)
		mouseEvents++
	}
}

type windowsInputRecord struct {
	EventType uint16
	_         uint16
	Event     [16]byte
}

type windowsMouseEventRecord struct {
	Position        windows.Coord
	ButtonState     uint32
	ControlKeyState uint32
	EventFlags      uint32
}

var readConsoleInputW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

func readWindowsConsoleInput(input windows.Handle, record *windowsInputRecord) error {
	var count uint32
	result, _, callErr := readConsoleInputW.Call(
		uintptr(input),
		uintptr(unsafe.Pointer(record)),
		1,
		uintptr(unsafe.Pointer(&count)),
	)
	if result == 0 {
		return callErr
	}
	if count != 1 {
		return fmt.Errorf("ReadConsoleInputW read %d records", count)
	}
	return nil
}

func readUntil(t *testing.T, reader io.Reader, marker string, timeout time.Duration) string {
	t.Helper()
	result := make(chan string, 1)
	go func() {
		var output strings.Builder
		buffer := make([]byte, 4096)
		for !strings.Contains(output.String(), marker) {
			count, err := reader.Read(buffer)
			if count > 0 {
				output.Write(buffer[:count])
			}
			if err != nil {
				break
			}
		}
		result <- output.String()
	}()
	select {
	case output := <-result:
		return output
	case <-time.After(timeout):
		t.Fatalf("terminal output did not contain %q before timeout", marker)
		return ""
	}
}

func readUntilPattern(t *testing.T, reader io.Reader, pattern *regexp.Regexp, timeout time.Duration) []string {
	t.Helper()
	result := make(chan []string, 1)
	go func() {
		var output strings.Builder
		buffer := make([]byte, 4096)
		for {
			count, err := reader.Read(buffer)
			if count > 0 {
				output.Write(buffer[:count])
				if match := pattern.FindStringSubmatch(output.String()); match != nil {
					result <- match
					return
				}
			}
			if err != nil {
				result <- nil
				return
			}
		}
	}()
	select {
	case match := <-result:
		if match == nil {
			t.Fatalf("terminal output ended before matching %s", pattern)
		}
		return match
	case <-time.After(timeout):
		t.Fatalf("terminal output did not match %s before timeout", pattern)
		return nil
	}
}

func TestBundledConPTYCacheRepairsUnexpectedContents(t *testing.T) {
	root := t.TempDir()
	files := []conptyBundleFile{{path: "conpty.dll", content: []byte("expected")}}
	if err := materializeConPTYBundle(root, files); err != nil {
		t.Fatalf("materializeConPTYBundle() error = %v", err)
	}
	target := filepath.Join(root, "conpty.dll")
	if err := os.WriteFile(target, []byte("changed"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := materializeConPTYBundle(root, files); err != nil {
		t.Fatalf("materializeConPTYBundle() repair error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "expected" {
		t.Fatalf("cached content = %q", content)
	}
}
