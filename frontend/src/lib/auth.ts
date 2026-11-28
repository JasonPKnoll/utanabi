// Sign-in for server mode, with passkeys. This is the one part of the UI that calls URLs of its own (/auth/*), not
// a bound Go service: a session has to be a cookie scripts can't read, and only an HTTP response can set one. The
// desktop and phone apps have no accounts, and there /auth/me isn't served at all.
import type { AppError } from "./errors";

export interface Me {
  // server: accounts are on, and user is who's signed in, or null. local: the app has no accounts.
  mode: "server" | "local";
  user: string | null;
  // Whether the signed-in user is an admin, who can make invites.
  admin: boolean;
  // Whether this server takes new accounts.
  signUp: boolean;
}

// Who's signed in, or that this app has no accounts.
export async function me(): Promise<Me> {
  try {
    const res = await fetch("/auth/me", { credentials: "same-origin", headers: { Accept: "application/json" } });
    const body: unknown = res.ok ? await res.json() : null;
    if (isRecord(body) && typeof body.signUp === "boolean") {
      const user = isRecord(body.user) && typeof body.user.name === "string" ? body.user.name : null;
      const admin = isRecord(body.user) && body.user.admin === true;
      return { mode: "server", user, admin, signUp: body.signUp };
    }
  } catch {
    // Not served, or not JSON: an app with no accounts.
  }
  return { mode: "local", user: null, admin: false, signUp: false };
}

// The first step of making an account: the server checks the username and sign-up code, and returns what the
// device needs to make a passkey. The second step, finishSignUp, should run straight from a tap, so Safari lets it
// open the passkey sheet.
export async function beginSignUp(name: string, code: string): Promise<CreationJSON> {
  const { options } = (await post("/auth/signup/begin", { name, code })) as { options: CreationJSON };
  return options;
}

// Saves a passkey on this device and makes the account with it. Returns the username as saved.
export async function finishSignUp(options: CreationJSON): Promise<string> {
  const credential = await create(options);
  return userName(await post("/auth/signup/finish", attestationJSON(credential)));
}

// Signs in with any of this site's passkeys on this device, or one nearby (a phone, by QR code). A background
// sign-in (conditional) opens no prompt: it waits for the person to pick a passkey from the username field's
// autofill, until signal aborts it.
export async function signIn(background?: { signal: AbortSignal }): Promise<string> {
  const { options } = (await post("/auth/signin/begin", { conditional: !!background })) as {
    options: RequestJSON;
  };
  const credential = await get(options, background?.signal);
  return userName(await post("/auth/signin/finish", assertionJSON(credential)));
}

// Whether this browser can offer passkeys in a username field's autofill, which iOS 16 and later do.
export async function canAutofillPasskeys(): Promise<boolean> {
  try {
    return (
      typeof window.PublicKeyCredential?.isConditionalMediationAvailable === "function" &&
      (await PublicKeyCredential.isConditionalMediationAvailable())
    );
  } catch {
    return false;
  }
}

// Whether a failed sign-in was only stopped on purpose, which is nothing to show.
export function wasStopped(err: unknown): boolean {
  return err instanceof Error && isAppError(err.cause) && err.cause.code === "aborted";
}

const hadAccountKey = "utanabi.hadAccount";

// Whether someone has signed in on this browser before, so the sign-in screen can lead with signing in rather than
// making an account. Storage can refuse; then it's a first visit.
export function hadAccountHere(): boolean {
  try {
    return window.localStorage.getItem(hadAccountKey) === "1";
  } catch {
    return false;
  }
}

export function rememberAccountHere(): void {
  try {
    window.localStorage.setItem(hadAccountKey, "1");
  } catch {
    // Without storage the screen leads with making an account, which still works.
  }
}

// Adds a passkey on this device to the signed-in account, so losing one device doesn't lock its owner out.
export async function addPasskey(): Promise<void> {
  const { options } = (await post("/auth/passkeys/begin", {})) as { options: CreationJSON };
  const credential = await create(options);
  await post("/auth/passkeys/finish", attestationJSON(credential));
}

export async function signOut(): Promise<void> {
  await post("/auth/signout", {});
}

async function post(path: string, body: unknown): Promise<unknown> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  if (res.status === 204) {
    return null;
  }
  const json: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    throw failure(isAppError(json) ? json : { code: "internal", message: "Something went wrong. Please try again." });
  }
  return json;
}

function userName(body: unknown): string {
  if (isRecord(body) && isRecord(body.user) && typeof body.user.name === "string") {
    return body.user.name;
  }
  throw failure({ code: "internal", message: "Something went wrong. Please try again." });
}

// An Error that toAppError understands, with the AppError in its cause like a failed bound call.
function failure(cause: AppError): Error {
  return new Error(cause.message, { cause });
}

