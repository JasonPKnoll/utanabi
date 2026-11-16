# Utanabi

歌学び (うたなび): learn Japanese through song lyrics.

A Wails v3 app with a Go backend and a Svelte frontend. It builds for desktop (Windows, macOS, Linux), the web (Wails server mode), and iOS and Android (experimental).

## Quick start

Install Go (the version in `go.mod`), Node 26.13.0, and on Linux `libgtk-4-dev` and `libwebkitgtk-6.0-dev`. Then:

```bash
make tools
```

```bash
make dev
```

## Common commands

| Command | Does |
| --- | --- |
| `make dev` | Runs the desktop app with live reload. |
| `make test` | Go and frontend unit tests. |
| `make test-integration` | Go integration tests against a real SQLite file. |
| `make lint` | All Go and frontend linters and the type check. |
| `make test-e2e` | Server-mode build plus a headless browser smoke test. |
| `wails3 task build` | Desktop build for this OS, into `bin/`. |
| `wails3 task build:server` | Server-mode build, into `bin/utanabi-server`. |
| `wails3 task run:ios` | Runs in the iOS Simulator (needs Xcode). |
| `wails3 task run:android` | Runs in the Android emulator (needs the Android SDK and NDK). |

## Contributing

Read [AGENTS.md](AGENTS.md) first. It covers the layout, the slice template, and the rules every change follows.
