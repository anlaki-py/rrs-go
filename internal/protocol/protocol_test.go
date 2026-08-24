package protocol

import (
	"errors"
	"testing"
)

func TestParseResize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    Size
		wantErr bool
	}{
		{name: "normal", message: `{"rows":24,"cols":80}`, want: Size{Rows: 24, Cols: 80}},
		{name: "boundaries", message: `{"rows":1,"cols":4096}`, want: Size{Rows: 1, Cols: 4096}},
		{name: "missing field", message: `{"rows":24}`, wantErr: true},
		{name: "unknown field", message: `{"rows":24,"cols":80,"x":1}`, wantErr: true},
		{name: "fraction", message: `{"rows":24.5,"cols":80}`, wantErr: true},
		{name: "zero", message: `{"rows":0,"cols":80}`, wantErr: true},
		{name: "too large", message: `{"rows":24,"cols":4097}`, wantErr: true},
		{name: "trailing value", message: `{"rows":24,"cols":80} {}`, wantErr: true},
		{name: "malformed", message: `{`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseResize([]byte(test.message))
			if test.wantErr {
				if !errors.Is(err, ErrInvalidResize) {
					t.Fatalf("ParseResize() error = %v, want ErrInvalidResize", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseResize() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ParseResize() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestEncodeResize(t *testing.T) {
	t.Parallel()

	message, err := EncodeResize(Size{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("EncodeResize() error = %v", err)
	}
	if got, want := string(message), `{"rows":24,"cols":80}`; got != want {
		t.Fatalf("EncodeResize() = %q, want %q", got, want)
	}

	if _, err := EncodeResize(Size{}); !errors.Is(err, ErrInvalidResize) {
		t.Fatalf("EncodeResize(Size{}) error = %v, want ErrInvalidResize", err)
	}
}
