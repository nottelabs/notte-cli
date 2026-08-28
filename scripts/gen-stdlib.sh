#!/usr/bin/env bash
# Regenerate internal/bundle/stdlib.go from the runner's Python version.
#
# The list mirrors how notte-api builds its allowlist — set(sys.stdlib_module_names)
# minus a denylist — so the bundler can reject a blocked import locally instead of
# after a multipart upload. The denylist in allowlist.go is applied on top.
#
# The version is pinned rather than taken from whatever python3 is on PATH,
# because stdlib_module_names differs between releases and the whole point is to
# agree with the runner. It tracks the base image in
# apps/back/infrastructure/terraform/workflows-lambda/Dockerfile.fastapi; bump
# both together. That file already carries a scar from a path hardcoded to 3.11
# against a 3.12 base, which pointed at a directory that never existed.
set -euo pipefail
cd "$(dirname "$0")/.."

PYTHON_VERSION="${PYTHON_VERSION:-3.12}"

if command -v uv >/dev/null 2>&1; then
	run_python=(uv run --quiet --python "$PYTHON_VERSION" python)
elif command -v "python$PYTHON_VERSION" >/dev/null 2>&1; then
	run_python=("python$PYTHON_VERSION")
else
	echo "need uv or python$PYTHON_VERSION to generate against the runner's version" >&2
	echo "(refusing to fall back to \$(python3 --version), which would silently disagree)" >&2
	exit 1
fi

"${run_python[@]}" - > internal/bundle/stdlib.go <<'PY'
import sys

if sys.version_info[:2] != (3, 12):
    raise SystemExit(
        f"expected CPython 3.12 to match the runner image, got {sys.version.split()[0]}"
    )

names = sorted(n for n in sys.stdlib_module_names if not n.startswith("_"))
print("package bundle")
print()
print("// Code generated from sys.stdlib_module_names; DO NOT EDIT BY HAND.")
print("// Regenerate with: make generate-stdlib")
print("//")
print(f"// Captured from CPython {sys.version_info.major}.{sys.version_info.minor}, matching the runner image")
print("// (python:3.12.0-slim-bookworm in workflows-lambda/Dockerfile.fastapi). Generating")
print("// from a different version silently disagrees with the runtime in both")
print("// directions, so the generator pins it and refuses to run otherwise.")
print("//")
print("// The denylist in allowlist.go is applied on top of this and always wins.")
print("var stdlibModules = map[string]bool{")
for n in names:
    print('\t"%s": true,' % n)
print("}")
print()
print("// DefaultStdlib is the standard library set used when a caller does not supply")
print("// one, which is every caller that is not a test.")
print("func DefaultStdlib() map[string]bool { return stdlibModules }")
PY

gofmt -w internal/bundle/stdlib.go
echo "wrote internal/bundle/stdlib.go from CPython $PYTHON_VERSION"
