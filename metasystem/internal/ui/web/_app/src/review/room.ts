import type { Deposit, Subject } from "../partner/api";
import type { Store } from "../partner/conversation";
import { ACCEPTED, answerOf, entriesIn, FIX, followUp, LEFT_OPEN, outcomeIn, pressable, type Entry, type Standing } from "../partner/sitting";
import type { Recorded, Verdict as RowVerdict } from "../backlog/api";
import type { Candidate } from "./candidate";

/**
 * The room's own rules (g1-s65 §3, D5, D9, D10), for every sitting (g1-s67): a
 * review's, and the one a sitting that shapes an intent or a design has.
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

/** A record's own sections, its level-two headings, in the order it writes them. */
export function sectionsIn(source: string): string[] {
  return source
    .split("\n")
    .filter((line) => line.startsWith("## "))
    .map((line) => line.slice(3).trim())
    .filter((section) => section !== "");
}

/**
 * The desk a sitting that shapes a record opens on (g1-s67 D2): the record's own
 * sections in the strip, in its order, and the first of them up.
 */
export function firstDesk(record: string, source: string): Desk {
  const items = sectionsIn(source).map((section): DeskItem => ({ kind: "section", record, section }));
  return items.length === 0 ? EMPTY_DESK : { items, current: 0 };
}

/**
 * The desk a room opens on: the one its mark kept, and where the mark kept none
 * — a first visit, or a sitting that stood before rooms were kept — a shaping
 * sitting's record sections. A review opens on an empty desk, as it always has.
 */
export function openingDesk(kept: Desk | undefined, purpose: string, record: string, source: string): Desk {
  if (kept !== undefined && kept.items.length > 0) {
    return kept;
  }
  return purpose === "review" ? EMPTY_DESK : firstDesk(record, source);
}

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

/* -------------------------------------------------------------- purposes -- */

/** The one word the room's header says for what the sitting is for (g1-s67 D1). */
export function roomWord(purpose: string): string {
  switch (purpose) {
    case "review":
      return "Reviewing";
    case "shape intent":
      return "Shaping the intent";
    default:
      return "Shaping the design";
  }
}

/** What a selection on the desk makes beside Ask: a review's Finding, and a shaping sitting's Fact (g1-s67 D4). */
export function selectionPress(purpose: string): { label: string; kind: string } {
  return purpose === "review" ? { label: "Finding", kind: "finding" } : { label: "Fact", kind: "fact" };
}

/**
 * A card the human made from a selection, as the deposit it stands for until it
 * is recorded: its kind, the words they wrote and the anchor it was made at.
 */
export function localDeposit(local: Draft, subject: Subject | undefined): Deposit {
  return {
    kind: local.kind ?? "finding", text: local.text, anchor: local.clause, consequence: local.consequence ?? "",
    offered: true, subject,
  };
}

/**
 * Whether the page's conversation snapshot is this room's own: read for this
 * record and answered. The page holds the snapshot of wherever it was before
 * the room until the room's own is read, and that one may carry a sitting — the
 * ordinary conversation's, marked before D16 (g1-s67 D6) — which the room must
 * not take for its own and then close on when the store moves to the room.
 */
export function ownSnapshot(store: Pick<Store, "conversation" | "state">, record: string): boolean {
  return store.conversation === record && store.state !== "loading";
}

/** Which End sheet a room mounts (g1-s67 D7): a review's verdicts, or a shaping sitting's Outcome. */
export function endOf(purpose: string): "verdict" | "outcome" {
  return purpose === "review" ? "verdict" : "outcome";
}

