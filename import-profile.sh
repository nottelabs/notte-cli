#!/bin/sh
# Import the logins from a browser on this machine into a Notte profile.
#
#   curl -fsSL https://notte.cc/import-profile.sh | sh -s -- <profile-id>
#
# Uses the notte CLI already on PATH, or installs it to ~/.local/bin, logs in
# when no API key is stored, then runs `notte profiles sync`. That command lets
# you pick the browser profile and asks before anything is uploaded. Arguments
# after the profile id are passed to it, e.g. --domain github.com.
#
# Environment variables:
#   NOTTE_API_URL      API to use (default: production)
#   NOTTE_INSTALL_DIR  where to install the CLI when needed (default: ~/.local/bin)
#   NOTTE_TTY          terminal to read answers from (default: /dev/tty; for tests)

set -eu

INSTALL_URL="https://raw.githubusercontent.com/nottelabs/notte-cli/main/install.sh"

fail() {
	echo "notte import: $*" >&2
	exit 1
}

# Everything runs from main, called on the last line, so a truncated download
# piped into sh never executes a partial script.
main() {
	[ $# -ge 1 ] || fail "usage: curl -fsSL https://notte.cc/import-profile.sh | sh -s -- <profile-id>"
	profile_id=$1
	shift
	case "$profile_id" in
	"" | -* | *[!A-Za-z0-9_-]*) fail "invalid profile id: $profile_id" ;;
	esac

	# Piped from curl, standard input is this script: questions are read from
	# the terminal instead.
	tty="${NOTTE_TTY:-/dev/tty}"
	(: <"$tty") 2>/dev/null || fail "run this in an interactive terminal"

	# Any CLI that has `profiles sync` will do; otherwise install the latest
	# one for this user only, so no sudo password is needed.
	notte=$(command -v notte 2>/dev/null || true)
	if [ -z "$notte" ] || ! "$notte" profiles sync --help >/dev/null 2>&1; then
		command -v curl >/dev/null 2>&1 || fail "curl is required"
		dir="${NOTTE_INSTALL_DIR:-$HOME/.local/bin}"
		curl -fsSL "$INSTALL_URL" | NOTTE_INSTALL_DIR="$dir" sh || fail "could not install the notte CLI"
		notte="$dir/notte"
	fi

	if ! "$notte" whoami >/dev/null 2>&1; then
		"$notte" login <"$tty" || fail "login failed"
	fi

	if ! "$notte" profiles sync --profile-id "$profile_id" "$@" <"$tty"; then
		echo "If the profile was not found, this CLI may be logged in to another workspace: run \`$notte login\` and try again." >&2
		exit 1
	fi
}

main "$@"
