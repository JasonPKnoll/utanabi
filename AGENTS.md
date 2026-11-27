# AGENTS.md

The one guide for anyone changing this code, person or agent. Read it before your first change.

## What this is

Utanabi is an app for learning Japanese through song lyrics. It is a Go app built with Wails v3, with a Svelte and TypeScript frontend. One codebase builds a desktop app (Windows, macOS, Linux), a web app (Wails server mode), and iOS and Android apps (Wails mobile, experimental). The Go side keeps its data in a local SQLite database. The frontend talks only to Go services bound through Wails. There is no backend API yet.

## Code layout

| Path | Job |
| --- | --- |
| `main.go` | Wiring only: platform folders, config, logger, database and its migrations, services, `app.New`, `Run`. |
| `internal/config` | The typed `Config`: build-time values, the user settings file, and server-mode env vars, validated at startup. |
| `internal/logger` | The JSON slog logger, `From` and `Into` for the context, and the `Err` and `Panic` attributes. |
| `internal/app` | The Wails wiring (options, window, platform folders per build target), `app.Fail` (the one frontend error mapping), `AboutService` (name, version, platform, and the data credits), and the shutdown hook. |
| `internal/common/fault` | The project's error sentinels (`ErrNotFound`, `ErrConflict`, `ErrCanceled`, `ErrTimeout`) and `InvalidError`. |
| `internal/common/redact` | The `Secret` type, which never prints its value. |
| `internal/common/jtext` | Japanese text normalization (NFC, full-width katakana), the hiragana check for readings, katakana to hiragana, furigana alignment (`Ruby`), and modified Hepburn romaji (`Romaji`, `RomajiLine`). |
| `internal/common/morph` | The morphological analyzer: kagome with the IPA dictionary, loaded on first use. Splits text into tokens with a hiragana reading, a coarse part of speech, and the raw IPA fields. `Attribution` is the IPA dictionary's credit. |
| `internal/dictionary` | Reads `dictionary.db`, the read-only reference database: `Open`, `Lookup` (JMdict words with their spellings and senses), `Entries` (by ID), `Tags` (codes in words), `GrammarPoint`, `GrammarPoints`, and `Attribution`. All its SQL is in `sqlite.go`. `grammar.json` is the hand-written grammar list, also built into the app (`Grammar`), so grammar points work without `dictionary.db`. |
| `internal/dictionary/dictbuild` | Builds `dictionary.db` from the pinned jmdict-simplified release and `grammar.json`: download with a checked SHA-256, read, and write. Build time only, never in the app. |
| `internal/dictionary/cmd/dictbuild` | The command `task dictionary` runs. |
| `internal/features` | Feature slices, one folder each. |
| `internal/features/songs` | The songs the user is learning: search online lyrics sources (LRCLIB in `lrclib_api.go`) and add a match, or add a song with your own lyrics; list, read, and delete. Synced (LRC) lyrics keep each line's start time. Each line is analysed into tokens when saved (through the `Analyzer` port, `morph.go` wraps `internal/common/morph`), and its furigana and romaji are computed from them. The tokens are grouped into words (`words.go`, with particle uses in `particles.go` and spoken forms such as ねえ for ない in `spoken.go`), whose meanings come through the `Dictionary` port (`dictionary.go` wraps `internal/dictionary`; `meanings.go` picks the entry and sense). `explain.go` builds what the song view shows: each word with its meaning, grammar, and conjugation steps, the word-by-word gloss, and `LookupWord` for the word sheet. |
| `internal/features/study` | Flashcards: a song's deck of word cards (one card per word meaning, shared by every song it's in) and your progress on each. A right answer grows a card's streak, a wrong one ends it, and a card is learned at `study_streak` right in a row (3 by default). |
| `internal/identity` | Who a call is for: the signed-in `User` in the request context (`Into`, `From`), and `Owners`, which every repository asks for the owner of its data (`Accounts()` in server mode, `OnDevice()` on desktop and mobile, where the owner is the device, 0). |
| `internal/features/accounts` | Sign-in for server mode with passkeys (WebAuthn, through go-webauthn): sign-up with an invite or the server's code, sign-in, adding a passkey, and sign-out, as HTTP endpoints under `/auth/` (`http.go`), plus the middleware that requires a session for every bound call and puts the user in the context. `service.go` is the admins' bound service for invites. Sessions are HttpOnly cookies; the database keeps only token hashes, invite hashes, and public keys. |
| `cmd/admin` | The admin command, run on the server's machine: `go run ./cmd/admin -list`, `-grant <name>`, `-revoke <name>`. The only way to make someone an admin. |
| `internal/store` | The SQLite database: open, `Migrate`, `WithTx`, `Querier`, and `Translate` for driver errors. |
| `internal/store/migrations` | The schema: numbered `.sql` files, embedded in the binary and applied at startup. |
| `internal/store/storetest` | A fresh database file per test, for integration tests. |
| `frontend/src` | The Svelte UI. `App.svelte` picks the screen from the URL hash (`#/`, `#/add`, `#/song/<id>`). |
| `frontend/src/screens` | One Svelte component per screen. |
| `frontend/src/components` | Svelte components shared by screens, such as `WordSheet`, the bottom sheet for a tapped word, and `Firework`, the animated mark that `SongBuilding` shows while a song is built. |
| `frontend/src/lib` | Shared frontend code, such as `toAppError`, the route parser, and lyrics helpers. |
| `frontend/bindings` | Generated by `wails3 generate bindings`. Never edit by hand. |
| `frontend/e2e` | The Playwright smoke test that runs against the server-mode build. |
| `build/` | Per-platform build config and Taskfiles from `wails3 init`. |
| `docs/` | Longer detail: [architecture](docs/architecture.md), [errors](docs/errors.md), [logging](docs/logging.md), [testing](docs/testing.md), [security](docs/security.md), [design](docs/design.md), [data model](docs/data-model.md), [analyzer workarounds](docs/workarounds.md). |

