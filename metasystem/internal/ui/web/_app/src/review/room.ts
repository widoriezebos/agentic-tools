import type { Deposit, Message, Subject } from "../partner/api";
import type { Store } from "../partner/conversation";
import {
  ACCEPTED, answerOf, EARLIER, entriesIn, FIX, followUp, NOT_A_PROBLEM, outcomeIn,
  type Card, type Entry, type Standing,
} from "../partner/sitting";
import { goalSentence } from "../goalTitle";
import type { Recorded, Verdict as RowVerdict } from "../backlog/api";
import type { Candidate } from "./candidate";
import type { Outcome as WriteOutcome } from "../partner/recording";
import type { About } from "../stickies/api";

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
 * puts on the desk. The whiteboard (g1-s71) adds two: a file of the review's
 * evidence by its evidence-relative path — "" is the listing — and a drawing,
 * carried whole, so a reload restores both as they were read.
 */
export type DeskItem =
  | { kind: "source"; path: string; from: number; to: number }
  | { kind: "changes"; since?: boolean }
  | { kind: "diff"; path: string; since?: boolean }
  | { kind: "section"; record: string; section: string }
  | { kind: "evidence"; record: string; path: string }
  | { kind: "drawing"; id: string; source: string; caption: string };

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
    case "evidence":
      return `evidence:${item.record}#${item.path}`;
    case "drawing":
      return `drawing:${item.id}`;
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
    case "evidence":
      return item.path === "" ? "the evidence" : baseName(item.path);
    case "drawing":
      return `drawing · ${shortCaption(item.caption)}`;
  }
}

function shortCaption(caption: string): string {
  const said = caption.replace(/\s+/gu, " ").trim();
  return said.length <= 40 ? said : `${said.slice(0, 40)}…`;
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
    case "evidence":
      return item.path === "" ? "the evidence" : `${item.path} in the evidence`;
    case "drawing":
      return `the drawing for "${item.caption.replace(/\s+/gu, " ").trim()}"`;
  }
}

const SECTION_ANCHOR = /^(\S+\.md) § (.+)$/u;
// A remark's fact or finding names the commit its lines were read at (g1-s71
// D1): "owner.go:41-46 at 9c1f0a2e9". The desk reads the lines as it reads now.
const LINE_ANCHOR = /^([\w./-]*\/?[\w.-]+\.[A-Za-z0-9]+):(\d+)(?:-(\d+))?(?: at [0-9a-f]{7,64})?$/u;

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

/** What the End sheet says above a Clear to land it refuses. */
export const CLEAR_REFUSED = "Clear to land waits until every finding is answered. These are not:";

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

