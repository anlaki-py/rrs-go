# Decision 0001: rewrite RRS in Go

Status: accepted

## Context

The TypeScript implementation requires Node.js, npm, and native modules for
PTY and Windows console access. RRS is a command-line remote shell, so a
single executable and explicit process ownership are more useful than the
Node.js runtime.

## Decision

RRS will use Go 1.26 as its minimum language version. The rewrite keeps the
client, server, and protocol in one executable. Platform PTY and console code
will live behind small internal packages and build tags.

The first milestone supports Linux. Windows builds must compile, but Windows
support will remain unavailable until ConPTY process-tree cleanup and console
restoration pass on a Windows runner.

## Consequences

The Go and TypeScript implementations may coexist during migration. The Go
protocol is versioned as `rrs.v1` and intentionally removes automatic TLS
downgrade and malformed-text fallback. Cloudflare tunnel and self-update
support will follow the terminal core instead of blocking it.
