/**
 * What the board is narrowed to, and what the Done lane reaches back to.
 *
 * The rules are here rather than in the board so that the ones worth arguing
 * about are readable and tested: what a text match is, how "unassigned" and
 * "no arc" are told apart from a seat or an arc that happens to be spelled
 * that way, and which recorded conclusions a window of days contains.
 *
 * A filter narrows what is shown and changes nothing about what is there.
 * Every lane is filtered by the same set, so a goal that leaves one column
 * leaves all of them, and each lane's count is the count of what it is
 * showing rather than the server's count of what it holds — a count that did
 * not follow its own column would be the one number on the board a human
 * could not trust.
 */

import type { Row } from "./api";

/** The value a select carries for "any": no restriction at all. */
export const ANY = "";

/**
 * The value a select carries for the goals that have none of this thing: no
 * seat holds it, or it is in no arc.
 *
 * A named value travels with a leading "=" so that this can never collide
 * with one. A seat really called "?" and an arc really called "" are not
 * things the ledger can carry, but a rule that depends on that is a rule that
 * breaks the day it can; the prefix means nothing depends on it.
 */
export const NONE = "?";

/** How a named seat or arc travels through a select and through storage. */
export function named(value: string): string {
  return `=${value}`;
}

/** The name a select's value carries, or null where it names none. */
export function nameOf(value: string): string | null {
  return value.startsWith("=") ? value.slice(1) : null;
}

/** A rank select: any, or one of the three the engine has. */
export type Rank = "" | "1" | "2" | "3";

export const RANKS: readonly Rank[] = ["1", "2", "3"];

export type Filters = {
  /** Matched against the id and the intent, in any case, anywhere in either. */
  text: string;
  priority: Rank;
  tier: Rank;
  /** ANY, NONE for work no seat holds, or a named seat. */
  seat: string;
  /** ANY, NONE for a goal in no arc, or a named arc. */
  arc: string;
};

export const noFilters: Filters = { text: "", priority: "", tier: "", seat: "", arc: "" };

/** True when this set narrows anything, which is when "clear" is offered. */
export function anySet(filters: Filters): boolean {
  return (
    filters.text.trim() !== "" ||
    filters.priority !== ANY ||
    filters.tier !== ANY ||
    filters.seat !== ANY ||
    filters.arc !== ANY
  );
}

/** A rank the stored value is not one of is no preference at all. */
export function rankOf(value: string | null): Rank {
  return value !== null && RANKS.includes(value as Rank) ? (value as Rank) : "";
}

export function matchesText(row: Row, text: string): boolean {
  const wanted = text.trim().toLowerCase();
  if (wanted === "") {
    return true;
  }
  return row.ref.id.toLowerCase().includes(wanted) || row.intent.toLowerCase().includes(wanted);
}

/** A rank of 0 is a record that declares none, which no rank filter matches. */
export function matchesRank(value: Rank, rank: number): boolean {
  return value === ANY || String(rank) === value;
}

export function matchesSeat(row: Row, value: string): boolean {
  if (value === ANY) {
    return true;
  }
  const machine = row.claim?.machine ?? "";
  return value === NONE ? machine === "" : machine === nameOf(value);
}

export function matchesArc(row: Row, value: string): boolean {
  if (value === ANY) {
    return true;
  }
  return value === NONE ? row.arc === "" : row.arc === nameOf(value);
}

export function matches(row: Row, filters: Filters): boolean {
  return (
    matchesText(row, filters.text) &&
    matchesRank(filters.priority, row.priority) &&
    matchesRank(filters.tier, row.tier) &&
    matchesSeat(row, filters.seat) &&
    matchesArc(row, filters.arc)
  );
}

/**
 * Every seat holding one of these goals, by the claim's own machine.
 *
 * The list is what is on the board rather than what the fleet could hold: a
 * select offering a seat no card carries offers an empty board, and a seat
 * that appears the moment it claims something is the list a human is reading
 * against.
 */
export function seatsOn(rows: readonly Row[]): string[] {
  return distinct(rows.map((row) => row.claim?.machine ?? ""));
}

/** Every arc these goals are in, by name. */
export function arcsOn(rows: readonly Row[]): string[] {
  return distinct(rows.map((row) => row.arc));
}

function distinct(values: readonly string[]): string[] {
  return [...new Set(values.filter((value) => value !== ""))].sort((left, right) => left.localeCompare(right));
}

/* --------------------------------------------------- the Done lane's reach -- */

/**
 * How far back the Done lane reaches, in days, with null for every recorded
 * conclusion.
 *
 * It opens on one day because that is the question the board is asked most —
 * what landed since yesterday — and because this ledger's Done lane holds
 * four hundred records, which is a wall rather than an answer. The rest of
 * them are one select away and none of them is hidden.
 */
export type Window = number | null;

export const WINDOWS: readonly Window[] = [1, 2, 3, 7, 14, 30, 90, null];

export const DEFAULT_WINDOW: Window = 1;

const DAY = 24 * 60 * 60 * 1000;

/** What a window is called, in the select and in the lane's own head. */
export function windowTitle(days: Window): string {
  return days === null ? "all" : String(days);
}

/**
 * True when this goal's recorded conclusion falls inside the window.
 *
 * A goal whose conclusion nothing dated is in no window of days: the record
 * does not say when it happened, and putting it in the last day because it is
 * done would be the board inventing the very fact the window asks about. It
 * is in "all", which asks for every recorded conclusion rather than for a
 * date.
 *
 * A stamp ahead of the reader's clock counts as inside. The two clocks are
 * not the same clock, and a goal concluded a second ago must not vanish
 * because this browser is a second behind the machine that concluded it.
 */
export function concludedWithin(row: Row, days: Window, now: Date): boolean {
  if (days === null) {
    return true;
  }
  if (row.doneAt === "") {
    return false;
  }
  const at = new Date(row.doneAt).getTime();
  return !Number.isNaN(at) && now.getTime() - at < days * DAY;
}
