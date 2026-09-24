import {
  ANY,
  DEFAULT_WINDOW,
  noFilters,
  rankOf,
  WINDOWS,
  windowTitle,
  type Filters,
  type Window,
} from "./backlog/filters";
import { normalizeFace, normalizeLeading, normalizeSize, type Typeface } from "./partner/typeface";
import { scopeOf, type ScopeFilter } from "./project/pane";
import { normalizeTheme, THEME_KEY, type ThemePreference } from "./theme";

/**
 * The eighteen keys this build remembers, and nothing else.
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
 * Which tab of a page was last open. The two pages with tabs keep one key
 * each, because they are two different readings: a human working through the
 * project's designs and a human checking a goal's open questions are not
 * asking the same page for the same thing, and one key would have each of
 * them reopening the other's tab.
 *
 * What is stored is the tab's own name, which the address uses too. A name
 * the page has no tab for reads as no preference at all, which is what
 * happens when a page's sections change under a browser that remembers one of
 * the old ones.
 */
export const PROJECT_TAB_KEY = "ms.ui.project.tab";
export const GOAL_TAB_KEY = "ms.ui.goal.tab";

/**
 * Which records the Project page is showing: its own, the ones under goals,
 * or all of them.
 *
 * It is one key and not one per tab. Scope is what a human is doing rather
 * than what they are reading — settling the project's own shape, or working
 * through what a goal is about — and a human who asked for the goal-scoped
 * designs is asking the same of the decisions beside them. The Project page
 * is the only page with the control, so the key names it; a goal page is one
 * goal's records by definition and has no scope to remember.
 */
export const PROJECT_SCOPE_KEY = "ms.ui.project.scope";

/**
 * What the board is narrowed to, one key per field rather than one key
 * holding an encoded object.
 *
 * A filter set is five independent answers, and a human who clears the text
 * has not changed their mind about the seat. Five plain values also mean
 * every key in this file is a string a person can read in their browser's
 * storage inspector and recognize, which an encoded blob is not, and that a
 * value written by an older build is read field by field rather than thrown
 * away whole.
 *
 * Every one of them is validated on the way out: a priority that is not one
 * of the three, or a window that is not one this build offers, reads as no
 * preference rather than as a board showing nothing.
 */
export const BACKLOG_TEXT_KEY = "ms.ui.backlog.filter.text";
export const BACKLOG_PRIORITY_KEY = "ms.ui.backlog.filter.priority";
export const BACKLOG_TIER_KEY = "ms.ui.backlog.filter.tier";
export const BACKLOG_SEAT_KEY = "ms.ui.backlog.filter.seat";
export const BACKLOG_ARC_KEY = "ms.ui.backlog.filter.arc";

/** How far back the Done lane reaches, in days, or "all". */
export const BACKLOG_DONE_KEY = "ms.ui.backlog.done";

/**
 * The newest notification this viewer has seen.
 *
 * It is a per-viewer convenience of exactly the kind this file is for: what is
 * unread is what sorts above it, and a browser with site data blocked counts
 * everything as unread rather than failing. The server keeps no such mark —
 * two people at two browsers on the same workspace have read different things,
 * and a mark the server held would be one of them overwriting the other.
 */
export const NOTIFICATIONS_SEEN_KEY = "ms.ui.notifications.seen";

/**
 * The face, the size and the line spacing the conversation is read in.
 *
 * Three keys and not one, for the reason the board's five filters are five
 * keys: a human who changes the size has not changed their mind about the
 * face, and a value written by an older build is read field by field rather
 * than thrown away whole. All three are validated on the way out by the module
 * that decides what a face, a size and a rhythm may be, so a name nothing
 * could render, a size nothing could read and a spacing nothing ever offered
 * reach the page as the defaults.
 */
export const PARTNER_FONT_KEY = "ms.ui.partner.font";
export const PARTNER_FONT_SIZE_KEY = "ms.ui.partner.font-size";
export const PARTNER_LINE_HEIGHT_KEY = "ms.ui.partner.line-height";

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

/**
 * The tab a page was last left on, as the name it was stored under, or null.
 * Whether that name is still a tab of the page is the page's question, and it
 * is asked where the page's own tabs are known.
 */
