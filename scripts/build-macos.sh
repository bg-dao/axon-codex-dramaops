#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

if [[ "$(uname -s)" != "Darwin" || "$(uname -m)" != "arm64" ]]; then
  echo "DramaOps v0.2 macOS release builds require an Apple Silicon Mac." >&2
  exit 1
fi

go test -race ./...
npm --prefix frontend ci
npm --prefix frontend test
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -clean -platform darwin/arm64

app_path="$repo_root/build/bin/DramaOps.app"
if [[ -n "${APPLE_SIGN_IDENTITY:-}" ]]; then
  codesign --force --deep --options runtime --timestamp --sign "$APPLE_SIGN_IDENTITY" "$app_path"
else
  echo "APPLE_SIGN_IDENTITY is unset; built artifact is a local pre-release only."
fi

if [[ -n "${APPLE_NOTARY_PROFILE:-}" && -n "${APPLE_SIGN_IDENTITY:-}" ]]; then
  archive_path="$repo_root/build/bin/DramaOps-notarization.zip"
  ditto -c -k --keepParent "$app_path" "$archive_path"
  xcrun notarytool submit "$archive_path" --keychain-profile "$APPLE_NOTARY_PROFILE" --wait
  xcrun stapler staple "$app_path"
else
  echo "Notarization credentials are unset; no formal public DMG will be produced."
fi
