import type { Approved, Need, Page, Ruling, Where } from "./api";
import type { Row } from "../backlog/api";
import { dateAndTime, minuteTime } from "../backlog/format";
import { transitions } from "../backlog/moves";
import { documentPath, goalPath, projectPath } from "../routes";

/**
 * What the Decisions page says, decided here rather than in the pane.
 *
 * The rules worth arguing about are the ones a human reads and acts on: what
 * each kind of waiting thing is called, where each row opens, which approvals
 * the board would let a human withdraw, and which rulings a Find box leaves on
 * the screen. They are here because each of them is a statement a test can
 * point at.
 *
 * Nothing here judges the ledger. The server said what is waiting and why, and
 * what happens if nobody answers; this turns those answers into a name, a
 * destination and an order on screen. The one judgement made here is the
 * board's own, borrowed rather than reinvented: whether a row is in a lane the
 * board's transition table has a withdrawal for.
 *
 * Nothing here reaches the network, reads the clock on its own, or renders
 * anything.
 */

/* ----------------------------------------------------------- needs you -- */

/**
 * What each kind of waiting thing is called on its chip.
 *
 * The chip is the kind and not the act: a row already says what is being
 * asked in its own sentence, and a chip that repeated the sentence would be
 * noise. A kind this build has no word for shows the server's own, which is
 * the master's rule for an unresolved reference rather than a blank chip.
 */
const KIND_LABELS: Readonly<Record<string, string>> = {
  approval: "approval",
  renewal: "renewal",
  ask: "ask",
  question: "question",
  parked: "parked",
  stopped: "stopped",
  draft: "draft",
  landed: "landed",
  "ruling-review": "review",
  alert: "alert",
};

export function kindLabel(kind: string): string {
  return KIND_LABELS[kind] ?? kind;
}

/**
 * Where one row opens.
 *
 * Three of them are not addresses at all, which is why this is not a path: the
 * steward's messages live in a panel over the page, the register is a document
 * of the checkout, and a seat's question is answered on the fleet channel
 * rather than anywhere in this interface. A kind this build has no surface
 * for is "none" rather than a link that would refuse.
 */
export type Destination =
  | { kind: "link"; to: string }
  | { kind: "notifications"; at: string }
  | { kind: "channel" }
  | { kind: "none" };

export function destinationFor(where: Where): Destination {
  switch (where.kind) {
    case "goal":
      return where.id === "" ? { kind: "none" } : { kind: "link", to: goalPath(where.id) };
    case "record":
    case "register":
      return where.id === "" ? { kind: "none" } : { kind: "link", to: documentPath(where.id) };
    case "question":
      return { kind: "link", to: projectPath("questions") };
    case "notifications":
      return { kind: "notifications", at: where.id };
    case "channel":
      return { kind: "channel" };
    default:
      return { kind: "none" };
  }
}

/**
 * What a row's destination is called, where it is a way through rather than an
 * act.
 *
 * The words are the design's own, one per kind of destination: a seat's
 * question is answered where the seat can hear the answer, and the other three
 * open something this interface serves.
 */
export function wayThrough(where: Where): string {
  switch (destinationFor(where).kind) {
    case "channel":
      return "Answer on the fleet channel with your code";
    case "notifications":
      return "Open the message";
    case "link":
      return openingWords(where.kind);
    default:
      return "";
  }
}

function openingWords(kind: string): string {
  switch (kind) {
    case "goal":
      return "Open the goal";
    case "record":
      return "Open the record";
    case "register":
      return "Open the register";
    case "question":
      return "Open the register";
    default:
      return "Open it";
  }
}

/** The label on the act button, for the two acts this interface has. */
export function actLabel(act: Need["act"]): string {
  return act === "approve" ? "Approve" : "Withdraw approval";
}

/**
 * The muted line under a row: who asked, how long ago, and what silence does.
 *
 * It is one line because the three facts are read together — a human decides
 * from "who wants this, how long have they waited, and what happens if I walk
 * away" — and three lines would be three things to find.
 */
