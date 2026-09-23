import type { Page } from "./api";
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
  /** The line the drawer showed, which is what the human read. */
  label: string;
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

  // The subject: the chosen one where there is one, and the page's otherwise.
  // A chosen subject replaces the page's rather than standing beside it,
  // because "this" means one thing at a time.
  if (from.chosen !== null) {
    const chosen = from.chosen;
    if (chosen.kind === "passage") {
      page.quote = chosen.quote ?? "";
      put(page, "quoteFrom", chosen.id);
      put(page, "quoteRevision", chosen.revision);
      put(page, "quoteAnchor", chosen.anchor);
    } else {
      page.kind = subjectKind(chosen.kind);
      page.subject = chosen.id;
      put(page, "title", chosen.title);
      put(page, "revision", chosen.revision);
    }
  } else {
    put(page, "kind", page_.kind);
    put(page, "subject", page_.subject);
    put(page, "title", page_.title);
    put(page, "revision", page_.revision);
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
 * with — which is what puts "Refresh what it sees" on the chip.
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
  return parts.join(" · ");
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
