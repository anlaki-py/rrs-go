package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anlaki-py/rrs/internal/client"
)

const connectHelp = `Usage: rrs connect [options] <url>

HTTP URLs are converted to their WebSocket equivalent.

Options:
  --token <value>        WebSocket bearer token (RRS_TOKEN)
  --insecure             Disable TLS certificate verification
  --allow-plaintext      Permit remote ws:// connections (default)
  -h, --help             Show this help
`

func parseConnect(args []string, environment environment) (client.Config, bool, error) {
	flags := newFlagSet("connect")
	token := flags.String("token", environmentValue(environment, "RRS_TOKEN", ""), "")
	insecure := flags.Bool("insecure", false, "")
	allowPlaintext := flags.Bool("allow-plaintext", true, "")
	helpRequested := flags.Bool("help", false, "")
	flags.BoolVar(helpRequested, "h", false, "")
	ordered, err := reorderConnectArgs(args)
	if err != nil {
		return client.Config{}, false, err
	}
	if err := flags.Parse(ordered); err != nil {
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

func reorderConnectArgs(args []string) ([]string, error) {
	options := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--":
			positionals = append(positionals, args[index+1:]...)
			index = len(args)
		case argument == "--token":
			if index+1 >= len(args) {
				return nil, &usageError{message: "flag needs an argument: --token"}
			}
			options = append(options, argument, args[index+1])
			index++
		case strings.HasPrefix(argument, "--token="):
			options = append(options, argument)
		case strings.HasPrefix(argument, "-"):
			options = append(options, argument)
		default:
			positionals = append(positionals, argument)
		}
	}
	return append(options, positionals...), nil
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
