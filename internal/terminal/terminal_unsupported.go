//go:build !linux

package terminal

import "fmt"

func startPlatform(StartOptions) (platformSession, error) {
	return nil, fmt.Errorf("start terminal: %w", errUnsupported)
}
