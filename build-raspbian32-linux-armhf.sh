#!/usr/bin/env bash
# Robust, idempotent setup + build script for kaack/elrs-joystick-control on Raspberry Pi
# Uses system SDL2 (libsdl2-dev). No 'static' build tag (go-sdl2 vendored libs break on ARM).

set -euo pipefail

echo "=== System prep ==="
sudo apt-get update -y
sudo apt-get install -y git curl tar build-essential pkg-config libsdl2-dev

# --------------------------------------------------------------------
# Arch detection (armhf vs arm64) and Go toolchain choice
# --------------------------------------------------------------------
GO_VERSION=1.20.7
KERNEL_ARCH="$(uname -m)"
if [[ "$KERNEL_ARCH" == "aarch64" ]]; then
  GO_TARBALL="go${GO_VERSION}.linux-arm64.tar.gz"
  GOARCH_VAL="arm64"
  MULTIARCH="aarch64-linux-gnu"
  unset GOARM
else
  GO_TARBALL="go${GO_VERSION}.linux-armv6l.tar.gz"
  GOARCH_VAL="arm"
  MULTIARCH="arm-linux-gnueabihf"
  # Pick a sensible GOARM based on kernel
  if [[ "$KERNEL_ARCH" == "armv6l" ]]; then
    export GOARM=6
  else
    export GOARM=7
  fi
fi

# Ensure pkg-config can find sdl2.pc for this arch
export PKG_CONFIG_PATH="/usr/lib/${MULTIARCH}/pkgconfig:${PKG_CONFIG_PATH:-}"

# --------------------------------------------------------------------
# Clean any stale libiconv build dir that might be root-owned
# --------------------------------------------------------------------
if [[ -d iconv ]]; then
  echo "=== Cleaning previous ./iconv directory (may require sudo) ==="
  sudo rm -rf iconv || true
fi

# --------------------------------------------------------------------
# Build & install GNU libiconv statically in a temp dir (optional but kept)
# --------------------------------------------------------------------
echo "=== Download, compile, and install libiconv (static) ==="
ICONV_TMP="$(mktemp -d -t iconv-build-XXXXXX)"
trap 'sudo rm -rf "$ICONV_TMP"' EXIT

curl -sfL -o "$ICONV_TMP/libiconv.tar.gz" https://ftp.gnu.org/pub/gnu/libiconv/libiconv-1.17.tar.gz
mkdir -p "$ICONV_TMP/src"
tar -xzvf "$ICONV_TMP/libiconv.tar.gz" -C "$ICONV_TMP/src" --strip-components=1
pushd "$ICONV_TMP/src"
./configure --enable-static --disable-shared --prefix=/usr
make -j"$(nproc)"
sudo make install
popd

# --------------------------------------------------------------------
# Install Go toolchain locally (GOROOT in repo dir)
# --------------------------------------------------------------------
echo "=== Download and install Go ${GO_VERSION} for ${KERNEL_ARCH} ==="
rm -rf go-sdk go go.tar.gz || sudo rm -rf go-sdk go go.tar.gz || true
curl -sfL -o go.tar.gz "https://dl.google.com/go/${GO_TARBALL}"
tar -xzvf go.tar.gz
mv go go-sdk
mkdir -p go

# Go environment
export CC=gcc
export CGO_ENABLED=1
export GOPATH="${PWD}/go"
export GOROOT="${PWD}/go-sdk"
export GOARCH="${GOARCH_VAL}"
export GOOS=linux
export PATH="${GOROOT}/bin:${GOPATH}/bin:${PATH}"

echo "=== Go env ==="
go version
go env GOPATH GOROOT GOOS GOARCH GOARM || true

# Sanity: ensure pkg-config sees SDL2 from system
echo "=== Checking SDL2 with pkg-config ==="
if ! pkg-config --exists sdl2; then
  echo "ERROR: pkg-config cannot find sdl2. Ensure libsdl2-dev is installed and PKG_CONFIG_PATH is correct." >&2
  exit 1
fi
echo "SDL2 version: $(pkg-config --modversion sdl2)"

# --------------------------------------------------------------------
# Prepare webapp assets from the 'github-page' branch WITHOUT switching branches
# Creates dist.tar.gz containing the contents of 'docs' from origin/github-page
# --------------------------------------------------------------------
echo "=== Grab latest webapp (docs) from origin/github-page (no checkout) ==="
git fetch origin github-page
rm -f dist.tar.gz
git archive --format=tar origin/github-page docs | gzip > dist.tar.gz

# Ensure we're on main for the build (ignore if it doesn't exist locally)
echo "=== Switch to main branch for build (if present) ==="
git checkout -q main || true

# Extract the archived docs into webapp/dist (strip the leading 'docs/' path component)
echo "=== Extract webapp dist ==="
mkdir -p webapp/dist
tar -xzvf dist.tar.gz --strip-components=1 -C webapp/dist

# --------------------------------------------------------------------
# Build project (NOTE: NO '-tags static' to avoid go-sdl2 vendored libs)
# --------------------------------------------------------------------
echo "=== Generate version file ==="
go generate pkg/server/version.go

echo "=== Compile binary (system SDL2 via pkg-config) ==="
go build -trimpath -ldflags '-s -w' -o elrs-joystick-control ./cmd/elrs-joystick-control/.

echo "=== Create distribution zip file ==="
go run scripts/cmd/build-release-zip/build-release-zip.go --location . --prefix elrs-joystick-control --files *-control,LICENSE*

echo "=== Done! Artifacts: ./elrs-joystick-control and release zip ==="