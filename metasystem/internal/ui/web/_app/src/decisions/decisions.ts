import type { Approved, Counts, Need, Page, Ruling, Where } from "./api";
import type { Budget, Row } from "../backlog/api";
import { dateAndTime, minuteTime } from "../backlog/format";
import { prefillFor, transitions, type BudgetSource } from "../backlog/moves";
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

/** The tabs of what was decided, in the order the design names them. */
export type TabId = "rulings" | "decisions" | "answered" | "approved" | "not-now";

export const tabOrder: readonly TabId[] = ["rulings", "decisions", "answered", "approved", "not-now"];

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
    // Not now is a decided thing and belongs here: a human who wrote down a
    // reason and a date has decided, and counting it as undecided was the
    // one place this page told them otherwise.
    { id: "not-now", title: `Not now ${String(decided.notNow.length)}` },
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
 *
 * A row can carry both, and the register does: R-29-m2 is due on a date AND
 * on a terminal re-arm, whichever comes first, which is also how the sweep
 * reads it. So the two are said together rather than the date winning and the
 * event disappearing off the card — the event is half of what the human wrote
 * down, and a card that dropped it would be showing a condition nobody set.
 */
export function reviewChip(ruling: Ruling, now: Date): string {
  const said: string[] = [];
  if (ruling.due !== "") {
    said.push(
      ruling.duePassed ? `review passed ${overdueWords(ruling.due, now)}` : `review due ${dayWords(ruling.due)}`,
    );
  }
  if (ruling.event !== "") {
    said.push(said.length === 0 ? `review on ${ruling.event}` : `or on ${ruling.event}`);
  }
  return said.length === 0 ? ruling.condition : said.join(" · ");
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

/* ------------------------------------------------------------- the queue -- */

/**
 * The queue group: the goals nobody has authorized.
 *
 * Everything below is a statement about what a human sees that a test can
 * point at. Nothing here reaches the network, reads a clock on its own or
 * renders anything; the pane holds the state and this decides what it means.
 */

/** How the queue is ordered: the backlog's own rank, or newest first. */
export type QueueOrder = "backlog" | "newest";

/** The origin chips, which are "yours" and "seats'" and nothing else. */
export const ANY_ORIGIN = "any";
export const YOURS = "human";
export const SEATS = "main";

/** What narrows the queue. None of it is persisted: it is a sitting's state. */
export type Narrowing = { find: string; label: string; origin: string; order: QueueOrder };

export const noNarrowing: Narrowing = { find: "", label: "", origin: ANY_ORIGIN, order: "backlog" };

export function isNarrowed(narrowing: Narrowing): boolean {
  return narrowing.find.trim() !== "" || narrowing.label !== "" || narrowing.origin !== ANY_ORIGIN;
}

/**
 * The Find box reads the id, the intent and the labels.
 *
 * It is the board's `matchesText` extended by the labels, because a label is
 * what a human types when they mean a family of work, and the board's own box
 * would answer nothing for it.
 */
export function matchesQueueText(need: Need, text: string): boolean {
  const wanted = text.trim().toLowerCase();
  if (wanted === "") {
    return true;
  }
  const row = need.row;
  const intent = row === null ? need.title : row.intent;
  const labels = (row === null ? [] : row.labels).join(" ");
  return `${need.id} ${intent} ${labels}`.toLowerCase().includes(wanted);
}

/** A goal's origin, as the payload's row carries it. */
export function originOf(need: Need): string {
  return need.row === null ? "" : need.row.origin;
}

/**
 * The label chips this narrowing offers: the labels of the rows Find and the
 * origin chip leave, with how many carry each, commonest first.
 *
 * Drawn from the rows shown rather than from the whole queue, so a chip never
 * offers a family that a search has already taken off the screen and a count
 * beside a chip is the number of rows it would leave.
 *
 * The label the human already chose is the one thing not applied here. Chips
 * drawn from rows a label chip had already filtered would leave that one chip
 * on the line, with every other family gone and no way back to it.
 */
export function labelChips(needs: readonly Need[], narrowing: Narrowing): { label: string; count: number }[] {
  return labelsIn(shownQueue(needs, { ...narrowing, label: "" }));
}

/** The labels of a set of rows, with how many carry each, commonest first. */
export function labelsIn(needs: readonly Need[]): { label: string; count: number }[] {
  const counted = new Map<string, number>();
  for (const need of needs) {
    for (const label of need.row === null ? [] : need.row.labels) {
      counted.set(label, (counted.get(label) ?? 0) + 1);
    }
  }
  return [...counted.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => (a.count === b.count ? a.label.localeCompare(b.label) : b.count - a.count));
}

/** The rows the tools leave on screen, in the order the toggle asks for. */
export function shownQueue(needs: readonly Need[], narrowing: Narrowing): Need[] {
  const shown = needs.filter((need) => {
    if (!matchesQueueText(need, narrowing.find)) {
      return false;
    }
    if (narrowing.label !== "" && !(need.row === null ? [] : need.row.labels).includes(narrowing.label)) {
      return false;
    }
    if (narrowing.origin === YOURS) {
      return originOf(need) === YOURS;
    }
    if (narrowing.origin === SEATS) {
      return originOf(need) !== YOURS;
    }
    return true;
  });
  if (narrowing.order !== "newest") {
    return shown;
  }
  // Newest first by when the goal was opened. It is a second order over the
  // server's rather than a re-rank: the server's order is the backlog's own,
  // and that is what the default shows.
  return [...shown].sort((a, b) => {
    const left = a.row === null ? a.since : a.row.openedAt;
    const right = b.row === null ? b.since : b.row.openedAt;
    if (left === right) {
      return 0;
    }
    return left > right ? -1 : 1;
  });
}

/**
 * The count in the block's head. It says what was narrowed away rather than
 * quietly showing fewer rows than the number beside the title.
 */
export function queueCount(total: number, shown: number): string {
  const waiting = `${String(total)} waiting`;
  return shown === total ? waiting : `${waiting} · ${String(shown)} shown`;
}

/** A budget tuple in the five words the sheet names them by, or "". */
export function budgetWords(budget: Budget | undefined): string {
  if (budget === undefined) {
    return "";
  }
  return [
    `${budget.elapsedLimit} elapsed`,
    `${String(budget.attemptLimit)} attempts`,
    `${String(budget.reservedJobMinutesLimit)} reserved job minutes`,
    `${String(budget.activeJobLimit)} active jobs`,
    `${String(budget.reviewRoundLimit)} review rounds`,
  ].join(" · ");
}

/** What an opened row says about the budget the goal already carries. */
export function budgetLine(need: Need): string {
  const words = budgetWords(need.row === null ? undefined : need.row.budget);
  return words === "" ? "no budget recorded" : words;
}

/** What is holding a goal up, in one line, or "" where nothing is. */
export function blockedLine(need: Need): string {
  const row = need.row;
  if (row === null) {
    return "";
  }
  if (row.openBlockers.length > 0) {
    return `waits for ${row.openBlockers.join(", ")}`;
  }
  return row.blockedBy.length === 0 ? "" : `waits for ${row.blockedBy.join(", ")}, all done`;
}

/** Which band a goal stands in, and where in it. */
export function bandLine(need: Need): string {
  const row = need.row;
  if (row === null || row.priority < 1) {
    return "no priority band";
  }
  const place = row.sequence > 0 ? `, position ${String(row.sequence)}` : "";
  return `priority ${String(row.priority)}${place}`;
}

/* ------------------------------------------------------ select and act -- */

/**
 * One line of a bulk sheet: the goal, and either the budget it would be
 * approved with or the reason it is not being sent.
 *
 * A goal whose prefill is null is listed and excluded rather than silently
 * dropped or sent with five empty fields: the human sees it named, and
 * approves it alone where the sheet can ask them for the tuple.
 */
export type Planned = { id: string; title: string; budget: Budget | null; source: string; excluded: string };

/** The words the approve sheet uses for where a prefilled budget came from. */
const SOURCE_WORDS: Readonly<Record<BudgetSource, string>> = {
  goal: "the tuple this goal already carries",
  project: "the project's budget law for this goal's tier",
  "last-approved": "the goal approved most recently",
  none: "nothing: this project declares no budget law and no goal has been approved yet",
};

export const NEEDS_ITS_BUDGET = "needs its budget first: approve it alone";

/**
 * What an "Approve n selected" would send, in the order it would send them.
 *
 * The budget is `prefillFor`'s, exactly as the single-goal sheet computes it,
 * so a human who reads one sheet has read the other.
 */
export function approvePlan(
  selected: readonly Need[],
  defaults: Partial<Record<string, Budget>>,
  rows: readonly Row[],
): Planned[] {
  return selected.map((need) => {
    const row = need.row;
    if (row === null) {
      return { id: need.id, title: need.title, budget: null, source: "", excluded: NEEDS_ITS_BUDGET };
    }
    const prefill = prefillFor(row, defaults, rows);
    if (prefill.budget === null) {
      return { id: need.id, title: need.title, budget: null, source: "", excluded: NEEDS_ITS_BUDGET };
    }
    return {
      id: need.id, title: need.title, budget: prefill.budget,
      source: SOURCE_WORDS[prefill.source], excluded: "",
    };
  });
}

/** The same list for a park, where nothing is excluded: a park takes no budget. */
export function parkPlan(selected: readonly Need[]): Planned[] {
  return selected.map((need) => ({
    id: need.id, title: need.title, budget: null, source: "", excluded: "",
  }));
}

/** The goals a run would actually send. */
export function sendable(plan: readonly Planned[]): Planned[] {
  return plan.filter((one) => one.excluded === "");
}

/** How far a run has got, while it runs. */
export function progressLine(done: number, total: number): string {
  return `${String(done)} of ${String(total)}`;
}

/**
 * How far a run of one-publication-per-goal has got.
 *
 * Three states and no fourth. "ready" has sent nothing; "running" is in
 * flight; "stopped" is a run that met an answer it could not read, and it is
 * TERMINAL — see maySend.
 */
export type RunState =
  | { state: "ready" }
  | { state: "running"; done: number; total: number }
  | { state: "stopped"; line: string };

/**
 * Whether the sheet may be dismissed.
 *
 * Not while a run is in flight. The loop publishes whether or not anything is
 * on screen, so a sheet a human could close mid-run would go on writing to
 * the ledger behind a page that had stopped saying so — and the progress line
 * is the only place that says how far it has got.
 */
export function mayDismiss(run: RunState): boolean {
  return run.state !== "running";
}

/**
 * Whether the act button may send.
 *
 * Only from "ready". A stopped run is over: its earlier goals were published
 * and a second press would send them again from the first, which is the one
 * way this sheet could publish the same act twice. The way on is a new
 * selection from the page's re-read, which is what the note says.
 */
export function maySend(run: RunState, blocked: string): boolean {
  return blocked === "" && run.state === "ready";
}

/** What the sheet says once a run has stopped. */
export const RUN_IS_OVER =
  "This run is over. The page below has read the ledger again; select what is still waiting and start a new run.";

/**
 * One publication per goal, in order, stopping at the first answer that is
 * not one.
 *
 * The loop is here rather than in the component so that what it sends, in
 * what order, and where it stops are facts a test can state without a server.
 * It never retries and never continues past a failure: the goals after the
 * one that failed are not sent at all.
 */
export async function runInOrder<T>(
  plan: readonly T[],
  send: (one: T) => Promise<void>,
  onSent: (done: number) => void,
): Promise<{ sent: number; stoppedAt: T | null; reason: unknown }> {
  let sent = 0;
  for (const one of plan) {
    try {
      await send(one);
    } catch (reason: unknown) {
      return { sent, stoppedAt: one, reason };
    }
    sent += 1;
    onSent(sent);
  }
  return { sent, stoppedAt: null, reason: null };
}

/**
 * What a run says when an answer failed.
 *
 * It names the goal and gives the engine's own words, and it says the goal is
 * UNRESOLVED rather than refused: a failed answer can follow a publication
 * that landed, so only the page's next read can say what the ledger did. The
 * goals after it were not sent at all, which is the stop rule.
 */
export function stoppedLine(at: Planned, reason: string, sent: number, total: number): string {
  // The goal that failed is not one of the goals after it: with three sent
  // and the fourth refused of four, nothing followed it.
  const left = total - sent - 1;
  const rest = left <= 0 ? "" : ` ${String(left)} after it were not sent.`;
  // A dash rather than a full stop between the engine's sentence and this
  // one: the engine's words end how the engine ends them, and a page that
  // assumed a period would run two sentences together.
  return `Stopped at ${at.id}: ${reason} — whether it landed is unresolved; the re-read below says what the ledger did.${rest}`;
}
