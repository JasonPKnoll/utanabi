#!/bin/sh
# Packages the Mac app for a release: a universal build (Apple silicon and Intel) with the reference dictionary inside
# it, signed, zipped, and with the zip's SHA-256 beside it, in bin/release. The Homebrew cask and install.sh install
# this zip from the GitHub release.
#
# It signs with MACOS_SIGN_IDENTITY, a Developer ID Application identity in the keychain, when that's set, and
# notarizes with the notarytool keychain profile MACOS_NOTARY_PROFILE when that's set too. Without them it signs ad
# hoc: the app runs, but macOS asks before opening it the first time when it's downloaded by a browser or Homebrew.
#
#   make dictionary          # once, for data/dictionary.db
#   scripts/package-macos.sh
set -eu

cd "$(dirname "$0")/.."

dictionary=${DICTIONARY:-data/dictionary.db}
identity=${MACOS_SIGN_IDENTITY:-}
profile=${MACOS_NOTARY_PROFILE:-}
version=$(sed -n 's/^  version: "\([^"]*\)".*/\1/p' build/config.yml | head -n 1)
out=bin/release
app="$out/Utanabi.app"
zip="$out/Utanabi-$version-macos.zip"

if [ -z "$version" ]; then
  echo "package-macos: can't read info.version from build/config.yml" >&2
  exit 1
fi
if [ ! -f "$dictionary" ]; then
  echo "package-macos: no dictionary at $dictionary; run make dictionary first" >&2
  exit 1
fi

wails3 task darwin:package:universal

rm -rf "$out"
mkdir -p "$out"
cp -R bin/utanabi.app "$app"
cp "$dictionary" "$app/Contents/Resources/dictionary.db"
xattr -cr "$app"

if [ -n "$identity" ]; then
  codesign --force --deep --options runtime --timestamp --sign "$identity" "$app"
else
  codesign --force --deep --sign - "$app"
fi
codesign --verify --deep --strict "$app"

ditto -c -k --keepParent "$app" "$zip"
if [ -n "$identity" ] && [ -n "$profile" ]; then
  xcrun notarytool submit "$zip" --keychain-profile "$profile" --wait
  xcrun stapler staple "$app"
  rm "$zip"
  ditto -c -k --keepParent "$app" "$zip"
fi

shasum -a 256 "$zip" | awk '{print $1}' >"$zip.sha256"
echo "package-macos: $zip ($(du -h "$zip" | awk '{print $1}')), sha256 $(cat "$zip.sha256")"
