package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/anlaki-py/rrs-go/internal/buildinfo"
)

const help = `RRS - Random Remote Shell

Usage:
  rrs --help
  rrs --version
  rrs serve [options]
  rrs connect [options] <address>

Commands:
  serve                 Start the HTTP and WebSocket shell server
  connect <address>     Connect this terminal to an RRS server

Run "rrs serve --help" or "rrs connect --help" for command options.
`

type environment interface {
	LookupEnv(string) (string, bool)
}

type processEnvironment struct{}

func (processEnvironment) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

type usageError struct {
	message string
}

func (e *usageError) Error() string {
	return e.message
}

func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	err := run(ctx, args, processEnvironment{}, stdout, stderr)
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "rrs: %v\n", err)
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

func run(ctx context.Context, args []string, environment environment, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(stdout, help)
		return err
	}
	if args[0] == "--version" || args[0] == "-v" {
		_, err := fmt.Fprintln(stdout, buildinfo.Version)
		return err
	}

	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], environment, stdout, stderr)
	case "connect":
		return runConnect(ctx, args[1:], environment, stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown command %q; run \"rrs --help\"", args[0])}
	}
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	return flags
}

func environmentValue(environment environment, key, fallback string) string {
	if value, exists := environment.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func loopbackListenerHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, &usageError{message: "port must be an integer from 1 through 65535"}
	}
	return port, nil
}

func flagError(err error) error {
	return &usageError{message: err.Error()}
}

func logger(stderr io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
