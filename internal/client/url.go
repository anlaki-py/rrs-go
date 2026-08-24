package client

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func normalizeURL(input string) (*url.URL, error) {
	candidate := input
	if !strings.Contains(candidate, "://") {
		candidate = "wss://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return nil, fmt.Errorf("parse WebSocket URL: %w", err)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return nil, errors.New("URL must use http, https, ws, or wss")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("WebSocket URL requires a host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("WebSocket URL must not contain user information")
	}
	if parsed.Fragment != "" {
		return nil, fmt.Errorf("WebSocket URL must not contain a fragment")
	}
	return parsed, nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
