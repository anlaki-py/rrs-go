package tunnel

import (
	"context"
	"errors"
	"testing"
)

func TestStartRejectsEmptyLocalURL(t *testing.T) {
	if _, err := Start(context.Background(), " "); err == nil {
		t.Fatal("Start() accepted an empty local URL")
	}
}

func TestResolveCommandPrefersCloudflared(t *testing.T) {
	command, args, err := resolveCommand(func(name string) (string, error) {
		if name == "cloudflared" {
			return `C:\tools\cloudflared.exe`, nil
		}
		return "", errors.New("not found")
	})
	if err != nil || command != `C:\tools\cloudflared.exe` || len(args) != 0 {
		t.Fatalf("resolveCommand() = %q, %#v, %v", command, args, err)
	}
}

func TestResolveCommandFallsBackToNpx(t *testing.T) {
	command, args, err := resolveCommand(func(name string) (string, error) {
		if name == "npx" {
			return `C:\tools\npx.cmd`, nil
		}
		return "", errors.New("not found")
	})
	if err != nil || command != `C:\tools\npx.cmd` || len(args) != 2 || args[0] != "--yes" || args[1] != "cloudflared" {
		t.Fatalf("resolveCommand() = %q, %#v, %v", command, args, err)
	}
}

func TestResolveCommandRejectsMissingCommands(t *testing.T) {
	_, _, err := resolveCommand(func(string) (string, error) { return "", errors.New("not found") })
	if err == nil {
		t.Fatal("resolveCommand() accepted missing commands")
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"quick tunnel", "INF https://random-words.trycloudflare.com", "wss://random-words.trycloudflare.com"},
		{"ignores API URL", "INF https://api.trycloudflare.com/tunnel", ""},
		{"finds URL in surrounding output", `created at [https://a-b.trycloudflare.com}]`, "wss://a-b.trycloudflare.com"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseURL(test.text); got != test.want {
				t.Fatalf("ParseURL() = %q, want %q", got, test.want)
			}
		})
	}
}
