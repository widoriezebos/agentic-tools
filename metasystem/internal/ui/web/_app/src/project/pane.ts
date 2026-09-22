import type { Book, Chapter, DocumentFile, Pane, ProjectRecord, Question } from "./api";
import { areaPath, documentPath } from "../routes";

/**
 * What the pane shows, worked out from what the server answered.
 *
 * The rules are here rather than in the component so that the ones worth
 * arguing about are readable and tested: which records an area shows, what a
 * chapter resolves to, and how the rest of the checkout's Markdown is grouped.
 * Nothing here fetches, and nothing here invents a row the payload did not
 * carry.
 */

/** One line of a section: a title that may open, and the facts beside it. */
export type Row = {
  key: string;
  title: string;
  /** Where the title opens, or null for a row this build cannot open. */
  to: string | null;
  status: string;
  areas: string[];
  /** The checkout-relative path, shown in mono and never wrapped. */
  path: string;
};

export type Section = { id: string; title: string; rows: Row[] };

/** One row of the left column: everything, or one declared area. */
export type AreaRow = { slug: string | null; name: string; to: string; count: number };

/** One collapsed run of the checkout's other documents. */
export type DocumentGroup = { id: string; title: string; files: DocumentFile[] };

/**
 * The left column: the whole project first, then each declared area in the
 * order the intent index declares it, each with the number of records that
 * name it. A record naming two areas is counted under both, because it is in
 * both.
 */
export function areaRows(pane: Pane): AreaRow[] {
  const rows: AreaRow[] = [
    { slug: null, name: "Project", to: "/project", count: pane.records.length },
  ];
  for (const area of pane.areas) {
    rows.push({
      slug: area.slug,
      name: area.name === "" ? area.slug : area.name,
      to: areaPath(area.slug),
      count: pane.records.filter((record) => record.areas.includes(area.slug)).length,
    });
  }
  return rows;
}

/** True when a selection shows this record. A null slug is everything. */
function selected(areas: string[], slug: string | null): boolean {
  return slug === null || areas.includes(slug);
}

/**
 * The five sections of the selection, in the order the design gives them.
 * Documents are not among them: they declare no kind, so they are listed apart
 * and belong to no area.
 */
export function sectionsFor(pane: Pane, slug: string | null): Section[] {
  return [
    { id: "intent", title: "Intent", rows: bookRows(pane, pane.intent, "intent", slug) },
    { id: "doctrine", title: "Doctrine", rows: bookRows(pane, pane.doctrine, "doctrine", slug) },
    { id: "decisions", title: "Decisions", rows: recordRows(pane, "decision", slug) },
    { id: "designs", title: "Designs", rows: recordRows(pane, "design", slug) },
    { id: "questions", title: "Open questions", rows: questionRows(pane, slug) },
  ];
}

/**
 * A book: its index, its chapters in reading order, and then any record of the
 * kind the index does not name — which is a record nobody would otherwise see.
 *
 * Under an area, a chapter that names a record is shown when that record names
 * the area; a chapter that binds a document has no areas of its own, so it
 * belongs to the book and is shown when the index is.
 */
function bookRows(pane: Pane, book: Book, kind: string, slug: string | null): Row[] {
  const rows: Row[] = [];
  const index = book.index;
  // The book belongs to the areas its index names; with no index at all it
  // belongs to the whole project and to no area.
  const shown = index === null ? slug === null : selected(index.areas, slug);
  if (index !== null && shown) {
    rows.push(rowOf(index, "index"));
  }
  const named = new Set<string>(index === null ? [] : [index.id]);
  for (const [position, chapter] of book.chapters.entries()) {
    const row = chapterRow(pane, chapter, position, shown, slug);
    if (chapter.id !== undefined) {
      named.add(chapter.id);
    }
    if (row !== null) {
      rows.push(row);
    }
  }
  for (const record of pane.records) {
    if (record.kind === kind && !named.has(record.id) && selected(record.areas, slug)) {
      rows.push(rowOf(record, record.path));
    }
  }
  return rows;
}

/**
 * One chapter, as the book named it. A chapter naming a record carries that
 * record's facts; a chapter binding a document carries its path and nothing
 * else, because a document claims no kind. A chapter naming a record no head
 * declares is shown as the index wrote it, unopenable, which is what the check
 * verb refuses in the same breath.
 */
function chapterRow(
  pane: Pane,
  chapter: Chapter,
  position: number,
  shown: boolean,
  slug: string | null,
): Row | null {
  const key = `chapter-${String(position)}`;
  if (chapter.path !== undefined && chapter.path !== "") {
    return shown
      ? { key, title: chapter.title, to: documentPath(chapter.path), status: "", areas: [], path: chapter.path }
      : null;
  }
  const record = pane.records.find((candidate) => candidate.id === chapter.id);
  if (record === undefined) {
    return shown ? { key, title: chapter.title, to: null, status: "", areas: [], path: "" } : null;
  }
  if (!selected(record.areas, slug)) {
    return null;
  }
  return { ...rowOf(record, key), title: chapter.title === "" ? record.title : chapter.title };
}

function recordRows(pane: Pane, kind: string, slug: string | null): Row[] {
  return pane.records
    .filter((record) => record.kind === kind && selected(record.areas, slug))
    .map((record) => rowOf(record, record.path));
}

function rowOf(record: ProjectRecord, key: string): Row {
  return {
    key,
    title: record.title,
    to: documentPath(record.path),
    status: record.status,
    areas: record.areas,
    path: record.path,
  };
}

/** A question reads as a row too: its text, where it stands, and its areas. */
function questionRows(pane: Pane, slug: string | null): Row[] {
  return pane.questions
    .filter((question: Question) => selected(question.areas, slug))
    .map((question) => ({
      key: question.id === "" ? question.question : question.id,
      title: question.question,
      to: null,
      status: question.status,
      areas: question.areas,
      path: "",
    }));
}

/**
 * The checkout's other Markdown, in collapsed runs.
 *
 * A run is the first two segments of a file's directory, or the one segment it
 * has, or the checkout root for a file that sits in it: enough to tell
 * metasystem/docs from metasystem/plans without putting every subdirectory of
 * every home on the screen. The order is the payload's, which is path order.
 */
export const ROOT_GROUP = "The checkout root";

export function documentGroups(files: DocumentFile[]): DocumentGroup[] {
  const groups: DocumentGroup[] = [];
  const byID = new Map<string, DocumentGroup>();
  for (const file of files) {
    const id = groupOf(file.path);
    let group = byID.get(id);
    if (group === undefined) {
      group = { id, title: id === "" ? ROOT_GROUP : id, files: [] };
      byID.set(id, group);
      groups.push(group);
    }
    group.files.push(file);
  }
  return groups;
}

function groupOf(path: string): string {
  const segments = path.split("/");
  return segments.slice(0, Math.min(2, segments.length - 1)).join("/");
}
