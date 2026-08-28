#!/usr/bin/env bash
# Regenerate internal/bundle/stdlib.go from the local interpreter.
#
# The list mirrors how notte-api builds its allowlist (set(sys.stdlib_module_names)
# minus a denylist), so the bundler can reject a blocked import locally instead
# of after a multipart upload. The denylist in allowlist.go is applied on top.
set -euo pipefail
cd "$(dirname "$0")/.."

python3 - > internal/bundle/stdlib.go <<'PY'
import sys
names = sorted(n for n in sys.stdlib_module_names if not n.startswith("_"))
print("package bundle")
print()
print("// Code generated from sys.stdlib_module_names; DO NOT EDIT BY HAND.")
print("// Regenerate with: make generate-stdlib")
print("//")
print(f"// Captured from CPython {sys.version_info.major}.{sys.version_info.minor}. The runner image may be on a")
print("// different minor version, so this list can name a module the runtime does not")
print("// have. That direction is the safe one: the upload still rejects it and the")
print("// local check merely fails to catch it early. The denylist in allowlist.go is")
print("// applied on top and always wins.")
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
echo "wrote internal/bundle/stdlib.go"
