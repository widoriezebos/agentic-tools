import type { CapturedSticky, Page } from "./api";
import type { SheetDraft } from "./drafting";
import type { Chosen } from "./subject";
import { activeSection } from "../routes";
import type { Subject } from "../shell/about";

/**
 * One capture per question.
 *
 * Astra's first finding, and the whole of contract 1: the sheet that shows
 * what the Partner will be given, the question that carries it, the message
 * that keeps it, and the chip that takes a human back to it weeks later are
 * one composition, made once, at the moment Send is pressed. Recomposing any
 * of them from the page a moment later is how a preview comes to certify a
 * page nobody was looking at.
 *
 * So a capture is a value: the reading the page rendered from, the view, the
 * filters, the Done window, the tab, the chosen subject or passage, the ids
 * the page names, and the address it returns to. Nothing in it is looked up
 * afterwards.
 */

/** What a capture is made of, from the page and from whatever was chosen. */
export type Capturing = {
  pathname: string;
  /** What the pane on screen says it is about. */
  page: Subject;
  /** The subject a human chose, or null while the subject follows the page. */
  chosen: Chosen | null;
  /**
   * A passage a human selected, or null. It stands beside the subject rather
   * than in place of it: they are two attachments, made by two acts, and the
   * document a passage was quoted from is still what the page is about.
   */
  passage?: Chosen | null;
  /** The line the drawer showed, which is what the human read. */
  label: string;
  /** The sheet open over the work area, by its own name, or "". */
  sheet?: string;
  /** A sheet a human offered with "Ask about this", or null. */
  draft?: SheetDraft | null;
  /**
   * The human's own notepad as the page was showing it, and how many of it is
   * open. Astra's F2: a note about no subject reaches the Partner only through
   * the panel, so what travels is what the panel shows while it is open and
   * what the page shows otherwise — which the notepad's own owner decides, not
   * this file.
   */
  notepad?: { stickies: CapturedSticky[]; open: number };
};

/** Compose one capture. */
export function captureOf(from: Capturing): Page {
  const section = activeSection(from.pathname);
  const page: Page = { section: section?.title ?? "", path: from.pathname };
  const page_ = from.page;
  put(page, "tab", page_.tab);
  put(page, "view", page_.view);
  put(page, "tip", page_.tip);
  put(page, "observedAt", page_.observedAt);
  put(page, "window", page_.window);
  put(page, "label", from.label);
  // Where the human is standing, which is now a sheet as often as a page. The
  // name travels and nothing else does: a sheet half filled in is the human's
  // until they hand it over, and handing it over is the draft below.
  put(page, "sheet", from.sheet);
  if (from.draft !== undefined && from.draft !== null) {
    page.draft = from.draft;
  }

  // The subject: the chosen one where there is one, and the page's otherwise.
  // A chosen subject replaces the page's rather than standing beside it,
  // because "this" means one thing at a time.
  const chosen = from.chosen;
  if (chosen !== null) {
    page.kind = subjectKind(chosen.kind);
    page.subject = chosen.id;
    put(page, "title", chosen.title);
    put(page, "revision", chosen.revision);
  } else {
    put(page, "kind", page_.kind);
    put(page, "subject", page_.subject);
    put(page, "title", page_.title);
    put(page, "revision", page_.revision);
  }

  // A passage is its own attachment, so it travels beside whatever the
  // question is about rather than in place of it: what was quoted, and where
  // it was read, are two facts and the Partner is given both.
  const passage = from.passage ?? null;
  if (passage !== null) {
    page.quote = passage.quote ?? "";
    put(page, "quoteFrom", passage.id);
    put(page, "quoteRevision", passage.revision);
    put(page, "quoteAnchor", passage.anchor);
  }

  if (page_.filters !== undefined && page_.filters.length > 0) {
    page.filters = page_.filters;
  }
  // The board's own rows. They travel because they are nowhere else: the goals
  // are in the accepted ledger commit, not in the checkout's files.
  if (page_.lanes !== undefined && page_.lanes.length > 0) {
    page.lanes = page_.lanes;
  }
  if (page_.records !== undefined && page_.records.length > 0) {
    page.records = page_.records;
  }
  // The fleet's own rows, for the reason the board's are: the standings were
  // judged as this page was composed, and a reading taken afterwards would be
  // a different answer to the question that was asked.
  if (page_.fleet !== undefined) {
    page.fleet = page_.fleet;
  }
  // The notepad, last: it is what the human wrote to themselves rather than
  // what the project says, and the count travels even when no sticky does so
  // that "none of them is about this page" is an answer rather than a silence.
  if (from.notepad !== undefined && (from.notepad.open > 0 || from.notepad.stickies.length > 0)) {
    if (from.notepad.stickies.length > 0) {
      page.stickies = from.notepad.stickies;
    }
    page.stickiesOpen = from.notepad.open;
  }

  put(page, "return", returnTo(from));
  return page;
}

