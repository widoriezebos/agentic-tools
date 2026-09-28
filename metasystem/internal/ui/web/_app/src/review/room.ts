import { ACCEPTED, entriesIn, FIX, followUp, LEFT_OPEN, type Entry } from "../partner/sitting";

/**
 * The review room's own rules (g1-s65 §3, D5, D9, D10).
 *
 * The room is the sitting's recorder, cards and table with two things of its own
 * beside them: a desk that shows one thing at a time, large, with a strip of what
 * has been on it; and the working state that lets a human step out and come
 * back. Everything here is a function a human would notice if it were wrong, so
 * each one can be read and tested without a browser.
 */

/* ------------------------------------------------------------------ the desk -- */

/**
 * One thing on the desk: a file of the reviewed tree at a range of lines, the
 * change index, one file's diff, or a record's section. `since` reads the change
 * between the reviewed tip and the branch now, which is what Show what changed
 * puts on the desk.
 */
export type DeskItem =
  | { kind: "source"; path: string; from: number; to: number }
  | { kind: "changes"; since?: boolean }
  | { kind: "diff"; path: string; since?: boolean }
  | { kind: "section"; record: string; section: string };

/** The desk: its strip, newest first, and which item of it is up. */
export type Desk = { items: readonly DeskItem[]; current: number };

export const EMPTY_DESK: Desk = { items: [], current: -1 };

/** How many things the strip keeps. A sitting is a morning, not a year. */
const STRIP = 40;

/** One item's identity, so the strip holds each thing once. */
export function deskKey(item: DeskItem): string {
  switch (item.kind) {
    case "source":
      return `source:${item.path}:${String(item.from)}-${String(item.to)}`;
    case "changes":
      return `changes:${item.since === true ? "since" : ""}`;
    case "diff":
      return `diff:${item.path}:${item.since === true ? "since" : ""}`;
    case "section":
      return `section:${item.record}#${item.section}`;
  }
}

/**
 * Put one item on the desk: it goes to the front of the strip and is what the
 * desk shows. An item the strip already holds moves to the front rather than
 * standing twice, because pressing an earlier item is bringing it back.
 */
export function onDesk(desk: Desk, item: DeskItem): Desk {
  const key = deskKey(item);
  const rest = desk.items.filter((one) => deskKey(one) !== key);
  return { items: [item, ...rest].slice(0, STRIP), current: 0 };
}

/** What a strip entry says, in a few words. */
export function deskLabel(item: DeskItem): string {
  switch (item.kind) {
    case "source":
      return item.from > 0 ? `${baseName(item.path)}:${String(item.from)}-${String(item.to)}` : baseName(item.path);
    case "changes":
      return item.since === true ? "what changed since" : "changes";
    case "diff":
      return `${baseName(item.path)} diff${item.since === true ? " since" : ""}`;
    case "section":
      return `${baseName(item.record)} § ${item.section}`;
  }
}

function baseName(path: string): string {
  return path.split("/").at(-1) ?? path;
}

/* --------------------------------------------------------------- anchors -- */

/**
 * A finding's anchor as the record writes it, and as a chip reads it back: a
 * file and its lines, or a record and its section.
 */
export function anchorOf(item: DeskItem): string {
  switch (item.kind) {
    case "source":
      if (item.from <= 0) {
        return item.path;
      }
      return item.to > item.from ? `${item.path}:${String(item.from)}-${String(item.to)}` : `${item.path}:${String(item.from)}`;
    case "diff":
      return item.path;
    case "section":
      return `${item.record} § ${item.section}`;
    case "changes":
      return "the change as a whole";
  }
}

const SECTION_ANCHOR = /^(\S+\.md) § (.+)$/u;
const LINE_ANCHOR = /^([\w./-]*\/?[\w.-]+\.[A-Za-z0-9]+):(\d+)(?:-(\d+))?$/u;

/** A written anchor back as the desk item it names, or null for words. */
export function parseAnchor(said: string): DeskItem | null {
  const text = said.trim();
  const section = SECTION_ANCHOR.exec(text);
  if (section !== null) {
    return { kind: "section", record: section[1], section: section[2].trim() };
  }
  const line = LINE_ANCHOR.exec(text);
  if (line !== null) {
    const from = Number(line[2]);
    return { kind: "source", path: line[1], from, to: line[3] === undefined ? from : Number(line[3]) };
  }
  return null;
}

/** Anywhere in an answer: a path with an extension, a colon and a line or two. */
const ANCHORS = /(?:[\w.-]+\/)*[\w.-]+\.[A-Za-z][A-Za-z0-9]*:\d+(?:-\d+)?/gu;

/**
 * Every anchor an answer's words carry, in order, each as the desk item it puts
 * on the desk (D5). A chip in the conversation is one of these.
 */
