//go:build windows

package console

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestWindowsInputReadStopsWhenContextEnds(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer input.Close()
	defer output.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = readInput(ctx, input, make([]byte, 1))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readInput() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("readInput() took %v to honor cancellation", elapsed)
	}
}
