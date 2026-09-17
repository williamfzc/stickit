#!/bin/sh
# Full check suite for stickit. Run before opening a change.
set -e
cd "$(dirname "$0")/.."
python3 scripts/validate_docs.py