export function askedLine(need: Need, now: Date): string {
  const parts: string[] = [];
  if (need.by !== "") {
    parts.push(`asked by ${need.by}`);
  }
  const age = ageLine(need.since, now);
  if (age !== "") {
    parts.push(age);
  }
  const head = parts.join(", ");
  const silence = `if you do nothing: ${need.silence}`;
  return head === "" ? silence : `${head}; ${silence}`;
}

/**
 * How long ago something happened, in the largest unit that still says
 * something: "today", "yesterday", "5 days", "3 weeks".
 *
 * An instant nothing recorded says nothing rather than "just now": a row with
 * no date is a row whose record does not carry one, and inventing a moment for
 * it is the one way a page of ages can mislead.
 */
export function ageLine(at: string, now: Date): string {
  if (at === "") {
    return "";
  }
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) {
    return "";
  }
  const days = wholeDaysBetween(when, now);
  if (days <= 0) {
    return "today";
  }
  if (days === 1) {
    return "yesterday";
  }
  if (days < 14) {
    return `${String(days)} days`;
  }
  if (days < 60) {
    return `${String(Math.floor(days / 7))} weeks`;
  }
  return `${String(Math.floor(days / 30))} months`;
}

function wholeDaysBetween(at: Date, now: Date): number {
  const day = 24 * 60 * 60 * 1000;
  const from = Date.UTC(at.getFullYear(), at.getMonth(), at.getDate());
  const to = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((to - from) / day);
}

/* -------------------------------------------------------------- decided -- */

/** The four tabs of what was decided, in the order the design names them. */
export type TabId = "rulings" | "decisions" | "answered" | "approved";

export const tabOrder: readonly TabId[] = ["rulings", "decisions", "answered", "approved"];

export type TabName = { id: TabId; title: string };

/**
 * The tab strip, with each tab's count in its name.
 *
 * The count is in the name because it is what a human chooses by: a tab that
 * says "Answered" tells them what is behind it, and one that says "Answered 12"
 * tells them whether it is worth opening.
 */
export function tabs(page: Page): TabName[] {
  const decided = page.decided;
  return [
    { id: "rulings", title: `Rulings ${String(page.counts.rulings)}` },
    { id: "decisions", title: `Decisions ${String(decided.decisions.length)}` },
    { id: "answered", title: `Answered ${String(decided.answered.length)}` },
    { id: "approved", title: `Approved ${String(decided.approved.length)}` },
  ];
}

/**
 * The classes a human can narrow the rulings to: every class the register
 * actually uses, in the order the register uses them, with the rulings that
 * declare none.
 *
 * The list is the register's rather than the grammar's four, because a filter
 * offering a class no ruling carries is a control that can only ever empty the
 * list.
 */
export const NO_CLASS = "none";
export const ANY_CLASS = "any";

export function classesIn(rulings: readonly Ruling[]): string[] {
  const found: string[] = [];
  let standing = false;
  for (const ruling of rulings) {
    if (ruling.class === "") {
      standing = true;
      continue;
    }
    if (!found.includes(ruling.class)) {
      found.push(ruling.class);
    }
  }
  found.sort();
  return standing ? [...found, NO_CLASS] : found;
}

/**
 * The rulings a Find box and a class filter leave on the screen.
 *
 * Find reads the words and the context and nothing else: an id is already the
 * first thing on every card, and a search that also matched the owner would
 * return the whole register for "Wido".
 */
export function shownRulings(
  rulings: readonly Ruling[],
  find: string,
  narrowed: string,
): Ruling[] {
  const wanted = find.trim().toLowerCase();
  return rulings.filter((ruling) => {
    if (narrowed !== ANY_CLASS) {
      const declared = ruling.class === "" ? NO_CLASS : ruling.class;
      if (declared !== narrowed) {
        return false;
      }
    }
    if (wanted === "") {
      return true;
    }
    return `${ruling.words} ${ruling.context}`.toLowerCase().includes(wanted);
  });
}

/**
 * What a ruling's review condition says, as a chip.
 *
 * A date is said in days, because "review passed 11 days ago" is a fact a
 * human acts on and "2026-09-14" is one they have to do arithmetic on. An
 * event is shown exactly as the register wrote it and is never judged: what an
 * event means is the steward's own evaluation, with outcomes it calls
 * unobservable, and a page that guessed would be calling a ruling overdue on
 * no evidence. A ruling with no schedulable condition shows the words the
 * register carries, or nothing where it carries none.
 */
