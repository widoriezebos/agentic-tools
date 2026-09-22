import type {
  Book,
  Chapter,
  DocumentFile,
  DocumentPayload,
  Pane,
  ProjectRecord,
  Question,
} from "./api";
import { areaPath, documentPath } from "../routes";

/**
 * What the briefing shows, and what the reader shows around one document,
 * worked out from what the server answered.
 *
 * The rules are here rather than in the components so that the ones worth
 * arguing about are readable and tested: what a book's own words are, which
 * records an area shows, how designs group by status, which records are a
 * record's siblings, and where previous and next go. Nothing here fetches, and
 * nothing here invents a row the payload did not carry.
 */

/** One line of a section: a title that may open, and the facts beside it. */
export type Row = {
  key: string;
  title: string;
  /** Where the title opens, or null for a row this build cannot open. */
  to: string | null;
  /** The record's own first words, or "" where it has none. */
  summary: string;
  status: string;
  areas: string[];
  /** The checkout-relative path, shown in mono and never wrapped. */
  path: string;
};

/** One row of the left column: everything, or one declared area. */
export type AreaRow = { slug: string | null; name: string; to: string; count: number };

/** One collapsed run of the checkout's other documents. */
export type DocumentGroup = { id: string; title: string; files: DocumentFile[] };

/** One entry of a book's table of contents, titled as the index wrote it. */
export type TocEntry = { key: string; title: string; to: string | null; summary: string };

/** A book as the briefing opens it: in its own words, then its reading order. */
export type BookBriefing = {
  id: string;
  title: string;
  /** Where the whole book is read, or null where the home has no index. */
  to: string | null;
  status: string;
  /** The index's first paragraph, which is what the book says it is. */
  lede: string;
  chapters: TocEntry[];
};

/** A book under one area: the chapters that name it, and nothing else. */
export type AreaBook = { id: string; title: string; rows: Row[]; none: string };

/** The designs: what governs, open; what is finished, in one run each. */
export type DesignRuns = { open: Row[]; runs: { status: string; rows: Row[] }[] };

/** The block an area page opens with: what this area is, in the index's word. */
export type AreaBriefing = { slug: string; name: string; declared: boolean; count: number };

/** The aside's first block: what in this scope is waiting for a human. */
export type NeedsYou = { questions: number; designs: number };

/** The aside's second block: the size and health of what was read. */
export type CheckoutFacts = { records: number; homes: number; areas: number; problems: number };

export type Briefing = {
  /** The area this briefing is scoped to, or null for the whole project. */
  area: AreaBriefing | null;
  /** The two books, opened in full. Empty under an area. */
  books: BookBriefing[];
  /** The two books scoped to one area. Empty for the whole project. */
  areaBooks: AreaBook[];
  decisions: Row[];
  designs: DesignRuns;
  questions: Row[];
  needsYou: NeedsYou;
  checkout: CheckoutFacts;
};

/** The two books, in reading order, with the words the briefing calls them. */
const BOOKS: readonly { id: string; kind: string; title: string; word: string }[] = [
  { id: "intent", kind: "intent", title: "Intent", word: "intent" },
  { id: "doctrine", kind: "doctrine", title: "Doctrine", word: "doctrine" },
];

/** The statuses a design is finished in, each collapsed into its own run. */
export const FINISHED = ["done", "superseded"];

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
      name: nameOf(pane, area.slug),
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

/** The name the intent index gave a slug, or the slug where it gave none. */
export function nameOf(pane: Pane, slug: string): string {
  const area = pane.areas.find((candidate) => candidate.slug === slug);
  return area === undefined || area.name === "" ? slug : area.name;
}

/**
 * The briefing: the whole project, or one area of it.
 *
 * For the project the two books open with their own first paragraph and their
 * reading order. For an area they do not open at all — a project-wide book
 * repeated under every area says nothing — and instead each lists only the
 * chapters that name the area, or says in one line that there are none and
 * that the project-wide book applies.
 */