/**
 * The address this capture takes a human back to.
 *
 * The page composes it, because the page owns its own address grammar: the
 * board knows how to say "board, tier 1, Done reaching back seven days, and
 * show this goal" as a query, and the reader knows how to say "this heading".
 *
 * It is the page's address and not the subject's own page. The question was
 * asked from the board with a card chosen on it, so "these" means the board it
 * was asked from; the subject's own page is one click further on, from the
 * pinned panel.
 */
function returnTo(from: Capturing): string {
  const at = from.chosen?.at ?? "";
  if (at !== "") {
    return at;
  }
  return from.page.returnTo ?? from.pathname;
}

/**
 * The server's word for a kind. It knows two — a goal and a document — and
 * everything else is carried as a document, which is what a record, a lane and
 * an Overview item resolve to when the server looks them up.
 */
function subjectKind(kind: Chosen["kind"]): string {
  switch (kind) {
    case "goal":
      return "goal";
    case "record":
    case "document":
      return "document";
    default:
      return kind;
  }
}

/** Only what the page actually said travels; an empty field says nothing. */
function put<K extends keyof Page>(page: Page, key: K, value: string | undefined): void {
  if (value !== undefined && value !== "") {
    (page[key] as string) = value;
  }
}

/**
 * Whether the page has moved on from the capture the last question was sent
 * with — which is what puts "page moved on · Show it the latest" on the chip.
 *
 * It is the reading and nothing else: a filter a human changed is a new
 * capture the next question will make anyway, and offering to refresh for it
 * would be offering to do what sending already does. A tip or an observation
 * that has moved is different: the page is now showing a different ledger than
 * the last question was asked about, and nothing says so.
 */
export function readingMoved(last: Page | null, page: Subject): boolean {
  if (last === null) {
    return false;
  }
  const tip = page.tip ?? "";
  const observed = page.observedAt ?? "";
  if (tip === "" && observed === "") {
    return false;
  }
  return (last.tip ?? "") !== tip || (last.observedAt ?? "") !== observed;
}

/** The one line the "Seeing:" chip shows, from the capture it will send. */
export function seeingLine(capture: Page): string {
  const parts: string[] = [];
  if (capture.section !== "") {
    parts.push(capture.section);
  }
  if (capture.view !== undefined && capture.view !== "") {
    parts.push(capture.view);
  }
  if (capture.tab !== undefined && capture.tab !== "") {
    parts.push(capture.tab);
  }
  const rows = shownRows(capture);
  if (rows > 0) {
    parts.push(`${String(rows)} goals shown`);
  }
  if (capture.records !== undefined && capture.records.length > 0) {
    parts.push(`${String(capture.records.length)} records shown`);
  }
  if (capture.filters !== undefined && capture.filters.length > 0) {
    parts.push(`filters ${capture.filters.join(", ")}`);
  }
  if (capture.subject !== undefined && capture.subject !== "") {
    parts.push(capture.subject);
  }
  if (capture.quote !== undefined && capture.quote !== "") {
    parts.push("a selected passage");
  }
  const reading = readingOf(capture);
  if (reading !== "") {
    parts.push(reading);
  }
  // Last, because it is the newest thing to be true and the one a human is
  // looking at while they read this line.
  if (capture.sheet !== undefined && capture.sheet !== "") {
    parts.push(`${capture.sheet} sheet open`);
  }
  return parts.join(" · ");
}

/**
 * What a question asked over an open sheet says about itself, in the header
 * row of the message it became.
 *
 * It is a note on the asking and not part of the address the chip beside it
 * returns to: going back to the board a question was asked from must not
 * reopen a form the human has long since closed or sent.
 */
export function sheetNote(capture: Page): string {
  const sheet = capture.sheet ?? "";
  return sheet === "" ? "" : `asked with the ${sheet} sheet open`;
}