export function anchorsIn(text: string): { text: string; item: DeskItem }[] {
  const found: { text: string; item: DeskItem }[] = [];
  for (const match of text.matchAll(ANCHORS)) {
    const item = parseAnchor(match[0]);
    if (item !== null) {
      found.push({ text: match[0], item });
    }
  }
  return found;
}

/* ----------------------------------------------------------------- the door -- */

/** What the door line reads: the record's counts and the room's time. */
export type Door = { findings: number; unanswered: number; steppedOutAt: string };

/**
 * The Review lane card's door line (D9): "In review · you stepped out 2h ago ·
 * 3 findings, 1 unanswered". The counts are the record's, so an unanswered
 * finding is a fact of the record and not of one browser (Astra S65-01).
 */
export function doorLine(door: Door, now: Date): string {
  const parts = ["In review"];
  const out = agoOf(door.steppedOutAt, now);
  if (out !== "") {
    parts.push(`you stepped out ${out}`);
  }
  parts.push(countsLine(door.findings, door.unanswered));
  return parts.join(" · ");
}

/** The room's own header count, the door's without the time. */
export function countsLine(findings: number, unanswered: number): string {
  if (findings === 0) {
    return "no findings yet";
  }
  const named = `${String(findings)} ${findings === 1 ? "finding" : "findings"}`;
  return `${named}, ${unanswered === 0 ? "none" : String(unanswered)} unanswered`;
}

function agoOf(at: string, now: Date): string {
  if (at.trim() === "") {
    return "";
  }
  const then = new Date(at).getTime();
  if (Number.isNaN(then)) {
    return "";
  }
  const minutes = Math.floor((now.getTime() - then) / 60_000);
  if (minutes < 1) {
    return "just now";
  }
  if (minutes < 60) {
    return `${String(minutes)}m ago`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    return `${String(hours)}h ago`;
  }
  return `${String(Math.floor(hours / 24))}d ago`;
}

/* ------------------------------------------------------------------ ending -- */

/** What a finding's Answer line says until the human answers it. */
export const UNANSWERED = "unanswered";

/** Every recorded finding still unanswered: what Clear to land refuses on. */
export function unansweredIn(entries: readonly Entry[]): Entry[] {
  return entries.filter((entry) => entry.section === "Findings" && (entry.answer ?? "").trim() === UNANSWERED);
}

/** The three ways a review ends, each with the consequence it has in this build (D10). */
export const VERDICTS = [
  {
    verdict: "clear to land",
    label: "Clear to land",
    consequence:
      "Records that you examined this work and see nothing standing in the way of it landing. In this build it is your recorded word only: nothing lands because of it.",
  },
  {
    verdict: "send back",
    label: "Send back",
    consequence:
      "Records that the findings marked fix go back to the builder as the correction brief. In this build it is recorded intent only: nothing returns to construction yet.",
  },
  {
    verdict: "no verdict",
    label: "End without a verdict",
    consequence:
      "Records that this review ended without a verdict. What you recorded stays in the record, and nothing is decided by it.",
  },
] as const;

export type Verdict = (typeof VERDICTS)[number]["verdict"];

/** What the End sheet says above a Clear to land it refuses. */
export const CLEAR_REFUSED = "Clear to land waits until every finding is answered. These are not:";

/**
 * The nod line: a review whose piles are empty has produced nothing anyone can
 * be held to, and the End sheet says so plainly rather than recording a nod as
 * a review (R8).
 */
export const NOD_LINE =
  "Nothing is on the board: no finding, no fact, no decision, no open question. A review that ends in a nod has produced nothing anyone can be held to.";

export function nodded(entries: readonly Entry[]): boolean {
  return entries.length === 0;
}

/** What was examined, from the desk's strip, for the Outcome's second line: each thing once, oldest first. */
export function examinedLine(items: readonly DeskItem[]): string {
  if (items.length === 0) {
    return "Examined: nothing was put on the desk";
  }
  const named: string[] = [];
  for (const item of [...items].reverse()) {
    const said = examinedAs(item);
    if (!named.includes(said)) {
      named.push(said);
    }
  }
  return `Examined: ${named.join("; ")}`;
}

function examinedAs(item: DeskItem): string {
  switch (item.kind) {
    case "changes":
      return item.since === true ? "what changed since the reviewed tip" : "the change index";
    case "diff":
      return item.since === true ? `what changed since the reviewed tip in ${item.path}` : `${item.path}, as changed`;
    default:
      return anchorOf(item);
  }
}

/**
 * The Outcome's words with the verdict as its first line and what was examined
 * after it, so a nod cannot pass as a review. The first line is exactly
 * `Verdict: <the chosen verdict>` and an `Examined:` line follows it before any
 * other words; a draft missing either gets it, once. A draft that opens with
 * another verdict line is refused in words rather than rewritten (Sol SOL-A-07).
 */
