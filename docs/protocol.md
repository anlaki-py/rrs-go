# RRS protocol version 1

RRS carries an interactive terminal byte stream over WebSockets. Clients and
servers negotiate the `rrs.v1` WebSocket subprotocol. There is no fallback to
an unversioned protocol.

## Handshake

The client connects to `/` and sends `Sec-WebSocket-Protocol: rrs.v1`.
When the server has a token, the client also sends:

```text
Authorization: Bearer <token>
```

The server authenticates the request before accepting the WebSocket. Browser
origins are rejected. Per-message compression is disabled.

## Messages

Binary messages contain terminal bytes. Client-to-server bytes are written to
the remote PTY. Server-to-client bytes are written to the local terminal.
Message boundaries have no terminal meaning.

The only text message is a client-to-server resize command:

```json
{"rows":24,"cols":80}
```

Both values are required integers from 1 through 4096. Unknown fields,
trailing JSON, and malformed messages are protocol errors. A malformed control
message is never treated as terminal input.

Each WebSocket message is limited to 1 MiB. RRS does not interpret terminal
escape sequences.

## Local terminal behavior

The reference client enters the alternate screen while connected and restores
the previous screen on exit. It also enables SGR mouse reporting so Windows
console applications behind ConPTY can receive native click, release, drag,
scroll, and hover events. Mouse reports remain binary terminal input; they do
not add a protocol message type.

## Closure

Normal shell exit uses WebSocket status 1000. Server shutdown uses status 1001.
Invalid messages use status 1007 or 1008. A terminal startup or I/O failure uses
status 1011.
