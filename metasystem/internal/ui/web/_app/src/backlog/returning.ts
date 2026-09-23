import { ANY, named, nameOf, noFilters, rankOf, WINDOWS, windowTitle, type Filters, type Window } from "./filters";
import { SHOWN_GOAL } from "../routes";
import type { BacklogView } from "../storage";

/**
 * Going back to the board a question was asked from.
 *
 * Astra's ninth finding is what this file answers. Weeks later a human clicks
 * the chip on "why are these waiting?" — and opening /backlog reconstructs
 * neither the set nor the viewpoint that made "these" mean anything, because
 * their browser now prefers the list and the filters are whatever they last
 * left them on. Showing today's page as the old one is the outcome the
 * critique refuses.
 *
 * So the capture carries an address, and the address carries the view, the
 * filters, the Done window and the goal. The board applies them for that one
 * arrival, says out loud what it applied, and writes none of it to storage —
 * exactly as the landing already does for a goal. What it cannot apply, it
 * names.
 */

/** The query members a return address carries, beside the goal. */
const VIEW = "view";
const WINDOW = "window";
const TEXT = "text";
const PRIORITY = "priority";
const TIER = "tier";
const SEAT = "seat";
const ARC = "arc";

/**
 * The address this board returns to: its path with what it is showing.
 *
 * Only what narrows the board travels. A board nobody narrowed returns as
 * /backlog with its view, which is the honest address for "the whole board as
 * I had it".
 */
export function returnAddress(view: BacklogView, filters: Filters, reach: Window, goal?: string): string {
  const query = new URLSearchParams();
  if (goal !== undefined && goal !== "") {
    query.set(SHOWN_GOAL, goal);
  }
  query.set(VIEW, view);
  query.set(WINDOW, windowTitle(reach));
  if (filters.text.trim() !== "") {
    query.set(TEXT, filters.text.trim());
  }
  if (filters.priority !== ANY) {
    query.set(PRIORITY, filters.priority);
  }
  if (filters.tier !== ANY) {
    query.set(TIER, filters.tier);
  }
  if (filters.seat !== ANY) {
    query.set(SEAT, filters.seat);
  }
  if (filters.arc !== ANY) {
    query.set(ARC, filters.arc);
  }
  return `/backlog?${query.toString()}`;
}

/** What an arrival asked for, and the words the page says it applied. */
export type Asked = {
  view: BacklogView | null;
  filters: Filters | null;
  reach: Window | null;
  goal: string;
  /** The clauses the status line is made of, in the order they are read. */
  applied: string[];
};

/** Read one arrival's address. Anything it does not name is left alone. */
export function askedFor(query: URLSearchParams): Asked {
  const asked: Asked = { view: null, filters: null, reach: null, goal: query.get(SHOWN_GOAL) ?? "", applied: [] };
  const view = query.get(VIEW);
  if (view === "board" || view === "list") {
    asked.view = view;
    asked.applied.push(`the ${view}`);
  }
  const window = query.get(WINDOW);
  if (window !== null) {
    const found = WINDOWS.find((candidate) => windowTitle(candidate) === window);
    if (found !== undefined) {
      asked.reach = found;
      asked.applied.push(found === null ? "Done reaching back over every recorded conclusion" : `Done reaching back ${String(found)} day${found === 1 ? "" : "s"}`);
    }
  }
  const filters: Filters = {
    text: query.get(TEXT) ?? "",
    priority: rankOf(query.get(PRIORITY)),
    tier: rankOf(query.get(TIER)),
    seat: valueOf(query.get(SEAT)),
    arc: valueOf(query.get(ARC)),
  };
  if (JSON.stringify(filters) !== JSON.stringify(noFilters)) {
    asked.filters = filters;
    asked.applied.push(`filters ${spelled(filters).join(", ")}`);
  } else if (asked.view !== null) {
    // An address that named a view and no filter is a board nobody narrowed,
    // and saying so is what stops today's filters standing in for it. An
    // address naming a view this build has not got asked for nothing at all.
    asked.filters = noFilters;
    asked.applied.push("no filters");
  }
  return asked;
}

/** The status line one arrival prints, or "" where it asked for nothing. */
export function appliedLine(asked: Asked, missing: string): string {
  if (asked.applied.length === 0 && missing === "") {
    return "";
  }
  const said = asked.applied.length === 0 ? "" : `Showing this board as it was: ${asked.applied.join(", ")}.`;
  return missing === "" ? said : `${said} ${missing}`.trim();
}

/** A seat or an arc as the select carries it: a name, or "none", or any. */
function valueOf(value: string | null): string {
  if (value === null || value === "") {
    return ANY;
  }
  return nameOf(value) === null && value !== "?" ? named(value) : value;
}

/** The filters in the words the board's own chip uses. */
function spelled(filters: Filters): string[] {
  const said: string[] = [];
  if (filters.text.trim() !== "") {
    said.push(`text "${filters.text.trim()}"`);
  }
  if (filters.priority !== ANY) {
    said.push(`priority ${filters.priority}`);
  }
  if (filters.tier !== ANY) {
    said.push(`tier ${filters.tier}`);
  }
  if (filters.seat !== ANY) {
    said.push(filters.seat === "?" ? "held by no seat" : `seat ${nameOf(filters.seat) ?? filters.seat}`);
  }
  if (filters.arc !== ANY) {
    said.push(filters.arc === "?" ? "in no arc" : `arc ${nameOf(filters.arc) ?? filters.arc}`);
  }
  return said;
}