/** How many goals the captured board was showing, across its lanes. */
export function shownRows(capture: Page): number {
  return (capture.lanes ?? []).reduce((total, lane) => total + lane.total, 0);
}

/** The reading, short: the tip and the time of day it was observed at. */
function readingOf(capture: Page): string {
  const tip = capture.tip ?? "";
  if (tip === "") {
    return "";
  }
  const at = clockOf(capture.observedAt ?? "");
  return at === "" ? `tip ${shortTip(tip)}` : `tip ${shortTip(tip)}, ${at}`;
}

/** The time of day an instant was observed at, in this browser's own clock. */
export function clockOf(at: string): string {
  if (at === "") {
    return "";
  }
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) {
    return "";
  }
  return when.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

/* ----------------------------------------------------- a stamp, shortened -- */

/**
 * What an answer was given, read back into its parts.
 *
 * The server writes it as one sentence — "the accepted tip 6984cde…, observed
 * 2026-09-23T17:54:00Z", with the page's own reading after a semicolon where
 * the two differed. An answer's meta line has room for the short tip and the
 * clock and for nothing else; the sentence itself stays whole in the Looked
 * list's own entry and in the Seeing sheet, so shortening it here loses
 * nothing.
 *
 * A source that is no tip at all — a document read as it stands — carries no
 * hash and no instant, and travels through as it was written.
 */
export type Stamp = { tip: string; at: string; what: string };

const TIP = "the accepted tip ";
const OBSERVED = ", observed ";

export function stampOf(source: string): Stamp {
  const head = source.split(";")[0].trim();
  if (!head.startsWith(TIP)) {
    return { tip: "", at: "", what: head };
  }
  const rest = head.slice(TIP.length);
  const at = rest.indexOf(OBSERVED);
  if (at < 0) {
    return { tip: rest.trim(), at: "", what: "" };
  }
  return { tip: rest.slice(0, at).trim(), at: rest.slice(at + OBSERVED.length).trim(), what: "" };
}

/** How much of a reading a human says out loud: seven characters. */
export const TIP_LENGTH = 7;

export function shortTip(tip: string): string {
  return tip.slice(0, TIP_LENGTH);
}

/**
 * When something happened, in this browser's own clock: the time of day, with
 * the date before it only where it was not today.
 *
 * Almost everything a conversation stamps happened minutes ago, and a date on
 * every line of it is a date nobody reads; a date on the one line that needs
 * one is read.
 */
export function whenOf(at: string, now: Date = new Date()): string {
  const clock = clockOf(at);
  if (clock === "") {
    return "";
  }
  const when = new Date(at);
  const today =
    when.getFullYear() === now.getFullYear() &&
    when.getMonth() === now.getMonth() &&
    when.getDate() === now.getDate();
  if (today) {
    return clock;
  }
  return `${when.toLocaleDateString(undefined, { day: "numeric", month: "short" })}, ${clock}`;
}

/**
 * The first half of an answer's meta line: what it was given, short enough to
 * stand beside what it looked at.
 */
export function sawLine(source: string, now: Date = new Date()): string {
  const stamp = stampOf(source);
  if (stamp.tip === "") {
    return stamp.what === "" ? "" : `Saw ${stamp.what}`;
  }
  const said = `Saw tip ${shortTip(stamp.tip)}`;
  const when = whenOf(stamp.at, now);
  return when === "" ? said : `${said} · ${when}`;
}

/**
 * What a message's own chip says: the page, the view, the filters and the
 * subject it was asked from, in the order a human reads them.
 */
export function chipOf(capture: Page): string {
  const parts: string[] = [];
  if (capture.section !== "") {
    parts.push(capture.section);
  }
  if (capture.view !== undefined && capture.view !== "") {
    parts.push(capture.view);
  }
  if (capture.tab !== undefined && capture.tab !== "") {
    parts.push(capture.tab);
  }
  for (const filter of capture.filters ?? []) {
    // The Done window is on every board, narrowed or not, so it says nothing
    // about this one; the chip carries what made this board different. The
    // window is in the sheet, and in the address the chip returns to.
    if (filter.startsWith("Done reaches back")) {
      continue;
    }
    parts.push(filter);
  }
  if (capture.subject !== undefined && capture.subject !== "") {
    parts.push(capture.subject);
  }
  if (capture.quote !== undefined && capture.quote !== "") {
    parts.push("a passage");
  }
  return parts.join(" · ");
}
