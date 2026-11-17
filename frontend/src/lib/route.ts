// The app's screens, read from the URL hash so the back button and reloads work without a router library.
export type Route = { name: "home" } | { name: "add" } | { name: "song"; id: number } | { name: "missing" };

export const homeHref = "#/";
export const addHref = "#/add";

export function songHref(id: number): string {
  return `#/song/${id}`;
}

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, "") || "/";
  if (path === "/") {
    return { name: "home" };
  }
  if (path === "/add") {
    return { name: "add" };
  }
  const id = /^\/song\/([1-9]\d{0,14})$/.exec(path)?.[1];
  if (id !== undefined) {
    return { name: "song", id: Number(id) };
  }
  return { name: "missing" };
}