/** Whether the sitting's Outcome card was recorded, which is how a sitting ends with its Outcome written. */
export function recordedOutcome(cards: readonly { kind: string; standing: Standing }[]): boolean {
  return cards.some((card) => card.kind === "outcome" && card.standing === "recorded");
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
export type ToPerform = {
  goal: string;
  fixes: number;
  /** The follow-up goals the verdict's own decisions opened, for the banner. */
  followUps?: string[];
  /**
   * The verdict as goal review is asked it, with the version and the saved
   * record the person decided on (RF-02), which the server compares with the
   * record it publishes.
   */
  asked: { record: string; verdict: string; brief: string; work: string; tip: string; revision: string };
};

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
  revision = "",
  followUps: readonly string[] = [],
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
    followUps: [...followUps],
    asked: {
      record, verdict, work: "",
      brief: verdict === "send-back" ? (brief ?? correctionBrief(entries, record, read.tip)) : "",
      tip: read.tip, revision,
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
export function strandedVerdict(
  source: string, record: string, standing: RowVerdict | undefined, brief: string | null, revision = "",
): ToPerform | null {
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
  return verdictToPerform({ kind: "outcome", verdict }, "review", record, source, brief, revision);
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

/** What the room says once the verdict is on the goal, in words (§3). */
export function recordedLine(
  recorded: Pick<Recorded, "verdict" | "tip" | "by">, fixes: number, goal: string, followUps: readonly string[] = [],
): string {
  if (recorded.verdict === "send-back") {
    return `Recorded on ${goal} as your verdict: send it back. The builder gets your ${fixes === 1 ? "must-fix decision" : `${String(fixes)} must-fix decisions`} ` +
      "as a correction, and the goal leaves Review until it comes back fixed.";
  }
  const opened = followUps.length === 0
    ? ""
    : ` ${String(followUps.length)} ${followUps.length === 1 ? "follow-up" : "follow-ups"} opened: ${followUps.join(", ")}.`;
  return `Recorded on ${goal} as your verdict: looks good, land it. The seat lands it on its next turn.${opened}`;
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
    return { ...none, state: "unread", words: "Reading whether this version runs on this computer…" };
  }
  if ("refusal" in read) {
    return read.code === "no-contract"
      ? { ...none, state: "no-contract", words: "This project does not say how to start its application, so no version of it can be started here." }
      : { ...none, state: "unread", words: read.refusal, run: true };
  }
  const address = read.address ?? "";
  const running = read.commit ?? "";
  const port = portOf(address);
  switch (read.state) {
    case "running":
      break;
    case "starting":
      return { state: "starting", words: `Starting this version${port === "" ? "" : ` on port ${port}`}…`, address, run: false, stop: true };
    default:
      return { ...none, state: "stopped", words: TRY_IT, run: true };
  }
  if (read.readiness !== "answering" && read.readiness !== "observed-at-startup") {
    return { state: "not-answering", words: `This version runs on port ${port} and is not answering yet.`, address, run: false, stop: true };
  }
  if (reviewed !== "" && running === reviewed) {
    return { state: "reviewed", words: `Running on port ${port}.`, address, run: false, stop: true };
  }
  return {
    state: "moved",
    words: `Running on port ${port}, but not the version you are reviewing: what runs is another version of this goal.`,
    address, run: false, stop: true,
  };
}

/** What Start does, said before it is pressed (RF-04): this exact version, here, until Stop. */
export const TRY_IT =
  "Start runs this exact version on this computer, on a port of its own, so you can click through it. It stays up until you press Stop.";

/** The port an address listens on, or the address where it names none. */
function portOf(address: string): string {
  return /:(\d+)\/?$/u.exec(address)?.[1] ?? address;
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
  /** What an unwritten remark is about (g1-s71 D1): its words ride as `text`. */
  about?: About;
};

export type Drafts = Readonly<Record<string, Draft>>;

/** What a remark being written is kept under in the room's drafts (g1-s71 D1). */
export const REMARK_DRAFT = "remark-";

/** The remarks being written among the drafts the mark carried back, each with what it is about. */
export function remarksIn(drafts: Drafts): Record<string, Draft> {
  const found: Record<string, Draft> = {};
  for (const [id, draft] of Object.entries(drafts)) {
    if (id.startsWith(REMARK_DRAFT) && draft.about !== undefined) {
      found[id] = draft;
    }
  }
  return found;
}

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

/* ------------------------------------------- a review you can decide -- */

/**
 * One finding as the guided review reads it (review-findings-read-as-decisions
 * §3): the plain layers a person decides it by, its evidence, and the answer
 * the record carries for it — "" while the record does not carry it.
 */
export type RoomFinding = {
  id: string;
  title: string;
  why: string;
  /** blocks, fix or note; "" for a finding recorded before the layers. */
  severity: string;
  /** must-fix, fix-later, not-a-problem or accept; "" where nothing was recommended. */
  recommend: string;
  /** Why the reviewer recommends what it recommends. */
  reason: string;
  /** The finding as the reviewer wrote it, with what it cites. */
  evidence: string;
  anchor: string;
  consequence: string;
  answer: string;
  recorded: boolean;
};

/** The four decisions a finding takes, as the record's Answer line names them. */
export type Decision = "fix" | "follow-up" | "not a problem" | "accepted";

/** Each severity's word, as the card's chip says it. */
export function severityWord(severity: string): string {
  switch (severity) {
    case "blocks":
      return "Blocks landing";
    case "fix":
      return "Worth fixing";
    case "note":
      return "Note";
    default:
      return "";
  }
}

/**
 * The four decisions, each with the consequence it has in this build, said on
 * the press before it is pressed. Every consequence is true of what step 1
 * records: an accepted risk is recorded on this review in the person's name,
 * and the goal's own acceptance is a separate step at a terminal (Wido's first
 * condition on the build).
 */
export const DECISIONS = [
  {
    decision: "fix", recommend: "must-fix", label: "Must fix before landing", words: "must fix before landing",
    consequence: "The builder gets this as a correction when you send the goal back; landing over it asks you first.",
  },
  {
    decision: "follow-up", recommend: "fix-later", label: "Fix after landing", words: "fix after landing",
    consequence: "A follow-up goal is opened with this finding. This goal may land.",
  },
  {
    decision: "not a problem", recommend: "not-a-problem", label: "Not a problem", words: "not a problem",
    consequence: "Say why. Your reason is recorded with the finding.",
  },
  {
    decision: "accepted", recommend: "accept", label: "I accept this risk", words: "accept the risk",
    consequence: "Say why. Your acceptance and your reason are recorded on this review, in your name.",
  },
] as const satisfies readonly { decision: Decision; recommend: string; label: string; words: string; consequence: string }[];

/** What accepting a blocking finding adds: the goal's own acceptance is a terminal's step. */
export const ACCEPT_ON_THE_GOAL =
  "Accepting it on the goal itself is a separate step, at a terminal: metasystem goal accept-risk.";

/** A decision's own entry of the four. */
export function decisionNamed(decision: Decision): (typeof DECISIONS)[number] {
  return DECISIONS.find((one) => one.decision === decision) ?? DECISIONS[0];
}

/** The decisions a finding offers by its severity: a note is fixed later or is not a problem. */
export function decisionsFor(severity: string): Decision[] {
  return severity === "note" ? ["follow-up", "not a problem"] : DECISIONS.map((one) => one.decision);
}

/** The decision a record's Answer line says, or "" for none. */
export function decisionOfAnswer(answer: string): Decision | "" {
  const said = answerOf(answer);
  return DECISIONS.some((one) => one.decision === said) ? (said as Decision) : "";
}

/** The decision the reviewer recommends, or "" for none. */
export function decisionOfRecommend(recommend: string): Decision | "" {
  return DECISIONS.find((one) => one.recommend === recommend.trim())?.decision ?? "";
}

/** The decision that stands: the person's, else the reviewer's. */
export function effectiveDecision(finding: RoomFinding): Decision | "" {
  return decisionOfAnswer(finding.answer) || decisionOfRecommend(finding.recommend);
}

function numbered(count: number, one: string, many: string): string {
  return `${String(count)} ${count === 1 ? one : many}`;
}

function oneOr(count: number): string {
  return count === 1 ? "one" : String(count);
}

function joinedAnd(parts: readonly string[]): string {
  return parts.length < 2 ? parts.join("") : `${parts.slice(0, -1).join(", ")}, and ${parts[parts.length - 1]}`;
}

/** The way the reviewer's own recommendations point: send it back where any of them is must fix. */
export function reviewerWay(findings: readonly RoomFinding[]): "send back" | "land" {
  return findings.some((one) => decisionOfRecommend(one.recommend) === "fix") ? "send back" : "land";
}

/**
 * The recommended way (RF-01, R-143-m1e): Send it back while any finding's
 * decision — the person's, or the reviewer's where the person decided nothing
 * — is must fix before landing, whatever its severity; otherwise land it.
 */
export function recommendedWay(findings: readonly RoomFinding[]): "send back" | "land" {
  return findings.some((one) => effectiveDecision(one) === "fix") ? "send back" : "land";
}

/** The summary line before any card: the counts by severity and what the reviewer recommends, and why. */
export function findingsSummary(findings: readonly RoomFinding[]): { counts: string; recommends: string } {
  const of = (severity: string) => findings.filter((one) => one.severity === severity).length;
  const parts: string[] = [];
  if (of("blocks") > 0) {
    parts.push(`${String(of("blocks"))} ${of("blocks") === 1 ? "blocks" : "block"} landing`);
  }
  if (of("fix") > 0) {
    parts.push(`${String(of("fix"))} ${of("fix") === 1 ? "is" : "are"} worth fixing`);
  }
  if (of("note") > 0) {
    parts.push(of("note") === 1 ? "1 is a note" : `${String(of("note"))} are notes`);
  }
  const counts = findings.length === 0 ? "" : `${numbered(findings.length, "finding", "findings")}.${parts.length === 0 ? "" : ` ${parts.join(", ")}.`}`;
  if (reviewerWay(findings) === "land") {
    return { counts, recommends: "land it." };
  }
  const blocking = findings.filter((one) => one.severity === "blocks" && decisionOfRecommend(one.recommend) === "fix").length;
  if (blocking > 0) {
    return { counts, recommends: `send it back, because of the ${blocking === 1 ? "one that blocks" : `${String(blocking)} that block`}.` };
  }
  const fixes = findings.filter((one) => decisionOfRecommend(one.recommend) === "fix").length;
  return {
    counts,
    recommends: `send it back, because ${fixes === 1 ? "one finding" : `${String(fixes)} findings`} must be fixed before landing.`,
  };
}

/** How far the reviewer's look at the version on screen got (RF-06). */
export type Examination = "reviewing" | "complete" | "stopped" | "failed";

/** The question that opens a review's examination, asked by the interface. */
const OPENING = "Open this review";

/**
 * How far the examination of this version got (RF-06; fix round 2, F-2): the
 * newest opening the interface asked — a review's first, or the one Review the
 * current version asks again — and every walk asked after it. Any still
 * running, or not answered yet, is still looking; else any that failed or was
 * refused failed, any stopped stopped; only when all ended done is it
 * complete. Zero findings is a clean review only then.
 */
export function examinationOf(messages: readonly Message[], liveTurn: string): Examination {
  const at = messages.map((one) => one.interface === true && one.text.startsWith(OPENING)).lastIndexOf(true);
  if (at < 0) {
    return "reviewing";
  }
  const asked = [messages[at], ...messages.slice(at + 1).filter((one) => one.interface === true && WALKED.test(one.text))];
  const outcomes = asked.map((question) => {
    if (liveTurn !== "" && liveTurn === question.turn) {
      return undefined;
    }
    return messages.find((one) => one.role === "partner" && one.turn === question.turn && one.outcome !== undefined)?.outcome;
  });
  if (outcomes.some((one) => one === undefined)) {
    return "reviewing";
  }
  if (outcomes.some((one) => one === "failed" || one === "refused")) {
    return "failed";
  }
  return outcomes.some((one) => one === "stopped") ? "stopped" : "complete";
}

/** A walk the interface asked in a review, by its fixed words. */
const WALKED = /^Walk me through \w+ for the review/u;

/** The opening turn of this version's examination, or "", which says which findings are about an older one. */
export function openingTurnOf(messages: readonly Message[]): string {
  return [...messages].reverse().find((one) => one.interface === true && one.text.startsWith(OPENING))?.turn ?? "";
}

/** What step 2 says where nothing was raised, by how far the look got (RF-06). */
export function nothingRaised(examination: Examination): { lead: string; body: string } {
  switch (examination) {
    case "complete":
      return {
        lead: "The reviewer found nothing to raise.",
        body: "It looked at what was asked, what was built, how it was examined, how it was proven, and how it behaves.",
      };
    case "stopped":
      return {
        lead: "The reviewer's look at this version was stopped before it finished.",
        body: "Nothing was raised before it stopped, so nothing here says the work is sound. Ask the reviewer to look again, or decide without its report.",
      };
    case "failed":
      return {
        lead: "The reviewer's look at this version did not finish.",
        body: "Nothing was raised before it ended, so nothing here says the work is sound. Ask the reviewer to look again, or decide without its report.",
      };
    default:
      return {
        lead: "The reviewer is still looking at this version.",
        body: "Nothing has been raised so far; a finding appears here as soon as it is raised.",
      };
  }
}

/** What one undecided finding will be recorded as, said before the nod records it. */
function followingSaid(finding: RoomFinding, decision: Decision | ""): string {
  switch (decision) {
    case "follow-up":
      return "fix after landing: a follow-up goal is opened for it";
    case "not a problem":
      return `not a problem, with the reviewer's reason: ${finding.reason}`;
    case "accepted":
      return `not decided: the reviewer's recommendation to accept the risk is kept (${finding.reason})`;
    case "fix":
      return "must fix before landing";
    default:
      return "recorded as not decided";
  }
}

/**
 * What one press of Looks good, land it does (§3, RF-01): the corrections
 * that stand — every finding whose decision is must fix — which the person
 * accepts with one reason after reading the impact; the undecided findings,
 * each with what will be recorded for it; and the follow-up goals to open. It
 * asks once where there is anything in either list, and is one press otherwise.
 */
export type NodPlan = {
  /** Every finding, in the order the room lists them. */
  all: RoomFinding[];
  corrections: RoomFinding[];
  /**
   * The undecided findings that need the person's own decision before landing
   * (fix round 4, R-143-m1e): one that blocks landing, or one the reviewer
   * recommends accepting. A nod records each as a risk accepted in the
   * person's name with their own reason, never from a recommendation alone.
   */
  risks: RoomFinding[];
  following: { finding: RoomFinding; decision: Decision | ""; said: string }[];
  followUps: RoomFinding[];
  impact: string;
  asks: boolean;
};

/** Whether an undecided finding needs the person's own decision before landing. */
function needsOwnDecision(finding: RoomFinding): boolean {
  return decisionOfAnswer(finding.answer) === "" && effectiveDecision(finding) !== "fix" &&
    (finding.severity === "blocks" || decisionOfRecommend(finding.recommend) === "accepted");
}

export function nodPlan(findings: readonly RoomFinding[]): NodPlan {
  const corrections = findings.filter((one) => effectiveDecision(one) === "fix");
  const risks = findings.filter(needsOwnDecision);
  const following = findings
    .filter((one) => decisionOfAnswer(one.answer) === "" && effectiveDecision(one) !== "fix" && !needsOwnDecision(one))
    .map((finding) => {
      const decision = effectiveDecision(finding);
      return { finding, decision, said: followingSaid(finding, decision) };
    });
  return {
    all: [...findings],
    corrections,
    risks,
    following,
    followUps: following.filter((one) => one.decision === "follow-up").map((one) => one.finding),
    impact: [correctionsImpact(corrections), risksImpact(risks)].filter((one) => one !== "").join(" "),
    asks: corrections.length > 0 || risks.length > 0 || following.length > 0,
  };
}

/**
 * What landing over undecided findings that need the person's own decision
 * means, said before the reason is asked (fix round 4, R-143-m1e): whose word
 * each is, that each is recorded as a risk the person accepts, what it lets
 * through, the risk it leaves, and how to undo it.
 */
export function risksImpact(risks: readonly RoomFinding[]): string {
  if (risks.length === 0) {
    return "";
  }
  const blocking = risks.filter((one) => one.severity === "blocks").length;
  const accepting = risks.length - blocking;
  const parts: string[] = [];
  if (blocking > 0) {
    parts.push(`${oneOr(blocking)} ${blocking === 1 ? "blocks" : "block"} landing in the reviewer's view`);
  }
  if (accepting > 0) {
    parts.push(`the reviewer recommends accepting ${oneOr(accepting)}`);
  }
  const many = risks.length > 1;
  return `${many ? `${String(risks.length)} findings you have not decided` : "One finding you have not decided"} would be ` +
    `recorded as ${many ? "risks" : "a risk"} you accept, in your name and with your reason: ${joinedAnd(parts)}. ` +
    `Landing lets through what ${many ? "they describe" : "it describes"}, and that risk stays with the goal after it lands. ${UNDO}`;
}

/**
 * The impact a nod over standing corrections has, said before the reason is
 * asked (RF-01, R-143-m1e): which findings must be fixed and whose word that is,
 * what landing now means for them, and how to keep them.
 */
export function correctionsImpact(corrections: readonly RoomFinding[]): string {
  if (corrections.length === 0) {
    return "";
  }
  const yours = corrections.filter((one) => decisionOfAnswer(one.answer) === "fix").length;
  const blocking = corrections.filter((one) => decisionOfAnswer(one.answer) === "" && one.severity === "blocks").length;
  const reviewers = corrections.length - yours - blocking;
  const parts: string[] = [];
  if (blocking > 0) {
    parts.push(`${oneOr(blocking)} ${blocking === 1 ? "blocks" : "block"} landing in the reviewer's view`);
  }
  if (reviewers > 0) {
    parts.push(`the reviewer recommends fixing ${oneOr(reviewers)} first`);
  }
  if (yours > 0) {
    parts.push(`you decided ${oneOr(yours)} must be fixed`);
  }
  const many = corrections.length > 1;
  return `${many ? `${String(corrections.length)} findings` : "One finding"} must be fixed before landing: ${joinedAnd(parts)}. ` +
    `Landing records ${many ? "each of these" : "it"} as a risk you accept, in your name and with your reason; the builder gets no correction. ` +
    `To keep ${many ? "them" : "it"}, press Send it back instead; to reverse it once recorded and before it lands, start a new review from the board and send it back.`;
}

/** What the Outcome says above the impact it carries, where the person landed over corrections (RF-01). */
export const ACCEPTED_LANDING =
  "Landed over findings that had to be fixed before landing or needed the person's own decision; each is recorded as a risk accepted in the person's name, with their reason. What was shown before landing:";

/** How to undo a verdict before the goal lands, said with every override (R-143-m1e). */
export const UNDO = "To undo it before it lands, start a new review from the board and send it back.";

/**
 * What a look that has not finished means for landing, said before the press
 * (fix round 1, R-143-m1e): the findings are what was raised so far, more may
 * come, and landing lets through what was not looked at.
 */
function unfinished(examination: Examination): string {
  return examination === "reviewing"
    ? "The reviewer has not finished looking at this version: what you see is what it has raised so far, and more may still come."
    : "The reviewer's look at this version did not finish: what you see is what it raised before it ended, and it may have raised more.";
}

/** The two ways' consequences, said on each before it is pressed. */
export function wayConsequence(way: "land" | "send back", findings: readonly RoomFinding[], examination: Examination): string {
  const plan = nodPlan(findings);
  const corrections = plan.corrections.length;
  if (way === "send back") {
    if (corrections === 0) {
      return "Say what must change first. Your words go to the builder as a correction, and the goal leaves Review until it comes back fixed." +
        (plan.following.length + plan.risks.length === 0 ? "" : " What you did not decide is recorded as the reviewer recommends.");
    }
    const opened = findings.filter((one) => decisionOfAnswer(one.answer) === "follow-up").length;
    return `The builder gets your must-fix ${corrections === 1 ? "decision" : "decisions"} as a correction. ` +
      "The goal leaves Review until it comes back fixed." +
      (opened === 0 ? "" : ` Your fix-after-landing ${opened === 1 ? "follow-up stays" : "follow-ups stay"} open.`) +
      (plan.following.length + plan.risks.length === 0 ? "" : " What you did not decide is recorded as the reviewer recommends.");
  }
  if (findings.length === 0) {
    if (examination !== "complete") {
      return `${examination === "reviewing" ? "The reviewer has not finished looking at this version" : "The reviewer's look at this version did not finish"}, ` +
        "so landing now lands it without the reviewer's report. The seat lands it on its next turn, and this is recorded on the goal in your name. " +
        UNDO;
    }
    return "The seat lands it on its next turn. Nothing else follows: no finding, no follow-up. This is recorded on the goal in your name.";
  }
  const lead = examination === "complete" ? "" : `${unfinished(examination)} Landing now lets through whatever it has not looked at yet. `;
  const tail = examination === "complete" ? "" : ` ${UNDO}`;
  if (corrections > 0) {
    return `${lead}The seat lands it on its next turn. ${corrections === 1 ? "One finding" : `${String(corrections)} findings`} ` +
      `must be fixed before landing: you will be asked to accept that risk and say why, in one step, before anything is recorded.${tail}`;
  }
  if (plan.risks.length > 0) {
    return `${lead}The seat lands it on its next turn. ${plan.risks.length === 1 ? "One finding needs" : `${String(plan.risks.length)} findings need`} ` +
      `your own decision before landing: you will be asked to accept that risk and say why, in one step, before anything is recorded.${tail}`;
  }
  const followUps = findings.filter((one) => effectiveDecision(one) === "follow-up").length;
  return `${lead}The seat lands it on its next turn.` +
    (followUps === 0 ? "" : ` ${followUps === 1 ? "One follow-up goal is" : `${String(followUps)} follow-up goals are`} opened from your fix-after-landing decisions.`) +
    ` This is recorded on the goal in your name.${tail}`;
}

/** What the Outcome says where the person landed before the reviewer's look finished (R-143-m1e). */
export const LANDED_UNFINISHED =
  "Landed before the reviewer finished looking at this version: the findings above are what it had raised by then.";

/**
 * The answers one verdict records in its one write (§3; fix round 1, F-2):
 * on a nod, every correction accepted with the person's reason; on a
 * send-back, every correction the person did not decide as must fix, as the
 * reviewer recommended; and on both, every other finding the person did not
 * decide as the reviewer recommends — a fix after landing naming the follow-up
 * goal opened for it, and nothing where none was opened (R-4). A decision the
 * person made stands as it is.
 */
export function verdictAnswers(
  way: "land" | "send back", plan: NodPlan, reason: string, opened: Readonly<Record<string, string>>,
): { id: string; answer: string }[] {
  const answers: { id: string; answer: string }[] = [];
  for (const one of plan.all) {
    const undecided = decisionOfAnswer(one.answer) === "";
    if (plan.corrections.includes(one)) {
      if (way === "land") {
        answers.push({ id: one.id, answer: ACCEPTED(reason) });
      } else if (undecided) {
        answers.push({ id: one.id, answer: followedAnswer(one, "fix") });
      }
      continue;
    }
    if (plan.risks.includes(one)) {
      const decision = effectiveDecision(one);
      if (way === "send back" && decision === "follow-up" && (opened[one.id] ?? "") === "") {
        continue;
      }
      answers.push({
        id: one.id,
        answer: way === "land" ? ACCEPTED(reason)
          : decision === "accepted" ? notDecidedAccept(one) : followedAnswer(one, decision, opened[one.id] ?? ""),
      });
      continue;
    }
    const following = plan.following.find((each) => each.finding === one);
    if (following === undefined) {
      continue;
    }
    const goal = opened[one.id] ?? "";
    if (following.decision === "follow-up" && goal === "") {
      continue;
    }
    answers.push({ id: one.id, answer: followedAnswer(one, following.decision, goal) });
  }
  return answers;
}

/**
 * The follow-up goals a verdict opens in turn before it is recorded: on a nod
 * the undecided findings that follow a fix after landing; on a send-back those
 * and the untouched risks whose recommendation is a fix after landing, which a
 * send-back records as recommended (fix round 4).
 */
export function followUpsFor(plan: NodPlan, way: "land" | "send back"): RoomFinding[] {
  return way === "land"
    ? plan.followUps
    : plan.all.filter((one) => plan.followUps.includes(one) || (plan.risks.includes(one) && effectiveDecision(one) === "follow-up"));
}

/** The undecided findings a verdict's ask lists with what will be recorded for each. */
export function askedFollowing(plan: NodPlan, way: "land" | "send back"): NodPlan["following"] {
  if (way === "land") {
    return plan.following;
  }
  return plan.all.flatMap((one) => {
    const listed = plan.following.find((each) => each.finding === one);
    if (listed !== undefined) {
      return [listed];
    }
    if (!plan.risks.includes(one)) {
      return [];
    }
    const decision = effectiveDecision(one);
    return [{ finding: one, decision, said: followingSaid(one, decision) }];
  });
}

/**
 * What a send-back records for an untouched finding the reviewer recommends
 * accepting: not an acceptance in the person's name, which only their own
 * reason makes (fix round 4, R-143-m1e), but the recommendation, kept as not
 * decided, so the next version's look still has it.
 */
function notDecidedAccept(finding: RoomFinding): string {
  return `not decided — the reviewer recommends accepting the risk: ${finding.reason}`;
}

/** End without a verdict, said as what it does (RF-07). */
export const END_WITHOUT_SAID =
  "the sitting ends and your hold is released; the goal still waits for a decision to land, and your decisions stay in the record.";

/** Leave for now, said as what it does. */
export const LEAVE_SAID = "everything stays as it is, and the goal waits for you.";

/** One finding's answer line as it is recorded when it follows the reviewer at the verdict. */
export function followedAnswer(finding: RoomFinding, decision: Decision | "", goal = ""): string {
  switch (decision) {
    case "fix":
      return `${FIX} (as the reviewer recommended)`;
    case "follow-up":
      return followUp(goal);
    case "not a problem":
      return `${NOT_A_PROBLEM(finding.reason)} (as the reviewer recommended)`;
    case "accepted":
      return `${ACCEPTED(finding.reason)} (as the reviewer recommended)`;
    default:
      return "not decided — the reviewer recommended nothing";
  }
}

/**
 * The Outcome's words after its verdict and Reviewed at lines, composed by
 * the interface from the record (§3): what was looked at, then every finding
 * with the decision the record carries for it.
 */
export function outcomeBody(findings: readonly RoomFinding[], examined: string): string {
  if (findings.length === 0) {
    return `${examined}\n\nFindings: none was raised.`;
  }
  const lines = findings.map((one) => {
    const severity = severityWord(one.severity);
    return `- ${severity === "" ? "" : `${severity}: `}${one.title} — ${one.answer === "" ? "not decided" : one.answer}`;
  });
  return `${examined}\n\nFindings:\n${lines.join("\n")}`;
}

/** What was looked at, for the Outcome: the reviewer's report, the walks asked, and what was opened. */
export function lookedAt(items: readonly DeskItem[], walks: readonly string[]): string {
  const named = ["the reviewer's report"];
  const asked = WALKS.filter((one) => walks.includes(one.part)).map((one) => WALK_WORDS[one.part]);
  if (asked.length > 0) {
    named.push(`its walk through ${joinedAnd(asked).replace(", and ", " and ")}`);
  }
  for (const item of [...items].reverse()) {
    const said = examinedAs(item);
    if (!named.includes(said)) {
      named.push(said);
    }
  }
  return `Examined: ${named.join("; ")}`;
}

/** The five walks, in words (§3): what was asked, what was built, and so on. */
export const WALK_WORDS: Readonly<Record<string, string>> = {
  asked: "what was asked",
  built: "what was built",
  examined: "how it was examined",
  proven: "how it was proven",
  behaves: "how it behaves",
};

/** The walks a review's transcript asked, by their parts. */
export function walksAsked(messages: readonly Message[]): string[] {
  const asked: string[] = [];
  for (const one of messages) {
    const walk = /^Walk me through (\w+) for the review/u.exec(one.interface === true ? one.text : "");
    const part = walk === null ? "" : WALKS.find((each) => each.label === walk[1])?.part ?? "";
    if (part !== "" && !asked.includes(part)) {
      asked.push(part);
    }
  }
  return asked;
}

/**
 * The correction brief a send-back carries (§3): each finding that must be
 * fixed, its title, why it matters, where it sits and the reviewer's evidence,
 * in the order the record carries them, and the person's own words where they
 * wrote what must change.
 */
export function briefOf(corrections: readonly RoomFinding[], record: string, tip: string): string {
  const lines = ["# Correction brief", "", `## What must change, from the review in ${record} of ${tip.slice(0, 9)}`, ""];
  corrections.forEach((one, index) => {
    lines.push(`${String(index + 1)}. ${one.title.trim()}${one.anchor.trim() === "" ? "" : ` (${one.anchor.trim()})`}`);
    if (one.why.trim() !== "") {
      lines.push(`   Why it matters: ${one.why.trim()}`);
    }
    if (one.evidence.trim() !== "") {
      lines.push(`   As the reviewer found it: ${one.evidence.replace(/\s+/gu, " ").trim()}`);
    }
  });
  return `${lines.join("\n")}\n`;
}

const MONTH_PARTS = ["day", "month", "hour", "minute"] as const;

/** A version's moment as a person reads it: "2 October, 14:19", in the browser's own time zone. */
export function versionWhen(at: string, timeZone?: string): string {
  const moment = new Date(at);
  if (at.trim() === "" || Number.isNaN(moment.getTime())) {
    return "";
  }
  const parts = new Intl.DateTimeFormat("en-GB", {
    day: "numeric", month: "long", hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone,
  }).formatToParts(moment);
  const [day, month, hour, minute] = MONTH_PARTS.map((part) => parts.find((one) => one.type === part)?.value ?? "");
  return `${day} ${month}, ${hour}:${minute}`;
}

/** A goal as the review's title names it: its own sentence, as its page heads it, else its id. */
export function goalHeading(goal: string, intent: string): string {
  const sentence = goalSentence(intent);
  return sentence === "" ? goal : sentence;
}

/**
 * The record moved on to the current version for a fresh look (RF-03): the
 * Reviewed line names the current commit, as Review the new tip did, and every
 * finding recorded about the version before moves to Earlier findings, with
 * its answer, so nothing carries forward by itself — it is read as earlier,
 * never decided again, and the engine counts the Findings section alone. Null
 * where the record names no branch tip to move.
 */
export function retippedAfresh(source: string, current: string): string | null {
  const moved = retipped(source, current);
  if (moved === null) {
    return null;
  }
  const lines = moved.split("\n");
  const findings = sectionSpan(lines, "Findings");
  if (findings === null) {
    return moved;
  }
  const raised = lines.slice(findings.at + 1, findings.end).filter((line) => line.trim() !== "");
  if (raised.length === 0) {
    return moved;
  }
  const emptied = [...lines.slice(0, findings.at + 1), "", ...lines.slice(findings.end)];
  const earlier = sectionSpan(emptied, EARLIER);
  if (earlier === null) {
    const after = findings.at + 2;
    return [...emptied.slice(0, after), `## ${EARLIER}`, "", ...raised, "", ...emptied.slice(after)].join("\n");
  }
  let last = earlier.end;
  while (last > earlier.at + 1 && emptied[last - 1].trim() === "") {
    last -= 1;
  }
  const opened = last === earlier.at + 1 ? [""] : [];
  return [...emptied.slice(0, last), ...opened, ...raised, "", ...emptied.slice(earlier.end)].join("\n");
}

/** Where one level-two section stands: its heading's line and the next heading's, or null. */
function sectionSpan(lines: readonly string[], section: string): { at: number; end: number } | null {
  const at = lines.findIndex((line) => line.trim() === `## ${section}`);
  if (at < 0) {
    return null;
  }
  let end = lines.length;
  for (let index = at + 1; index < lines.length; index += 1) {
    if (/^#{1,6}\s+/u.test(lines[index])) {
      end = index;
      break;
    }
  }
  return { at, end };
}

/** The turn a deposit card arrived in, from its id. */
function turnOfCard(id: string): string {
  return /^deposit:(.*)#\d+$/u.exec(id)?.[1] ?? "";
}

/**
 * The turns of a review's transcript before it last moved to the current
 * version (RF-03): what the reviewer raised in them was about an earlier
 * version. A look asked once more on the same version (fix round 3, F-2) is
 * not such a move, so what was raised before it stays this version's.
 */
export function earlierTurnsOf(messages: readonly Message[]): string[] {
  const at = messages.map((one) => one.interface === true && one.text.startsWith(MOVED_ON)).lastIndexOf(true);
  const before = at < 0 ? [] : messages.slice(0, at);
  return [...new Set(before.map((one) => one.turn))].filter((turn) => turn !== messages[at]?.turn);
}

/** The opening asked when the record moved to the current version (RF-03). */
const MOVED_ON = "Open this review again";

/** The walk part that asks a look that did not finish once more, on the same version (fix round 3, F-2). */
export const WALK_RESUME = "resume";

/**
 * The findings a room reads (§3): every finding the reviewer offered this
 * record, with the answer the record carries for it, then every finding the
 * record carries that no card offered — the person's own, or one written by
 * hand. Those raised about an earlier version are apart, as earlier (RF-03).
 * A card not offered, offered to another sitting, or made by the person and
 * not yet recorded is not here.
 */
export function roomFindingsOf(
  cards: readonly Card[], entries: readonly Entry[], earlierTurns: readonly string[],
): { current: RoomFinding[]; earlier: RoomFinding[] } {
  const current: RoomFinding[] = [];
  const earlier: RoomFinding[] = [];
  const seen = new Set<string>();
  const held = (section: string) => section === "Findings" || section === EARLIER;
  for (const card of cards) {
    if (card.kind !== "finding" || card.standing === "refused" || card.standing === "elsewhere") {
      continue;
    }
    const entry = entries.find((one) => one.mark === card.id && held(one.section));
    if (entry === undefined && card.id.startsWith("local-")) {
      continue;
    }
    seen.add(card.id);
    const titled = (card.title ?? "").trim() !== "";
    const finding: RoomFinding = {
      id: card.id,
      title: entry?.text ?? (titled ? card.title ?? "" : card.mark.text),
      why: entry?.why ?? card.why ?? "",
      severity: entry?.severity || card.severity || "",
      recommend: entry?.recommends || card.recommend || "",
      reason: entry?.reason ?? card.reason ?? "",
      evidence: titled ? card.text : "",
      anchor: (entry?.clause ?? "") !== "" ? entry?.clause ?? "" : card.anchor ?? "",
      consequence: entry?.consequence ?? card.consequence ?? "",
      answer: entry?.answer ?? "",
      recorded: entry !== undefined,
    };
    const before = entry === undefined ? earlierTurns.includes(turnOfCard(card.id)) : entry.section === EARLIER;
    once(before ? earlier : current, finding);
  }
  for (const entry of entries) {
    if (!held(entry.section) || (entry.mark !== "" && seen.has(entry.mark))) {
      continue;
    }
    once(entry.section === EARLIER ? earlier : current, findingOfEntry(entry));
  }
  return { current, earlier };
}

/**
 * Add one finding to a list once: a finding offered again — as a resumed
 * session's opening offers what it found before — is the one already listed
 * when it says the same thing about the same place: its title, its own words,
 * where it sits, how much it matters and what the reviewer recommends (fix
 * rounds 1 and 4). Two different findings under one title both stay, each
 * with its own decision. Where only the later one is recorded, it stands.
 */
function once(list: RoomFinding[], finding: RoomFinding): void {
  const at = list.findIndex((one) => one.id === finding.id || sameFinding(one, finding));
  if (at < 0) {
    list.push(finding);
    return;
  }
  if (!list[at].recorded && finding.recorded) {
    list[at] = finding;
  }
}

/** Whether two findings are one finding offered twice. */
function sameFinding(one: RoomFinding, other: RoomFinding): boolean {
  const plain = (said: string) => said.replace(/\s+/gu, " ").trim().toLowerCase();
  return [one.title, one.evidence, one.anchor, one.severity, one.recommend].map(plain).join("\u0000") ===
    [other.title, other.evidence, other.anchor, other.severity, other.recommend].map(plain).join("\u0000");
}

/** A finding as the record alone carries it: its title, its layers and its answer. */
export function findingOfEntry(entry: Entry): RoomFinding {
  return {
    id: entry.mark, title: entry.text, why: entry.why ?? "", severity: entry.severity ?? "",
    recommend: entry.recommends ?? "", reason: entry.reason ?? "", evidence: "", anchor: entry.clause,
    consequence: entry.consequence ?? "", answer: entry.answer ?? "", recorded: true,
  };
}

/**
 * What a verdict refused because the review changed under it says (RF-02):
 * nothing was recorded, what changed — another version, or the findings
 * another room added — and that the room now shows the review as it stands
 * for the person to decide again.
 */
export function changedSince(before: string, after: string): string {
  if (reviewedOf(before).tip !== reviewedOf(after).tip) {
    return "Your verdict was not recorded: this review now names another version than the one you decided on. It shows that version now; decide again.";
  }
  const known = new Set(entriesIn(before).map((one) => `${one.section}|${one.mark}|${one.text}`));
  const added = entriesIn(after)
    .filter((one) => (one.section === "Findings" || one.section === EARLIER) && !known.has(`${one.section}|${one.mark}|${one.text}`))
    .map((one) => one.text);
  return "Your verdict was not recorded: this review changed after you decided." +
    (added.length === 0 ? "" : ` Added since: ${added.join("; ").replace(/([^.!?])$/u, "$1.")}`) +
    " It shows the review as it stands now; decide again.";
}

/** The refusal code the verdict is answered with when the review changed under it (RF-02). */
export const VERSION_CHANGED = "version-changed";

/** The refusal code the verdict is answered with when a newer version of the goal exists (fix round 1, F-3). */
export const VERSION_MOVED = "version-moved";

/** What the room says when the verdict is refused for a version it was not about. */
export const VERDICT_ON_OLDER =
  "Your verdict was not recorded: a newer version of this goal exists, and a verdict counts only for the version " +
  "that would land. The page shows it now: press Review the current version, then decide.";

/**
 * What the room says when the verdict was refused because what it was about
 * moved: a newer version of the goal (F-3), or the review changed under it
 * (RF-02), said by what changed between the two readings.
 */
export function verdictRefused(code: string, before: string, after: string): string {
  return code === VERSION_MOVED ? VERDICT_ON_OLDER : changedSince(before, after);
}

/** The walk part that asks the opening again on the version the record names now (RF-03, RF-06). */
export const WALK_AGAIN = "again";

/** What the verdict's own write says when the review moved on while the person decided (RF-02). */
export const VERDICT_MOVED =
  "This review moved on to another version while you decided, so nothing was recorded. Look at it as it stands and decide again.";

/**
 * One verdict as the room gives it (§3): the words the Outcome opens with, the
 * answers the press records — each finding by its id, with its Answer line —
 * the person's own must-fix words where they wrote what must change, the
 * Outcome's body, the version the person decided on, the follow-up goals the
 * press opened, and a send-back's correction brief.
 */
export type VerdictAsked = {
  verdict: "clear to land" | "send back" | "no verdict";
  answers: readonly { id: string; answer: string }[];
  own: string;
  outcome: string;
  tip: string;
  followUps: readonly string[];
  brief: string;
};

/**
 * What one choice says on a finding's card (§3): what was recorded, where it is
 * the person's decision; what follows if nothing is decided, where it is the
 * reviewer's recommendation and says more than its consequence; and otherwise
 * its consequence, before it is pressed. Accepting a blocking finding says the
 * goal's own acceptance is a separate step at a terminal (Wido's condition a).
 */
export function choiceSaid(decision: Decision, finding: RoomFinding): string {
  const chosen = decisionOfAnswer(finding.answer);
  const detail = finding.answer.split(" — ").slice(1).join(" — ").trim();
  if (chosen === decision) {
    switch (decision) {
      case "follow-up":
        return `Follow-up opened: ${detail.replace(/^goal\s+/u, "")}.`;
      case "not a problem":
        return `Recorded: ${detail}`;
      case "accepted":
        return `Recorded in your name: ${detail}`;
      default:
        return "Recorded. The builder gets this as a correction when you send the goal back.";
    }
  }
  if (chosen === "" && decisionOfRecommend(finding.recommend) === decision && finding.reason.trim() !== "") {
    if (decision === "not a problem") {
      return `If you decide nothing, it is recorded with the reviewer's reason: ${finding.reason}`;
    }
  }
  const said = decisionNamed(decision).consequence;
  return decision === "accepted" && finding.severity === "blocks" ? `${said} ${ACCEPT_ON_THE_GOAL}` : said;
}

/**
 * What a finding's evidence opens (§3): the change, in the file the finding
 * sits in where its anchor names one, else the change as a whole; and the
 * record it cites, where its anchor names a record or a record's section.
 */
export function seeing(anchor: string): { change: DeskItem; cites: DeskItem | null } {
  const item = parseAnchor(anchor);
  if (item?.kind === "section") {
    return { change: { kind: "changes" }, cites: item };
  }
  if (item?.kind === "source") {
    return { change: { kind: "diff", path: item.path }, cites: null };
  }
  const path = anchor.trim().split(/\s+/u)[0] ?? "";
  if (/^[\w./-]+\.md$/u.test(path)) {
    return { change: { kind: "changes" }, cites: { kind: "source", path, from: 0, to: 0 } };
  }
  return { change: { kind: "changes" }, cites: null };
}

/**
 * The reviewer's own paragraph for step 1 (§3): what its opening answer says
 * before its first heading — the goal in one sentence, what this version adds,
 * how many files it touches — or "" where it has said none yet.
 */
export function leadParagraph(text: string): string {
  const lead: string[] = [];
  for (const line of text.split("\n")) {
    if (/^#{1,6}\s/u.test(line)) {
      break;
    }
    lead.push(line);
  }
  return lead.join("\n").trim();
}

/** Whether the nod's one press waits for the person's reason: it lands over a correction and none is written (RF-01). */
export function landingWaits(plan: NodPlan, reason: string): boolean {
  return plan.corrections.length + plan.risks.length > 0 && reason.trim() === "";
}

/**
 * What a decision's or a verdict's write says when it was not written (fix
 * round 2, N-2): where the record moved under it, that nothing was written and
 * the press there is now — the room has no Record it — and otherwise the
 * write's own words.
 */
export function writeRefused(outcome: WriteOutcome, what: "verdict" | "decision"): string {
  if (outcome.kind === "recorded") {
    return "";
  }
  if (outcome.kind !== "conflict") {
    return outcome.reason;
  }
  return what === "verdict"
    ? "The review changed while your verdict was being written, so nothing was written. Give your verdict again: it is given on the review as it now stands."
    : "The review changed while your decision was being written, so nothing was written. Press your decision again: it is recorded on the review as it now stands.";
}
