import { describe, expect, it } from "vitest";

import { creationOptions, fromBase64url, requestOptions, toBase64url } from "./auth";

const bytes = (buffer: ArrayBuffer | BufferSource) => Array.from(new Uint8Array(buffer as ArrayBuffer));

describe("base64url", () => {
  it.each([[[]], [[0]], [[251, 255]], [[1, 2, 3, 4, 5]]])("round-trips %j", (list) => {
    const encoded = toBase64url(new Uint8Array(list).buffer);
    expect(encoded).not.toMatch(/[+/=]/);
    expect(bytes(fromBase64url(encoded))).toEqual(list);
  });

  it("reads the URL-safe alphabet", () => {
    expect(bytes(fromBase64url("-_8"))).toEqual([251, 255]);
  });
});

describe("passkey options", () => {
  it("turns the server's creation options into the browser's", () => {
    const options = creationOptions({
      publicKey: {
        rp: { id: "localhost", name: "Utanabi" },
        user: { id: "AQID", name: "hana", displayName: "hana" },
        challenge: "BAUG",
        pubKeyCredParams: [{ type: "public-key", alg: -7 }],
        excludeCredentials: [{ type: "public-key", id: "Bwg" }],
      },
    });
    expect(bytes(options.challenge)).toEqual([4, 5, 6]);
    expect(bytes(options.user.id)).toEqual([1, 2, 3]);
    expect(bytes(options.excludeCredentials?.[0]?.id as ArrayBuffer)).toEqual([7, 8]);
    expect(options.rp.id).toBe("localhost");
  });

  it("turns the server's sign-in options into the browser's", () => {
    const options = requestOptions({
      publicKey: { challenge: "BAUG", rpId: "localhost", userVerification: "required" },
    });
    expect(bytes(options.challenge)).toEqual([4, 5, 6]);
    expect(options.allowCredentials).toEqual([]);
    expect(options.userVerification).toBe("required");
  });
});
