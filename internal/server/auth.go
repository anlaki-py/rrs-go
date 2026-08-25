package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/anlaki-py/rrs-go/internal/protocol"
)

func authorized(configuredToken, authorization string) bool {
	expected := []byte("Bearer " + configuredToken)
	supplied := []byte(authorization)
	if len(expected) != len(supplied) {
		return false
	}
	return subtle.ConstantTimeCompare(expected, supplied) == 1
}

func offersSubprotocol(request *http.Request) bool {
	for _, header := range request.Header.Values("Sec-WebSocket-Protocol") {
		for value := range strings.SplitSeq(header, ",") {
			if strings.TrimSpace(value) == protocol.Subprotocol {
				return true
			}
		}
	}
	return false
}
