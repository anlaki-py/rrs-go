package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/anlaki-py/rrs-go/internal/client"
)

type testEnvironment map[string]string

func (e testEnvironment) LookupEnv(key string) (string, bool) {
	value, exists := e[key]
	return value, exists
}

func TestParseServeUsesFlagsOverEnvironment(t *testing.T) {
	t.Parallel()

	config, help, err := parseServe(
		[]string{"--host", "127.0.0.2", "--port", "9000", "--token", "flag-token"},
		testEnvironment{"HOST": "127.0.0.1", "PORT": "8000", "RRS_TOKEN": "env-token"},
	)
	if err != nil {
		t.Fatalf("parseServe() error = %v", err)
	}
	if help {
		t.Fatal("parseServe() help = true")
	}
	if config.host != "127.0.0.2" || config.port != 9000 || config.token != "flag-token" {
		t.Fatalf("parseServe() = %#v", config)
	}
}

func TestParseServeRejectsPublicUnauthenticatedListener(t *testing.T) {
	t.Parallel()

	_, _, err := parseServe([]string{"--host", "0.0.0.0"}, testEnvironment{})
	if err == nil || !strings.Contains(err.Error(), "requires --token") {
		t.Fatalf("parseServe() error = %v", err)
	}
}

func TestParseServeEnablesTunnel(t *testing.T) {
	t.Parallel()

	config, help, err := parseServe([]string{"--tunnel"}, testEnvironment{})
	if err != nil || help || !config.tunnel {
		t.Fatalf("parseServe() = %#v, help=%v, err=%v", config, help, err)
	}
}

func TestTunnelLocalAddressUsesLoopbackForWildcardListeners(t *testing.T) {
	t.Parallel()

	if got := tunnelLocalAddress("0.0.0.0", "0.0.0.0:7860"); got != "127.0.0.1:7860" {
		t.Fatalf("tunnelLocalAddress() = %q", got)
	}
	if got := tunnelLocalAddress("::", "[::]:7860"); got != "[::1]:7860" {
		t.Fatalf("tunnelLocalAddress() = %q", got)
	}
}

func TestListeningURLsShowsAvailableIPv4AddressesForWildcardListener(t *testing.T) {
	t.Parallel()

	interfaceAddresses := func() ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.ParseIP("192.168.1.20"), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
			&net.IPNet{IP: net.ParseIP("2001:db8::20"), Mask: net.CIDRMask(64, 128)},
		}, nil
	}

	got, err := listeningURLs("0.0.0.0", "[::]:7000", interfaceAddresses)
	if err != nil {
		t.Fatalf("listeningURLs() error = %v", err)
	}
	want := []string{"ws://127.0.0.1:7000", "ws://192.168.1.20:7000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeningURLs() = %q, want %q", got, want)
	}
}

func TestListeningURLsFormatsIPv6Addresses(t *testing.T) {
	t.Parallel()

	interfaceAddresses := func() ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
			&net.IPNet{IP: net.ParseIP("2001:db8::20"), Mask: net.CIDRMask(64, 128)},
		}, nil
	}

	got, err := listeningURLs("::", "[::]:7000", interfaceAddresses)
	if err != nil {
		t.Fatalf("listeningURLs() error = %v", err)
	}
	want := []string{"ws://[2001:db8::20]:7000", "ws://[::1]:7000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeningURLs() = %q, want %q", got, want)
	}
}

func TestListeningURLsUsesLoopbackWhenInterfaceLookupFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("interface lookup failed")
	got, err := listeningURLs("0.0.0.0", "0.0.0.0:7000", func() ([]net.Addr, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("listeningURLs() error = %v, want %v", err, wantErr)
	}
	want := []string{"ws://127.0.0.1:7000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeningURLs() = %q, want %q", got, want)
	}
}

func TestListeningURLsUsesBoundAddressWithoutInterfaceLookup(t *testing.T) {
	t.Parallel()

	called := false
	got, err := listeningURLs("127.0.0.2", "127.0.0.2:7000", func() ([]net.Addr, error) {
		called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("listeningURLs() error = %v", err)
	}
	if called {
		t.Fatal("listeningURLs() looked up interfaces for a specific listener")
	}
	want := []string{"ws://127.0.0.2:7000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listeningURLs() = %q, want %q", got, want)
	}
}

func TestParseConnect(t *testing.T) {
	t.Parallel()

	config, help, err := parseConnect(
		[]string{"--allow-plaintext", "ws://example.com"},
		testEnvironment{"RRS_TOKEN": "secret"},
	)
	if err != nil {
		t.Fatalf("parseConnect() error = %v", err)
	}
	want := client.Config{URL: "ws://example.com", Token: "secret", AllowPlaintext: true}
	if help || config != want {
		t.Fatalf("parseConnect() = %#v, %v; want %#v, false", config, help, want)
	}
}

func TestParseConnectAcceptsOptionsAfterURL(t *testing.T) {
	t.Parallel()

	config, help, err := parseConnect(
		[]string{"ws://192.168.1.20:7000", "--token", "ooo", "--allow-plaintext"},
		testEnvironment{},
	)
	if err != nil {
		t.Fatalf("parseConnect() error = %v", err)
	}
	want := client.Config{URL: "ws://192.168.1.20:7000", Token: "ooo", AllowPlaintext: true}
	if help || config != want {
		t.Fatalf("parseConnect() = %#v, %v; want %#v, false", config, help, want)
	}
}

func TestParseConnectAllowsRemotePlaintextByDefault(t *testing.T) {
	t.Parallel()

	config, help, err := parseConnect([]string{"ws://192.168.1.20:7000"}, testEnvironment{})
	if err != nil || help || !config.AllowPlaintext {
		t.Fatalf("parseConnect() = %#v, help=%v, error=%v", config, help, err)
	}
}

func TestParseServeUsesPort7000ByDefault(t *testing.T) {
	t.Parallel()

	config, help, err := parseServe(nil, testEnvironment{})
	if err != nil || help || config.port != 7000 {
		t.Fatalf("parseServe() = %#v, help=%v, error=%v", config, help, err)
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	if err := run(context.Background(), nil, testEnvironment{}, &stdout, io.Discard); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Random Remote Shell") {
		t.Fatalf("run() output = %q", stdout.String())
	}
}
