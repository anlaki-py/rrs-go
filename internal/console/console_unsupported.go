//go:build !linux && !windows

package console

import (
	"context"
	"fmt"
	"os"

	"github.com/anlaki-py/rrs/internal/protocol"
)

func enterRaw(*os.File) (platformState, error) {
	return nil, fmt.Errorf("enter raw mode: %w", errUnsupported)
}

func terminalSize(*os.File) (protocol.Size, error) {
	return protocol.Size{}, fmt.Errorf("read terminal size: %w", errUnsupported)
}

func readInput(context.Context, *os.File, []byte) (int, error) {
	return 0, fmt.Errorf("read terminal input: %w", errUnsupported)
}

func resizeEvents(context.Context, *os.File) <-chan protocol.Size {
	events := make(chan protocol.Size)
	close(events)
	return events
}
