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
# Stop at the download so the test never reaches codegen or writes the
# generator's shared temporary paths.
exit 17
STUB
chmod +x "$fixture_dir/curl"
check() {
  local url="$1" expected="$2" key="${3-fixture-key}" status=0
  if [[ "$key" == unset ]]; then
    unset NOTTE_API_KEY
  else
    export NOTTE_API_KEY="$key"
  fi
  PATH="$fixture_dir:$PATH" NOTTE_API_URL="$url" \
    bash "$script_dir/generate.sh" >/dev/null 2>&1 || status=$?
  [[ "$status" == 1 ]] || { echo "Generator did not report the fixture download failure"; exit 1; }
  [[ $(cat "$fixture_dir/header") == "$expected" ]] || { echo "Unexpected schema authorization"; exit 1; }
  [[ $(head -n 1 "$fixture_dir/args") == --disable ]] || { echo "curl must ignore local configuration first"; exit 1; }
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
check https://us-staging.notte.cc '' unset
check https://us-staging.notte.cc '' ''

# Exercise real curl with a redirect-enabling configuration. The wrapper only
# maps the approved URL and output file into this local fixture, then stops
# generation after curl so no codegen or shared temporary files are touched.
python3 - "$script_dir/generate.sh" "$fixture_dir" <<'PY'
import os
import pathlib
import shutil
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

generator, fixture = sys.argv[1], pathlib.Path(sys.argv[2])
real_curl = shutil.which("curl")
assert real_curl
requests = []

class Redirect(BaseHTTPRequestHandler):
    def do_GET(self):
        requests.append((self.path, self.headers.get("Authorization")))
        self.send_response(302 if self.path == "/openapi.json" else 200)
        self.send_header("Location", "/elsewhere")
        self.end_headers()

    def log_message(self, *args):
        pass

server = ThreadingHTTPServer(("127.0.0.1", 0), Redirect)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
try:
    (fixture / ".curlrc").write_text("location\nlocation-trusted\n")
    # Prove that curl reads this config and follows redirects without --disable.
    env = {**os.environ, "CURL_HOME": str(fixture), "HOME": str(fixture)}
    endpoint = f"http://127.0.0.1:{server.server_port}/openapi.json"
    subprocess.run([real_curl, "-s", "--noproxy", "*", endpoint], env=env, check=True)
    assert [path for path, _ in requests] == ["/openapi.json", "/elsewhere"]
    requests.clear()
    (fixture / "curl").write_text('''#!/usr/bin/env python3
import os
import subprocess
import sys
args = [os.environ["LOCAL_SCHEMA_URL"] if arg == "https://us-staging.notte.cc/openapi.json"
        else os.environ["LOCAL_SCHEMA_OUTPUT"] if arg == "/tmp/notte-openapi.json"
        else arg for arg in sys.argv[1:]]
subprocess.run([os.environ["REAL_CURL"], *args], check=True)
sys.exit(17)
''')
    env.update(PATH=f"{fixture}:{env['PATH']}", REAL_CURL=real_curl,
               LOCAL_SCHEMA_URL=endpoint, LOCAL_SCHEMA_OUTPUT=str(fixture / "schema"),
               NOTTE_API_URL="https://us-staging.notte.cc", NOTTE_API_KEY="fixture-key",
               NO_PROXY="*", no_proxy="*")
    result = subprocess.run(["bash", generator], env=env, capture_output=True)
    assert result.returncode == 1
    assert requests == [("/openapi.json", "Bearer fixture-key")], requests
finally:
    server.shutdown()
    server.server_close()
    thread.join()
PY
echo "Schema authorization tests passed"
