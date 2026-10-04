#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
release_mode="$(node -p 'require("./scripts/force-release-policy.cjs").releaseMode()')"
if [[ "$release_mode" == community && ! "${FORCE_MACOS_CERT_SHA256:-}" =~ ^[[:xdigit:]]{64}$ ]]; then
    echo "FORCE_MACOS_CERT_SHA256 must be a SHA-256 hex fingerprint" >&2
    exit 1
fi

version="$(node -p 'require("./package.json").version')"
app="make/mac-arm64/Force Terminal.app"
binary="$app/Contents/MacOS/Force Terminal"
backend="$app/Contents/Resources/app.asar.unpacked/dist/bin/wavesrv.arm64"
if [[ ! -d "$app" || ! -f "$binary" || ! -f "$backend" ]]; then
    echo "macOS arm64 app or backend missing" >&2
    exit 1
fi
[[ "$(lipo -archs "$binary")" == arm64 ]]
[[ "$(lipo -archs "$backend")" == arm64 ]]
go version -m "$backend" | grep -q 'GOARCH=arm64'

check_bundle() {
    local packaged_app="$1"
    local plist="$packaged_app/Contents/Info.plist"
    local resources="$packaged_app/Contents/Resources"
    [[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$plist")" == io.github.calneymgp.force-terminal ]]
    [[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleName' "$plist")" == 'Force Terminal' ]]
    [[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleDisplayName' "$plist")" == 'Force Terminal' ]]
    [[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$plist")" == "$version" ]]
    [[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$plist")" == 'Force Terminal' ]]
    node - "$resources/app-update.yml" <<'JS'
const fs = require('node:fs');
const yaml = require('yaml');
const config = yaml.parse(fs.readFileSync(process.argv[2], 'utf8'));
if (config.provider !== 'github' || config.owner !== 'calneymgp' || config.repo !== 'force-terminal' || config.updaterCacheDirName !== 'force-terminal-updater') {
    throw new Error('Packaged update provider or cache does not match Force Terminal');
}
JS
    if [[ "$release_mode" != adhoc ]]; then
        [[ ! -e "$resources/force-adhoc.json" ]]
    else
        node - "$resources/force-adhoc.json" <<'JS'
const fs = require('node:fs');
const marker = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
if (marker.kind !== 'force-terminal-adhoc' || marker.officialUpdates !== false) throw new Error('Ad hoc marker missing or invalid');
JS
    fi
}

tmp="$(mktemp -d)"
mounted=false
cleanup() {
    if [[ "$mounted" == true ]]; then hdiutil detach "$tmp/dmg" -quiet || true; fi
    rm -rf "$tmp"
}
trap cleanup EXIT
ditto -x -k "make/force-terminal-darwin-arm64-$version.zip" "$tmp/zip"
mkdir -p "$tmp/dmg"
hdiutil attach -readonly -nobrowse -quiet -mountpoint "$tmp/dmg" "make/force-terminal-darwin-arm64-$version.dmg"
mounted=true
for packaged_app in "$tmp/zip/Force Terminal.app" "$tmp/dmg/Force Terminal.app"; do
    [[ -d "$packaged_app" ]]
    [[ "$(lipo -archs "$packaged_app/Contents/MacOS/Force Terminal")" == arm64 ]]
done
for packaged_app in "$app" "$tmp/zip/Force Terminal.app" "$tmp/dmg/Force Terminal.app"; do
    check_bundle "$packaged_app"
    codesign --verify --deep --strict "$packaged_app"
done

if [[ "$release_mode" != adhoc ]]; then
    for packaged_app in "$app" "$tmp/zip/Force Terminal.app" "$tmp/dmg/Force Terminal.app"; do
        if [[ "$release_mode" == official ]]; then
            sign_details="$(codesign -dv --verbose=4 "$packaged_app" 2>&1)"
            [[ "$sign_details" == *"Authority=Developer ID Application:"* ]]
        else
            cert_dir="$(mktemp -d "$tmp/cert.XXXXXX")"
            (cd "$cert_dir" && codesign -d --extract-certificates "$packaged_app" >/dev/null 2>&1)
            [[ -f "$cert_dir/codesign0" ]] || { echo "Signed certificate missing" >&2; exit 1; }
            actual_fingerprint="$(openssl x509 -inform DER -in "$cert_dir/codesign0" -noout -fingerprint -sha256 | sed 's/.*=//' | tr -d ':[:space:]' | tr '[:lower:]' '[:upper:]')"
            expected_fingerprint="$(printf '%s' "$FORCE_MACOS_CERT_SHA256" | tr '[:lower:]' '[:upper:]')"
            [[ "$actual_fingerprint" == "$expected_fingerprint" ]] || { echo "Community signing certificate fingerprint mismatch" >&2; exit 1; }
        fi
        codesign -d --entitlements :- "$packaged_app" > "$tmp/entitlements.plist" 2>/dev/null
        [[ "$(/usr/libexec/PlistBuddy -c 'Print :com.apple.security.cs.allow-jit' "$tmp/entitlements.plist")" == true ]]
        if [[ "$release_mode" == official ]]; then
            spctl --assess --type execute "$packaged_app"
            xcrun stapler validate "$packaged_app"
        fi
    done
fi

node scripts/force-release.mjs create --version "$version" "${1:+--tag}" "${1:-}"
node scripts/force-release.mjs verify --version "$version" "${1:+--tag}" "${1:-}"
