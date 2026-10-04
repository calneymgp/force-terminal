#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
mode="${1:-}"
export FORCE_RELEASE_MODE="${FORCE_RELEASE_MODE:-adhoc}"
node scripts/force-release-policy.cjs
if [[ "$FORCE_RELEASE_MODE" == adhoc ]]; then
    export CSC_IDENTITY_AUTO_DISCOVERY=false
elif [[ "$FORCE_RELEASE_MODE" == community ]]; then
    export CSC_LINK="data:application/x-pkcs12;base64,$FORCE_MACOS_CERT_P12_BASE64"
    export CSC_KEY_PASSWORD="$FORCE_MACOS_CERT_PASSWORD"
    export CSC_IDENTITY_AUTO_DISCOVERY=false
fi
if [[ "$(uname -s)" != Darwin || "$(uname -m)" != arm64 ]]; then
    echo "Force macOS build requires an Apple Silicon Mac" >&2
    exit 1
fi
if [[ "$mode" != dev && "$mode" != test-profile && "$mode" != package ]]; then
    echo "Usage: force-build-mac.sh dev|test-profile|package" >&2
    exit 1
fi

# Legacy Wave endpoint settings must not leak from the invoking shell.
unset WAVETERM_ENVFILE WCLOUD_PING_ENDPOINT WCLOUD_ENDPOINT WCLOUD_WS_ENDPOINT
if [[ "$mode" == test-profile ]]; then
    test_root="${FORCE_TERMINAL_TEST_ROOT:-$(mktemp -d "${TMPDIR:-/tmp}/force-terminal-test.XXXXXX")}"
    [[ "$test_root" == /* ]] || { echo "FORCE_TERMINAL_TEST_ROOT must be absolute" >&2; exit 1; }
    export FORCE_TERMINAL_DATA_HOME="$test_root/data"
    export FORCE_TERMINAL_CONFIG_HOME="$test_root/config"
    export FORCE_TERMINAL_CACHE_HOME="$test_root/cache"
    mkdir -p "$FORCE_TERMINAL_DATA_HOME" "$FORCE_TERMINAL_CONFIG_HOME" "$FORCE_TERMINAL_CACHE_HOME"
    echo "Force Terminal test profile: $test_root"
fi

version="$(node -p 'require("./package.json").version')"
build_time="$(git show -s --format=%cd --date=format:%Y%m%d%H%M HEAD)"
if [[ "$mode" == package ]]; then
    # These are generated workspace outputs only. A package must never reuse old architectures or versions.
    rm -rf dist/bin dist/schema dist/tsunamiscaffold make
fi
mkdir -p dist/bin dist/schema
cp schema/*.json dist/schema/

CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -mod=readonly -tags osusergo,sqlite_omit_load_extension \
    -ldflags "-X main.BuildTime=$build_time -X main.WaveVersion=$version" \
    -o dist/bin/wavesrv.arm64 cmd/server/main-server.go

for target in darwin:arm64 darwin:amd64 linux:arm64 linux:amd64 linux:mips linux:mips64 windows:amd64 windows:arm64; do
    goos="${target%:*}"
    goarch="${target#*:}"
    normalized="$goarch"
    [[ "$goarch" == amd64 ]] && normalized=x64
    ext=""
    [[ "$goos" == windows ]] && ext=.exe
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -mod=readonly \
        -ldflags "-s -w -X main.BuildTime=$build_time -X main.WaveVersion=$version" \
        -o "dist/bin/wsh-$version-$goos.$normalized$ext" cmd/wsh/main-wsh.go
done

npm run --workspace tsunami-frontend build
scaffold=tsunami/frontend/scaffold
rm -rf "$scaffold" dist/tsunamiscaffold
mkdir -p "$scaffold"
cp scripts/force-scaffold/package.json scripts/force-scaffold/package-lock.json "$scaffold/"
cp tsunami/templates/gitignore.tmpl "$scaffold/.gitignore"
npm ci --prefix "$scaffold" --no-workspaces --no-audit --no-fund --ignore-scripts
mv "$scaffold/node_modules" "$scaffold/nm"
mkdir -p "$scaffold/dist/tw"
cp -R tsunami/frontend/dist/. "$scaffold/dist/"
cp tsunami/templates/*.go.tmpl tsunami/templates/tailwind.css "$scaffold/"
cp tsunami/frontend/src/element/*.tsx "$scaffold/dist/tw/"
cp tsunami/ui/*.go tsunami/engine/errcomponent.go "$scaffold/dist/tw/"
mkdir -p dist/tsunamiscaffold
cp -R "$scaffold/." dist/tsunamiscaffold/
cp tsunami/templates/empty-gomod.tmpl dist/tsunamiscaffold/go.mod

if [[ "$mode" == package ]]; then
    npm run build:prod
    npm exec electron-builder -- -c electron-builder.config.cjs --mac --arm64 --publish never
else
    npm run dev
fi
