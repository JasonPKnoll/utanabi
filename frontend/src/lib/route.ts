// The app's screens, read from the URL hash so the back button and reloads work without a router library.
export type Route =
  | { name: "home" }
  | { name: "search" }
  | { name: "add" }
  | { name: "about" }
  | { name: "song"; id: number }
  | { name: "study"; id: number; card?: number }
  | { name: "quiz"; id: number }
  | { name: "invites" }
  | { name: "join"; code: string }
  | { name: "missing" };

export const homeHref = "#/";
export const searchHref = "#/search";
export const addHref = "#/add";
export const aboutHref = "#/about";
export const invitesHref = "#/invites";

// An invite link: the code rides in the hash, so it's never sent to the server or written in its logs.
export function joinHref(code: string): string {
  return `#/join/${encodeURIComponent(code)}`;
}

export function songHref(id: number): string {
  return `#/song/${id}`;
}

// A song's words to study, as a list, or one of them as a card when cardId is given.
export function studyHref(songId: number, cardId?: number): string {
  return cardId === undefined ? `#/song/${songId}/study` : `#/song/${songId}/study/${cardId}`;
}

export function quizHref(songId: number): string {
  return `#/song/${songId}/quiz`;
}

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, "") || "/";
  if (path === "/") {
    return { name: "home" };
  }
  if (path === "/search") {
    return { name: "search" };
  }
  if (path === "/add") {
    return { name: "add" };
  }
  if (path === "/about") {
    return { name: "about" };
  }
  if (path === "/invites") {
    return { name: "invites" };
  }
  const join = /^\/join\/([A-Za-z0-9-]{1,64})$/.exec(path)?.[1];
  if (join !== undefined) {
    return { name: "join", code: join };
  }
  const song = /^\/song\/([1-9]\d{0,14})(?:\/(study|quiz)(?:\/([1-9]\d{0,14}))?)?$/.exec(path);
  if (song?.[1] !== undefined) {
    const id = Number(song[1]);
    if (song[2] === "study") {
      return song[3] === undefined ? { name: "study", id } : { name: "study", id, card: Number(song[3]) };
    }
    if (song[2] === "quiz") {
      return song[3] === undefined ? { name: "quiz", id } : { name: "missing" };
    }
    return { name: "song", id };
  }
  return { name: "missing" };
}
