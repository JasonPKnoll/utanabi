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
- On desktop and mobile, there is one user: the person using the device. Everything saved there is the device's (owner `0`).
- In server mode, people sign in with **passkeys** (WebAuthn), and the accounts middleware (`internal/features/accounts/http.go`, through `application.AssetOptions.Middleware`) puts the signed-in user in the request context. Wails hands that context to each bound method, and `internal/identity` reads it.

### Passkeys and sessions

- **No passwords.** A passkey's private key stays on the user's device or in their password manager. The server stores only the public key (`passkeys`), which can't sign anyone in, so a leaked database has nothing to crack.
- **Every passkey must check who's using it** (Face ID, a fingerprint, or a PIN: user verification is required), and is discoverable, so signing in needs no username. New passkeys are asked for on the device itself first (the `client-device` hint). The sign-in screen also waits in the background for a passkey picked from the username field's autofill (conditional mediation); that ceremony has its own cookie, so it never gets in the way of signing up.
- **Each ceremony is one-time and bound to its site.** The challenge is kept server-side for five minutes, in memory, under a random ID in an HttpOnly, SameSite=Strict cookie; it's taken once, and only for the same kind of ceremony on the same host and origin it began on. go-webauthn checks the relying party ID, origin, challenge, and signature.
- **A copied passkey can't sign in.** A sign count that goes backwards (go-webauthn's clone warning) is refused.
- **Sessions are random 256-bit tokens** in an `HttpOnly`, `Secure` (except on localhost, which is plain HTTP), `SameSite=Lax` cookie that lasts 30 days. The database keeps only the token's SHA-256, so the `sessions` table alone signs no one in. Signing out deletes the session; run-out sessions are cleared when the next one is made.
- **Sign-up needs a code**, `UTANABI_SIGNUP_CODE`, at least 12 characters. Unset means sign-up is closed: existing accounts still sign in. Codes are compared in constant time. At most 20 wrong codes are tried in any 10 minutes, server-wide, and at most 30 accounts are made in any hour, before sign-up pauses.
- **Adding a passkey on another device** (About, "Add a passkey on this device") needs the account signed in, and the ceremony is tied to that account.

### What the middleware enforces

- **Every call to a bound service needs a session.** Everything under `/wails/` but the three runtime scripts is refused with 401 and `unauthorized` without one. The page and its assets load signed out, so the sign-in screen can show.
- **A request that changes anything must come from the app's own origin** (its `Origin` header), on top of the `SameSite` cookies. `/auth/` requests must also be JSON, which a cross-site form can't send.
- **Every request is checked against the app's sites**: the exact hosts in `UTANABI_ALLOWED_ORIGINS`, over HTTPS, and `localhost` on `WAILS_SERVER_PORT`. A passkey only ever works on the host it was made on.
- **Each slice's repository scopes every query to the caller** (`identity.Owners`): a user reads and changes only their own library, cards, and progress, and someone else's song or card is simply not found. In server mode a call with no user is refused rather than treated as the device's, so a call that somehow skipped the middleware sees nothing.
- **Security headers on every response**: a Content-Security-Policy that keeps the page to its own scripts, styles, and connections and can't be framed (`frame-ancestors 'none'`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: no-referrer`.

### Known limits

- The `/wails/events` and `/wails/stream/ws` WebSockets are served by Wails before the middleware runs, so they aren't signed in. The app sends no events and opens no streams; never put a user's data in one.
- Passkey ceremonies waiting for their second half are capped at 10,000 server-wide, so a flood of unfinished sign-ins can make others wait up to five minutes. Behind a proxy every request comes from the proxy's address, so there's no limit per person yet.
- Songs saved before accounts existed (owner `0`) aren't in any account's library on a server.
- A free ngrok tunnel decrypts HTTPS at ngrok's servers, so ngrok can see the traffic (never private keys). For real use, end TLS on a machine you control.

## Secrets on the device

- Never ship secrets in the app: no API keys, database credentials, signing keys, or tokens in code, config files, `ldflags`, or `frontend/`. Anyone can unpack the binary and read them.
- A value that must stay secret belongs on a server you control, not in the app.
- In server mode a user's session token lives only in an `HttpOnly` cookie, and the database keeps its hash. When the desktop and phone apps sign in to sync, their token goes in the OS keychain (Keychain on macOS and iOS, Credential Manager on Windows, Secret Service on Linux, Keystore on Android). Never in `settings.json`, the database, or a log.
- Carry secrets in memory as `redact.Secret`, so they never reach a log or an error message.
- Files the app writes get mode `0600`, and folders `0700`.

## No remote content in the app window

- The window loads only the embedded frontend (`frontend/dist`). Never point a window at a remote URL.
- Never render HTML from a remote source or from user input with `{@html}`. Lyrics and other text are shown as text.
- Open external links in the system browser (`app.Browser.OpenURL(url)` on the Wails app, from Go), never inside the app window.
- Never load scripts, styles, or fonts from a CDN. Bundle them with Vite.
- The frontend never calls a URL of its own. ESLint forbids `fetch`, `XMLHttpRequest`, `WebSocket`, and `EventSource` in `frontend/src`. Remote APIs are called from Go adapters. The one exception is `frontend/src/lib/auth.ts`, which may `fetch` the app's own `/auth/` endpoints: a session must be a cookie scripts can't read, and only an HTTP response can set one.

## Outgoing requests

- **Every outside lyrics service can be switched off** with the `lyrics_sources` setting (`settings.json`, `UTANABI_LYRICS_SOURCES` in server mode, or a build-time default). Set it to `[]` or `none` and restart: the app makes no outgoing requests at all, and search is hidden. Use it the moment a source has a problem: downtime, a change in its terms, or a licensing question.
- The app talks to one outside service: **LRCLIB** (`https://lrclib.net`), from `internal/features/songs/lrclib_api.go`. It sends only what you type into song search, and the ID of a song you add. Nothing about you or your library is sent.
- Searches go out only when you press Search, never as you type, to respect LRCLIB's rate limits.
- Requests name the app in the `User-Agent` (`Utanabi <version> (https://github.com/JasonPKnoll/utanabi)`), as LRCLIB asks. No API key is used or needed.
- Every request has a timeout, and responses are capped in size before they are read.
- Everything from a source is untrusted: it is normalized, checked against the same limits as your own input, and shown as text.
- A new source gets its own `<source>_api.go` and a line here.
- **The app makes no request for the dictionary.** `dictionary.db` is a local file, opened read-only. Only the build step, `task dictionary`, goes online: it downloads one pinned jmdict-simplified release from GitHub (`https://github.com/scriptin/jmdict-simplified/releases`) and refuses it unless its SHA-256 matches the one pinned in `internal/dictionary/dictbuild/source.go`. A verified copy is kept in `.cache/dictionary`, so a rebuild works offline. The download has a time limit and a size cap.

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
