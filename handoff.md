# RRS handoff

## Current state

RRS is a pure-Go remote shell for Linux and Windows amd64. The active branch is
`master`. A safety copy of the pre-redo history exists locally as
`backup/pre-model-redo-a54af55`.

The Windows port now uses Microsoft's redistributable ConPTY rather than the
Windows 10 inbox implementation. Version `1.24.260710001` is embedded in the
Windows executable and materialized under the user's versioned cache at
runtime. This is required for native Windows fullscreen TUI mouse input on
older Windows 10 builds.

## Implemented behavior

- `rrs serve` defaults to port 7000.
- `rrs connect` accepts flags before or after its URL.
- Remote `ws://` connections are allowed by default; `--allow-plaintext`
  remains accepted for compatibility.
- Windows sessions start Windows PowerShell with normal profile loading.
- Windows process trees are assigned to a kill-on-close Job Object before the
  suspended shell starts.
- Normal PowerShell `exit` closes terminal output and the WebSocket session.
- Clients enter an alternate screen and restore it on disconnect.
- Clients enable click, drag, scroll, and hover mouse reporting during a
  session.
- Windows client reads are canceled with `CancelSynchronousIo`, with no
  abandoned read goroutine.
- Cloudflare tunnels use `cloudflared` when installed, then
  `npx --yes cloudflared`, and error only if neither command is available.
- Windows executable artifacts are ignored.

## Windows verification

Native Windows tests cover PowerShell I/O, resize, normal shell exit,
descendant-process cleanup, client input cancellation, ConPTY cache repair,
and conversion of remote SGR click/release/hover reports into native Windows
`MOUSE_EVENT` records. The race suite passes with MSYS2 UCRT64 GCC.

Run the full completion checks from PowerShell:

```powershell
gofmt -w .
go test ./...
$env:CGO_ENABLED = '1'
$env:Path = 'C:\msys64\ucrt64\bin;' + $env:Path
go test -race -timeout 3m ./...
go vet ./...
$env:CGO_ENABLED = '0'
go build -trimpath -o dist/rrs-windows-amd64.exe ./cmd/rrs
```

The protocol is documented in `docs/protocol.md`; the Windows backend decision
is documented in `docs/decisions/0002-redistributable-conpty.md`.
