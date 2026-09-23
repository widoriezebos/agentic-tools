import type { Changed, Group, Health, Item, Lane, NeedsYou, Page, Where } from "./api";
import { laneFor } from "../backlog/lanes";
import { dateAndTime, minuteTime } from "../backlog/format";
import { backlogPath, documentPath, goalPath, projectPath } from "../routes";

/**
 * What the Overview says, decided here rather than in the pane.
 *
 * The rules worth arguing about are the ones that turn a payload into what a
 * human reads at a glance: which number stands at the top of the page and
 * whether it is urgent, which window a visit compares against and how it is
 * named, which few things out of three lists are the newest, and what three
 * words say about the machinery. They are here because each of them is a
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
 * Where a block stands on this page, for the glance strip to land on.
 *
 * Three of the six tiles name something that is on this page rather than
 * somewhere else: a human who presses "Needs you" wants the list, not another
 * page. So those tiles are fragments of this address, and the block carries
 * the same name as its id.
 */
export function blockAnchor(id: BlockId): string {
  return `overview-${id}`;
}

/* ------------------------------------------------------------- the glance -- */

/**
 * What a number means, said as meaning rather than as a colour.
 *
 * The stylesheet owns which token each of these paints with, so a page that
 * decides "this is the one that wants you" never decides "this is ochre", and
 * the two can be changed apart.
 */
export type Tone = "plain" | "accent" | "ok" | "warn" | "bad";

/** One tile of the strip across the top: a number, a word, and a way through. */
export type Tile = {
  id: string;
  /** The number, or the one word that stands where a number would. */
  value: string;
  label: string;
  tone: Tone;
  /** Where it lands: an address, or a fragment of this one. */
  to: string;
  /** True where `to` is a place on this page rather than another page. */
  anchor: boolean;
  /** True for the one tile whose answer is a word rather than a figure. */
  word: boolean;
};

/**
 * The six numbers the page is read by.
 *
 * Each one is a count the payload already carries, taken from the field that
 * is not capped: Next up shows the lane's own count rather than the three
 * goals the server sends to list, because a strip that said "3" beside a lane
 * of eleven would be counting the list instead of the work.
 *
 * Health is the one tile that is a word. A number of problems is not a thing a
 * human wants to compare; whether there are any is.
 */
export function glance(page: Page): Tile[] {
  const work = page.work;
  return [
    {
      id: "needs-you",
      value: String(page.needsYou.total),
      label: "Needs you",
      tone: page.needsYou.total > 0 ? "accent" : "plain",
      to: `#${blockAnchor("needs-you")}`,
      anchor: true,
      word: false,
    },
    {
      id: "in-progress",
      value: String(work.inProgress.length),
      label: "In progress",
      tone: "plain",
      to: backlogPath(),
      anchor: false,
      word: false,
    },
    {
      id: "next",
      value: String(laneCount(work.lanes, "ready")),
      label: "Next up",
      tone: "plain",
      to: backlogPath(),
      anchor: false,
      word: false,
    },
    {
      id: "waiting",
      value: String(work.waiting.count),
      label: "Waiting",
      tone: "plain",
      to: backlogPath(),
      anchor: false,
      word: false,
    },
    {
      id: "changed",
      value: String(page.changed.total),
      label: "Since your visit",
      tone: "plain",
      to: `#${blockAnchor("changed")}`,
      anchor: true,
      word: false,
    },
    {
      id: "health",
      value: page.health.ok ? "ok" : "attention",
      label: "Health",
      tone: page.health.ok ? "plain" : "warn",
      to: `#${blockAnchor("health")}`,
      anchor: true,
      word: true,
    },
  ];
}

function laneCount(lanes: readonly Lane[], id: string): number {
  return lanes.find((lane) => lane.id === id)?.count ?? 0;
}

/* ----------------------------------------------------------- needs you -- */

/**
 * One kind of thing that is waiting on a human: what it is called, how many
 * there are, and where the rest of them live.
 *
 * `to` is null for the one kind that is not an address: the steward's messages
 * are a panel over this page.
 */
