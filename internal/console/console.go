package console

import (
	"context"
	"errors"
	"os"

	"github.com/anlaki-py/rrs/internal/protocol"
)

var errUnsupported = errors.New("interactive console is unsupported on this platform")

type State struct {
	platform platformState
}

type platformState interface {
	restore(*os.File) error
}

func EnterRaw(input *os.File) (*State, error) {
	platform, err := enterRaw(input)
	if err != nil {
		return nil, err
	}
	return &State{platform: platform}, nil
}

func Restore(input *os.File, state *State) error {
	if state == nil || state.platform == nil {
		return nil
	}
	return state.platform.restore(input)
}

func Size(output *os.File) (protocol.Size, error) {
	return terminalSize(output)
}

func Read(ctx context.Context, input *os.File, buffer []byte) (int, error) {
	return readInput(ctx, input, buffer)
}

func ResizeEvents(ctx context.Context, output *os.File) <-chan protocol.Size {
	return resizeEvents(ctx, output)
}
