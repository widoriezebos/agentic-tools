import { normalizeTheme, THEME_KEY, type ThemePreference } from "./theme";

/**
 * The four keys this build remembers, and nothing else.
 *
 * Every access is wrapped: a browser with site data blocked throws on the very
 * first read, and view state is never worth an error a human has to read. An
 * invalid value yields the default rather than a broken layout. Each workspace
 * server is its own origin, so the keys carry no workspace prefix.
 */

export const RAIL_KEY = "ms.ui.rail";
export const DOCK_KEY = "ms.ui.dock";
export const DOCK_WIDTH_KEY = "ms.ui.dock.width";

export const DEFAULT_DOCK_WIDTH = 400;
export const MINIMUM_DOCK_WIDTH = 320;
export const MAXIMUM_DOCK_WIDTH = 640;
export const MINIMUM_WORK_WIDTH = 480;
export const SEPARATOR_WIDTH = 8;

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
  return read(DOCK_KEY, store) !== "closed";
}

export function writeDockOpen(open: boolean, store: Store | null = browserStore()): void {
  write(DOCK_KEY, open ? "open" : "closed", store);
}

/**
 * The stored width, clamped to what the dock may ever be. A value that is not
 * a plain count of pixels is not a width at all, so it reads as the default
 * rather than as zero.
 */
export function clampDockWidth(value: string | null): number {
  if (value === null || !/^\d+$/.test(value.trim())) {
    return DEFAULT_DOCK_WIDTH;
  }
  return Math.min(Math.max(Number.parseInt(value.trim(), 10), MINIMUM_DOCK_WIDTH), MAXIMUM_DOCK_WIDTH);
}

/**
 * The width the dock can actually have in this viewport: the work area keeps
 * its minimum, so a wide stored dock is narrowed to the room rather than
 * pushing the work area below 480. The stored value is left alone, so widening
 * the window restores it.
 */
export function fitDockWidth(width: number, viewportWidth: number, railWidth: number): number {
  const room = viewportWidth - railWidth - SEPARATOR_WIDTH - MINIMUM_WORK_WIDTH;
  return Math.max(MINIMUM_DOCK_WIDTH, Math.min(width, room));
}

export function readDockWidth(store: Store | null = browserStore()): number {
  return clampDockWidth(read(DOCK_WIDTH_KEY, store));
}

export function writeDockWidth(width: number, store: Store | null = browserStore()): void {
  write(DOCK_WIDTH_KEY, String(Math.round(width)), store);
}
