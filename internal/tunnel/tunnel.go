package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const startupTimeout = 30 * time.Second

var quickTunnelURL = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type Running struct {
	URL string

	process   *exec.Cmd
	wait      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func Start(ctx context.Context, localURL string) (*Running, error) {
	if strings.TrimSpace(localURL) == "" {
		return nil, errors.New("start Cloudflare tunnel: local URL is empty")
	}
	command, arguments, err := resolveCommand(exec.LookPath)
	if err != nil {
		return nil, err
	}
	process := exec.Command(command, append(arguments, "tunnel", "--url", localURL)...)
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("start Cloudflare tunnel: capture stdout: %w", err)
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("start Cloudflare tunnel: capture stderr: %w", err)
	}
	if err := process.Start(); err != nil {
		return nil, fmt.Errorf("start Cloudflare tunnel: %w", err)
	}

	output := make(chan string, 2)
	go readOutput(stdout, output)
	go readOutput(stderr, output)
	wait := make(chan struct{})
	go func() {
		_ = process.Wait()
		close(wait)
	}()

	timer := time.NewTimer(startupTimeout)
	defer timer.Stop()
	for {
		select {
		case text, open := <-output:
			if !open {
				output = nil
				continue
			}
			if url := ParseURL(text); url != "" {
				return &Running{URL: url, process: process, wait: wait}, nil
			}
		case <-wait:
			return nil, errors.New("start Cloudflare tunnel: cloudflared exited before providing a tunnel URL")
		case <-timer.C:
			_ = process.Process.Kill()
			return nil, errors.New("start Cloudflare tunnel: timed out waiting for a tunnel URL")
		case <-ctx.Done():
			_ = process.Process.Kill()
			return nil, fmt.Errorf("start Cloudflare tunnel: %w", context.Cause(ctx))
		}
	}
}

func resolveCommand(lookPath func(string) (string, error)) (string, []string, error) {
	if command, err := lookPath("cloudflared"); err == nil {
		return command, nil, nil
	}
	if command, err := lookPath("npx"); err == nil {
		return command, []string{"--yes", "cloudflared"}, nil
	}
	return "", nil, errors.New("start Cloudflare tunnel: neither cloudflared nor npx was found on PATH")
}

func ParseURL(output string) string {
	for _, index := range quickTunnelURL.FindAllStringIndex(output, -1) {
		if index[1] < len(output) && !strings.ContainsRune(" \t\r\n\"'<>|,}]", rune(output[index[1]])) {
			continue
		}
		return strings.Replace(output[index[0]:index[1]], "https://", "wss://", 1)
	}
	return ""
}

func readOutput(reader io.Reader, output chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case output <- scanner.Text():
		default:
		}
	}
}

func (t *Running) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		select {
		case <-t.wait:
			return
		default:
		}
		if err := t.process.Process.Kill(); err != nil {
			t.closeErr = fmt.Errorf("stop Cloudflare tunnel: %w", err)
		}
		<-t.wait
	})
	return t.closeErr
}
