# RRS

RRS provides an interactive shell over WebSockets as one Go executable. Linux
and Windows amd64 clients and servers are supported.

Windows sessions launch Windows PowerShell by default, load the user's normal
PowerShell profiles, and run inside Microsoft's redistributable ConPTY host.
The host is embedded in `rrs.exe` and extracted to a versioned user cache when
needed, so there is no separate runtime installation.

## Install

One-command install on Linux. The script detects your architecture, verifies
the sha256 checksum, and installs to `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/anlaki-py/rrs-go/master/install.sh | sh
```

Pin a version or change the install directory with environment variables:

```sh
RRS_VERSION=v0.0.1 RRS_INSTALL_DIR=/other/bin sh install.sh
```

Windows has no installer yet; download `rrs-windows-amd64.exe` from the
[releases page](https://github.com/anlaki-py/rrs-go/releases). Checksums for
every asset are in each release's `checksums.txt`.

## Build

RRS requires Go 1.26 or newer.

```sh
go build -o rrs ./cmd/rrs
go test ./...
```

On Windows, the native build command is:

```powershell
go build -o rrs.exe ./cmd/rrs
```

Release builds are pure Go:

```sh
CGO_ENABLED=0 go build -trimpath -o rrs ./cmd/rrs
```

PowerShell equivalent:

```powershell
$env:CGO_ENABLED = '0'
go build -trimpath -o rrs.exe ./cmd/rrs
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

When the server listens on a wildcard address, it prints a `ws://` URL for
each available IP address instead of displaying `0.0.0.0` or `[::]`.

Expose a local server through a Cloudflare Quick Tunnel. RRS uses an installed
`cloudflared` command when available, then falls back to `npx --yes cloudflared`:

```sh
./rrs serve --tunnel
```

Connect from an interactive terminal:

```sh
./rrs connect 127.0.0.1:7000 --token 'replace-me'
```

An address without a scheme uses `ws://`. You can also pass HTTP URLs; RRS
converts `http://` to `ws://` and `https://` to `wss://` before connecting.

Plain `ws://` connections are enabled by default, including remote addresses.
They do not encrypt terminal contents or bearer tokens, so use `wss://` through
a trusted reverse proxy outside a controlled network. `--allow-plaintext`
remains accepted for command compatibility but is no longer required.

The client uses the terminal's alternate screen and enables SGR mouse reports
for the duration of a connection. Disconnecting restores the previous screen,
input mode, and mouse mode.

## Security

RRS grants shell access as the account running the server. It does not provide
user isolation, authorization roles, auditing, or sandboxing. Public listeners
require a bearer token. TLS certificate verification never disables itself.

The wire contract is documented in [docs/protocol.md](docs/protocol.md).

## Rewrite status

- Linux PTY server and interactive client
- Windows amd64 server and client with redistributable ConPTY
- Native Windows TUI click, drag, and hover input from remote terminals
- Versioned WebSocket protocol
- Authentication and safe listener defaults
- Cloudflare Quick Tunnel support
- Self-update: pending
