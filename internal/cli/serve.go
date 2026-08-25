package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/anlaki-py/rrs-go/internal/server"
	"github.com/anlaki-py/rrs-go/internal/tunnel"
)

const serveHelp = `Usage: rrs serve [options]

Options:
  --host <address>       Listener address (HOST, default: 127.0.0.1)
  --port <number>        Listener port (PORT, default: 7000)
  --token <value>        WebSocket bearer token (RRS_TOKEN)
  --no-auth              Explicitly run without authentication
  --max-sessions <count> Maximum concurrent shells (default: 8)
  --tunnel               Expose the server through a Cloudflare Quick Tunnel
  -h, --help             Show this help
`

type serveConfig struct {
	host        string
	port        int
	token       string
	noAuth      bool
	maxSessions int
	tunnel      bool
}

func parseServe(args []string, environment environment) (serveConfig, bool, error) {
	flags := newFlagSet("serve")
	host := flags.String("host", environmentValue(environment, "HOST", "127.0.0.1"), "")
	portValue := flags.String("port", environmentValue(environment, "PORT", "7000"), "")
	token := flags.String("token", environmentValue(environment, "RRS_TOKEN", ""), "")
	noAuth := flags.Bool("no-auth", false, "")
	maxSessions := flags.Int("max-sessions", 8, "")
	quickTunnel := flags.Bool("tunnel", false, "")
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
		tunnel:      *quickTunnel,
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
	var runningTunnel *tunnel.Running
	if config.tunnel {
		localURL := fmt.Sprintf("http://%s", tunnelLocalAddress(config.host, listener.Addr().String()))
		runningTunnel, err = tunnel.Start(ctx, localURL)
		if err != nil {
			return err
		}
		defer func() { _ = runningTunnel.Close() }()
	}
	_, _ = fmt.Fprintf(stdout, "RRS listening on %s\n", listener.Addr())
	if runningTunnel != nil {
		_, _ = fmt.Fprintf(stdout, "RRS tunnel available at %s\n", runningTunnel.URL)
	}
	return service.Serve(ctx, listener)
}

func tunnelLocalAddress(configuredHost, listenerAddress string) string {
	host, port, err := net.SplitHostPort(listenerAddress)
	if err != nil {
		return listenerAddress
	}
	if strings.EqualFold(configuredHost, "0.0.0.0") || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if configuredHost == "::" || host == "::" {
		host = "::1"
	}
	return net.JoinHostPort(host, port)
}
