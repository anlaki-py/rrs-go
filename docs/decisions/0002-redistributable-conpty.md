# Use Microsoft's redistributable ConPTY on Windows

## Status

Accepted.

## Context

RRS needs to carry VT terminal input and output between remote terminal
emulators and native Windows console applications. The inbox ConPTY on older
Windows 10 releases consumes application mouse-mode sequences but does not
reliably translate incoming VT mouse reports into native `MOUSE_EVENT` records.
That makes fullscreen Windows TUI applications unusable through RRS even when
the same client works against a Linux server.

Microsoft publishes `Microsoft.Windows.Console.ConPTY`, a native NuGet package
that supports Windows 10 build 17763 and newer. It contains a matched
`conpty.dll` and architecture-specific `OpenConsole.exe` hosts.

## Decision

Windows amd64 builds embed the stable package version `1.24.260710001`. At
runtime RRS writes the files to a versioned directory below the user's cache,
verifies their SHA-256 content, and loads `conpty.dll` by absolute path. The
bundle includes x64 and ARM64 hosts because an x64 process may run under ARM64
emulation.

RRS releases the ConPTY reference after attaching the shell and closes the
parent copies of the pseudo-console pipe ends. This lets terminal output reach
EOF when PowerShell exits instead of leaving the WebSocket session hanging.

## Consequences

- Windows mouse input and terminal behavior no longer depend on the operating
  system's inbox ConPTY version.
- `rrs.exe` remains the only file users distribute, but the Windows binary is
  larger and writes its signed backend files to the user cache on first use.
- Updating ConPTY requires an explicit package-version update, binary review,
  integration test run, and notice/hash update.

Package: <https://www.nuget.org/packages/Microsoft.Windows.Console.ConPTY/1.24.260710001>

Source: <https://github.com/microsoft/terminal>