// What the browser says when a passkey prompt doesn't finish, in words a person can act on.
function promptFailure(err: unknown): Error {
  const name = err instanceof DOMException ? err.name : "";
  switch (name) {
    case "AbortError":
      return failure({ code: "aborted", message: "Stopped." });
    case "NotAllowedError":
      return failure({ code: "canceled", message: "The passkey prompt was closed or timed out. Try again." });
    case "InvalidStateError":
      return failure({ code: "conflict", message: "This device already has a passkey for this account." });
    case "SecurityError":
      return failure({ code: "invalid_input", message: "Passkeys only work on this app's own web address." });
  }
  return failure({ code: "internal", message: "Your device couldn't make or use a passkey here." });
}

async function create(options: CreationJSON): Promise<PublicKeyCredential> {
  if (!window.PublicKeyCredential) {
    throw failure({ code: "invalid_input", message: "This browser doesn't support passkeys." });
  }
  try {
    const credential = await navigator.credentials.create({ publicKey: creationOptions(options) });
    if (credential instanceof PublicKeyCredential) {
      return credential;
    }
  } catch (err) {
    throw promptFailure(err);
  }
  throw promptFailure(null);
}

async function get(options: RequestJSON, signal?: AbortSignal): Promise<PublicKeyCredential> {
  if (!window.PublicKeyCredential) {
    throw failure({ code: "invalid_input", message: "This browser doesn't support passkeys." });
  }
  try {
    const credential = await navigator.credentials.get({
      publicKey: requestOptions(options),
      ...(signal ? { mediation: "conditional" as CredentialMediationRequirement, signal } : {}),
    });
    if (credential instanceof PublicKeyCredential) {
      return credential;
    }
  } catch (err) {
    throw promptFailure(err);
  }
  throw promptFailure(null);
}

// The server's WebAuthn options and the browser's answers travel as JSON, with binary fields in base64url.

interface DescriptorJSON {
  type: PublicKeyCredentialType;
  id: string;
  transports?: AuthenticatorTransport[];
}

export interface CreationJSON {
  publicKey: Omit<PublicKeyCredentialCreationOptions, "challenge" | "user" | "excludeCredentials"> & {
    challenge: string;
    user: { id: string; name: string; displayName: string };
    excludeCredentials?: DescriptorJSON[];
  };
}

export interface RequestJSON {
  publicKey: Omit<PublicKeyCredentialRequestOptions, "challenge" | "allowCredentials"> & {
    challenge: string;
    allowCredentials?: DescriptorJSON[];
  };
}

export function creationOptions(json: CreationJSON): PublicKeyCredentialCreationOptions {
  const pk = json.publicKey;
  return {
    ...pk,
    challenge: fromBase64url(pk.challenge),
    user: { ...pk.user, id: fromBase64url(pk.user.id) },
    excludeCredentials: (pk.excludeCredentials ?? []).map((c) => ({ ...c, id: fromBase64url(c.id) })),
  };
}

export function requestOptions(json: RequestJSON): PublicKeyCredentialRequestOptions {
  const pk = json.publicKey;
  return {
    ...pk,
    challenge: fromBase64url(pk.challenge),
    allowCredentials: (pk.allowCredentials ?? []).map((c) => ({ ...c, id: fromBase64url(c.id) })),
  };
}

function attestationJSON(c: PublicKeyCredential) {
  const r = c.response as AuthenticatorAttestationResponse;
  return {
    id: c.id,
    rawId: toBase64url(c.rawId),
    type: c.type,
    authenticatorAttachment: c.authenticatorAttachment,
    clientExtensionResults: c.getClientExtensionResults(),
    response: {
      clientDataJSON: toBase64url(r.clientDataJSON),
      attestationObject: toBase64url(r.attestationObject),
      transports: typeof r.getTransports === "function" ? r.getTransports() : [],
    },
  };
}

function assertionJSON(c: PublicKeyCredential) {
  const r = c.response as AuthenticatorAssertionResponse;
  return {
    id: c.id,
    rawId: toBase64url(c.rawId),
    type: c.type,
    authenticatorAttachment: c.authenticatorAttachment,
    clientExtensionResults: c.getClientExtensionResults(),
    response: {
      clientDataJSON: toBase64url(r.clientDataJSON),
      authenticatorData: toBase64url(r.authenticatorData),
      signature: toBase64url(r.signature),
      userHandle: r.userHandle ? toBase64url(r.userHandle) : null,
    },
  };
}

export function toBase64url(buffer: ArrayBuffer): string {
  let binary = "";
  for (const byte of new Uint8Array(buffer)) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function fromBase64url(text: string): ArrayBuffer {
  const base64 = text
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .padEnd(Math.ceil(text.length / 4) * 4, "=");
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isAppError(value: unknown): value is AppError {
  return isRecord(value) && typeof value.code === "string" && typeof value.message === "string";
}