export function outcomeWithVerdict(verdict: string, text: string, examined: string): { text: string } | { refusal: string } {
  const line = `Verdict: ${verdict}`;
  let said = text.replace(/^\s+/u, "");
  const first = said.split("\n", 1)[0].trim();
  if (/^verdict\s*:/iu.test(first)) {
    if (first !== line) {
      return {
        refusal: `This Outcome opens with "${first}", but the verdict you chose is ${verdict}. Make its first line "${line}", or remove it, and press Record it again.`,
      };
    }
    said = said.slice(said.indexOf(first) + first.length).replace(/^\s+/u, "");
  }
  const parts = [line];
  if (!/^examined\s*:/iu.test(said)) {
    parts.push(examined.trim() === "" ? examinedLine([]) : examined);
  }
  if (said !== "") {
    parts.push(said);
  }
  return { text: parts.join("\n\n") };
}

/**
 * The review's Outcome as the recorder composes it, over the reading as it
 * stands when the press runs: Clear to land is refused while that reading's
 * Findings carry an unanswered finding, named, and the verdict and Examined
 * lines are held (Sol SOL-A-02, SOL-A-07). The End sheet's refusal is the
 * first line of defence; this one holds after a conflict's reread.
 */
export function reviewOutcome(verdict: string, examined: string): (source: string, entry: Entry) => Entry | { refusal: string } {
  return (source, entry) => {
    if (verdict === "clear to land") {
      const open = unansweredIn(entriesIn(source));
      if (open.length > 0) {
        return { refusal: `${CLEAR_REFUSED} ${open.map((finding) => finding.text).join("; ")}` };
      }
    }
    const composed = outcomeWithVerdict(verdict, entry.text, examined);
    return "refusal" in composed ? composed : { ...entry, text: composed.text };
  };
}

/* ------------------------------------------------------------ a moved tip -- */

/**
 * Whether a finding's anchor sits in a file that changed since the tip it was
 * found at, which is the "may have moved" mark (D9).
 */
export function mayHaveMoved(entry: Entry, changed: readonly string[]): boolean {
  const item = parseAnchor(entry.clause);
  if (item === null || item.kind !== "source") {
    return false;
  }
  return changed.includes(item.path);
}

/** What the moved-tip banner says. */
export function movedLine(reviewed: string, current: string): string {
  return `The branch moved while you were out: reviewed at ${reviewed.slice(0, 9)}, now at ${current.slice(0, 9)}.`;
}

/* -------------------------------------------------------- the working state -- */

/**
 * One card's unfinished words (Astra S65-03): the text and the clause as the
 * human left them, and for a card the human made from a selection, what it is
 * and where it is anchored. An open Decide sheet's reason rides as `reason`.
 */
export type Draft = {
  text: string;
  clause: string;
  kind?: string;
  consequence?: string;
  reason?: string;
  /** Which sheet was open over the card, where one was: accept. */
  sheet?: string;
};

export type Drafts = Readonly<Record<string, Draft>>;

export function withDraft(drafts: Drafts, id: string, draft: Draft): Drafts {
  return { ...drafts, [id]: draft };
}

/** A card recorded or dismissed takes its draft with it: drafts never touch the record. */
export function withoutDraft(drafts: Drafts, id: string): Drafts {
  if (!(id in drafts)) {
    return drafts;
  }
  const next: Record<string, Draft> = { ...drafts };
  delete next[id];
  return next;
}

export type Face = "desk" | "board";

/** The room as the mark keeps it, in the shapes the server stores whole. */
export type RoomState = { desk: Desk; face: Face; drafts: Drafts };

export function roomState(desk: Desk, face: Face, drafts: Drafts): RoomState {
  return { desk, face, drafts };
}

/** The room as the mark carries it back, or an empty room for a first visit. */
export function roomOf(kept: { desk?: unknown; face?: string; drafts?: unknown; at?: string } | null | undefined): RoomState {
  if (kept === null || kept === undefined) {
    return { desk: EMPTY_DESK, face: "desk", drafts: {} };
  }
  const desk = isDesk(kept.desk) ? kept.desk : EMPTY_DESK;
  const drafts = kept.drafts !== null && typeof kept.drafts === "object" ? (kept.drafts as Drafts) : {};
  return { desk, face: kept.face === "board" ? "board" : "desk", drafts };
}

function isDesk(value: unknown): value is Desk {
  return (
    value !== null &&
    typeof value === "object" &&
    Array.isArray((value as Desk).items) &&
    typeof (value as Desk).current === "number"
  );
}

/**
 * Whether the room is due to be kept, a second after the last keep: at most once
 * a second while a human types, and always when they leave the field, step out
 * or leave the page. This build sets no timer (src/cuts.test.ts), so the second
 * is measured between keystrokes and the last one is kept by the leaving.
 */
