//go:build !linux && !windows

package terminal

import "fmt"

func startPlatform(StartOptions) (platformSession, error) {
	return nil, fmt.Errorf("start terminal: %w", errUnsupported)
}
