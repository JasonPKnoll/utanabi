import { describe, expect, it } from "vitest";

import { parseRoute, quizHref, songHref, studyHref } from "./route";

describe("parseRoute", () => {
  it.each([
    ["", { name: "home" }],
    ["#", { name: "home" }],
    ["#/", { name: "home" }],
    ["#/add", { name: "add" }],
    ["#/search", { name: "search" }],
    ["#/about", { name: "about" }],
    ["#/settings", { name: "settings" }],
    ["#/song/12", { name: "song", id: 12 }],
    [songHref(7), { name: "song", id: 7 }],
    ["#/song/0", { name: "missing" }],
    ["#/song/-1", { name: "missing" }],
    ["#/song/abc", { name: "missing" }],
    ["#/song/12/edit", { name: "missing" }],
    ["#/song/12/study", { name: "study", id: 12 }],
    [studyHref(12), { name: "study", id: 12 }],
    ["#/song/12/study/34", { name: "study", id: 12, card: 34 }],
    [studyHref(12, 34), { name: "study", id: 12, card: 34 }],
    ["#/song/12/study/0", { name: "missing" }],
    ["#/song/12/study/x", { name: "missing" }],
    ["#/song/12/quiz", { name: "quiz", id: 12 }],
    [quizHref(12), { name: "quiz", id: 12 }],
    ["#/song/12/quiz/3", { name: "missing" }],
    ["#/invites", { name: "invites" }],
    ["#/tools", { name: "tools" }],
    ["#/join/sszt-x59r-5rx2-emm7", { name: "join", code: "sszt-x59r-5rx2-emm7" }],
    ["#/join/", { name: "missing" }],
    ["#/join/<script>", { name: "missing" }],
    ["#/song/0/study", { name: "missing" }],
    ["#/nowhere", { name: "missing" }],
  ])("reads %j", (hash, want) => {
    expect(parseRoute(hash)).toEqual(want);
  });
});
