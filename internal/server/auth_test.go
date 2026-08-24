package server

import "testing"

func TestAuthorized(t *testing.T) {
	t.Parallel()

	if !authorized("secret", "Bearer secret") {
		t.Fatal("authorized() rejected the configured token")
	}
	for _, supplied := range []string{"", "secret", "Bearer wrong", "Bearer secret "} {
		if authorized("secret", supplied) {
			t.Errorf("authorized() accepted %q", supplied)
		}
	}
}