export function briefingFor(pane: Pane, slug: string | null): Briefing {
  const decisions = recordRows(pane, "decision", slug);
  const designs = designRuns(pane, slug);
  const questions = questionRows(pane, slug);
  return {
    area: slug === null ? null : areaBriefing(pane, slug),
    books: slug === null ? BOOKS.map((book) => bookBriefing(pane, book.id, book.kind, book.title)) : [],
    areaBooks: slug === null ? [] : BOOKS.map((book) => areaBook(pane, book, slug)),
    decisions,
    designs,
    questions,
    needsYou: {
      questions: questions.filter((row) => row.status === "open").length,
      // An accepted design is one whose work has not shipped: shipping it is
      // what makes it done.
      designs: designs.open.filter((row) => row.status === "accepted").length,
    },
    checkout: {
      records: pane.records.length,
      homes: new Set(pane.records.map((record) => record.home)).size,
      areas: pane.areas.length,
      problems: pane.problems.length,
    },
  };
}

function areaBriefing(pane: Pane, slug: string): AreaBriefing {
  return {
    slug,
    name: nameOf(pane, slug),
    declared: pane.areas.some((area) => area.slug === slug),
    count: pane.records.filter((record) => record.areas.includes(slug)).length,
  };
}

/**
 * One book, opened: what it says it is, and every chapter it names, followed
 * by the records of its kind the index does not name — which are records
 * nobody would otherwise see.
 */
function bookBriefing(pane: Pane, id: string, kind: string, title: string): BookBriefing {
  const book = id === "intent" ? pane.intent : pane.doctrine;
  const index = book.index;
  const chapters: TocEntry[] = [];
  const named = new Set<string>(index === null ? [] : [index.id]);
  for (const [position, chapter] of book.chapters.entries()) {
    if (chapter.id !== undefined) {
      named.add(chapter.id);
    }
    chapters.push(tocEntry(pane, chapter, position));
  }
  for (const record of pane.records) {
    if (record.kind === kind && !named.has(record.id)) {
      chapters.push({ key: record.path, title: record.title, to: documentPath(record.path), summary: record.summary });
    }
  }
  return {
    id,
    title,
    to: index === null ? null : documentPath(index.path),
    status: index === null ? "" : index.status,
    lede: index === null ? "" : index.summary,
    chapters,
  };
}

/**
 * One chapter, as the book named it. A chapter that binds a document opens
 * that document; a chapter that names a record opens the record's own reading
 * view, which is how a chapter of the intent that is itself a record is read.
 * A chapter naming a record no head declares opens nothing, which is what the
 * check verb refuses in the same breath.
 */
function tocEntry(pane: Pane, chapter: Chapter, position: number): TocEntry {
  const key = `chapter-${String(position)}`;
  if (chapter.path !== undefined && chapter.path !== "") {
    return { key, title: chapter.title, to: documentPath(chapter.path), summary: chapter.summary };
  }
  const record = recordWithID(pane, chapter.id);
  if (record === null) {
    return { key, title: chapter.title, to: null, summary: chapter.summary };
  }
  return {
    key,
    title: chapter.title === "" ? record.title : chapter.title,
    to: documentPath(record.path),
    summary: chapter.summary === "" ? record.summary : chapter.summary,
  };
}

/**
 * One book under one area: the chapters of it that are records naming the
 * area, in reading order, and then the records of its kind the index does not
 * name. A bound document declares no area, so it belongs to the book rather
 * than to any part of it and is not repeated here.
 */
function areaBook(pane: Pane, book: { id: string; kind: string; title: string; word: string }, slug: string): AreaBook {
  const source: Book = book.id === "intent" ? pane.intent : pane.doctrine;
  const rows: Row[] = [];
  const named = new Set<string>();
  const index = source.index;
  if (index !== null) {
    named.add(index.id);
    if (selected(index.areas, slug)) {
      rows.push(rowOf(index, "index", slug));
    }
  }
  for (const [position, chapter] of source.chapters.entries()) {
    if (chapter.id === undefined) {
      continue;
    }
    named.add(chapter.id);
    const record = recordWithID(pane, chapter.id);
    if (record === null || !selected(record.areas, slug)) {
      continue;
    }
    const row = rowOf(record, `chapter-${String(position)}`, slug);
    rows.push(chapter.title === "" ? row : { ...row, title: chapter.title });
  }
  for (const record of pane.records) {
    if (record.kind === book.kind && !named.has(record.id) && selected(record.areas, slug)) {
      rows.push(rowOf(record, record.path, slug));
    }
  }
  return {
    id: book.id,
    title: `${book.title} for this area`,
    rows,
    none: `No ${book.word} chapter for ${nameOf(pane, slug)} yet; the project-wide ${book.word} applies.`,
  };
}

