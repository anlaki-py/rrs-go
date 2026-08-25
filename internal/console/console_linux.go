//go:build linux

package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/anlaki-py/rrs-go/internal/protocol"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const inputPollInterval = 250

type linuxState struct {
	terminal *term.State
}

func enterRaw(input *os.File) (platformState, error) {
	if !term.IsTerminal(int(input.Fd())) {
		return nil, fmt.Errorf("enter raw mode: input is not an interactive terminal")
	}
	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return nil, fmt.Errorf("enter raw mode: %w", err)
	}
	return &linuxState{terminal: state}, nil
}

func (s *linuxState) restore(input *os.File) error {
	if err := term.Restore(int(input.Fd()), s.terminal); err != nil {
		return fmt.Errorf("restore terminal: %w", err)
	}
	return nil
}

func terminalSize(output *os.File) (protocol.Size, error) {
	cols, rows, err := term.GetSize(int(output.Fd()))
	if err != nil {
		return protocol.Size{}, fmt.Errorf("read terminal size: %w", err)
	}
	size := protocol.Size{Rows: uint16(rows), Cols: uint16(cols)}
	if err := size.Validate(); err != nil {
		return protocol.Size{}, fmt.Errorf("read terminal size: %w", err)
	}
	return size, nil
}

func readInput(ctx context.Context, input *os.File, buffer []byte) (int, error) {
	descriptors := []unix.PollFd{{Fd: int32(input.Fd()), Events: unix.POLLIN | unix.POLLHUP | unix.POLLERR}}
	for {
		if err := context.Cause(ctx); err != nil {
			return 0, err
		}
		ready, err := unix.Poll(descriptors, inputPollInterval)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("poll terminal input: %w", err)
		}
		if ready == 0 {
			continue
		}
		count, err := input.Read(buffer)
		if errors.Is(err, os.ErrClosed) && context.Cause(ctx) != nil {
			return count, context.Cause(ctx)
		}
		if count == 0 && err == nil {
			return 0, io.ErrNoProgress
		}
		return count, err
	}
}

func resizeEvents(ctx context.Context, output *os.File) <-chan protocol.Size {
	events := make(chan protocol.Size, 1)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	go func() {
		defer close(events)
		defer signal.Stop(signals)
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				size, err := terminalSize(output)
				if err != nil {
					continue
				}
				select {
				case events <- size:
				default:
					select {
					case <-events:
					default:
					}
					events <- size
				}
			}
		}
	}()

	return events
}
