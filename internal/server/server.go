package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anlaki-py/rrs/internal/protocol"
	"github.com/coder/websocket"
)

const (
	shutdownTimeout = 5 * time.Second
	headerTimeout   = 5 * time.Second
)

type Config struct {
	Token                string
	AllowUnauthenticated bool
	MaxSessions          int
}

type Server struct {
	config    Config
	logger    *slog.Logger
	slots     chan struct{}
	sessions  sync.WaitGroup
	activeMu  sync.Mutex
	active    int
	startTerm terminalStarter
}

func New(config Config, logger *slog.Logger) (*Server, error) {
	if config.Token == "" && !config.AllowUnauthenticated {
		return nil, errors.New("server requires a token or explicit unauthenticated access")
	}
	if config.Token != "" && config.AllowUnauthenticated {
		return nil, errors.New("server token and unauthenticated access cannot both be enabled")
	}
	if config.MaxSessions < 1 {
		return nil, errors.New("maximum sessions must be at least 1")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Server{
		config:    config,
		logger:    logger,
		slots:     make(chan struct{}, config.MaxSessions),
		startTerm: startTerminal,
	}, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	httpServer := &http.Server{
		Handler:           s.handler(serverCtx),
		ReadHeaderTimeout: headerTimeout,
		MaxHeaderBytes:    16 * 1024,
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- httpServer.Serve(listener)
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-serveResult:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			cancel()
		}
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	shutdownErr := httpServer.Shutdown(shutdownCtx)
	sessionsErr := s.waitForSessions(shutdownCtx)

	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", serveErr)
	}
	if shutdownErr != nil {
		return fmt.Errorf("shut down HTTP server: %w", shutdownErr)
	}
	if sessionsErr != nil {
		return sessionsErr
	}
	return nil
}

func (s *Server) handler(ctx context.Context) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /", func(response http.ResponseWriter, request *http.Request) {
		if !websocketUpgrade(request) {
			handleInfo(response)
			return
		}
		s.handleUpgrade(ctx, response, request)
	})
	return mux
}

func handleHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("OK\n"))
}

func handleInfo(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("RRS remote shell. Connect with the rrs client.\n"))
}

func websocketUpgrade(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket")
}

func (s *Server) handleUpgrade(ctx context.Context, response http.ResponseWriter, request *http.Request) {
	if s.config.Token != "" && !authorized(s.config.Token, request.Header.Get("Authorization")) {
		response.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(response, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !offersSubprotocol(request) {
		http.Error(response, "WebSocket subprotocol rrs.v1 is required", http.StatusBadRequest)
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		http.Error(response, "Too many active sessions", http.StatusServiceUnavailable)
		return
	}
	s.sessions.Add(1)
	defer s.sessions.Done()

	connection, err := websocket.Accept(response, request, &websocket.AcceptOptions{
		Subprotocols:    []string{protocol.Subprotocol},
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		s.logger.Warn("WebSocket upgrade failed", "remote", request.RemoteAddr, "error", err)
		return
	}
	connection.SetReadLimit(protocol.MaxMessageSize)

	s.setActive(1)
	defer func() {
		s.setActive(-1)
	}()

	err = s.runSession(ctx, connection)
	if err != nil && !expectedSessionError(err) {
		s.logger.Warn("terminal session ended", "remote", request.RemoteAddr, "error", err)
	}
}

func (s *Server) waitForSessions(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.sessions.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for terminal sessions: %w", context.Cause(ctx))
	}
}

func (s *Server) setActive(change int) {
	s.activeMu.Lock()
	s.active += change
	s.activeMu.Unlock()
}

func (s *Server) activeSessions() int {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	return s.active
}