export function keepDue(lastKeptAt: number, now: number): boolean {
  return lastKeptAt === 0 || now - lastKeptAt >= 1000;
}

/* ------------------------------------------------------------------ walks -- */

/** The five walks, in the order the conversation's presses show them (D6). */
export const WALKS = [
  { part: "asked", label: "Asked" },
  { part: "built", label: "Built" },
  { part: "examined", label: "Examined" },
  { part: "proven", label: "Proven" },
  { part: "behaves", label: "Behaves" },
] as const;

/* ------------------------------------------------------------ the answers -- */

/**
 * The four answers a recorded finding takes (g1-s65 D8), each with the
 * consequence it has, said on the press before it is pressed.
 */
export const ANSWERS = [
  {
    answer: "fix",
    label: "Fix in this goal",
    consequence:
      "Marks it for the builder: the candidate will return to construction and be examined again before it comes back.",
  },
  {
    answer: "follow-up",
    label: "Follow-up goal",
    consequence: "Opens a new goal with this finding as its intent, and lets this one land.",
  },
  {
    answer: "accepted",
    label: "Accept, with reason",
    consequence: "Records the risk as accepted, with your reason, on the finding's own line.",
  },
  {
    answer: "left open",
    label: "Leave open",
    consequence: "Keeps the question open, with its consequence, for whoever reads the record next.",
  },
] as const;

export type AnswerKind = (typeof ANSWERS)[number]["answer"];

/**
 * The Answer line one press writes, or the refusal that stops it: a follow-up is
 * written only once the goal it names has been opened, and an acceptance only
 * with the human's reason.
 */
export function answerLine(answer: AnswerKind, detail: string): { line: string } | { refusal: string } {
  switch (answer) {
    case "fix":
      return { line: FIX };
    case "left open":
      return { line: LEFT_OPEN };
    case "follow-up":
      return detail.trim() === "" ? { refusal: "A follow-up is written once its goal is open." } : { line: followUp(detail) };
    case "accepted":
      return detail.trim() === ""
        ? { refusal: "An accepted risk is recorded with your reason. Write the reason before accepting it." }
        : { line: ACCEPTED(detail) };
  }
}

/* ---------------------------------------------------------------- the head -- */

/** A review record's head, as the room reads it: the goal, the tip or the landed commits, and earlier tips. */
export type Reviewed = { goal: string; tip: string; landed: string[]; previously: string[] };

const COMMIT = /^[0-9a-f]{40,64}$/u;

function commitsIn(said: string): string[] {
  return said.split(/\s+/u).filter((word) => COMMIT.test(word));
}

/** The head is the list before the first section; a line of the same shape inside a section is somebody's words. */
function headLines(source: string): { lines: string[]; end: number } {
  const lines = source.split("\n");
  const end = lines.findIndex((line) => line.startsWith("## "));
  return { lines, end: end < 0 ? lines.length : end };
}

export function reviewedOf(source: string): Reviewed {
  const { lines, end } = headLines(source);
  const read: Reviewed = { goal: "", tip: "", landed: [], previously: [] };
  for (const line of lines.slice(0, end)) {
    const field = /^- (\w+):\s*(.*)$/u.exec(line);
    if (field === null) {
      continue;
    }
    const value = field[2];
    switch (field[1].toLowerCase()) {
      case "goals":
        read.goal = value.split(/\s+/u)[0] ?? "";
        break;
      case "reviewed": {
        const commits = commitsIn(value);
        if (value.includes("(the tip of")) {
          read.tip = commits[0] ?? "";
        } else {
          read.landed = commits;
        }
        break;
      }
      case "previously":
        read.previously = commitsIn(value);
        break;
    }
  }
  return read;
}

/**
 * The record with its Reviewed line moved to the branch's new tip and the old
 * tip appended to Previously (g1-s65 D9). It is the human's press, "Review the
 * new tip", and nothing else writes it; a record with no tip on its head
 * answers null.
 */
export function retipped(source: string, current: string): string | null {
  const { lines, end } = headLines(source);
  const at = lines.slice(0, end).findIndex((line) => /^- Reviewed:/u.test(line));
  if (at < 0) {
    return null;
  }
  const was = reviewedOf(source);
  if (was.tip === "") {
    return null;
  }
  const next = [...lines];
  next[at] = `- Reviewed: ${current} (the tip of goal/${was.goal})`;
  const previously = [...was.previously, was.tip].join(" ");
  const held = lines.slice(0, end).findIndex((line) => /^- Previously:/u.test(line));
  if (held >= 0) {
    next[held] = `- Previously: ${previously}`;
    return next.join("\n");
  }
  next.splice(at + 1, 0, `- Previously: ${previously}`);
  return next.join("\n");
}
