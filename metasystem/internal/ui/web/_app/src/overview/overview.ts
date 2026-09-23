import type { Group, Lane, Where } from "./api";
import { laneFor } from "../backlog/lanes";
import { dateAndTime, minuteTime } from "../backlog/format";
import { backlogPath, documentPath, goalPath, projectPath } from "../routes";

/**
 * What the Overview says, decided here rather than in the pane.
 *
 * The rules worth arguing about are the ones that turn a payload into
 * sentences: which window a visit compares against and how it is named, how a
 * capped list says what it is hiding, how far a design has got, and what each
 * lane of the strip is called. They are here because each of them is a
 * statement a human reads and acts on, and a statement a human acts on should
 * be a line a test can point at.
 *
 * Nothing here reaches the network, reads the clock on its own, or renders
 * anything: every function takes what it needs and answers a string, a
 * destination, or a list.
 */

/** The five blocks, in the order the design reads them top to bottom. */
export type BlockId = "needs-you" | "changed" | "work" | "memory" | "health";

export const blockOrder: readonly BlockId[] = ["needs-you", "changed", "work", "memory", "health"];

/**
 * The two columns at desk width.
 *
 * What needs a human and what happened while they were away are the two
 * questions the page leads with, so they take the left column, which is where
 * reading starts. What is being worked on, what the project remembers and
 * whether anything is wrong are standing answers rather than things to act on,
 * so they stand beside them. At phone width there is one column and it is
 * blockOrder above, which is why this returns the same five names rather than
 * a layout.
 */
export function columns(): { left: BlockId[]; right: BlockId[] } {
  return { left: ["needs-you", "changed"], right: ["work", "memory", "health"] };
}

/**
 * The window line under "Since your last visit".
 *
 * A first visit has no previous one to name, and naming an instant nobody was
 * there for would be inventing a visit; it says the day it looked back over
 * instead. Every other window is an instant, and it is said in day words where
 * a human has one — today and yesterday are the two days they can place
 * without doing arithmetic — and by date where they do not.
 */
export function windowLine(since: string, first: boolean, now: Date): string {
  if (first) {
    return "in the last 24 hours";
  }
  const at = new Date(since);
  if (since === "" || Number.isNaN(at.getTime())) {
    return "since a moment nothing recorded";
  }
  const day = dayWord(at, now);
  if (day === null) {
    return `since ${dateAndTime(since)}`;
  }
  return `since ${day} ${minuteTime(since)}`;
}

/** "today", "yesterday", or null for a day that needs its date. */
function dayWord(at: Date, now: Date): string | null {
  if (sameDay(at, now)) {
    return "today";
  }
  const yesterday = new Date(now.getTime());
  yesterday.setDate(yesterday.getDate() - 1);
  return sameDay(at, yesterday) ? "yesterday" : null;
}

function sameDay(at: Date, other: Date): boolean {
  return (
    at.getFullYear() === other.getFullYear() &&
    at.getMonth() === other.getMonth() &&
    at.getDate() === other.getDate()
  );
}

/**
 * When one row's fact happened, in the smallest form that still places it.
 *
 * A clock time alone places nothing older than today. A goal opened three
 * weeks ago showing "02:00" is a row that looks like it happened this morning,
 * which is the one way a page of times can mislead a human who is skimming it.
 * So today is a clock, yesterday is the word and a clock, and anything further
 * back is the date without one: the hour of a fortnight-old event is not a
 * fact anybody is reading for.
 */
export function whenLine(at: string, now: Date): string {
  if (at === "") {
    return "";
  }
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) {
    return "";
  }
  const day = dayWord(when, now);
  if (day === "today") {
    return minuteTime(at);
  }
  if (day === "yesterday") {
    return `yesterday ${minuteTime(at)}`;
  }
  return dateAndTime(at).split(" ")[0];
}

/**
 * How far a design in flight has got. The goals are the ones the design's own
 * head names, so "of n" is the design's claim about its own scope rather than
 * a count of anything the ledger chose.
 */
export function progressLine(done: number, goals: number): string {
  return `${String(done)} of ${plural(goals, "goal")} done`;
}

/**
 * What a capped list says about the rest of itself, or null where it is
 * showing all of it.
 *
 * The arrow is part of the sentence rather than decoration: the line is a way
 * through to where the rest of them are, and a human who reads "and 4 more"
 * with nothing after it has been told a number and offered nothing.
 */
export function moreLine(group: Group): string | null {
  const hidden = group.count - group.items.length;
  return hidden > 0 ? `and ${String(hidden)} more →` : null;
}

/** A count and the word for it, pluralised the one way English needs here. */
export function plural(count: number, word: string): string {
  return `${String(count)} ${word}${count === 1 ? "" : "s"}`;
}

/**
 * The lane the strip names that is not a lane of the board: the goals
 * concluded on the observation day.
 *
 * It is a day and not a window of hours. A human reading the page at nine in
 * the morning means "since I got up", and a count of the last twenty-four
 * hours would put yesterday evening's conclusions in today's number. The
 * server decides which conclusions are in it; this is what the count is
 * called, and the pair have to say the same thing.
 */
export const DONE_TODAY = "done-today";

/** What the strip calls one count: the board's own lane name, or Done today. */
export function laneName(id: string): string {
  if (id === DONE_TODAY) {
    return "Done today";
  }
  return laneFor(id)?.title ?? id;
}

/** The strip, as names and counts that all land on the board. */
export function laneStrip(lanes: readonly Lane[]): { id: string; title: string; count: number; to: string }[] {
  return lanes.map((lane) => ({ id: lane.id, title: laneName(lane.id), count: lane.count, to: backlogPath() }));
}

/**
 * Where one row opens.
 *
 * Two of them are not addresses at all, which is why this is not a path: the
 * steward's messages live in a panel over the page, and signing in is a sheet.
 * A kind this build has no surface for is "none" rather than a link that would
 * refuse, which is the master's rule for every unresolved reference.
 */
export type Destination =
  | { kind: "link"; to: string }
  | { kind: "notifications"; at: string }
  | { kind: "sign-in" }
  | { kind: "none" };

export function destinationFor(where: Where): Destination {
  switch (where.kind) {
    case "goal":
      return where.id === "" ? { kind: "none" } : { kind: "link", to: goalPath(where.id) };
    case "document":
      return where.id === "" ? { kind: "none" } : { kind: "link", to: documentPath(where.id) };
    case "question":
      return { kind: "link", to: projectPath("questions") };
    case "notification":
      return { kind: "notifications", at: where.id };
    case "backlog":
      return { kind: "link", to: backlogPath() };
    case "sign-in":
      return { kind: "sign-in" };
    default:
      return { kind: "none" };
  }
}

/**
 * The one calm line, when nothing is wrong: when this clone last caught up
 * with the canonical branch, and that the records answered.
 */
export function healthLine(syncedAt: string): string {
  return `Ledger synced ${minuteTime(syncedAt)} · records check clean`;
}
