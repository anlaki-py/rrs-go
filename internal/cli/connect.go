package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/anlaki-py/rrs/internal/client"
)

const connectHelp = `Usage: rrs connect [options] <url>

HTTP URLs are converted to their WebSocket equivalent.

Options:
  --token <value>        WebSocket bearer token (RRS_TOKEN)
  --insecure             Disable TLS certificate verification
  --allow-plaintext      Permit remote ws:// connections
  -h, --help             Show this help
`

func parseConnect(args []string, environment environment) (client.Config, bool, error) {
	flags := newFlagSet("connect")
	token := flags.String("token", environmentValue(environment, "RRS_TOKEN", ""), "")
	insecure := flags.Bool("insecure", false, "")
	allowPlaintext := flags.Bool("allow-plaintext", false, "")
	helpRequested := flags.Bool("help", false, "")
	flags.BoolVar(helpRequested, "h", false, "")
	if err := flags.Parse(args); err != nil {
		return client.Config{}, false, flagError(err)
	}
	if *helpRequested {
		return client.Config{}, true, nil
	}
	if flags.NArg() != 1 {
		return client.Config{}, false, &usageError{message: "connect requires exactly one URL"}
	}
	return client.Config{
		URL:            flags.Arg(0),
		Token:          *token,
		Insecure:       *insecure,
		AllowPlaintext: *allowPlaintext,
	}, false, nil
}

func runConnect(ctx context.Context, args []string, environment environment, stdout io.Writer) error {
	config, showHelp, err := parseConnect(args, environment)
	if err != nil {
		return err
	}
	if showHelp {
		_, err := io.WriteString(stdout, connectHelp)
		return err
	}
	if err := client.Run(ctx, config, os.Stdin, os.Stdout); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}
