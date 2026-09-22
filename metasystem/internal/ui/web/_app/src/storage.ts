import { normalizeTheme, THEME_KEY, type ThemePreference } from "./theme";

/**
 * The five keys this build remembers, and nothing else.
 *
 * Every access is wrapped: a browser with site data blocked throws on the very
 * first read, and view state is never worth an error a human has to read. An
 * invalid value yields the default rather than a broken layout. Each workspace
 * server is its own origin, so the keys carry no workspace prefix.
 *
 * The dock's key outlived the dock: the Project Partner is a drawer along the
 * bottom now, it is open or closed rather than open at some width, and it is
 * the same preference about the same collaborator, so it keeps the key a
 * browser already has a value under. The height beside it is the same family
 * of preference under the same family of key.
 *
 * The drawer is closed until a human opens it. A panel that took two fifths of
 * the work area on every first load took it from the one thing the work area
 * is for, and took it for a collaborator who is not connected yet; the bar
 * along the bottom says the Project Partner is there, and opening it is the
 * human's to ask for and this build's to remember.
 */

export const RAIL_KEY = "ms.ui.rail";
export const DOCK_KEY = "ms.ui.dock";
/**
 * How tall the open drawer stands, as a percentage of the work area rather
 * than a count of pixels: the window is resized more often than the drawer is,
 * and a share of the height survives that where a pixel count does not. It is
 * the unit the panel group itself lays out in, so nothing converts anything.
 */
export const DOCK_HEIGHT_KEY = "ms.ui.dock.height";
/**
 * Which way the Backlog is read. The board is the default, because the master
 * makes it the day-to-day surface and because the two acts are on it; a human
 * who chose the list gets the list back.
 */
export const BACKLOG_VIEW_KEY = "ms.ui.backlog.view";

/**
 * What the drawer is worth, in the work area's own height.
 *
 * Two fifths is what an opened drawer takes, and what double-clicking the
 * divider gives back. The two minima are pixels because they are about what
 * fits: a conversation panel shorter than its composer and one line of
 * messages is not a panel, and a work area shorter than a heading and a row is
 * not a work area.
 */
export const DEFAULT_DOCK_HEIGHT = 40;
export const MINIMUM_DOCK_HEIGHT = 160;
export const MINIMUM_WORK_HEIGHT = 200;

/** The share of the height the drawer may be dragged to, at either end. */
const SHORTEST = 10;
const TALLEST = 90;

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

/**
 * The stored height, clamped to the share the drawer may ever have. A value
 * that is not a plain number is not a height at all, so it reads as the
 * default rather than as zero; the panel group clamps it again to the two
 * pixel minima, which is where a short window is answered.
 */
export function clampDockHeight(value: string | null): number {
  if (value === null || !/^\d+(\.\d+)?$/.test(value.trim())) {
    return DEFAULT_DOCK_HEIGHT;
  }
  return Math.min(Math.max(Number.parseFloat(value.trim()), SHORTEST), TALLEST);
}

export function readDockHeight(store: Store | null = browserStore()): number {
  return clampDockHeight(read(DOCK_HEIGHT_KEY, store));
}

/** A tenth of a per cent is under a pixel here; nothing finer is remembered. */
export function writeDockHeight(height: number, store: Store | null = browserStore()): void {
  write(DOCK_HEIGHT_KEY, String(Math.round(height * 10) / 10), store);
}

/** How the Backlog is read: the board unless a human chose the list. */
export type BacklogView = "board" | "list";

export function readBacklogView(store: Store | null = browserStore()): BacklogView {
  return read(BACKLOG_VIEW_KEY, store) === "list" ? "list" : "board";
}

export function writeBacklogView(view: BacklogView, store: Store | null = browserStore()): void {
  write(BACKLOG_VIEW_KEY, view, store);
}
