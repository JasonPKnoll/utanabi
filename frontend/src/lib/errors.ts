// The error a bound Go method rejects with. Go builds it in internal/app (app.Fail), and Wails puts it in
// the rejected Error's `cause`. Show `message` to the user and switch on `code`; never show `Error.message`.
export interface AppError {
  code: string;
  message: string;
  field?: string;
}

const internal: AppError = {
  code: "internal",
  message: "Something went wrong. Please try again.",
};

function isAppError(value: unknown): value is AppError {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const v = value as Record<string, unknown>;
  return typeof v.code === "string" && typeof v.message === "string";
}

// Called when a call finds the session has ended, so the app can go back to the sign-in screen.
let onUnauthorized: () => void = () => {};

export function whenSignedOut(fn: () => void): void {
  onUnauthorized = fn;
}

export function toAppError(err: unknown): AppError {
  if (err instanceof Error && isAppError(err.cause)) {
    if (err.cause.code === "unauthorized") {
      onUnauthorized();
    }
    return err.cause;
  }
  return internal;
}