/**
 * The designs, grouped by what they are for: what is being written and what
 * governs stays open; what shipped and what was replaced go into one collapsed
 * run each, so a briefing is what is live rather than what has accumulated.
 */
function designRuns(pane: Pane, slug: string | null): DesignRuns {
  const designs = recordRows(pane, "design", slug);
  return {
    open: designs.filter((row) => !FINISHED.includes(row.status)),
    runs: FINISHED.map((status) => ({ status, rows: designs.filter((row) => row.status === status) })).filter(
      (run) => run.rows.length > 0,
    ),
  };
}

function recordRows(pane: Pane, kind: string, slug: string | null): Row[] {
  return pane.records
    .filter((record) => record.kind === kind && selected(record.areas, slug))
    .map((record) => rowOf(record, record.path, slug));
}

/**
 * One row. Inside an area page the area chips are left off: every row there is
 * in that area, and a chip that says so on every line says nothing.
 */
function rowOf(record: ProjectRecord, key: string, slug: string | null): Row {
  return {
    key,
    title: record.title,
    to: documentPath(record.path),
    summary: record.summary,
    status: record.status,
    areas: slug === null ? record.areas : [],
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
      summary: "",
      status: question.status,
      areas: slug === null ? question.areas : [],
      path: "",
    }));
}

function recordWithID(pane: Pane, id: string | undefined): ProjectRecord | null {
  if (id === undefined || id === "") {
    return null;
  }
  return pane.records.find((record) => record.id === id) ?? null;
}

/* ------------------------------------------------------------- the reader -- */

/** One step of the trail above a document. The last step opens nothing. */
export type Crumb = { label: string; to: string | null };

/** One neighbour in the left rail: where it opens, and where it stands. */
export type Sibling = { key: string; title: string; to: string; note: string; current: boolean };

/** The left rail: what this document sits among, and how to walk it. */
export type SiblingRail = {
  title: string;
  siblings: Sibling[];
  previous: Sibling | null;
  next: Sibling | null;
};

/** The word each kind reads by where a section of the briefing names it. */
const KIND_TITLES: Readonly<Record<string, string>> = {
  intent: "Intent",
  doctrine: "Doctrine",
  decision: "Decisions",
  design: "Designs",
};

export function kindTitle(kind: string): string {
  return KIND_TITLES[kind] ?? "Documents";
}

/** What a record names of the areas the intent index actually declares. */
function declaredAreas(pane: Pane, areas: string[]): string[] {
  return areas.filter((area) => pane.areas.some((declared) => declared.slug === area));
}

/** The book whose reading order names this document, or null. */
function bookOf(pane: Pane, document: DocumentPayload): { id: string; title: string; book: Book } | null {
  for (const { id, title } of BOOKS) {
    const book = id === "intent" ? pane.intent : pane.doctrine;
    const named = book.chapters.some(
      (chapter) =>
        (chapter.path !== undefined && chapter.path === document.id) ||
        (chapter.id !== undefined && document.record !== null && chapter.id === document.record.id),
    );
    if (named || (book.index !== null && book.index.path === document.id)) {
      return { id, title, book };
    }
  }
  return null;
}

/**
 * The trail above a document: the section, where it belongs, what kind it is,
 * and what it is called. A chapter bound into a book is named by its book
 * instead, because that is the only place it belongs.
 */