export type Kind = { id: string; label: string; group: Group; to: string | null };

/**
 * The kinds that have anything in them, in the order the design names them.
 *
 * A kind with nothing in it is not a row a human is missing. Five zeroes say
 * less than the block's own one sentence, so a zero is dropped here rather
 * than rendered as an empty disclosure.
 */
export function needsKinds(needs: NeedsYou): Kind[] {
  const all: Kind[] = [
    { id: "approvals", label: "Goals awaiting your approval", group: needs.approvals, to: backlogPath() },
    { id: "questions", label: "Open questions", group: needs.questions, to: projectPath("questions") },
    { id: "drafts", label: "Drafts to accept", group: needs.drafts, to: projectPath("designs") },
    { id: "designs", label: "Designs to mark done", group: needs.designs, to: projectPath("designs") },
    { id: "alerts", label: "Alerts and handoffs", group: needs.alerts, to: null },
  ];
  return all.filter((kind) => kind.group.count > 0);
}

/* ------------------------------------------------------- what changed -- */

/** One line of the timeline: when, what happened, and what it happened to. */
export type Entry = { key: string; at: string; chip: string; title: string; where: Where };

/** How many entries the timeline shows before it offers the rest. */
export const TIMELINE = 5;

/**
 * The three lists as one timeline, newest first.
 *
 * A human coming back does not think in kinds; they think in what happened,
 * and the most recent of it first. So the goals that landed, the goals that
 * moved and the records that changed are one sequence, and the kind is the
 * chip rather than the heading.
 *
 * The chip is what happened rather than which list the entry came from: a
 * conclusion is "landed", a move is the ledger's own verb where the
 * projection could attribute one, and a record is its kind. A move the
 * projection would not attribute says "moved", which is what the page can
 * stand behind.
 *
 * An entry nothing dated sorts last rather than first: an empty instant is not
 * the beginning of time, it is a fact the record does not carry.
 */
export function timeline(changed: Changed, cap: number = TIMELINE): Entry[] {
  const entries: Entry[] = [
    ...changed.concluded.items.map((item, at) => entry("landed", "landed", item, at)),
    ...changed.moved.items.map((item, at) => entry("moved", item.note === "" ? "moved" : item.note, item, at)),
    ...changed.records.items.map((item, at) => entry("record", item.note === "" ? "record" : item.note, item, at)),
  ];
  entries.sort((left, right) => instant(right.at) - instant(left.at));
  return entries.slice(0, cap);
}

function entry(kind: string, chip: string, item: Item, position: number): Entry {
  return {
    key: `${kind}-${item.id}-${String(position)}`,
    at: item.at,
    chip,
    title: item.title === "" ? item.id : item.title,
    where: item.where,
  };
}

/** An instant as a number to sort by, and the beginning of nothing where none. */
function instant(at: string): number {
  const when = new Date(at);
  return at === "" || Number.isNaN(when.getTime()) ? Number.NEGATIVE_INFINITY : when.getTime();
}

/**
 * The counts, above the timeline, in one muted line.
 *
 * Every count is there whether it is zero or not, because this line is read as
 * a shape — four numbers in the same four places every time — rather than as a
 * sentence. The timeline below says which things they were.
 */
export function changedCounts(changed: Changed): string {
  return [
    `${String(changed.moved.count)} moved`,
    `${String(changed.concluded.count)} landed`,
    `${String(changed.records.count)} records`,
    `${String(changed.messages)} messages`,
  ].join(" · ");
}

/**
 * Where the rest of the changes are, or null where the timeline is showing all
 * of them.
 *
 * There is one way through rather than one per kind, because one line under a
 * short list is the whole point of the short list. It goes to the board while
 * any goal change is hidden and to the Project tab otherwise: the board is
 * where a goal's movement is read, and a page hiding only record changes has
 * nothing there for a human to find.
 */
