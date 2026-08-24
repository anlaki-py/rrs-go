# Handoff: one-command GitHub installation

## Goal

Make RRS easy to install from `https://github.com/anlaki-py/rrs-go` with one
command. The install flow must support Linux and Windows amd64 without requiring
users to clone the repository or install Go.

## Current state

The Windows amd64 port is complete. Linux amd64 and arm64 builds also work. The
`master` branch matches `origin/master` as of 2026-08-25.

`.github/workflows/ci.yml` tests and builds all supported targets, but it does
not publish release artifacts. There is no installer script or release workflow
yet.

## Files involved

- `.github/workflows/ci.yml` tests and builds supported targets.
- `README.md` contains the current build and usage instructions.
- `internal/buildinfo/buildinfo.go` contains build metadata support.

## Constraints

- Release and cross-build commands must set `CGO_ENABLED=0`.
- Support Linux amd64, Linux arm64, and Windows amd64.
- Verify downloaded binaries with checksums.
- Do not require Go on the user's machine.
- Keep installation non-interactive by default and report where `rrs` was put.

## Next steps

First, decide the cleanest one-command installation experience for Linux shells
and Windows PowerShell. Then add tagged GitHub release builds, checksums, and the
small installer scripts those commands download.

The installer must select the correct OS and architecture, download a pinned or
latest release from GitHub, verify its checksum, install it into a sensible user
binary directory, and explain any required PATH change.

Update `README.md` with the final commands and test installation on a clean Linux
environment and a clean Windows environment.

## How to verify current state

Run:

```sh
git status --short --branch
go test ./...
```

Then inspect `.github/workflows/ci.yml` and the GitHub Releases page before
designing the release workflow.
