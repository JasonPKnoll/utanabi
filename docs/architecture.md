# Architecture

## The big picture

```
frontend/src (Svelte)
   │  calls generated functions in frontend/bindings
   ▼
Wails bridge (IPC on desktop and mobile, HTTP and WebSocket in server mode)
   │
   ▼
service.go (primary adapter) ──► app.Fail ──► error the frontend sees
   │
   ▼
<slice>.go (use case and ports)
   │
   ▼
sqlite.go / <source>_api.go (secondary adapters) ──► internal/store ──► SQLite file
```

`main.go` builds everything in this order: platform folders, config, logger, database (opened, then migrated), services, then `app.New` and `Run`. When the app quits, Wails calls `ServiceShutdown` on each service in reverse order. The shutdown hook from `app.CloseOnShutdown` closes the database. `main` closes the log file last.

## Slices

A feature is one folder under `internal/features/<slice>/`. The use case in `<slice>.go` holds the logic and declares the ports it needs. Adapters implement those ports. Imports point inward only:

- `service.go` imports the use case, `internal/app`, and Wails.
- `sqlite.go` imports the use case and `internal/store`.
- `<slice>.go` imports neither. It may import `internal/common/...` and the standard library.

depguard in `.golangci.yml` enforces these rules. A slice never imports another slice. Code two slices need moves to `internal/common/<pkg>`.

## Services and the frontend

Wails binds every exported method of every service passed to `app.New`. Wails then generates TypeScript for those methods into `frontend/bindings`. The frontend imports the generated functions and never builds a URL of its own.

A method may take `context.Context` as its first parameter. Wails fills it in. In server mode the context carries the HTTP request's values, which is where auth middleware will put the caller's identity.

`internal/app` holds services that belong to the app rather than to a feature, such as `AboutService`. Feature services live in their slice.

## Build targets

Each target picks its own files through build tags. There is exactly one `Platform()` per target.

| Target | Tags | Platform file | Folders | Logs |
| --- | --- | --- | --- | --- |
| Desktop | none | `platform_desktop.go` | OS config and app-data folders | file in the OS log folder |
| Server mode | `server` | `platform_server.go` | `UTANABI_DATA_DIR`, `UTANABI_CONFIG_DIR` | stderr |
| iOS | `ios` | `platform_ios.go` | `application.IOS.StoragePath()` | stderr |
| Android | `android` | `platform_android.go` | `application.Android.StoragePath()` | stderr |

Desktop folders per OS:

| OS | Settings and database | Logs |
| --- | --- | --- |
| macOS | `~/Library/Application Support/Utanabi` | `~/Library/Logs/Utanabi` |
| Windows | `%APPDATA%\Utanabi` | `%LOCALAPPDATA%\Utanabi\logs` |
| Linux | `$XDG_CONFIG_HOME/utanabi`, `$XDG_DATA_HOME/utanabi` | `$XDG_STATE_HOME/utanabi/logs` |

The iOS and Android builds also use `build/ios/main_ios.go` and `build/android/main_android.go`. Wails's mobile Taskfiles overlay these into the root package at build time.

## Data

`internal/store` opens one SQLite database with foreign keys on, WAL journaling, a busy timeout, and immediate write transactions. The driver is `modernc.org/sqlite`, which is pure Go, so the same code builds for every target without a C toolchain for SQLite.

Slices get a `store.Querier` (satisfied by both the database and a transaction) and call `store.Translate` on every error.

### Migrations

The schema is a series of numbered SQL files in `internal/store/migrations`, embedded in the binary with `embed`. Each name is `<version>_<change>.sql`, and the versions run 1, 2, 3 with no gaps or repeats.

`main.go` calls `db.Migrate(ctx, store.Migrations())` right after `store.Open`. `Migrate` reads `PRAGMA user_version`, the number of the last file applied, and runs each later file in order, each in its own transaction. The same transaction sets `user_version` to the file's version, so a file either applies in full and is recorded, or leaves no trace. A failed file stops startup with an error that names it, such as `store: migration failed (002_add_songs.sql); store: query failed (<driver detail>)`, and the database stays at the last good version. A database already at the latest version applies nothing. A database newer than the app (after a downgrade) fails startup instead of running against a schema the app doesn't know.

`Migrate` takes an `fs.FS`, so tests pass their own files with `fstest.MapFS`.

To change the schema, add a file numbered one past the highest. Never edit a file that has been merged: a user's database may already have run it. A migration runs inside a transaction, so it can't change `PRAGMA foreign_keys` or `journal_mode`.

There are no down migrations. Undo a change with a new file.

## Japanese text analysis

`internal/common/morph` splits a line into tokens with [kagome](https://github.com/ikawaha/kagome) (`github.com/ikawaha/kagome/v2`), a pure Go morphological analyzer, and its IPA dictionary (`github.com/ikawaha/kagome-dict/ipa`). Each token has its surface, a hiragana reading, a coarse part of speech, and the IPA fields a later step needs (base form, conjugation type and form, pronunciation).

The dictionary is the expensive part:

- **Binary size:** it is embedded, and adds about 12 MB (the server build went from 23.8 MB to 36.4 MB).
- **Memory:** loading it takes about 145 MB, kept for the life of the app.

So `morph.New` loads nothing. The dictionary loads on the first line analysed, once, safely for concurrent callers, and never at startup. Opening a song whose lines all have tokens doesn't load it.

The songs slice reaches it through its own `Analyzer` port. `main.go` wires `songs.NewMorph(morph.New())` into the service. A song's lines are analysed when it is saved, in the same transaction, and a song saved before analysis existed is analysed the first time it's opened. Furigana and romaji are computed from the stored tokens in `internal/common/jtext` (`Ruby`, `RomajiLine`), never stored.

UniDic was tried and is far too heavy for a phone. The IPA dictionary dates from 2007, so it misreads some modern words and song readings. Readings are stored per token so a later issue can let you correct them.

## Why these choices

- **Pure Go SQLite:** one driver for desktop, server, and mobile cross-compiles.
- **kagome with the IPA dictionary:** pure Go, so it cross-compiles like the rest, and works offline with nothing to download. Loading it lazily keeps its memory off devices that don't need it yet.
- **`app.Fail` in every service method:** Wails copies `err.Error()` into the message it sends to the frontend. Mapping the error before Wails sees it is the only way to keep internal detail out.
- **wails3 pinned as a Go tool:** `go install tool` installs exactly the CLI version that matches the `github.com/wailsapp/wails/v3` module in `go.mod`.
