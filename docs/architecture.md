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

`main.go` builds everything in this order: platform folders, config, logger, database, services, then `app.New` and `Run`. When the app quits, Wails calls `ServiceShutdown` on each service in reverse order. The shutdown hook from `app.CloseOnShutdown` closes the database. `main` closes the log file last.

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

Slices get a `store.Querier` (satisfied by both the database and a transaction) and call `store.Translate` on every error. There is no migration tool yet. The first slice that needs a table adds one.

## Why these choices

- **Pure Go SQLite:** one driver for desktop, server, and mobile cross-compiles.
- **`app.Fail` in every service method:** Wails copies `err.Error()` into the message it sends to the frontend. Mapping the error before Wails sees it is the only way to keep internal detail out.
- **wails3 pinned as a Go tool:** `go install tool` installs exactly the CLI version that matches the `github.com/wailsapp/wails/v3` module in `go.mod`.
