# Errors

## Error strings

An error string is one or more segments: `<pkg>: <message>[ (<detail>)]`, joined by `; ` when one error wraps another.

- `<pkg>` is the package that creates the segment.
- `<message>` is the project's own words, with no `:`, `;`, or parentheses.
- `<detail>` holds text the project didn't write, such as a dependency's message.

```go
// A dependency's error goes in the detail, through %v.
return fmt.Errorf("store: query failed (%v)", err)

// The project's own error is wrapped with %w, after a new fact.
return fmt.Errorf("store: duplicate row (%v); %w", err, fault.ErrConflict)
// store: duplicate row (UNIQUE constraint failed: items.name); fault: conflict
```

`main.go` may use plain `action: %w` chains, such as `run app: %w`.

## Rules

- Never ignore an error. Never assign one to `_` only to silence the compiler.
- Wrap only to add a fact. A wrap that restates the inner error is noise.
- `%w` only for the project's own errors. errorlint's `%w` check is turned off for this reason.
- Match with `errors.Is` and `errors.As`, never on strings.
- Log or return, never both. Only the boundaries log: `app.Fail`, the Wails panic handler, background job runners, and `main`.
- `errors.Join` only for independent failures, such as closing several resources at shutdown.
- Concurrent work that can fail uses `errgroup`.
- Panics are for programmer errors only.

## The project's own errors

`internal/common/fault` holds the sentinels every layer can use:

| Error | Meaning |
| --- | --- |
| `fault.ErrNotFound` | The thing asked for does not exist. |
| `fault.ErrConflict` | The change clashes with existing data, such as a duplicate. |
| `fault.ErrCanceled` | The caller canceled. |
| `fault.ErrTimeout` | A deadline or a lock wait ran out. |
| `fault.Invalid(field, reason)` | Input failed validation. Returns `*fault.InvalidError`, which names the field. |

Adapters translate their dependency's errors into these. For the database, `store.Translate` does it:

| Driver error | Becomes |
| --- | --- |
| `sql.ErrNoRows` | `fault.ErrNotFound` |
| unique or primary key constraint | `fault.ErrConflict` |
| busy or locked | `fault.ErrTimeout` |
| `context.Canceled` | `fault.ErrCanceled` |
| `context.DeadlineExceeded` | `fault.ErrTimeout` |
| anything else | `store: query failed (<driver message>)`, not wrapped |

## The frontend error

Every bound service method returns its error through `app.Fail(ctx, err)`:

```go
func (s *Service) Get(ctx context.Context, id int64) (Thing, error) {
	thing, err := GetThing(ctx, s.repo, id)
	return thing, app.Fail(ctx, err)
}
```

`app.Fail` returns nil for nil. Otherwise it returns an `*app.Error` and logs the failure once, with the code and the original error:

| Domain error | `code` | Message | Log level |
| --- | --- | --- | --- |
| `*fault.InvalidError` | `invalid_input` | `<field> <reason>`, plus `field` | info |
| `fault.ErrNotFound` | `not_found` | Not found. | info |
| `fault.ErrConflict` | `conflict` | That already exists. | info |
| `fault.ErrCanceled`, `context.Canceled` | `canceled` | The request was canceled. | info |
| `fault.ErrTimeout`, `context.DeadlineExceeded` | `timeout` | That took too long. Please try again. | warn |
| anything else | `internal` | Something went wrong. Please try again. | error |

Passing an `*app.Error` to `app.Fail` again returns it unchanged and logs nothing.

### Why every method calls app.Fail

Wails builds the message it sends to the frontend from `err.Error()`. If a method returned a raw error, its text (file paths, SQL, a dependency's message) would reach the frontend. `app.Error.Error()` returns only the safe message.

As a backstop, the `MarshalError` hook in `internal/app` sends any error that is not an `*app.Error` as a plain `internal` error, and logs `error reached the frontend without app.Fail`. If you see that log line, a service method is missing its `app.Fail`.

## The shape the frontend sees

The rejected promise holds an `Error` whose `cause` is the JSON from `MarshalError`:

```json
{ "code": "invalid_input", "message": "title must not be empty", "field": "title" }
```

`field` appears only for `invalid_input`. In the frontend, always go through `toAppError`:

```ts
import { toAppError } from "./lib/errors";

try {
  await ThingService.Get(id);
} catch (err) {
  const e = toAppError(err);
  if (e.code === "not_found") { /* ... */ }
  show(e.message);
}
```

`toAppError` never returns the raw `Error.message`. Anything it can't read becomes `internal`.

## Adding a new error code

1. Add the sentinel or typed error to `internal/common/fault`.
2. Add a case to `toFrontend` in `internal/app/errors.go`, with a code, a safe message, and a log level.
3. Add a row to `TestFailMapsErrors`.
4. Add the row to the table above.
