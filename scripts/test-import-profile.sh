#!/bin/sh
# Tests import-profile.sh against a fake notte CLI: which commands it runs,
# in which order, and that a failed login stops the import.
#
#   sh scripts/test-import-profile.sh

set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# A notte that records each call. FAKE_WHOAMI and FAKE_LOGIN set the exit codes
# of `whoami` and `login`; `profiles sync --help` succeeds, so it is reused.
mkdir -p "$work/bin"
cat >"$work/bin/notte" <<'EOF'
#!/bin/sh
case "$1 ${2:-} ${3:-}" in
"profiles sync --help") exit 0 ;;
esac
echo "$*" >>"$FAKE_LOG"
case "$1" in
whoami) exit "${FAKE_WHOAMI:-0}" ;;
login) exit "${FAKE_LOGIN:-0}" ;;
esac
exit 0
EOF
chmod +x "$work/bin/notte"

failures=0

# run NAME EXPECTED_STATUS EXPECTED_CALLS [ENV...] -- ARGS...
run() {
	name=$1 want_status=$2 want_calls=$3
	shift 3
	log="$work/$name.log"
	: >"$log"
	status=0
	env PATH="$work/bin:$PATH" FAKE_LOG="$log" NOTTE_TTY=/dev/null "$@" \
		sh "$root/import-profile.sh" notte-profile-abc123 --domain github.com </dev/null >/dev/null 2>&1 || status=$?
	calls=$(cat "$log")
	if [ "$status" != "$want_status" ] || [ "$calls" != "$want_calls" ]; then
		echo "FAIL $name: status $status (want $want_status), calls:" >&2
		echo "$calls" >&2
		echo "want:" >&2
		echo "$want_calls" >&2
		failures=$((failures + 1))
	else
		echo "ok   $name"
	fi
}

nl='
'

run reuses_cli_when_logged_in 0 \
	"whoami${nl}profiles sync --profile-id notte-profile-abc123 --domain github.com" \
	FAKE_WHOAMI=0

run logs_in_without_a_key 0 \
	"whoami${nl}login${nl}profiles sync --profile-id notte-profile-abc123 --domain github.com" \
	FAKE_WHOAMI=1

run stops_when_login_fails 1 \
	"whoami${nl}login" \
	FAKE_WHOAMI=1 FAKE_LOGIN=1

# Invalid profile ids are refused before any command runs.
log="$work/invalid.log"
: >"$log"
if env PATH="$work/bin:$PATH" FAKE_LOG="$log" NOTTE_TTY=/dev/null \
	sh "$root/import-profile.sh" 'bad id' >/dev/null 2>&1 || [ -s "$log" ]; then
	echo "FAIL rejects_invalid_profile_id" >&2
	failures=$((failures + 1))
else
	echo "ok   rejects_invalid_profile_id"
fi

# Without a terminal it stops before running anything.
log="$work/notty.log"
: >"$log"
if env PATH="$work/bin:$PATH" FAKE_LOG="$log" NOTTE_TTY="$work/missing-tty" \
	sh "$root/import-profile.sh" notte-profile-abc123 >/dev/null 2>&1 || [ -s "$log" ]; then
	echo "FAIL requires_a_terminal" >&2
	failures=$((failures + 1))
else
	echo "ok   requires_a_terminal"
fi

[ "$failures" -eq 0 ] || exit 1
