#!/bin/sh
# Installs the rrs binary from GitHub releases into ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/anlaki-py/rrs-go/master/install.sh | sh
#
# The download is checksum-verified against the release's checksums.txt before
# anything is placed on disk, and the final move into the install directory is
# a rename inside that directory, so a failed or interrupted run cannot leave
# a half-written binary behind. Release assets use stable names, which lets
# this script resolve the latest release through a plain redirect instead of
# the GitHub API; no tokens, jq, or rate limits involved.
#
# Environment overrides:
#   RRS_VERSION       install a specific tag instead of the latest (e.g. v0.0.1)
#   RRS_INSTALL_DIR   target directory (default: ~/.local/bin)
#   RRS_RELEASES_URL  release base URL (default: the upstream repository)
#
# Only Linux is packaged today. rrs is built with CGO_ENABLED=0, so the
# binaries are static and work on musl and glibc systems alike.

set -eu

REPOSITORY="anlaki-py/rrs-go"
BINARY_NAME="rrs"
DEFAULT_INSTALL_DIR="$HOME/.local/bin"

log() {
	printf 'install.sh: %s\n' "$1"
}

warn() {
	printf 'install.sh: %s\n' "$1" >&2
}

die() {
	warn "error: $1"
	exit 1
}

usage() {
	cat <<'EOF'
install.sh installs the rrs binary from GitHub releases.

Usage:
  curl -fsSL https://raw.githubusercontent.com/anlaki-py/rrs-go/master/install.sh | sh

Options:
  -h, --help    show this help

Environment:
  RRS_VERSION       release tag to pin (default: latest release)
  RRS_INSTALL_DIR   install directory (default: ~/.local/bin)
EOF
}

for argument in "$@"; do
	case $argument in
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown argument '$argument'; try --help"
		;;
	esac
done

case "$(uname -s)" in
Linux) ;;
Darwin)
	die "no macOS build is packaged yet; see https://github.com/$REPOSITORY/releases for sources"
	;;
MINGW* | MSYS* | CYGWIN* | Windows*)
	die "this installer supports linux only; download rrs-windows-amd64.exe from https://github.com/$REPOSITORY/releases"
	;;
*)
	die "unsupported operating system '$(uname -s)'; this installer supports linux"
	;;
esac

case "$(uname -m)" in
x86_64 | amd64)
	arch="amd64"
	;;
aarch64 | arm64)
	arch="arm64"
	;;
*)
	die "unsupported architecture '$(uname -m)'; available builds: amd64, arm64"
	;;
esac

asset="$BINARY_NAME-linux-$arch"

releases_url="${RRS_RELEASES_URL:-https://github.com/$REPOSITORY/releases}"
if [ -n "${RRS_VERSION:-}" ]; then
	case $RRS_VERSION in
	'' | . | ..* | *..* | *[!A-Za-z0-9._-]*)
		die "RRS_VERSION '$RRS_VERSION' does not look like a release tag such as v0.0.1"
		;;
	esac
	download_base="$releases_url/download/$RRS_VERSION"
	log "installing pinned version $RRS_VERSION"
else
	download_base="$releases_url/latest/download"
	log "installing the latest release"
fi

fetch() {
	# fetch <url> <destination-file>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 15 \
			-o "$2" "$1" || die "download failed: $1"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" --tries=3 --waitretry=2 --timeout=30 \
			"$1" || die "download failed: $1"
	else
		die "need curl or wget to download files"
	fi
}

hash_file() {
	# hash_file <path> prints the sha256 digest
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		die "need sha256sum or shasum to verify downloads"
	fi
}

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/rrs-install.XXXXXX") || die "cannot create a temporary directory"
cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

log "downloading $asset and checksums.txt"
fetch "$download_base/$asset" "$work_dir/$asset"
fetch "$download_base/checksums.txt" "$work_dir/checksums.txt"

[ -s "$work_dir/$asset" ] || die "the downloaded binary is empty; the release may be broken"

entries=$(awk -v name="$asset" '$2 == name' "$work_dir/checksums.txt" | wc -l)
if [ "$entries" -ne 1 ]; then
	die "expected exactly one checksum entry for $asset, found $entries"
fi
expected_hash=$(awk -v name="$asset" '$2 == name { print $1 }' "$work_dir/checksums.txt")
actual_hash=$(hash_file "$work_dir/$asset")
if [ "$actual_hash" != "$expected_hash" ]; then
	die "checksum mismatch for $asset (expected $expected_hash, got $actual_hash)"
fi
log "checksum verified"

install_dir="${RRS_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
case $install_dir in
/*) ;;
*)
	die "RRS_INSTALL_DIR must be an absolute path (got '$install_dir')"
	;;
esac
destination="$install_dir/$BINARY_NAME"

if [ -e "$destination" ]; then
	if [ "$(hash_file "$destination" 2>/dev/null || true)" = "$actual_hash" ]; then
		log "$BINARY_NAME is already installed at $destination with this exact version"
		exit 0
	fi
	log "replacing the existing file at $destination"
fi

if [ ! -d "$install_dir" ]; then
	mkdir -p "$install_dir" || die "cannot create $install_dir"
	log "created $install_dir"
fi

# Stage inside the install directory so the final move is an atomic rename.
staged="$install_dir/.rrs.$$.tmp"
cp "$work_dir/$asset" "$staged" || {
	rm -f "$staged"
	die "cannot write to $install_dir"
}
chmod 0755 "$staged"
mv -f "$staged" "$destination" || {
	rm -f "$staged"
	die "cannot move the binary into place at $destination"
}

if installed_version=$("$destination" --version 2>/dev/null); then
	log "installed $BINARY_NAME $installed_version to $destination"
else
	warn "installed $BINARY_NAME to $destination but could not execute it to confirm the version"
fi

case ":${PATH}:" in
*":$install_dir:"*) ;;
*)
	warn "$install_dir is not in your PATH"
	case "$(basename "${SHELL:-}")" in
	zsh)
		warn "add it with: echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.zshrc"
		;;
	bash)
		warn "add it with: echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.bashrc"
		;;
	fish)
		warn "add it with: fish_add_path $install_dir"
		;;
	*)
		warn "add $install_dir to your PATH using your shell's configuration"
		;;
	esac
	;;
esac
