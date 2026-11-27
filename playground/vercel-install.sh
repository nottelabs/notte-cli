#!/bin/sh
# Vercel's build image has no Go toolchain: install the version go.mod asks for.
set -eu
version=$(sed -n 's/^go //p' ../go.mod)
curl -fsSL "https://go.dev/dl/go${version}.linux-amd64.tar.gz" | tar -xz -C /tmp
/tmp/go/bin/go version
