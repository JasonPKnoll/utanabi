import { me, type Me } from "./auth";

// Who the app is for right now: the signed-in user in server mode, or the device in the desktop and phone apps.
// Null until the app has asked.
export const account: { me: Me | null } = $state({ me: null });

export async function loadAccount(): Promise<void> {
  account.me = await me();
}

export function signedIn(name: string): void {
  if (account.me) {
    account.me = { ...account.me, user: name };
  }
}

// Back to the sign-in screen: after signing out, or when a call finds the session has ended.
export function signedOut(): void {
  if (account.me?.mode === "server") {
    account.me = { ...account.me, user: null };
  }
}
