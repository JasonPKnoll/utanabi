# Logging

## The logger

- `log/slog` with a JSON handler, built by `logger.New` or `logger.Open`.
- `main` sets `slog.Default()` once. There are no package-level loggers.
- The logger travels in the context: `logger.Into(ctx, l)` to attach it, `logger.From(ctx)` to read it. `From` falls back to `slog.Default()`, which is what a Wails service method gets, since Wails builds its own context for each call.
- Use the `Context` methods (`InfoContext`, `ErrorContext`) or `LogAttrs` with a context. sloglint enforces this when a context is in scope.
- Messages are static strings. Values go in attributes, in `snake_case`.

## Attributes

| Helper | Gives | Use |
| --- | --- | --- |
| `logger.Err(err)` | `"error": "<err.Error()>"` | Every logged error. sloglint forbids a raw `"error"` key outside the logger package. |
| `logger.Panic(v, stack)` | a `panic` group with `value` and `stack` | A recovered panic. Pass a nil stack inside a deferred `recover` to capture the current one. |

A slice adds its own name when it logs:

```go
log := logger.From(ctx).With(slog.String("slice", "things"))
```

## Where logs go

| Target | Destination |
| --- | --- |
| Desktop | `utanabi.log` in the OS log folder (macOS `~/Library/Logs/Utanabi`, Windows `%LOCALAPPDATA%\Utanabi\logs`, Linux `$XDG_STATE_HOME/utanabi/logs`) |
| Server mode | stderr |
| iOS and Android | stderr |

Wails's own messages go through the same logger with `"component": "wails"`.

## Levels

- Set `log_level` in `settings.json` (`debug`, `info`, `warn`, `error`). Server mode can override it with `UTANABI_LOG_LEVEL`. The default is `info`.
- At `debug`, Wails logs every binding call with its arguments and result. Never ship a build that defaults to `debug`, and never pass a secret as a bound method argument.
- `app.Fail` logs client mistakes (invalid input, not found, conflict, canceled) at info, timeouts at warn, and anything unknown at error.

## Who logs

Only the boundaries log errors: `app.Fail`, the Wails panic handler, the Wails error handler, background job runners, and `main`. Everything else returns its error.

## Secrets

Never log a secret. Carry it as a `redact.Secret`. Its `LogValue` prints `[REDACTED]`.
