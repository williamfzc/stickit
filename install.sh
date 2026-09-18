#!/bin/sh
# stickit installer — the one-line form:
#
#   curl -fsSL https://raw.githubusercontent.com/williamfzc/stickit/main/install.sh | sh
#
# Or run it from a checkout: ./install.sh
#
# Source of the binary, first that works: a prebuilt release asset
# (stickit-<os>-<arch>.tar.gz on the latest GitHub release, binary at the
# archive root), the source checkout the script runs inside, then the Go
# module proxy. Destination: $STICKIT_INSTALL_DIR, else $GOBIN, else
# ~/.local/bin.
set -eu

REPO=williamfzc/stickit
MODULE=github.com/williamfzc/stickit

say() { printf '%s\n' "$*"; }
die() { printf 'stickit install: %s\n' "$*" >&2; exit 1; }

case "${STICKIT_INSTALL_DIR:-${GOBIN:-}}" in
	"") DEST=$HOME/.local/bin ;;
	/*) DEST=${STICKIT_INSTALL_DIR:-$GOBIN} ;;
	*) die "STICKIT_INSTALL_DIR must be absolute" ;;
esac
mkdir -p "$DEST"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# fetch_release downloads the latest prebuilt asset for this platform.
fetch_release() {
	command -v curl >/dev/null 2>&1 || return 1
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	arch=$(uname -m)
	case $arch in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) return 1 ;; esac
	case $os in linux|darwin) ;; *) return 1 ;; esac
	curl -fsSL "https://github.com/$REPO/releases/latest/download/stickit-${os}-${arch}.tar.gz" \
		-o "$tmp/stickit.tar.gz" 2>/dev/null || return 1
	tar -xzf "$tmp/stickit.tar.gz" -C "$tmp" stickit
}

# build_local builds the checkout the script was invoked from, if it is one.
build_local() {
	command -v go >/dev/null 2>&1 || return 1
	d=$(dirname "$0")
	[ -f "$d/go.mod" ] && grep -q "module $MODULE" "$d/go.mod" || return 1
	(cd "$d" && go build -o "$tmp/stickit" .)
}

# install_via_go delegates to the module proxy.
install_via_go() {
	command -v go >/dev/null 2>&1 || return 1
	GOBIN=$DEST go install "$MODULE@latest"
}

if fetch_release; then
	mv "$tmp/stickit" "$DEST/stickit"
	say "downloaded the prebuilt binary"
elif build_local; then
	mv "$tmp/stickit" "$DEST/stickit"
	say "built from the local checkout"
elif install_via_go && [ -x "$DEST/stickit" ]; then
	say "installed via go install"
else
	die "could not fetch a prebuilt release for $(uname -s)/$(uname -m), and could not build from source (this is not a checkout, or Go is not installed). Install Go (https://go.dev) and retry."
fi
chmod +x "$DEST/stickit"

"$DEST/stickit" --skill >/dev/null 2>&1 || die "the installed binary did not run"

say ""
say "stickit installed at $DEST/stickit"
case :$PATH: in
	*":$DEST:"*) ;;
	*) say "note: $DEST is not on your PATH — add it to use stickit by name." ;;
esac
say "Teach an agent the contract: stickit --skill  (stickit --help points there too)"