export function crumbsFor(pane: Pane | null, document: DocumentPayload): Crumb[] {
  const crumbs: Crumb[] = [{ label: "Project", to: "/project" }];
  if (pane === null) {
    return [...crumbs, { label: document.title, to: null }];
  }
  const head = document.record;
  if (head === null) {
    const bound = bookOf(pane, document);
    if (bound !== null) {
      crumbs.push({ label: bound.title, to: bound.book.index === null ? null : documentPath(bound.book.index.path) });
      return [...crumbs, { label: document.title, to: null }];
    }
    return [...crumbs, { label: "Documents", to: null }, { label: document.title, to: null }];
  }
  const areas = declaredAreas(pane, head.areas);
  crumbs.push(
    areas.length === 0
      ? { label: "Everything", to: "/project" }
      : { label: nameOf(pane, areas[0]), to: areaPath(areas[0]) },
  );
  crumbs.push({ label: kindTitle(head.kind), to: null });
  return [...crumbs, { label: document.title, to: null }];
}

/**
 * The left rail: what this document is read among.
 *
 * A chapter of a book — bound or a record of its own — is read among the
 * book's chapters, in reading order, because that is the order it was written
 * to be read in. Every other record is read among the records of its kind in
 * its own areas, or, where it names no declared area, among every record of
 * its kind. A document that declares nothing has no siblings and no rail.
 */
export function railFor(pane: Pane, document: DocumentPayload): SiblingRail | null {
  const bound = bookOf(pane, document);
  const rail = bound === null ? kindRail(pane, document) : bookRail(pane, bound, document);
  if (rail === null || rail.siblings.length < 2) {
    return null;
  }
  const at = rail.siblings.findIndex((sibling) => sibling.current);
  return {
    ...rail,
    previous: at > 0 ? rail.siblings[at - 1] : null,
    next: at >= 0 && at + 1 < rail.siblings.length ? rail.siblings[at + 1] : null,
  };
}

function bookRail(
  pane: Pane,
  bound: { id: string; title: string; book: Book },
  document: DocumentPayload,
): SiblingRail | null {
  const siblings: Sibling[] = [];
  const index = bound.book.index;
  if (index !== null) {
    siblings.push(siblingOf(index.path, index.title, index.status, document));
  }
  for (const [position, chapter] of bound.book.chapters.entries()) {
    if (chapter.path !== undefined && chapter.path !== "") {
      siblings.push(siblingOf(chapter.path, chapter.title, "", document, `chapter-${String(position)}`));
      continue;
    }
    const record = recordWithID(pane, chapter.id);
    if (record === null) {
      continue;
    }
    const title = chapter.title === "" ? record.title : chapter.title;
    siblings.push(siblingOf(record.path, title, record.status, document, `chapter-${String(position)}`));
  }
  return { title: bound.title, siblings, previous: null, next: null };
}

function kindRail(pane: Pane, document: DocumentPayload): SiblingRail | null {
  const head = document.record;
  if (head === null) {
    return null;
  }
  const areas = declaredAreas(pane, head.areas);
  const siblings = pane.records
    .filter(
      (record) =>
        record.kind === head.kind && (areas.length === 0 || record.areas.some((area) => areas.includes(area))),
    )
    .map((record) => siblingOf(record.path, record.title, record.status, document));
  const where = areas.length === 0 ? "Everything" : areas.map((area) => nameOf(pane, area)).join(", ");
  return { title: `${kindTitle(head.kind)} · ${where}`, siblings, previous: null, next: null };
}

function siblingOf(path: string, title: string, note: string, document: DocumentPayload, key?: string): Sibling {
  return { key: key ?? path, title, to: documentPath(path), note, current: path === document.id };
}

/**
 * The id as a reader needs it: its ends, which is what tells two ULIDs apart,
 * with the whole of it kept for the title attribute. An id short enough to
 * read is left alone.
 */
export const ID_HEAD = 6;
export const ID_TAIL = 4;

export function shortID(id: string): string {
  return id.length <= ID_HEAD + ID_TAIL + 1 ? id : `${id.slice(0, ID_HEAD)}…${id.slice(-ID_TAIL)}`;
}

/* ---------------------------------------------------------- the documents -- */

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
