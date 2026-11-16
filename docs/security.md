# Security

## The bound-method surface

- Every exported method on every service passed to `app.New` can be called by the frontend. In server mode, that means anyone who can reach the server.
- Bind only what the UI needs. Keep helpers unexported, or put them on a type that is not a service.
- A service struct must not expose setters, debug methods, or methods that take a file path or SQL from the caller.
- `ServiceStartup`, `ServiceShutdown`, and `ServiceName` are never bound. That is why the shutdown hook in `internal/app` is safe to register.
- Review the generated `frontend/bindings` diff in every PR. A new file there is a new attack surface.

## Input validation

- Treat every argument of a bound method as untrusted, on every platform.
- Validate in the use case, not in the frontend. Frontend checks are only for the user's convenience.
- Return `fault.Invalid(field, reason)` for bad input. The frontend gets `invalid_input` with the field name.
- Use bound parameters (`?`) in SQL. Never build SQL from input.
- Limit sizes (string length, list length) wherever a caller could send a huge value.

## Identity

- Identity never comes from a method's arguments. A method never takes a user ID, role, or token to decide who the caller is.
- On desktop and mobile, there is one user: the person using the device.
- In server mode, identity will come from auth middleware (`application.AssetOptions.Middleware`) through the request context. There is no auth yet, so **a server-mode build must not be exposed to the internet** until auth exists.

## Secrets on the device

- Never ship secrets in the app: no API keys, database credentials, signing keys, or tokens in code, config files, `ldflags`, or `frontend/`. Anyone can unpack the binary and read them.
- A value that must stay secret belongs on a server you control, not in the app.
- A user's session token, when there is one, goes in the OS keychain (Keychain on macOS and iOS, Credential Manager on Windows, Secret Service on Linux, Keystore on Android). Never in `settings.json`, the database, or a log.
- Carry secrets in memory as `redact.Secret`, so they never reach a log or an error message.
- Files the app writes get mode `0600`, and folders `0700`.

## No remote content in the app window

- The window loads only the embedded frontend (`frontend/dist`). Never point a window at a remote URL.
- Never render HTML from a remote source or from user input with `{@html}`. Lyrics and other text are shown as text.
- Open external links in the system browser (`app.Browser.OpenURL(url)` on the Wails app, from Go), never inside the app window.
- Never load scripts, styles, or fonts from a CDN. Bundle them with Vite.
- The frontend never calls a URL of its own. ESLint forbids `fetch`, `XMLHttpRequest`, `WebSocket`, and `EventSource` in `frontend/src`. Remote APIs are called from Go adapters.

## Server mode

- Bind to localhost (the default, `WAILS_SERVER_HOST=localhost`). In production, put a reverse proxy in front that terminates TLS.
- Set `UTANABI_ALLOWED_ORIGINS` to the proxy's public origin, which becomes `WebSocketOriginPatterns`. Never set `WebSocketAllowAllOrigins`. Config rejects a bare `*`.
- On a loopback bind, Wails rejects requests whose `Host` header is not localhost, an IP, or an allowed origin. This blocks DNS rebinding.
- Events from Go are broadcast to every connected browser. Never put one user's data in an event.
- `/health` is public and returns only `{"status":"ok"}`.
- Keep `UTANABI_DATA_DIR` outside the web root, and back it up.

## Dependencies and builds

- Every Go module, npm package, Node version, and CI action is pinned to an exact version. Actions are pinned by commit SHA.
- `frontend/.npmrc` asks for packages at least 7 days old. pnpm and bun enforce it. With npm, pick versions by hand or with `npm install --before=<date>`.
- `govulncheck` runs in CI on every pull request.
- Release builds are signed and notarized only in a release workflow, with the certificates in CI secrets. No release workflow exists yet. Never commit a certificate or a keystore.
