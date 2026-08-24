package client

import "testing"

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "example.com", want: "wss://example.com"},
		{input: "http://example.com/path", want: "ws://example.com/path"},
		{input: "https://example.com", want: "wss://example.com"},
		{input: "ws://127.0.0.1:7860", want: "ws://127.0.0.1:7860"},
		{input: "ftp://example.com", wantErr: true},
		{input: "ws:///missing-host", wantErr: true},
		{input: "ws://user@example.com", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeURL(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("normalizeURL() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeURL() error = %v", err)
			}
			if got.String() != test.want {
				t.Fatalf("normalizeURL() = %q, want %q", got.String(), test.want)
			}
		})
	}
}

func TestLoopbackHost(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if !loopbackHost(host) {
			t.Errorf("loopbackHost(%q) = false", host)
		}
	}
	if loopbackHost("example.com") {
		t.Error("loopbackHost(example.com) = true")
	}
}
