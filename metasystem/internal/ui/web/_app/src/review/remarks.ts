import type { Source } from "./api";
import { parseAnchor } from "./room";
import type { About, Notepad, Sticky } from "../stickies/api";

/**
 * Remarks (g1-s71 D1): stickies with two subjects of their own — lines of a
 * file at the commit the desk read them at, and a record's section — made from
 * a selection on the desk or from the board, private as every sticky, and shown
 * on the board under Remarks and on the desk item they belong to.
 *
 * The one rule this file exists for is where a remark is drawn. A desk item
 * carries no commit of its own, so a remark matches the lines the desk shows by
 * path, range and commit. In a review the commit is the reviewed tip the desk
 * read at; after Review the new tip the desk reads other code, and the remark
 * leaves the lines and stays on the board naming its commit. In a shaping room
 * the desk reads the checkout as it stands, and an uncommitted edit changes the
 * lines under the same head, so the head is provenance only and the remark is
 * drawn only while the lines the desk shows are byte-identical to the ones it
 * was made on.
 */

/** One remark: the sticky, and the one of its subjects on this record. */
export type Remark = { sticky: Sticky; about: About };

/** What the board says of a shaping remark whose lines now read otherwise. */
export const EARLIER_READING = "at an earlier reading";

/** A remark on lines a desk read shows: at the read's commit, or, of the checkout, its head and its lines' text. */
export function remarkAbout(record: string, read: Source, from: number, to: number): About {
  const about: About = { kind: "source", id: record, record, path: read.path, from, to, commit: "" };
  if (read.checkout === true) {
    about.commit = read.head ?? "";
    about.lines = linesOf(read, from, to) ?? "";
    return about;
  }
  about.commit = read.commit;
  return about;
}

/** A remark on a record's section. */
export function sectionRemark(record: string, section: string): About {
  return { kind: "section", id: record, record, section };
}

/** The text of lines from..to as a read shows them, or null where it does not show them whole. */
function linesOf(read: Source, from: number, to: number): string | null {
  if (from < read.from || to > read.to) {
    return null;
  }
  const shown = read.lines.filter((line) => line.number >= from && line.number <= to);
  return shown.length === to - from + 1 ? shown.map((line) => line.text).join("\n") : null;
}

/** This record's open remarks, newest first as the notepad answers: one per subject on it. */
export function remarksOn(notepad: Notepad, record: string): Remark[] {
  const found: Remark[] = [];
  for (const sticky of notepad.stickies) {
    if (sticky.doneAt !== "") {
      continue;
    }
    for (const about of sticky.about) {
      if ((about.kind === "source" || about.kind === "section") && about.record === record) {
        found.push({ sticky, about });
      }
    }
  }
  return found;
}

/** Whether a remark on lines is drawn as a marker on the lines this read shows. */
export function remarkDrawn(about: About, read: Source, purpose: string): boolean {
  if (about.kind !== "source" || about.path !== read.path) {
    return false;
  }
  const from = about.from ?? 0;
  const to = about.to ?? from;
  if (purpose === "review" || read.checkout !== true) {
    return (about.commit ?? "") !== "" && about.commit === read.commit && to >= read.from && from <= read.to;
  }
  if ((about.commit ?? "") !== "" && (read.head ?? "") !== "" && about.commit !== read.head) {
    return false;
  }
  const now = linesOf(read, from, to);
  return now !== null && now === (about.lines ?? "");
}

/** What the room reads a remark against: its purpose, the reviewed tip, and the desk's last source read. */
export type Reading = { purpose: string; tip: string; read: Source | null };

/**
 * Where the board says a remark is: "on lines 41-46 of owner.go" while the desk
 * reads it where it was made, "on lines 41-46 at 9c1f0a2" once it reads another
 * commit, and in a shaping room "on lines 41-46 at an earlier reading" once a
 * read of those lines shows other bytes.
 */
export function remarkWhere(about: About, reading: Reading): string {
  if (about.kind === "section") {
    return `on § ${about.section ?? ""}`;
  }
  const lines = linesWords(about);
  const commit = about.commit ?? "";
  const here = `${lines} of ${baseName(about.path ?? "")}`;
  if (reading.purpose === "review") {
    return commit !== "" && commit !== reading.tip ? `${lines} at ${commit.slice(0, 7)}` : here;
  }
  const read = reading.read;
  if (read === null || read.checkout !== true) {
    return here;
  }
  if (commit !== "" && (read.head ?? "") !== "" && read.head !== commit) {
    return `${lines} at ${commit.slice(0, 7)}`;
  }
  if (read.path === about.path && linesOf(read, about.from ?? 0, about.to ?? 0) !== null && !remarkDrawn(about, read, reading.purpose)) {
    return `${lines} ${EARLIER_READING}`;
  }
  return here;
}

function linesWords(about: About): string {
  const from = about.from ?? 0;
  const to = about.to ?? from;
  return to > from ? `on lines ${String(from)}-${String(to)}` : `on line ${String(from)}`;
}

function baseName(path: string): string {
  return path.split("/").at(-1) ?? path;
}

/**
 * The anchor a fact or a finding made of a remark carries: the file and lines
 * with the remark's commit, which parseAnchor reads back as the lines, or the
 * record and its section.
 */
export function remarkAnchor(about: About): string {
  if (about.kind === "section") {
    return `${about.record ?? ""} § ${about.section ?? ""}`;
  }
  const from = about.from ?? 0;
  const to = about.to ?? from;
  const lines = to > from ? `${String(from)}-${String(to)}` : String(from);
  const commit = about.commit ?? "";
  return `${about.path ?? ""}:${lines}${commit === "" ? "" : ` at ${commit.slice(0, 9)}`}`;
}

/**
 * The subject of a remark pressed on a desk selection or a board pile, from the
 * anchor the selection or the pile names: lines of the file the desk last read,
 * or a record's section. Lines the desk did not read are refused (null): a
 * remark keeps the commit and the text it was made on, and only a read has them.
 */
export function remarkOf(anchor: string, record: string, read: Source | null): About | null {
  const item = parseAnchor(anchor);
  if (item === null) {
    return null;
  }
  if (item.kind === "section") {
    return sectionRemark(item.record, item.section);
  }
  if (item.kind !== "source" || read === null || read.path !== item.path || item.from < read.from || item.to > read.to) {
    return null;
  }
  return remarkAbout(record, read, item.from, item.to);
}

/** The cards a remark is made into: a fact, and in a review a finding (g1-s67: findings stay review-only). */
export function remarkPresses(purpose: string): { label: string; kind: string }[] {
  const fact = { label: "Record as a fact", kind: "fact" };
  return purpose === "review" ? [fact, { label: "Make a finding", kind: "finding" }] : [fact];
}

/** The card a press opens: the remark's words, and its anchor with its commit. */
export function cardOf(remark: Remark, kind: string): { anchor: string; kind: string; text: string } {
  return { anchor: remarkAnchor(remark.about), kind, text: remark.sticky.text };
}
