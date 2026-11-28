# Testing

## Commands

| Command | Runs |
| --- | --- |
| `make test` | Go unit tests with `-race`, then the frontend unit tests (Vitest). |
| `make test-integration` | Go tests with the `integration` build tag and `-race`. |
| `make test-e2e` | Builds `bin/utanabi-server` and runs the Playwright smoke test in headless Chromium. |
| `make lint` | golangci-lint for the desktop and server builds, ESLint, Prettier, and svelte-check. |

The first run of `make test-e2e` needs a browser: `cd frontend && npx playwright install chromium`.

## Unit tests

- Next to the code, in `_test.go` files.
- A use case is tested with a fake of its port. Mock at the port. If a test has to mock a struct, the abstraction is in the wrong place.
- Tests that need unexported names live in the same package (for example `internal/app`). The rest use the `_test` package.

## Integration tests

- Files start with `//go:build integration`.
- The database is real SQLite. It runs in process, so there is no container. `storetest.New(t)` opens a fresh database file in the test's temp folder and closes it when the test ends.
- Every query function in a `sqlite.go` file runs in at least one integration test. An untested query counts as broken.
- `storetest.New(t)` returns an empty database. A slice's test applies the real schema with `db.Migrate(t.Context(), store.Migrations())`, then inserts its own rows. Tests in `internal/store` create their own tables or pass their own migration files. No shared seed data, so tests run in parallel and in any order.
- Stub only what can't run for real: a remote service (behind an `httptest` server), a failure the real dependency can't produce on demand, or a clock.

A slice's `sqlite.go` test looks like this:

```go
//go:build integration

func TestThingByID(t *testing.T) {
	t.Parallel()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// insert a row, call the adapter, check the result
}
```

## Frontend tests

- Vitest runs `frontend/src/**/*.test.ts`.
- Keep logic in plain TypeScript modules under `frontend/src/lib` so it can be tested without a webview.
- `sweep.real.test.ts` scores the lyrics sweep against real singing: 38 Japanese songs timed character by character by hand, from the AMLL TTML DB (CC0) at a pinned commit. The sweep is given only each line's start, as line-synced lyrics give it, and the test bounds how far its character starts land from the real ones, how often a line's sweep ends over a second off the singing, and checks it beats an even split to the next line. The data holds only times and beat counts, never lyrics. Rebuild it with `cd frontend && node --experimental-strip-types scripts/real-timings.ts`. When you change the sweep's settings, score them here, and tune on songs outside this set so the test stays a fair check.

## Smoke test

`frontend/e2e/smoke.spec.ts` runs against the server-mode build. Playwright starts `bin/utanabi-server` on port 18080 (override with `SMOKE_PORT`), with its data in `.task/smoke-data`. It checks that `/health` returns 200, that the UI loads without page errors, and that a bound method answers. CI runs it on every pull request. No display server is needed.

## Writing tests

- Table-driven when cases share their setup and checks, with `got` and `want` in the failure message.
- Separate tests when the setup or the checks differ.
- No doc comments on tests. Name each case for its scenario.
- Helpers call `t.Helper()`.
- `t.Parallel()` wherever it's safe. Mark an exception with the reason in a `//nolint:paralleltest // <reason>` comment.
- Don't sleep to synchronize. Wait on a channel, or poll a condition with a deadline.
- Use `t.Context()` and `t.TempDir()`, not `context.Background()` or `os.MkdirTemp`.
