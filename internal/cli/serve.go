package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/anlaki-py/rrs/internal/server"
)

const serveHelp = `Usage: rrs serve [options]

Options:
  --host <address>       Listener address (HOST, default: 127.0.0.1)
  --port <number>        Listener port (PORT, default: 7860)
  --token <value>        WebSocket bearer token (RRS_TOKEN)
  --no-auth              Explicitly run without authentication
  --max-sessions <count> Maximum concurrent shells (default: 8)
  -h, --help             Show this help
`

type serveConfig struct {
	host        string
	port        int
	token       string
	noAuth      bool
	maxSessions int
}

func parseServe(args []string, environment environment) (serveConfig, bool, error) {
	flags := newFlagSet("serve")
	host := flags.String("host", environmentValue(environment, "HOST", "127.0.0.1"), "")
	portValue := flags.String("port", environmentValue(environment, "PORT", "7860"), "")
	token := flags.String("token", environmentValue(environment, "RRS_TOKEN", ""), "")
	noAuth := flags.Bool("no-auth", false, "")
	maxSessions := flags.Int("max-sessions", 8, "")
	helpRequested := flags.Bool("help", false, "")
	flags.BoolVar(helpRequested, "h", false, "")
	if err := flags.Parse(args); err != nil {
		return serveConfig{}, false, flagError(err)
	}
	if *helpRequested {
		return serveConfig{}, true, nil
	}
	if flags.NArg() != 0 {
		return serveConfig{}, false, &usageError{message: fmt.Sprintf("unexpected serve argument %q", flags.Arg(0))}
	}
	port, err := parsePort(*portValue)
	if err != nil {
		return serveConfig{}, false, err
	}
	if *maxSessions < 1 {
		return serveConfig{}, false, &usageError{message: "maximum sessions must be at least 1"}
	}
	if *token != "" && *noAuth {
		return serveConfig{}, false, &usageError{message: "--token and --no-auth cannot be used together"}
	}
	if *token == "" && !*noAuth && !loopbackListenerHost(*host) {
		return serveConfig{}, false, &usageError{message: "a public listener requires --token or explicit --no-auth"}
	}

	return serveConfig{
		host:        *host,
		port:        port,
		token:       *token,
		noAuth:      *noAuth || *token == "",
		maxSessions: *maxSessions,
	}, false, nil
}

func runServe(ctx context.Context, args []string, environment environment, stdout, stderr io.Writer) error {
	config, showHelp, err := parseServe(args, environment)
	if err != nil {
		return err
	}
	if showHelp {
		_, err := io.WriteString(stdout, serveHelp)
		return err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(config.host, strconv.Itoa(config.port)))
	if err != nil {
		return fmt.Errorf("listen on %s:%d: %w", config.host, config.port, err)
	}
	defer listener.Close()

	service, err := server.New(server.Config{
		Token:                config.token,
		AllowUnauthenticated: config.noAuth,
		MaxSessions:          config.maxSessions,
	}, logger(stderr))
	if err != nil {
		return err
	}
	if config.noAuth {
		_, _ = fmt.Fprintln(stderr, "rrs: warning: server is running without authentication")
	}
	_, _ = fmt.Fprintf(stdout, "RRS listening on %s\n", listener.Addr())
	return service.Serve(ctx, listener)
}