export function seeAll(changed: Changed, shown: readonly Entry[]): string | null {
  const goalsShown = shown.filter((one) => one.where.kind === "goal").length;
  if (changed.concluded.count + changed.moved.count > goalsShown) {
    return backlogPath();
  }
  if (changed.records.count > shown.length - goalsShown) {
    return projectPath("designs");
  }
  return null;
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

/* -------------------------------------------------------------- work now -- */

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

/* ------------------------------------------------------------- the health -- */

/** One of the three pills: what it says, how it reads, and where it opens. */
export type Pill = {
  id: "ledger" | "records" | "deliveries";
  words: string;
  tone: Tone;
  /** The first problem of this source, or null where the source is fine. */
  where: Where | null;
};

/**
 * What the composing package writes in a ledger problem's note. The three
 * sources are told apart by what the server already says about each of them:
 * the ledger names itself, a refused record names its document, and a message
 * the steward could not deliver names the notice.
 */
const LEDGER = "ledger";

/**
 * The machinery in three pills.
 *
 * The block used to be a sentence when all was well and a list of problems
 * when it was not, so a human had to read it to find out which. Three pills
 * always say the same three things in the same three places, and the only
 * question left is whether any of them is not the word it usually is.
 *
 * Each pill's words come from the problems the server composed rather than
 * from a second judgement here: this page has no opinion about the ledger, it
 * has the ledger's.
 */
export function healthPills(health: Health, now: Date): Pill[] {
  const problems = health.problems.items;
  const ledger = problems.find((problem) => problem.note === LEDGER) ?? null;
  const refused = problems.filter((problem) => problem.where.kind === "document");
  const undelivered = problems.filter((problem) => problem.where.kind === "notification");
  return [
    {
      id: LEDGER,
      words: ledger === null ? `synced ${minuteTime(health.syncedAt)}` : ledgerWords(ledger, health.syncedAt, now),
      tone: ledger === null ? "ok" : ledgerTone(ledger),
      where: ledger?.where ?? null,
    },
    {
      id: "records",
      words: refused.length === 0 ? "check clean" : plural(refused.length, "refusal"),
      tone: refused.length === 0 ? "ok" : "bad",
      where: refused[0]?.where ?? null,
    },
    {
      id: "deliveries",
      words: undelivered.length === 0 ? "all delivered" : `${String(undelivered.length)} undelivered`,
      tone: undelivered.length === 0 ? "ok" : "bad",
      where: undelivered[0]?.where ?? null,
    },
  ];
}

/**
 * A fetch that did not happen and a ledger that has fallen behind are two
 * different things to a human: one is a machine that cannot reach the
 * canonical branch, the other is a branch that has moved on. The engine's
 * words for the first always name the fetch, which is the one thing they have
 * in common, and the second is said as the gap itself, which is the fact a
 * human is actually reading for.
 */
function namesAFetch(title: string): boolean {
  return /\bfetch(es|ed|ing)?\b/i.test(title);
}

function ledgerWords(problem: Item, syncedAt: string, now: Date): string {
  if (namesAFetch(problem.title)) {
    return "fetch failed";
  }
  const gap = gapWords(syncedAt, now);
  return gap === "" ? "out of date" : `${gap} behind`;
}

function ledgerTone(problem: Item): Tone {
  return namesAFetch(problem.title) ? "bad" : "warn";
}

/**
 * How far behind, in the largest unit that still says something, and in whole
 * words rather than in a duration's notation: "30 minutes", not "30m0s". A
 * gap under a minute is still a minute, because a pill saying "0 minutes
 * behind" beside a ledger that is behind says nothing.
 */
function gapWords(from: string, now: Date): string {
  const at = new Date(from);
  if (from === "" || Number.isNaN(at.getTime())) {
    return "";
  }
  const minutes = Math.max(1, Math.floor((now.getTime() - at.getTime()) / 60000));
  if (minutes < 60) {
    return plural(minutes, "minute");
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    const rest = minutes % 60;
    return rest === 0 ? plural(hours, "hour") : `${plural(hours, "hour")} ${plural(rest, "minute")}`;
  }
  return plural(Math.floor(hours / 24), "day");
}

/* ----------------------------------------------------------- destinations -- */

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
