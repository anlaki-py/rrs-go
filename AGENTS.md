# RRS engineering guide

## Purpose

RRS provides an interactive platform shell over WebSockets as one executable.
It is a personal project with security-sensitive behavior because the server
grants access to its operating-system account.

## Architecture

- `cmd/rrs` owns process startup and signal handling.
- `internal/cli` parses configuration and invokes commands.
- `internal/protocol` owns the WebSocket wire contract.
- `internal/server` owns HTTP, WebSocket, and remote session lifecycles.
- `internal/client` connects a local terminal to a server.
- `internal/terminal` owns remote PTYs and child process trees.
- `internal/console` owns local raw terminal state.

Dependencies point inward from the CLI to these packages. Platform-specific
code uses Go build tags. Do not add generic `utils`, `common`, or `services`
packages.

## Commands

```sh
go build ./cmd/rrs
go test ./...
go test -race ./...
go vet ./...
gofmt -w .
```

Release and cross-build commands must set `CGO_ENABLED=0` explicitly.

## Conventions

- Keep operational errors contextual and wrap causes with `%w`.
- Log errors once. Never log bearer tokens or authorization headers.
- Every goroutine must have a bounded shutdown path.
- Test process cleanup and terminal restoration, not only successful output.
- Do not claim an operating system is supported until integration tests run on
  that operating system.

## Done

A change is done when formatting, tests, race tests, vet, native build, and
supported cross-builds pass. Update the README and protocol document when
behavior changes.
