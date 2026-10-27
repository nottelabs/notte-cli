#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT
export GENERATOR_AUTH_FIXTURE="$fixture_dir"
cat > "$fixture_dir/curl" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$GENERATOR_AUTH_FIXTURE/args"
cat > "$GENERATOR_AUTH_FIXTURE/header"
STUB
# Stop after downloading so this test never runs generators or changes code.
cat > "$fixture_dir/jq" <<'STUB'
#!/usr/bin/env bash
exit 17
STUB
chmod +x "$fixture_dir/curl" "$fixture_dir/jq"
check() {
  local url="$1" expected="$2" status=0
  PATH="$fixture_dir:$PATH" NOTTE_API_KEY=fixture-key NOTTE_API_URL="$url" \
    bash "$script_dir/generate.sh" >/dev/null 2>&1 || status=$?
  [[ "$status" == 17 ]] || { echo "Generator did not reach the conversion step"; exit 1; }
  [[ $(cat "$fixture_dir/header") == "$expected" ]] || { echo "Unexpected schema authorization"; exit 1; }
  if grep -q fixture-key "$fixture_dir/args"; then
    echo "Schema credential must not appear in curl arguments"; exit 1
  fi
  if grep -Eq '^(-L|--location)$' "$fixture_dir/args"; then
    echo "Schema downloads must not follow redirects"; exit 1
  fi
}
for host in api.notte.cc us-staging.notte.cc us-dev.notte.cc preview-1-dev-test.notte.cc preview-20-dev-test.notte.cc; do
  check "https://$host" 'Authorization: Bearer fixture-key'
done
for url in https://example.com http://us-staging.notte.cc https://us-staging.notte.cc.evil.test https://preview-21-dev-test.notte.cc https://preview-1@evil.test/dev-test.notte.cc; do
  check "$url" ''
done
echo "Schema authorization tests passed"