Every new package adds a row here.

## Slice template

Each feature is a **slice**: one folder under `internal/features/<slice>/` that owns its area end to end.

```
internal/features/<slice>/
  <slice>.go        # the use case, the ports (interfaces) it needs, and its request and response types
  service.go        # primary adapter: the Wails service whose exported methods the frontend calls
  sqlite.go         # secondary adapter: implements the repository port; the only file that talks to the database
  http.go           # primary adapter, only for a slice that must answer HTTP itself (accounts, for its cookies)
  <source>_api.go   # secondary adapter: a client for a remote API, when the slice needs one
```

- **Dependencies point inward.** The use case file imports no Wails, HTTP client, or database package. Adapters import the use case, never the reverse.
- **Ports live with their consumer.** The use case declares the interfaces it needs, and the adapters implement them.
- **Function first, struct when needed.** A use case with one operation is a plain function that takes its ports as arguments. Use a struct only for state or dependencies shared by several operations. Make the change when the second operation arrives, not before. A Wails service is always a struct, because Wails binds its methods.
- **No slice imports another slice.** Shared code moves to `internal/common/<pkg>`.
- **Cross-cutting concerns get their own package** (auth, identity, feature flags). They reach slices through context or explicit arguments, never through globals.
- **Feature code lives in slices.** No top-level `handlers/`, `services/`, `models/`, or `utils/` folders.

### Adding a slice, step by step

1. Create `internal/features/<slice>/<slice>.go` with the request and response types, the port interface, and the use case function. Validate every input here and return `fault.Invalid(field, reason)` for bad input.
2. Add the slice's tables in a new migration file, `internal/store/migrations/<NNN>_<change>.sql`, numbered one past the highest file there. Never edit a migration that has been merged; change the schema with a new file.
3. Add `sqlite.go` with a struct that implements the port. Take a `store.Querier`, write plain SQL, and pass every error from `database/sql` through `store.Translate`.
4. Add `service.go` with the Wails service struct. Each exported method takes `context.Context` first, calls the use case, and returns `result, app.Fail(ctx, err)`.
5. Register the service in `main.go`: `application.NewService(<slice>.NewService(...))`.
6. Run `task bindings`, then call the generated function from `frontend/src`. Catch errors with `toAppError`.
7. Add a unit test for the use case (mock the port) and an integration test for `sqlite.go`. The integration test applies the real schema with `db.Migrate(t.Context(), store.Migrations())` after `storetest.New(t)`.
8. Add a row to the code layout table above.

A skeleton, with generic names:

