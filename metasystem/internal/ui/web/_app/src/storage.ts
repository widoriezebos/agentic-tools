import { normalizeTheme, THEME_KEY, type ThemePreference } from "./theme";

/**
 * The three keys this build remembers, and nothing else.
 *
 * Every access is wrapped: a browser with site data blocked throws on the very
 * first read, and view state is never worth an error a human has to read. An
 * invalid value yields the default rather than a broken layout. Each workspace
 * server is its own origin, so the keys carry no workspace prefix.
 *
 * The dock's key outlived the dock: the Project Partner is a drawer along the
 * bottom now, it is open or closed rather than open at some width, and it is
 * the same preference about the same collaborator, so it keeps the key a
 * browser already has a value under.
 *
 * The drawer is closed until a human opens it. A panel that took two fifths of
 * the work area on every first load took it from the one thing the work area
 * is for, and took it for a collaborator who is not connected yet; the bar
 * along the bottom says the Project Partner is there, and opening it is the
 * human's to ask for and this build's to remember.
 */

export const RAIL_KEY = "ms.ui.rail";
export const DOCK_KEY = "ms.ui.dock";

/** The part of the browser's storage this build uses. */
export type Store = {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
};

/** The browser's own store, or null where there is none. Never throws. */
export function browserStore(): Store | null {
  try {
    return globalThis.localStorage as Store | null;
  } catch {
    return null;
  }
}

function read(key: string, store: Store | null): string | null {
  if (store === null) {
    return null;
  }
  try {
    return store.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string, store: Store | null): void {
  if (store === null) {
    return;
  }
  try {
    store.setItem(key, value);
  } catch {
    // A store that refuses to keep view state costs the next load a default.
  }
}

export function readTheme(store: Store | null = browserStore()): ThemePreference {
  return normalizeTheme(read(THEME_KEY, store));
}

export function writeTheme(preference: ThemePreference, store: Store | null = browserStore()): void {
  write(THEME_KEY, preference, store);
}

export function readRailExpanded(store: Store | null = browserStore()): boolean {
  return read(RAIL_KEY, store) !== "collapsed";
}

export function writeRailExpanded(expanded: boolean, store: Store | null = browserStore()): void {
  write(RAIL_KEY, expanded ? "expanded" : "collapsed", store);
}

export function readDockOpen(store: Store | null = browserStore()): boolean {
  return read(DOCK_KEY, store) === "open";
}

export function writeDockOpen(open: boolean, store: Store | null = browserStore()): void {
  write(DOCK_KEY, open ? "open" : "closed", store);
}
