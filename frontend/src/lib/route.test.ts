import { describe, expect, it } from "vitest";

import { parseRoute, songHref } from "./route";

describe("parseRoute", () => {
  it.each([
    ["", { name: "home" }],
    ["#", { name: "home" }],
    ["#/", { name: "home" }],
    ["#/add", { name: "add" }],
    ["#/song/12", { name: "song", id: 12 }],
    [songHref(7), { name: "song", id: 7 }],
    ["#/song/0", { name: "missing" }],
    ["#/song/-1", { name: "missing" }],
    ["#/song/abc", { name: "missing" }],
    ["#/song/12/edit", { name: "missing" }],
    ["#/nowhere", { name: "missing" }],
  ])("reads %j", (hash, want) => {
    expect(parseRoute(hash)).toEqual(want);
  });
});