```go
// <slice>.go
type Thing struct{ ID int64; Name string }

type ThingRepository interface {
	ThingByID(ctx context.Context, id int64) (Thing, error)
}

func GetThing(ctx context.Context, repo ThingRepository, id int64) (Thing, error) {
	if id <= 0 {
		return Thing{}, fault.Invalid("id", "must be positive")
	}
	return repo.ThingByID(ctx, id)
}

// service.go
type Service struct{ repo ThingRepository }

func NewService(repo ThingRepository) *Service { return &Service{repo: repo} }

func (s *Service) Get(ctx context.Context, id int64) (Thing, error) {
	thing, err := GetThing(ctx, s.repo, id)
	return thing, app.Fail(ctx, err)
}
```

## Project conventions

### The frontend boundary

- **Every exported method on a bound service is callable by the frontend.** Bind only what the UI needs. Treat every argument as untrusted input and validate it in the use case.
- **Service methods hand domain errors unchanged to `app.Fail`.** Every bound method ends with `return result, app.Fail(ctx, err)`. `app.Fail` in `internal/app` is the one function that turns a domain error into the error the frontend sees: a stable code and a safe message. It logs each failure once. Wails sends `err.Error()` to the frontend as the message, so a raw error must never leave a service method. A backstop in `MarshalError` masks any error that skipped `app.Fail` and logs it as a bug.
- **Identity never comes from a method's arguments.** In server mode the accounts middleware signs requests in with a passkey session and puts the user in the request context; `internal/identity` reads it. On desktop and mobile there are no accounts, and everything is the device's. Detail in [docs/security.md](docs/security.md#identity).
- **Every repository query is scoped to its owner.** A slice's `sqlite.go` takes `identity.Owners` and asks `owners.Of(ctx)` for the owner of every read and write: someone else's song or card is not found. In server mode a call with no signed-in user fails as `unauthorized`.
- **Bindings are generated** with `task bindings` (`wails3 generate bindings -clean=true -ts -i`) into `frontend/bindings`, and committed. CI regenerates them and fails on any diff.
- **Events are broadcasts.** In server mode an event from Go reaches every connected browser. Never put one user's data in an event.

### Data

- The local database is SQLite, through the pure Go driver `modernc.org/sqlite`. The file lives in the OS app-data folder (on mobile, the app's sandbox storage path; in server mode, `UTANABI_DATA_DIR`).
- Database code stays in the slices' `sqlite.go` files, and for the reference dictionary in `internal/dictionary/sqlite.go` and `internal/dictionary/dictbuild/sqlite.go`. Use plain SQL, scanned into typed structs. No ORM.
- **The schema lives in migrations:** numbered files in `internal/store/migrations` (`001_init.sql`, `002_<change>.sql`, and so on, no gaps). `main.go` runs `db.Migrate` right after `store.Open`. Each pending file runs in its own transaction, and `PRAGMA user_version` records the last one applied. A failed file stops startup with an error that names it. Never edit a merged migration; add a new one.
- **The data model is in [docs/data-model.md](docs/data-model.md):** lines, words, tokens, meanings, grammar, conjugation, and cards, and what is stored versus computed. Read it before adding a table or column. Every new rule gets a case in `internal/store/schema_integration_test.go` proving the database refuses a bad row.
- **Every correction for kagome or JMdict is listed in [docs/workarounds.md](docs/workarounds.md),** with its example and the test that pins it down, and so is every case still read wrong. A fix is a general rule for a pattern, never a special case for one song or word. Add its row and its test case with it.
- **Normalize Japanese text with `jtext.Normalize` before saving it,** so the same word is always the same bytes. Readings are hiragana only (`jtext.IsHiragana`).
- **Translate every driver error at the adapter** with `store.Translate`, into the project's own sentinels. Nothing above the adapter sees a driver error.
- `db.WithTx(ctx, fn)` runs `fn` in one transaction, and rolls back on an error or a panic.
- Every connection gets a busy timeout from config (`db_busy_timeout`, default 5s). Pass a context with a deadline for work that must not run long.
- **Never ship database credentials, API keys, or other secrets in the app.** A user can read anything inside it.

### The reference dictionary

- **`dictionary.db` is a second, read-only SQLite file:** JMdict word meanings and our grammar list. `utanabi.db` refers to it only by IDs that never change: the JMdict entry number and the grammar point ID. It has no migrations; it is rebuilt whole, and the app refuses a file whose `schema_version` in `meta` differs from `dictionary.SchemaVersion`. Bump that constant with every change to `internal/dictionary/dictbuild/schema.sql`.
- **Rebuild it with `task dictionary`** (or `make dictionary`), which writes `data/dictionary.db`, the server-mode default. `task dictionary OUT=<path>` writes elsewhere; for desktop dev, point it at the data folder (on macOS, `~/Library/Application Support/Utanabi/dictionary.db`) or set `dictionary_path`. Without the file the app runs, with word meanings unavailable, and logs one info line.
- **To move to a newer JMdict,** change `JMdictRelease`, `JMdictURL`, and `JMdictSHA256` together in `internal/dictionary/dictbuild/source.go`, then rebuild. Never drop the checksum.
- **Grammar point IDs in `internal/dictionary/grammar.json` are never renamed or reused.** Add a row for a new use; `go test ./internal/dictionary/...` checks the list.
- **Credit JMdict wherever the app shows its data.** Its licence (CC BY-SA 4.0) requires it: show `dictionary.Attribution()` on an About or Sources screen on desktop and mobile, and on each page that shows dictionary words in server mode. Never claim copyright over JMdict material. Detail in [docs/data-model.md](docs/data-model.md#licence-and-credit).

### Logging

- Use `log/slog` with a JSON handler. The logger travels in the context: `logger.From(ctx)` and `logger.Into(ctx, l)`.
- No package-level loggers. `main` sets `slog.Default()` once.
- Use the `Context` methods (`InfoContext`, `ErrorContext`).
- **Errors are an attribute** (`logger.Err(err)`), never part of the message. sloglint forbids a raw `"error"` key outside the logger package.
- The call logger carries the call's attributes, and each slice adds its own name.
- **Never log a secret:** carry it as a `Secret`.

Detail in [docs/logging.md](docs/logging.md).

### Config

- Settings come from build-time values (`-ldflags "-X github.com/JasonPKnoll/utanabi/internal/config.version=..."`) and the user settings file `settings.json` in the OS config folder, parsed into one typed `Config` at startup.
- **The version has one source: `info.version` in `build/config.yml`.** The root `Taskfile.yml` reads it into `VERSION`, and every production build (desktop, server, iOS, Android) stamps it in through `VERSION_LDFLAG`. Dev builds and plain `go build` report `dev`. To bump it, edit `info.version` (keep the value in double quotes on its own two-space-indented line), then run `wails3 task common:update:build-assets` so the Info.plist, Windows manifest, and package files match (it overwrites hand edits to those files).
- Invalid config fails startup with an error that names the setting.
- **Online lyrics sources sit behind one switch, `lyrics_sources`:** a list in `settings.json` (`["lrclib"]`, or `[]` for none), `UTANABI_LYRICS_SOURCES` in server mode (`lrclib`, or `none`), and a build default (`-X github.com/JasonPKnoll/utanabi/internal/config.defaultLyricsSources=none` ships a build with them off). With none on, search refuses to run, the app hides it, and nothing is sent to any lyrics service; your own lyrics still work. A new source is added to `config.LyricsSources()` and to `lyricsSources` in `main.go`, and is off unless listed.
- The server-mode build also reads env vars (`WAILS_SERVER_HOST`, `WAILS_SERVER_PORT`, and the `UTANABI_*` ones), listed in `.env.example` with comments. Desktop and mobile builds ignore env vars.
- A new setting adds a field to `settings` and `Config`, a validation with a test, and a line in `.env.example` if server mode reads it.

### Secrets

- Credentials and other sensitive values use `redact.Secret`, which never prints its value: formatting, logging, and JSON all show `[REDACTED]`.
- Reading the value takes an explicit `Reveal()` call, used only where the value leaves the process.
- A user's session token, when there is one, lives in the OS keychain, never in a file or the binary.

### Platforms

- **Desktop:** CGO and each OS's webview (WebView2 on Windows, WKWebView on macOS, WebKitGTK on Linux).
- **Server mode:** `-tags server`. Desktop-only code (tray, menus, native dialogs, screens) goes in a file with `//go:build !server`, so the server build compiles. It binds to localhost by default. Use TLS and a reverse proxy in production. `WebSocketOriginPatterns` comes from `UTANABI_ALLOWED_ORIGINS`. Never set `WebSocketAllowAllOrigins`.
- **Mobile:** experimental in Wails. Mobile-only APIs (`application.IOS`, `application.Android`) go in files with `//go:build ios && !server` or `//go:build android && !server`. One window only. Write files only inside the app sandbox.
- Platform code lives in `internal/app/platform_<target>.go`. Each target has exactly one `Platform()`.

## Frontend

- **Framework:** Svelte 5 with TypeScript, built by Vite. Node 26.13.0 (see `frontend/.nvmrc`).
- **Look:** "Night stage", a stage where the sung line stands out and the rest stays dim. It comes in palettes (Utanabi by default, Tokyo Night, Nord, Catppuccin, Solarized, Vivid), each with a dark and a light set; the mode follows the device's setting, dark by default, unless Light or Dark is chosen under Theme (in the Display sheet or on About). Palettes live in `frontend/src/themes.css`, and a test checks each one's contrast. Use the colour tokens and the shared `.screen`, `.actions`, and `.button` classes in `frontend/src/app.css`, never new hex values in a component, so both themes keep working. Check UI changes in both themes. Detail in [docs/design.md](docs/design.md).
- **Bindings:** import services and types from `frontend/bindings/...`. They are generated from the Go services. Never edit them. Run `task bindings` after changing a bound method.
- **The UI calls only bound services, never a URL of its own.** ESLint forbids `fetch`, `XMLHttpRequest`, `WebSocket`, and `EventSource` in `frontend/src`. The one exception is `src/lib/auth.ts`, which calls the app's own `/auth/` endpoints, because a session must be a cookie only an HTTP response can set.
- **Errors:** pass every rejection through `toAppError` (`frontend/src/lib/errors.ts`). Show its `message` and switch on its `code`. Never show the raw `Error.message`.
- **Commands:** `npm run lint` (ESLint and Prettier), `npm run check` (svelte-check types), `npm test` (Vitest), `npm run test:e2e` (Playwright, after `task build:server`). `task lint` and `task test` run them for you.

### Mobile first

Most people will use Utanabi on a phone. Design and build every screen for a phone first, then add room for bigger screens.

- **Start at 360px wide.** Write the base styles for a phone, then widen the layout with `min-width` media queries. Never start from a desktop layout and squeeze it down.
- **One column on a phone,** with no sideways scrolling of the page at 360px. Wide content (a table, a long word list) scrolls inside its own box.
- **Touch targets are at least 44×44px,** with space between them so a thumb doesn't hit the wrong one.
- **Nothing depends on hover.** Every action works with a tap and with a keyboard. Use real `<button>`, `<a>`, `<input>`, and `<label>` elements.
- **Main actions sit within thumb reach,** near the bottom of the screen: the tab bar, player controls, and "Save word".
- **Text is readable on a small screen.** Body text and form fields are at least 16px (smaller fields make iOS zoom in). Lyrics are bigger, with enough line height for furigana in `<ruby>` above them.
- **Respect the safe areas** on phones with a notch or a home bar: pad edges with `env(safe-area-inset-*)`.
- **Check every UI change at 360px wide first,** then at tablet and desktop widths.

## Error handling

Detail and examples in [docs/errors.md](docs/errors.md).

- **Never ignore an error,** and never assign one to `_` only to silence the compiler.
- **Error strings are segments:** `<pkg>: <message>[ (<detail>)]`, joined by `;` when wrapped.
    - `<pkg>` is the package that creates the segment.
    - `<message>` is the project's own words, with no `:`, `;`, or parentheses.
    - `<detail>` holds text the project didn't write, such as a dependency's message.
    - `main.go` may use plain `action: %w` chains.
- **Wrap only to add a fact.** Drop a wrap that restates the error inside it.
- **`%w` only for the project's own errors.** A dependency's error goes through `%v`, a sentinel, or a typed error, so callers never match another library's types.
- **Match with `errors.Is` and `errors.As`,** never on strings.
- **Log or return, never both.** Only the boundaries log: the frontend error mapping (`app.Fail`), the panic handler, background job runners, and `main`.
- `errors.Join` is only for independent failures, never to attach a cause.
- **Concurrent work that can fail uses `errgroup`.** A bare `go func()` whose error disappears is a bug.
- **Panics are for programmer errors only.** The Wails panic handler logs one, and the call fails as an internal error.

## Go conventions

- Accept interfaces, return concrete types.
- No global mutable state, and no `init()` functions.
- `context.Context` is the first parameter wherever work can block, and it is passed on.
- Clean up with `defer`, right after the resource is acquired.
- Keep packages small and single-purpose. Split files by concern, not by line count.
- Line width is 120 for code and comments (set in `.editorconfig`). Don't pre-wrap to 80.
- No banner comments (`// --- section ---`).
- **Default to no comments.** Write one only for a non-obvious why: a hidden constraint, a subtle invariant, or a workaround. Use plain words and keep it brief; a few lines is fine when that reads better than one crammed line.
- Leave formatting to gofmt and goimports.

## Testing

Detail in [docs/testing.md](docs/testing.md).

- Unit tests sit next to the code (`_test.go`). Run them with `make test`.
- Integration tests use the `integration` build tag and the real database. Run them with `make test-integration`. SQLite runs in process, so there is no container: `storetest.New(t)` gives each test a fresh database file.
- **Every query function in a database file runs in at least one integration test.** An untested query counts as broken.
- **Test against the real database,** not a SQL mock or an in-memory repository. Stub only what can't run for real: a remote service (behind an `httptest` server), a failure the real dependency can't produce on demand, or a clock.
- **Mock at the port.** If a test has to mock a struct, the abstraction is in the wrong place.
- Each test creates its own fixtures. No shared seed data, so tests run in parallel and in any order.
- The server-mode smoke test runs with `make test-e2e`: it builds `bin/utanabi-server` and checks `/health` and the UI in headless Chromium.

## Writing tests

- Use a table-driven test when cases share their setup and checks: `tests := []struct{ name string; ... }`, run with `t.Run(tt.name, ...)`, with `got` and `want` in the failure message.
- Write separate tests when the setup or the checks differ, instead of a table whose rows switch the loop with flags.
- No doc comments on tests. Name each case for its scenario.
- Helpers call `t.Helper()`.
- Call `t.Parallel()` wherever it's safe. Mark an exception with the reason.
- **Don't sleep to synchronize:** wait on a channel, or poll a condition with a deadline.

## Hard rules

No PR may break these. The linter enforces the ones marked (lint).

1. **Dependencies point inward.** A use case imports no Wails, HTTP client, or database package. (lint: depguard `use-case-has-no-adapters`, `database-only-in-database-files`)
2. **No slice imports another slice.** (lint: depguard `no-slice-imports-another-slice`)
3. **Wails's `application` package only in `main.go`, `internal/app`, and slices' `service.go` files.** (lint: depguard `wails-only-at-the-edge`)
4. **No ORM.** Plain SQL in `sqlite.go` files.
5. **One error shape** for the frontend: `app.Error`, built only by `app.Fail`.
6. **Identity never comes from method arguments.**
7. **No secrets in the app.** No API keys, database credentials, or tokens in code, config, or the binary.
8. **Bindings are generated, never edited.** (CI: the bindings job fails on any diff)

## Local dev stack

There are no containers. Everything runs on your machine.

**Prerequisites:** Go (the version in `go.mod`; with `GOTOOLCHAIN=auto` Go downloads it for you), Node 26.13.0 with npm 11, and on Linux `libgtk-4-dev` and `libwebkitgtk-6.0-dev`.

```bash
make tools          # installs wails3 at the version pinned in go.mod (go install tool)
make dev            # wails3 dev: live-reloading desktop app
make test           # Go and frontend unit tests
make test-integration
make lint           # golangci-lint (desktop and server builds), ESLint, Prettier, svelte-check
make test-e2e       # server-mode build plus the headless browser smoke test
make dictionary     # download the pinned JMdict release and build data/dictionary.db
```

- **Desktop dev:** `make dev` runs `wails3 dev`. It rebuilds Go on change and runs Vite for the frontend. Settings live in `settings.json` in the OS config folder (on macOS, `~/Library/Application Support/Utanabi/`). Logs go to the OS log folder (on macOS, `~/Library/Logs/Utanabi/utanabi.log`).
- **Server mode:** `task run:server` builds and starts `bin/utanabi-server` on http://localhost:8080. Copy `.env.example` to `.env` and export what you need. `/health` returns 200 when it is up. Logs go to stderr.
- **iOS Simulator:** `task run:ios`. Needs macOS with Xcode. Run `task ios:install:deps` once.
- **Android emulator:** `task run:android`. Needs the Android SDK, NDK, and an emulator. Run `task android:install:deps` once.
- Run `wails3 task --list` to see every task.
