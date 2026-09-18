#!/bin/sh
# Full check suite for stickit. Run before opening a change.
set -e
cd "$(dirname "$0")/.."
python3 scripts/validate_docs.py
GO=${GO:-go}
"$GO" vet ./...
"$GO" test ./...
bad=$(gofmt -l .)
[ -z "$bad" ] || { echo "gofmt needed for:"; echo "$bad"; exit 1; }
