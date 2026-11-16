import { describe, expect, it } from "vitest";

import { toAppError } from "./errors";

describe("toAppError", () => {
  it("returns the app error from the cause", () => {
    const err = new Error("raw text", { cause: { code: "not_found", message: "Not found." } });
    expect(toAppError(err)).toEqual({ code: "not_found", message: "Not found." });
  });

  it("keeps the field of an invalid input error", () => {
    const cause = { code: "invalid_input", message: "title must not be empty", field: "title" };
    expect(toAppError(new Error("raw", { cause }))).toEqual(cause);
  });

  it("never shows the raw error message", () => {
    const got = toAppError(new Error("store: query failed (/Users/someone/private.db)"));
    expect(got.code).toBe("internal");
    expect(got.message).not.toContain("private.db");
  });

  it("handles values that are not errors", () => {
    expect(toAppError("boom").code).toBe("internal");
    expect(toAppError(undefined).code).toBe("internal");
  });

  it("ignores a cause without a code", () => {
    expect(toAppError(new Error("raw", { cause: { message: "hi" } })).code).toBe("internal");
  });
});
