#!/bin/sh
# Writes the Homebrew cask for a published release into a checkout of the tap, JasonPKnoll/homebrew-utanabi, reading
# the zip's SHA-256 from the release. Commit and push the tap after.
#
#   scripts/update-cask.sh 0.1.0 ../homebrew-utanabi
set -eu

if [ $# -ne 2 ]; then
  echo "usage: scripts/update-cask.sh <version> <tap folder>" >&2
  exit 2
fi
version=${1#v}
tap=$2
repo=JasonPKnoll/utanabi
sha=$(curl -fsSL "https://github.com/$repo/releases/download/v$version/Utanabi-$version-macos.zip.sha256" | awk '{print $1}')
case "$sha" in
  [0-9a-f]*) ;;
  *)
    echo "update-cask: no published checksum for v$version" >&2
    exit 1
    ;;
esac

mkdir -p "$tap/Casks"
cat >"$tap/Casks/utanabi.rb" <<EOF
cask "utanabi" do
  version "$version"
  sha256 "$sha"

  url "https://github.com/$repo/releases/download/v#{version}/Utanabi-#{version}-macos.zip"
  name "Utanabi"
  desc "Learn Japanese through song lyrics"
  homepage "https://github.com/$repo"

  depends_on macos: ">= :monterey"

  app "Utanabi.app"

  zap trash: [
    "~/Library/Application Support/Utanabi",
    "~/Library/Logs/Utanabi",
  ]

  caveats <<~TEXT
    Utanabi isn't notarized by Apple yet. If macOS won't open it the first time, open
    System Settings, then Privacy & Security, and choose Open Anyway for Utanabi.
  TEXT
end
EOF
echo "update-cask: wrote $tap/Casks/utanabi.rb for $version"