export function readProjectTab(store: Store | null = browserStore()): string | null {
  return read(PROJECT_TAB_KEY, store);
}

export function writeProjectTab(tab: string, store: Store | null = browserStore()): void {
  write(PROJECT_TAB_KEY, tab, store);
}

export function readGoalTab(store: Store | null = browserStore()): string | null {
  return read(GOAL_TAB_KEY, store);
}

export function writeGoalTab(tab: string, store: Store | null = browserStore()): void {
  write(GOAL_TAB_KEY, tab, store);
}

/**
 * The scope the Project page was last left on. A value that is not one of the
 * three is checked here, because this build knows all three of them, and a
 * browser holding a word from a build that offered a fourth gets the default
 * rather than a page showing nothing.
 */
export function readProjectScope(store: Store | null = browserStore()): ScopeFilter {
  return scopeOf(read(PROJECT_SCOPE_KEY, store));
}

export function writeProjectScope(scope: ScopeFilter, store: Store | null = browserStore()): void {
  write(PROJECT_SCOPE_KEY, scope, store);
}

/**
 * What the board was last narrowed to.
 *
 * The seat and the arc are kept as written, because what they name is the
 * ledger's and this file has no way to know whether a seat still holds
 * anything; the board answers that by offering the remembered value only
 * while something on it carries that name, and a filter naming nothing shows
 * an empty lane with the "clear" link beside it rather than silently
 * widening. The two ranks are checked here, because this build knows all
 * three of them.
 */
export function readBacklogFilters(store: Store | null = browserStore()): Filters {
  return {
    text: read(BACKLOG_TEXT_KEY, store) ?? noFilters.text,
    priority: rankOf(read(BACKLOG_PRIORITY_KEY, store)),
    tier: rankOf(read(BACKLOG_TIER_KEY, store)),
    seat: read(BACKLOG_SEAT_KEY, store) ?? ANY,
    arc: read(BACKLOG_ARC_KEY, store) ?? ANY,
  };
}

export function writeBacklogFilters(filters: Filters, store: Store | null = browserStore()): void {
  write(BACKLOG_TEXT_KEY, filters.text, store);
  write(BACKLOG_PRIORITY_KEY, filters.priority, store);
  write(BACKLOG_TIER_KEY, filters.tier, store);
  write(BACKLOG_SEAT_KEY, filters.seat, store);
  write(BACKLOG_ARC_KEY, filters.arc, store);
}

/** How far back the Done lane reaches, or the default where nothing valid is stored. */
export function readDoneWindow(store: Store | null = browserStore()): Window {
  const stored = read(BACKLOG_DONE_KEY, store);
  // The position, not the value: "all" is null, and a null found is a window
  // this build offers rather than a window it did not find.
  const at = WINDOWS.findIndex((candidate) => windowTitle(candidate) === stored);
  return at < 0 ? DEFAULT_WINDOW : WINDOWS[at];
}

export function writeDoneWindow(days: Window, store: Store | null = browserStore()): void {
  write(BACKLOG_DONE_KEY, windowTitle(days), store);
}

/** The newest notification this viewer has seen, or null for none. */
export function readNotificationsSeen(store: Store | null = browserStore()): string | null {
  return read(NOTIFICATIONS_SEEN_KEY, store);
}

export function writeNotificationsSeen(id: string, store: Store | null = browserStore()): void {
  write(NOTIFICATIONS_SEEN_KEY, id, store);
}

/** The conversation's face, size and rhythm, or the defaults where none is stored. */
export function readPartnerTypeface(store: Store | null = browserStore()): Typeface {
  return {
    face: normalizeFace(read(PARTNER_FONT_KEY, store)),
    size: normalizeSize(read(PARTNER_FONT_SIZE_KEY, store)),
    leading: normalizeLeading(read(PARTNER_LINE_HEIGHT_KEY, store)),
  };
}

export function writePartnerTypeface(typeface: Typeface, store: Store | null = browserStore()): void {
  write(PARTNER_FONT_KEY, typeface.face, store);
  write(PARTNER_FONT_SIZE_KEY, String(typeface.size), store);
  write(PARTNER_LINE_HEIGHT_KEY, String(typeface.leading), store);
}
