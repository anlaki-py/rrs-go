# RRS

RRS provides an interactive Linux shell over WebSockets as one Go executable.
This repository is the in-progress rewrite of Random Remote Shell.

The current milestone supports Linux clients and servers. Windows binaries
compile with a clear unsupported-platform error while ConPTY support is being
built and tested.

## Build

RRS requires Go 1.26 or newer.

```sh
go build -o rrs ./cmd/rrs
go test ./...
```

Release builds are pure Go:

```sh
CGO_ENABLED=0 go build -trimpath -o rrs ./cmd/rrs
```

## Run

Start a loopback server without authentication:

```sh
./rrs serve
```

Start a network-accessible server with a token:

```sh
RRS_TOKEN='replace-me' ./rrs serve --host 0.0.0.0
```

Connect from an interactive terminal:

```sh
./rrs connect --token 'replace-me' ws://127.0.0.1:7860
```

Plain WebSockets are accepted only for loopback addresses. Use TLS through a
trusted reverse proxy for remote connections. `--allow-plaintext` is an
explicit escape hatch for controlled networks.

## Security

RRS grants shell access as the account running the server. It does not provide
user isolation, authorization roles, auditing, or sandboxing. Public listeners
require a bearer token. TLS certificate verification never disables itself.

The wire contract is documented in [docs/protocol.md](docs/protocol.md).

## Rewrite status

- Linux PTY server and interactive client
- Versioned WebSocket protocol
- Authentication and safe listener defaults
- Windows ConPTY support: pending
- Cloudflare tunnel support: pending
- Self-update: pending
