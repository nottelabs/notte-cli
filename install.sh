#!/bin/sh
# Install the notte CLI from its GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/nottelabs/notte-cli/main/install.sh | sh
#
# Environment variables:
#   NOTTE_VERSION      version to install, e.g. 0.0.48 (default: latest release)
#   NOTTE_INSTALL_DIR  directory to install into (default: /usr/local/bin, or
#                      ~/.local/bin when /usr/local/bin is not writable and sudo
#                      is unavailable)

set -eu

REPO="nottelabs/notte-cli"
RELEASES="https://github.com/$REPO/releases"

fail() {
	echo "notte install: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

# Install the extracted binary into $1, running mkdir and install under $2 (e.g. sudo).
install_to() {
	$2 mkdir -p -- "$1" && $2 install -m 755 -- "$tmp/notte" "$1/notte"
}

# Everything runs from main, called on the last line, so a truncated download
# piped into sh never executes a partial script.
main() {
	need curl
	need tar
	need uname

	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "unsupported OS $(uname -s); on Windows, download the zip from $RELEASES/latest" ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported architecture $(uname -m)" ;;
	esac

	version="${NOTTE_VERSION:-}"
	if [ -z "$version" ]; then
		# releases/latest redirects to the newest tag, e.g. .../releases/tag/v1.2.3
		latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$RELEASES/latest")
		version="${latest_url##*/tag/v}"
		if [ -z "$version" ] || [ "$version" = "$latest_url" ]; then
			fail "could not resolve the latest release"
		fi
	fi
	version="${version#v}"

	archive="notte-cli_${version}_${os}_${arch}.tar.gz"
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT

	echo "Downloading notte $version for $os/$arch"
	curl -fsSL -o "$tmp/$archive" "$RELEASES/download/v$version/$archive" ||
		fail "download failed: $RELEASES/download/v$version/$archive"
	curl -fsSL -o "$tmp/checksums.txt" "$RELEASES/download/v$version/checksums.txt" ||
		fail "could not download checksums.txt"

	expected=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
	[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
	elif command -v shasum >/dev/null 2>&1; then
		actual=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
	else
		fail "sha256sum or shasum is required to verify the download"
	fi
	[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive"

	tar -xzf "$tmp/$archive" -C "$tmp" notte

	if [ -n "${NOTTE_INSTALL_DIR:-}" ]; then
		dir="$NOTTE_INSTALL_DIR"
		install_to "$dir" "" || fail "could not install to $dir"
	else
		dir=/usr/local/bin
		if [ -w "$dir" ]; then
			install_to "$dir" "" || fail "could not install to $dir"
		elif command -v sudo >/dev/null 2>&1 && install_to "$dir" sudo; then
			:
		else
			# No usable sudo (missing, or denied): install for the current user instead.
			dir="$HOME/.local/bin"
			install_to "$dir" "" || fail "could not install to $dir"
		fi
	fi
	echo "Installed notte $version to $dir/notte"

	# Check which notte the shell will actually run: another copy earlier on PATH
	# (e.g. an older one in /usr/local/bin when sudo was denied) would shadow this one.
	resolved=$(command -v notte 2>/dev/null || true)
	if [ -z "$resolved" ]; then
		echo "Add $dir to your PATH to use notte"
	elif [ "$resolved" != "$dir/notte" ]; then
		echo "Warning: $resolved comes before $dir/notte on your PATH, so running notte still uses it." >&2
		echo "Remove $resolved or put $dir first on your PATH." >&2
	fi
}

main
