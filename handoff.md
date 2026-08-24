# RRS Go Rewrite Handoff

## Goal

Continue the RRS rewrite as a maintainable, self-contained Go executable. This is a personal project, but the code should follow professional Go engineering practices without unnecessary frameworks or abstraction. Linux terminal sessions work. The next major milestone is real Windows support using ConPTY, developed and tested on Windows.

## Current State

- Repository: `/home/projects/rrs-go`
- Branch: `master`
- Go module: `github.com/anlaki-py/rrs`
- Minimum Go version: 1.26
- The initial rewrite implements `rrs serve` and `rrs connect`.
- Linux PTY sessions, terminal resizing, bearer authentication, health/info endpoints, and orderly session shutdown are implemented and tested.
- The WebSocket protocol is documented and versioned as `rrs.v1`.
- Linux ARM64, Linux AMD64, and Windows AMD64 builds compile with `CGO_ENABLED=0`.
- Windows currently compiles through unsupported-platform stubs; it does not yet provide a usable terminal session.
- Tunneling, self-update, packaging, and release automation are not implemented.
- No Git remote is configured. This handoff is included in the initial commit, but the repository must be pushed or otherwise transferred before it can be cloned on Windows.

## What Was Implemented

- A small standard-library CLI under `cmd/rrs` and `internal/cli`.
- A strict WebSocket client/server protocol under `internal/protocol`.
- Server authentication and safe listener defaults under `internal/server`.
- Linux PTY lifecycle management under `internal/terminal`.
- Cancellable Linux console input and terminal-state restoration under `internal/console`.
- Unit and Linux integration tests for CLI parsing, URL policy, authentication, protocol validation, PTY behavior, session I/O, resizing, and shutdown.
- Architecture and protocol documentation under `docs`.
- GitHub Actions checks for Go 1.26 and 1.27 on Linux, plus cross-platform builds.

## Important Files

- `README.md`: supported features, commands, security behavior, and roadmap.
- `AGENTS.md`: repository-specific engineering rules.
- `docs/protocol.md`: exact `rrs.v1` wire protocol.
- `docs/decisions/0001-go-rewrite.md`: architecture decision record.
- `internal/server/session.go`: WebSocket-to-terminal session orchestration.
- `internal/terminal/terminal_linux.go`: Linux PTY and process-group lifecycle.
- `internal/terminal/terminal_unsupported.go`: current non-Linux stub.
- `internal/console/console_linux.go`: Linux raw console and cancellable reads.
- `internal/console/console_unsupported.go`: current non-Linux stub.
- `.github/workflows/ci.yml`: supported checks and build matrix.

## Engineering Constraints

- Keep one self-contained executable. Release builds should explicitly use `CGO_ENABLED=0`.
- Keep platform-specific code behind `internal/terminal` and `internal/console` build-tagged implementations.
- Do not add Cobra, Viper, dependency-injection frameworks, or generic utility packages without a demonstrated need.
- Preserve the strict `rrs.v1` protocol. Malformed control text must never reach the shell as input.
- Do not add automatic TLS downgrade. Remote `ws://` connections require the explicit `--allow-plaintext` option.
- Do not claim Windows support until terminal creation, resize, console restoration, and child-process cleanup have passed runtime tests on Windows.
- Keep errors contextual and preserve causes with `%w` where applicable.
- Keep shutdown paths idempotent and bounded. Avoid goroutines that can remain blocked after a session ends.
- Do not add `SysProcAttr.Setpgid` to the Linux PTY command while `github.com/creack/pty` starts it with `Setsid`; that combination failed in this environment.

## Environment and Failed Attempts

The source environment is Termux glibc on Linux ARM64 with the official Go 1.26.6 toolchain. Its default toolchain enables cgo, but portable release builds were verified with `CGO_ENABLED=0`.

Two failures were environmental or implementation-specific and should not be rediscovered:

1. `go test -race ./...` cannot run here because ThreadSanitizer exits with:

   ```text
   FATAL: ThreadSanitizer: unsupported VMA range
   FATAL: Found 39 - Supported 48
   ```

   The race check remains in Ubuntu CI. Local race testing was intentionally skipped after confirming this limitation.

2. Combining `SysProcAttr.Setpgid` with the session setup performed by `github.com/creack/pty` caused:

   ```text
   start bash terminal: fork/exec /data/data/com.termux/files/usr/bin/bash: operation not permitted
   ```

   Removing the redundant `Setpgid` fixed PTY startup. The PTY package already creates a new session/process group on Linux.

## Windows Work

1. Clone or transfer the repository to Windows, then establish the baseline:

   ```powershell
   go test -timeout 2m ./...
   go vet ./...
   go build ./cmd/rrs
   ```

2. Implement `internal/terminal/terminal_windows.go` and change the unsupported file's build constraint from non-Linux to non-Linux/non-Windows. `github.com/charmbracelet/x/conpty` is the preferred starting point, with `golang.org/x/sys/windows` for native lifecycle primitives. Confirm the dependency APIs against their current official source before integrating them.

3. Put the ConPTY shell and descendants in a Windows Job Object configured with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Ensure close, wait, and concurrent shutdown paths are safe and idempotent.

4. Implement `internal/console/console_windows.go` and narrow the unsupported console build constraint. Save and exactly restore the original input/output console modes. Enable virtual-terminal input/output only for the active session.

5. Add Windows runtime tests for PowerShell startup and I/O, ConPTY resizing, console restoration after success and error paths, clean session cancellation, and descendant-process termination through the Job Object.

6. Update support claims in `README.md` only after those tests pass on a real Windows host and in Windows CI.

7. After Windows support is stable, continue with tunneling, updater verification, packaging, and release automation as separate milestones.

## Verification Baseline

The following checks passed before the initial commit:

```sh
go vet ./...
go test -count=3 -timeout 2m ./...
CGO_ENABLED=0 go build ./cmd/rrs
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/rrs
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/rrs
```

When resuming, read `AGENTS.md`, this handoff, the protocol document, and the architecture decision before editing platform code. Keep Windows work isolated behind the existing interfaces rather than adding operating-system branches to the server or client packages.
