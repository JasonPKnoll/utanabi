#!/bin/sh
# Installs Utanabi for Mac from its latest GitHub release, or the version in UTANABI_VERSION:
#
#   curl -fsSL https://raw.githubusercontent.com/JasonPKnoll/utanabi/main/scripts/install.sh | sh
#
# It downloads the release's zip, checks it against the release's SHA-256, and puts Utanabi.app in /Applications, or
# ~/Applications when /Applications isn't writable, or UTANABI_INSTALL_DIR. Running it again updates the app; your
# songs stay, in ~/Library/Application Support/Utanabi. UTANABI_DOWNLOAD_BASE downloads from another place than the
# release, such as a folder served while testing.
set -eu

repo=JasonPKnoll/utanabi

if [ "$(uname -s)" != Darwin ]; then
  echo "install: this installs the Mac app. On other systems, use Utanabi in the browser." >&2
  exit 1
fi

if [ -n "${UTANABI_VERSION:-}" ]; then
  tag="v${UTANABI_VERSION#v}"
else
  # The latest release's page redirects to its tag's, which names the version.
  tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest" | sed 's|.*/tag/||')
fi
case "$tag" in
  v[0-9]*) ;;
  *)
    echo "install: couldn't find Utanabi's latest release" >&2
    exit 1
    ;;
esac
version=${tag#v}
zip="Utanabi-$version-macos.zip"
base=${UTANABI_DOWNLOAD_BASE:-"https://github.com/$repo/releases/download/$tag"}

dest=${UTANABI_INSTALL_DIR:-/Applications}
if [ -z "${UTANABI_INSTALL_DIR:-}" ] && [ ! -w "$dest" ]; then
  dest="$HOME/Applications"
fi
mkdir -p "$dest"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading Utanabi $version…"
curl -fSL --progress-bar -o "$tmp/$zip" "$base/$zip"
want=$(curl -fsSL "$base/$zip.sha256" | awk '{print $1}')
got=$(shasum -a 256 "$tmp/$zip" | awk '{print $1}')
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "install: the download doesn't match its checksum, so it wasn't installed" >&2
  exit 1
fi

ditto -x -k "$tmp/$zip" "$tmp/unpacked"
if [ ! -d "$tmp/unpacked/Utanabi.app" ]; then
  echo "install: the download has no Utanabi.app in it" >&2
  exit 1
fi
rm -rf "$dest/Utanabi.app"
mv "$tmp/unpacked/Utanabi.app" "$dest/Utanabi.app"

echo "Installed Utanabi $version in $dest."
echo "Open it from there, or run: open \"$dest/Utanabi.app\""