export function reviewChip(ruling: Ruling, now: Date): string {
  if (ruling.due !== "") {
    return ruling.duePassed
      ? `review passed ${overdueWords(ruling.due, now)}`
      : `review due ${dayWords(ruling.due)}`;
  }
  if (ruling.event !== "") {
    return `review on ${ruling.event}`;
  }
  return ruling.condition;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/**
 * A register date's own day, said the way a human writes one: "2 Oct".
 *
 * It is not rendered through a locale and not read as an instant. The register
 * writes a calendar date and means a calendar date — the day the review comes
 * round, in nobody's particular zone — so turning it into an instant would put
 * it on the day before in half the world.
 */
function dayWords(due: string): string {
  const parts = dayParts(due);
  if (parts === null) {
    return due;
  }
  return `${String(parts.day)} ${MONTHS[parts.month - 1]}`;
}

/** A "YYYY-MM-DD" as its three numbers, or null where it is not one. */
function dayParts(day: string): { year: number; month: number; day: number } | null {
  const matched = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (matched === null) {
    return null;
  }
  const year = Number(matched[1]);
  const month = Number(matched[2]);
  const date = Number(matched[3]);
  if (month < 1 || month > 12 || date < 1 || date > 31) {
    return null;
  }
  return { year, month, day: date };
}

/**
 * How long ago a register date passed, against the reader's own calendar day.
 *
 * Both sides are calendar days rather than instants, for the reason above: the
 * register's date is a day, the reader's "today" is a day, and comparing a day
 * with an instant is how a page comes to say "1 day ago" about this morning.
 */
function overdueWords(due: string, now: Date): string {
  const parts = dayParts(due);
  if (parts === null) {
    return due;
  }
  const day = 24 * 60 * 60 * 1000;
  const from = Date.UTC(parts.year, parts.month - 1, parts.day);
  const to = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const days = Math.round((to - from) / day);
  if (days <= 0) {
    return "today";
  }
  return `${String(days)} ${days === 1 ? "day" : "days"} ago`;
}

/**
 * Whether the board would let a human withdraw this approval.
 *
 * It is the board's own table and not a second rule: a withdrawal is a move
 * from one lane to another, and the lanes it is a move from are written down
 * once, in src/backlog/moves.ts, for the drag, the card menu and this page
 * alike.
 */
export function withdrawable(row: Row): boolean {
  return transitions.some((transition) => transition.move === "withdraw" && transition.from === row.lane);
}

/** What an approval's own line says: who admitted it, under what, and when. */
export function approvalLine(approved: Approved, now: Date): string {
  const parts: string[] = [];
  if (approved.by !== "") {
    parts.push(approved.by);
  }
  if (approved.authority !== "") {
    parts.push(approved.authority);
  }
  const age = ageLine(approved.at, now);
  if (age !== "") {
    parts.push(age);
  }
  if (approved.expired) {
    parts.push("expired");
  }
  return parts.join(" · ");
}

/** When something happened, in the smallest form that still places it. */
export function whenLine(at: string, now: Date): string {
  if (at === "") {
    return "";
  }
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) {
    return "";
  }
  const days = wholeDaysBetween(when, now);
  if (days === 0) {
    return minuteTime(at);
  }
  if (days === 1) {
    return `yesterday ${minuteTime(at)}`;
  }
  return dateAndTime(at).split(" ")[0];
}

/**
 * The line above the rulings that says what the register itself is missing, or
 * null where nothing is.
 *
 * The defects are listed rather than hidden — they are rows of the human's own
 * register that the reader could not read — and the line says how many so that
 * a register with forty of them is one quiet line rather than forty loud ones.
 */
export function defectLine(defects: readonly string[]): string | null {
  if (defects.length === 0) {
    return null;
  }
  const count = defects.length;
  return `${String(count)} ${count === 1 ? "row" : "rows"} of the register could not be read`;
}
