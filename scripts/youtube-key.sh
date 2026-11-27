#!/usr/bin/env bash
# Makes a YouTube Data API key for this server and saves it, sealed, in the server's database, so songs can find
# their videos on YouTube. It signs you in to Google in your browser (the one step it can't do for you), then makes
# or reuses a Google Cloud project for Utanabi, switches on the YouTube Data API, and makes a key that works for that
# API only. The key goes straight from Google to the admin command, which checks it and seals it: it's never printed
# or written to a file. Run it again any time; it reuses the project and the key.
#
# Needs Google's gcloud tool: brew install --cask google-cloud-sdk (or cloud.google.com/sdk/docs/install).
# UTANABI_GCP_PROJECT picks the project; UTANABI_DATA_DIR the server's data folder (default ./data).
set -euo pipefail

key_name="Utanabi YouTube search"
data="${UTANABI_DATA_DIR:-data}"

if ! command -v gcloud >/dev/null 2>&1; then
  echo "youtube-key: Google's gcloud tool isn't installed. Install it with:" >&2
  echo "  brew install --cask google-cloud-sdk" >&2
  echo "then run this again." >&2
  exit 1
fi

if [ -z "$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null)" ]; then
  echo "Signing in to Google in your browser…"
  gcloud auth login --brief
fi

project="${UTANABI_GCP_PROJECT:-}"
if [ -z "$project" ]; then
  project="$(gcloud projects list --filter='labels.app=utanabi' --format='value(projectId)' --limit=1)"
fi
if [ -z "$project" ]; then
  project="utanabi-$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 6)"
  echo "Making the Google Cloud project $project…"
  if ! gcloud projects create "$project" --name="Utanabi" --labels=app=utanabi; then
    echo "youtube-key: Google wouldn't make the project. If it asked you to accept Google Cloud's terms, accept" >&2
    echo "them at https://console.cloud.google.com, then run this again." >&2
    exit 1
  fi
fi
echo "Using the Google Cloud project $project."

echo "Switching on the YouTube Data API…"
gcloud services enable youtube.googleapis.com apikeys.googleapis.com --project="$project"

find_key() {
  gcloud services api-keys list --project="$project" --filter="displayName='$key_name'" \
    --sort-by=~createTime --limit=1 --format='value(name)'
}
name="$(find_key)"
if [ -z "$name" ]; then
  echo "Making a key for the YouTube Data API only…"
  gcloud services api-keys create --project="$project" --display-name="$key_name" \
    --api-target=service=youtube.googleapis.com >/dev/null
  name="$(find_key)"
fi
if [ -z "$name" ]; then
  echo "youtube-key: Google made no key; try again in a minute." >&2
  exit 1
fi

echo "Saving it in the server's database, sealed…"
gcloud services api-keys get-key-string "$name" --format='value(keyString)' |
  go run ./cmd/admin -data "$data" -youtube-key