/** The door line of a shaping sitting, on its record's page: "In a sitting · you stepped out 2h ago". */
export function sittingDoorLine(steppedOutAt: string, now: Date): string {
  const out = agoOf(steppedOutAt, now);
  return out === "" ? "In a sitting" : `In a sitting · you stepped out ${out}`;
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
      "Records your verdict on the goal, bound to the tip you reviewed, and commits this record beside it. It is the word the landing gate waits for: the seat that holds the goal lands it on its next turn, at this tip.",
  },
  {
    verdict: "send back",
    label: "Send back",
    consequence:
      "Sends the findings you answered fix to the seat that holds the goal as its correction brief. The goal leaves Review, and the holder revises from the brief.",
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
 * other words; a draft missing either gets it, once, and an `Examined:` line
 * with nothing after its colon is missing (Sol SOL-A-07). A draft that opens with
 * another verdict line is refused in words rather than rewritten (Sol SOL-A-07).
 */
export function outcomeWithVerdict(
  verdict: string,
  text: string,
  examined: string,
  tip = "",
): { text: string } | { refusal: string } {
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
  // The tip this Outcome was drafted for is the recorder's to write, under the
  // verdict (g1-s69 D1): a draft's own Reviewed at line gives way to it.
  const at = said.split("\n", 1)[0];
  if (/^reviewed at\s*:/iu.test(at.trim())) {
    said = said.slice(at.length).replace(/^\s+/u, "");
  }
  const parts = tip === "" ? [line] : [line, `${REVIEWED_AT} ${tip}`];
  const next = said.split("\n", 1)[0];
  const named = /^examined\s*:(.*)$/iu.exec(next.trim());
  if (named !== null && named[1].trim() === "") {
    said = said.slice(next.length).replace(/^\s+/u, "");
  }
  if (named === null || named[1].trim() === "") {
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
export function reviewOutcome(
  verdict: string,
  examined: string,
  tip = "",
): (source: string, entry: Entry) => Entry | { refusal: string } {
  return (source, entry) => {
    // The Outcome is bound to the tip it was drafted for (g1-s69 D1): a record
    // retipped since is refused, and End drafts one for the tip it names now.
    if (tip !== "" && reviewedOf(source).tip !== tip) {
      return { refusal: RETIPPED };
    }
    if (verdict === "clear to land") {
      const open = unansweredIn(entriesIn(source));
      if (open.length > 0) {
        return { refusal: `${CLEAR_REFUSED} ${open.map((finding) => finding.text).join("; ")}` };
      }
    }
    const composed = outcomeWithVerdict(verdict, entry.text, examined, tip);
    return "refusal" in composed ? composed : { ...entry, text: composed.text };
  };
}

/** What a review's Outcome that carries no verdict is refused with: only End drafts one that does. */
export const NO_VERDICT =
  "This Outcome carries no verdict, so it was not drafted by End. Press End, choose how this review ends, and record the Outcome that draft offers.";

/**
 * How a press of Record it shapes this card, from the card alone: a review's
 * Outcome records under the verdict its closing deposit carries, whatever this
 * page remembers, so a card rebuilt after a reload records under the same shape
 * (Sol SOL-A-02, SOL-A-07). Every other card is recorded as it reads.
 */
export function outcomeShape(
  card: Pick<Deposit, "kind" | "verdict" | "tip">,
  purpose: string | undefined,
  items: readonly DeskItem[],
): ((source: string, entry: Entry) => Entry | { refusal: string }) | undefined {
  if (card.kind !== "outcome" || purpose !== "review") {
    return undefined;
  }
  const verdict = card.verdict ?? "";
  return verdict === "" ? () => ({ refusal: NO_VERDICT }) : reviewOutcome(verdict, examinedLine(items), card.tip ?? "");
}

/* ------------------------------------------------ the verdict that acts -- */

/** The line the recorder writes under the verdict: the tip the Outcome was drafted for (g1-s69 D1). */
export const REVIEWED_AT = "Reviewed at:";

/** What Record it says when the record was retipped after its Outcome was drafted. */
export const RETIPPED = "the branch was retipped since this Outcome was drafted; press End again";

/**
 * What the room says when Review the new tip is pressed while an Outcome card
 * drafted for the old tip is still unrecorded: the head moves, the card's words
 * stay in its draft, and End drafts one for the new tip (g1-s69 D1).
 */
export const RETIP_ASKS_END =
  "The Outcome waiting in the conversation was drafted for the tip you reviewed before. Your words stay in its draft; press End again to draft the Outcome for the new tip.";

/** Whether the sitting's Outcome card was recorded, which is how a sitting ends with its Outcome written. */
export function recordedOutcome(cards: readonly { kind: string; standing: Standing }[]): boolean {
  return cards.some((card) => card.kind === "outcome" && card.standing === "recorded");
}

/** Whether an Outcome card still waits to be recorded, which a retip leaves drafted for the old tip. */
export function standingOutcome(cards: readonly { kind: string; standing: Standing }[]): boolean {
  return cards.some((card) => card.kind === "outcome" && pressable(card.standing));
}

/** The verdicts that act, as goal review spells them; No verdict performs nothing. */
export function actingVerdict(words: string): "clear-to-land" | "send-back" | "" {
  switch (words) {
    case "clear to land":
      return "clear-to-land";
    case "send back":
      return "send-back";
    default:
      return "";
  }
}

/** The verdict a recorded Outcome performs on its goal, and the words the room says after it. */
export type ToPerform = { goal: string; fixes: number; asked: { record: string; verdict: string; brief: string; work: string } };

/**
 * What Record it performs once the record has taken a review's Outcome (g1-s69
 * D1, D2): goal review with the card's own verdict, on the goal the record's
 * head names, with a send-back's brief — the one the human edited, else the one
 * composed from the findings answered fix. No verdict, and every sitting that
 * is not a review, performs nothing and answers null.
 */
export function verdictToPerform(
  card: Pick<Deposit, "kind" | "verdict">,
  purpose: string | undefined,
  record: string,
  source: string,
  brief: string | null,
): ToPerform | null {
  const verdict = card.kind === "outcome" && purpose === "review" ? actingVerdict(card.verdict ?? "") : "";
  if (verdict === "") {
    return null;
  }
  const read = reviewedOf(source);
  const entries = entriesIn(source);
  return {
    goal: read.goal,
    fixes: fixFindings(entries).length,
    asked: {
      record, verdict, work: "",
      brief: verdict === "send-back" ? (brief ?? correctionBrief(entries, record, read.tip)) : "",
    },
  };
}

/**
 * The verdict a record's recorded Outcome carries that the goal does not (Sol
 * SOL-S69-04): the Outcome opens with an acting verdict and a Reviewed at line
 * naming the record's own tip, and the goal's standing verdict is not this one
 * from this record at this tip. It is what the room offers again after a
 * reload stranded the act that Record it began, with the brief the human
 * edited, kept in the room's drafts, else the one composed from the findings
 * answered fix. Anything else answers null.
 */
export function strandedVerdict(source: string, record: string, standing: RowVerdict | undefined, brief: string | null): ToPerform | null {
  const said = (outcomeIn(source)?.text ?? "").split("\n").map((line) => line.trim()).filter((line) => line !== "");
  const verdict = /^Verdict:\s*(.*)$/u.exec(said[0] ?? "")?.[1].trim() ?? "";
  const at = said[1]?.startsWith(REVIEWED_AT) === true ? said[1].slice(REVIEWED_AT.length).trim() : "";
  const tip = reviewedOf(source).tip;
  if (actingVerdict(verdict) === "" || tip === "" || at !== tip) {
    return null;
  }
  if (standing !== undefined && standing.verdict === actingVerdict(verdict) && standing.tip === tip && sameRecord(standing.record, record)) {
    return null;
  }
  return verdictToPerform({ kind: "outcome", verdict }, "review", record, source, brief);
}

/** Whether two names of a review record are the same record: the ledger names it from its own root. */
function sameRecord(one: string, other: string): boolean {
  return one === other || one.endsWith(`/${other}`) || other.endsWith(`/${one}`);
}

/**
 * One of the room's acts under the human's session (Sol SOL-S69-03): a refusal
 * for want of a sign-in opens the sign-in sheet and, once the human has signed
 * in, the same press is made again — once, as the board's acts do it. Every
 * refusal is also said, so a sheet closed without signing in leaves the words
 * and the press to try again.
 */
export function pressSignedIn<T>(
  ports: {
    act: () => Promise<T>;
    done: (answer: T) => void;
    refused: (error: unknown) => void;
    signIn: (again: () => Promise<void>) => void;
  },
  again = true,
): Promise<void> {
  return ports.act().then(ports.done, (error: unknown) => {
    ports.refused(error);
    if (again && (error as { signIn?: unknown } | null)?.signIn === true) {
      ports.signIn(() => pressSignedIn(ports, false));
    }
  });
}

/** Every recorded finding the human answered fix, in the order the record carries them. */
export function fixFindings(entries: readonly Entry[]): Entry[] {
  return entries.filter((entry) => entry.section === "Findings" && answerOf(entry.answer ?? "") === "fix");
}

/** What Send back is refused with where no finding was answered fix. */
export const NO_FIX =
  "Send back carries the findings you answered fix as the correction brief, and none is answered fix. Answer the findings the builder must fix, or end the review another way.";

/** What the Send back sheet says over the brief. */
export const BRIEF_SAID = "These become the correction brief. Read it, edit it, and press Send back.";

/**
 * The correction brief a send-back carries (g1-s69 D2): the findings answered
 * fix, each with its words and its anchor, in the order recorded, under one
 * heading naming the record and the tip. "" where no finding is answered fix.
 */
export function correctionBrief(entries: readonly Entry[], record: string, tip: string): string {
  const fixes = fixFindings(entries);
  if (fixes.length === 0) {
    return "";
  }
  const lines = [`# Correction brief`, "", `## The findings answered fix in ${record} at ${tip.slice(0, 9)}`, ""];
  fixes.forEach((entry, index) => {
    const anchor = entry.clause.trim();
    lines.push(`${String(index + 1)}. ${entry.text.trim()}${anchor === "" ? "" : ` (${anchor})`}`);
  });
  return `${lines.join("\n")}\n`;
}

const COUNTED = ["no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];

function counted(count: number, one: string, many: string): string {
  return `${COUNTED[count] ?? String(count)} ${count === 1 ? one : many}`;
}

/** What the room says once the verdict is on the goal (g1-s69 §3). */
export function recordedLine(recorded: Pick<Recorded, "verdict" | "tip" | "by">, fixes: number, goal: string): string {
  if (recorded.verdict === "send-back") {
    return `Sent back with your ${counted(fixes, "finding", "findings")}; the goal has left Review, and the seat that holds ${goal} revises from your brief on its next turn.`;
  }
  return `Recorded. The goal now carries your verdict: clear to land at ${recorded.tip.slice(0, 7)}, reviewed by ${recorded.by}.`;
}

/**
 * The Review lane card's verdict line (g1-s69 §3, D2), read from the goal's own
 * history as every seat reads it, or "" where no verdict stands.
 */
export function verdictLine(verdict: RowVerdict | undefined, goal: string): string {
  if (verdict === undefined) {
    return "";
  }
  if (verdict.verdict === "clear-to-land") {
    return `reviewed by ${verdict.by} · clear to land`;
  }
  if ((verdict.candidates ?? []).length > 1) {
    return `the holder needs to know which work: ${orList(verdict.candidates ?? [])}`;
  }
  if ((verdict.attempt ?? 0) > 0) {
    return `attempt ${String(verdict.attempt)} started from your brief`;
  }
  // The holder's turn takes the step unprompted (g1-s70 D3, Sol SOL-S69-01),
  // so the card says who takes it and when.
  return `sent back by ${verdict.by} · the seat that holds ${goal} revises on its next turn`;
}

function orList(names: readonly string[]): string {
  return names.length < 2 ? names.join("") : `${names.slice(0, -1).join(", ")} or ${names[names.length - 1]}`;
}

/* ---------------------------------------------------------------- the pill -- */

/** The candidate's pill, as the room's header shows it (g1-s69 D3). */
export type Pill = {
  /** Which of its states: no contract, stopped, starting, running and not answering, running the reviewed tip, or running another. */
  state: "no-contract" | "unread" | "stopped" | "starting" | "not-answering" | "reviewed" | "moved";
  words: string;
  address: string;
  run: boolean;
  stop: boolean;
};

/**
 * The pill from what app status answered, or the words it was refused with, and
 * the tip the room reviews. It compares the running commit with that tip: the
 * branch the run follows can move while the room reviews the tip its record
 * names, and a run of another commit must say so (Astra S69-04).
 */
export function pillOf(read: Candidate | { refusal: string; code: string } | null, reviewed: string): Pill {
  const none = { address: "", run: false, stop: false };
  if (read === null) {
    return { ...none, state: "unread", words: "the candidate has not been read" };
  }
  if ("refusal" in read) {
    return read.code === "no-contract"
      ? { ...none, state: "no-contract", words: read.refusal }
      : { ...none, state: "unread", words: read.refusal, run: true };
  }
  const address = read.address ?? "";
  const running = read.commit ?? "";
  switch (read.state) {
    case "running":
      break;
    case "starting":
      return { state: "starting", words: `the candidate is starting${address === "" ? "" : ` on ${address}`}`, address, run: false, stop: true };
    default:
      return { ...none, state: "stopped", words: "the candidate is not running", run: true };
  }
  if (read.readiness !== "answering" && read.readiness !== "observed-at-startup") {
    return { state: "not-answering", words: `the candidate runs ${short(running)} and is not answering`, address, run: false, stop: true };
  }
  if (reviewed !== "" && running === reviewed) {
    return { state: "reviewed", words: `running the reviewed tip ${short(reviewed)}`, address, run: false, stop: true };
  }
  return {
    state: "moved",
    words: `running ${short(running)}, not the reviewed ${short(reviewed)}`,
    address, run: false, stop: true,
  };
}

function short(commit: string): string {
  return commit === "" ? "an unrecorded commit" : commit.slice(0, 7);
}

/** An address as a link opens it: the candidate's own origin. */
export function candidateHref(address: string): string {
  if (address === "") {
    return "";
  }
  return /^https?:\/\//u.test(address) ? address : `http://${address}/`;
}

/* ------------------------------------------------------------- the door -- */

/**
 * What Review it asks for: the goal, or — once a press was refused after the
 * server had made the record — that record, so the next press opens the sitting
 * on it rather than making another (Sol SOL-A-05).
 */
export function reviewStart(goal: string, made: string): { purpose: string; subject: Subject } {
  return made === ""
    ? { purpose: "review", subject: { kind: "goal", id: goal, title: goal } }
    : { purpose: "review", subject: { kind: "record", id: made, title: `Review of ${goal}` } };
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
 * or leave the page. The words typed last are kept by keepAfterSilence below.
 */
export function keepDue(lastKeptAt: number, now: number): boolean {
  return lastKeptAt === 0 || now - lastKeptAt >= 1000;
}

/** How long the room waits after the last keystroke before it keeps the drafts. */
export const KEEP_AFTER_SILENCE = 1000;

/**
 * Keep the room once a second has passed with no further change, answering the
 * cancel the next change calls. It is the one timer this build sets, a named
 * row of src/cuts.test.ts: it fires once, reads nothing, and calls the room's
 * own keep, so words typed just before the typing stops are kept without a blur.
 */
export function keepAfterSilence(keep: () => void): () => void {
  const settled = setTimeout(keep, KEEP_AFTER_SILENCE);
  return () => {
    clearTimeout(settled);
  };
}

/**
 * Keeps one room on its sitting's mark, in order. One keep is out at a time: a
 * keep asked while one is out waits for it and then sends the words as they
 * stand by then, so an older snapshot is never sent after a newer one, and the
 * keeps asked while waiting become one. Every keep carries the next number of
 * the room's sequence, which starts above what the mark held when the room was
 * opened, and above the clock, so a page opened later numbers above one opened
 * earlier; the server ignores a keep not numbered above the one it holds, so a
 * request that arrives late changes nothing. The keep made as the page goes
 * away does not wait, since nothing runs after the page has gone to send it;
 * its number is what keeps an older keep still out from overwriting it.
 */
export class RoomKeeper {
  private record = "";
  private seq = 0;
  private last = "";
  private out: Promise<boolean> | null = null;
  private answer: Promise<boolean> = Promise.resolve(true);

  constructor(
    private readonly send: (record: string, room: RoomState & { seq: number }, leaving: boolean) => Promise<unknown>,
  ) {}

  /** Take a room as its mark carried it back, with the sequence the mark holds. */
  open(record: string, kept: RoomState, held: number, now = Date.now()): void {
    this.record = record;
    this.seq = Math.max(held, now);
    this.last = JSON.stringify(kept);
  }

  /**
   * Keep the room as `room` reads when the keep is sent, answering whether it
   * landed. The same words as the last keep answer that keep's answer, unless
   * the page is going away while that keep is still out.
   */
  async keep(room: () => RoomState, leaving = false): Promise<boolean> {
    while (!leaving && this.out !== null) {
      await this.out;
    }
    if (this.record === "") {
      return true;
    }
    const kept = JSON.stringify(room());
    // The page going away sends its own request even for the words already
    // out, since the one out may die with the page; only its own outlives it.
    if (kept === this.last && (!leaving || this.out === null)) {
      return this.answer;
    }
    this.last = kept;
    this.seq += 1;
    const landed = this.send(this.record, { ...(JSON.parse(kept) as RoomState), seq: this.seq }, leaving).then(
      () => true,
      () => {
        // The next change keeps it again: a keep that could not be made is
        // words still on the screen.
        if (this.last === kept) {
          this.last = "";
        }
        return false;
      },
    );
    this.answer = landed;
    if (!leaving) {
      this.out = landed;
      void landed.then(() => {
        if (this.out === landed) {
          this.out = null;
        }
      });
    }
    return landed;
  }
}

/** What Step out says when the room's working state could not be kept (Sol SOL-A-01). */
export const KEEP_REFUSED = "Your unfinished words could not be kept; try Step out again in a moment.";

/** What one press of Step out comes to: say it stops the answer first, stay and say why, or leave. */
export type StepOut = { kind: "warn" } | { kind: "stay"; said: string } | { kind: "leave" };

/**
 * One press of Step out (D9, D16). During an answer the first press only says
 * that stepping out stops it; the next stops it and waits for the answer to
 * settle, and an answer that has not settled keeps the human in the room with
 * the service's words (Sol SOL-A-06). Then the room is kept, and a keep that did
 * not land keeps them in the room too, because leaving would lose the words
 * (Sol SOL-A-01). `stop` answers a refusal or ""; `keep` whether the keep landed.
 */
export async function steppingOut(
  answering: boolean,
  warned: boolean,
  stop: () => Promise<string>,
  keep: () => Promise<boolean>,
): Promise<StepOut> {
  if (answering && !warned) {
    return { kind: "warn" };
  }
  if (answering) {
    const refused = await stop();
    if (refused !== "") {
      return { kind: "stay", said: refused };
    }
  }
  return (await keep()) ? { kind: "leave" } : { kind: "stay", said: KEEP_REFUSED };
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

/** A shaping sitting's four walks (g1-s67 D3), the server's own parts in its order. */
export const SHAPING_WALKS = [
  { part: "records", label: "Records" },
  { part: "today", label: "Today" },
  { part: "cases", label: "Cases" },
  { part: "open", label: "Open" },
] as const;

/** The walks a room offers, by what its sitting is for. */
export function walksOf(purpose: string): readonly { part: string; label: string }[] {
  return purpose === "review" ? WALKS : SHAPING_WALKS;
}

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

/**
 * What one desk read is of: the item, at the commit the record says was
 * reviewed. After Review the new tip the same item is another read, so the desk
 * reads it again rather than showing the old tip under the new one's header
 * (Sol SOL-A-03). The strip's items keep `deskKey`, their identity.
 */
export function deskReadKey(item: DeskItem, reviewed: Reviewed): string {
  const at = reviewed.tip !== "" ? reviewed.tip : reviewed.landed.join(" ");
  return `${at}|${deskKey(item)}`;
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
