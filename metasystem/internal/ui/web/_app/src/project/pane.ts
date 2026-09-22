import type {
  Book,
  Chapter,
  DocumentFile,
  DocumentPayload,
  Goal,
  Pane,
  ProjectRecord,
  Question,
} from "./api";
import { documentPath, goalPath } from "../routes";

/**
 * What the briefing shows, and what the reader shows around one document,
 * worked out from what the server answered.
 *
 * The rules are here rather than in the components so that the ones worth
 * arguing about are readable and tested: what a book's own words are, which
 * records a goal shows, how designs group by status, which records are a
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
  /** The checkout-relative path, shown in mono and never wrapped. */
  path: string;
};

/**
 * One tab of the strip under the header: a section of this page, by the
 * anchor the page renders it under.
 */
export type PageSection = { id: string; title: string };

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

/** The designs: what governs, open; what is finished, in one run each. */
export type DesignRuns = { open: Row[]; runs: { status: string; rows: Row[] }[] };

/**
 * The block a goal page opens with: the goal's own id, its state and the
 * ledger's own reason for it. A goal the ledger does not carry is said to be
 * missing rather than invented.
 */
export type GoalBriefing = { id: string; title: string; state: string; intent: string; found: boolean; count: number };

/** The aside's first block: what in this scope is waiting for a human. */
export type NeedsYou = { questions: number; designs: number };

/** The aside's second block: the size and health of what was read. */
export type CheckoutFacts = { records: number; homes: number; goals: number; problems: number };

export type Briefing = {
  /** The goal this briefing is scoped to, or null for the whole project. */
  goal: GoalBriefing | null;
  /** The two books, opened in full. Empty under a goal: they are the project's. */
  books: BookBriefing[];
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
 * What the two sections no kind names are called. The tab and the block read
 * from the same word, so the anchor and the heading it lands on cannot drift
 * apart; the other sections take their word from kindTitle.
 */
export const QUESTIONS_TITLE = "Open questions";
export const DOCUMENTS_TITLE = "Documents";

/**
 * The strip under the header: this page's own sections, in the order the page
 * renders them, each naming the anchor it can be reached at.
 *
 * It is derived from the briefing rather than written twice, so a section the
 * payload does not produce — a book on a goal page, the checkout's documents
 * anywhere but the project — is absent from the outline for the same reason it
 * is absent from the page. The ledger's goals are not here at all: a goal is
 * the Backlog's, and the tabs of a page are what is on that page.
 */
export function pageSections(briefing: Briefing): PageSection[] {
  const rows: PageSection[] = [];
  if (briefing.goal !== null) {
    rows.push({ id: "goal", title: briefing.goal.title });
  }
  for (const book of briefing.books) {
    rows.push({ id: book.id, title: book.title });
  }
  rows.push({ id: "decisions", title: kindTitle("decision") });
  rows.push({ id: "designs", title: kindTitle("design") });
  rows.push({ id: "questions", title: QUESTIONS_TITLE });
  if (briefing.goal === null) {
    rows.push({ id: "documents", title: DOCUMENTS_TITLE });
  }
  return rows;
}

/** True when a selection shows this record. A null id is everything. */
function selected(goals: string[], id: string | null): boolean {
  return id === null || goals.includes(id);
}

/** What a goal reads by: its own title where the ledger has one, else its id. */
export function nameOf(goal: Goal): string {
  return goal.title === "" ? goal.id : goal.title;
}

/** The goal this id names, or null where the ledger does not carry it. */
export function goalWithID(pane: Pane, id: string): Goal | null {
  return pane.goals.find((candidate) => candidate.id === id) ?? null;
}

/**
 * The briefing: the whole project, or one goal of it.
 *
 * For the project the two books open with their own first paragraph and their
 * reading order. A goal page carries neither: the intent and the doctrine are
 * the project's, and repeating them under every goal would say nothing. What a
 * goal page carries is its own intent, from the ledger, and then the decisions,
 * designs and open questions whose Goals name it.
 */
export function briefingFor(pane: Pane, id: string | null): Briefing {
  const decisions = recordRows(pane, "decision", id);
  const designs = designRuns(pane, id);
  const questions = questionRows(pane, id);
  return {
    goal: id === null ? null : goalBriefing(pane, id),
    books: id === null ? BOOKS.map((book) => bookBriefing(pane, book.id, book.kind, book.title)) : [],
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
      goals: pane.goals.length,
      problems: pane.problems.length,
    },
  };
}

function goalBriefing(pane: Pane, id: string): GoalBriefing {
  const goal = goalWithID(pane, id);
  const count = pane.records.filter((record) => record.goals.includes(id)).length;
  if (goal === null) {
    return { id, title: id, state: "", intent: "", found: false, count };
  }
  return { id, title: nameOf(goal), state: goal.state, intent: goal.intent, found: true, count };
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
 * The designs, grouped by what they are for: what is being written and what
 * governs stays open; what shipped and what was replaced go into one collapsed
 * run each, so a briefing is what is live rather than what has accumulated.
 */
function designRuns(pane: Pane, goal: string | null): DesignRuns {
  const designs = recordRows(pane, "design", goal);
  return {
    open: designs.filter((row) => !FINISHED.includes(row.status)),
    runs: FINISHED.map((status) => ({ status, rows: designs.filter((row) => row.status === status) })).filter(
      (run) => run.rows.length > 0,
    ),
  };
}

function recordRows(pane: Pane, kind: string, goal: string | null): Row[] {
  return pane.records
    .filter((record) => record.kind === kind && selected(record.goals, goal))
    .map((record) => rowOf(record, record.path));
}

/** One row: what it is called, where it stands, and where it lives. */
function rowOf(record: ProjectRecord, key: string): Row {
  return {
    key,
    title: record.title,
    to: documentPath(record.path),
    summary: record.summary,
    status: record.status,
    path: record.path,
  };
}

/** A question reads as a row too: its text and where it stands. */
function questionRows(pane: Pane, goal: string | null): Row[] {
  return pane.questions
    .filter((question: Question) => selected(question.goals, goal))
    .map((question) => ({
      key: question.id === "" ? question.question : question.id,
      title: question.question,
      to: null,
      summary: "",
      status: question.status,
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

/** One goal a record is about, as a link to that goal's own page. */
export type About = { id: string; name: string; to: string; known: boolean };

/**
 * What a record says it is about, as links. A goal the ledger does not carry is
 * shown as the id it is and still opens its page, which says the ledger does
 * not have it — the check verb refuses that record in the same breath, and
 * hiding the name would hide the thing to act on.
 */
export function aboutOf(pane: Pane | null, goals: string[]): About[] {
  return goals.map((id) => {
    const goal = pane === null ? null : goalWithID(pane, id);
    return { id, name: goal === null ? id : nameOf(goal), to: goalPath(id), known: goal !== null };
  });
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
 * The trail above a document: the section, what kind it is, and what it is
 * called. A chapter bound into a book is named by its book instead, because
 * that is the only place it belongs.
 *
 * The goals a record is about are not a step of the trail: a record may be
 * about several, and the About line beneath the title names all of them as
 * links rather than one of them as a place this document lives.
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
  crumbs.push({ label: kindTitle(head.kind), to: null });
  return [...crumbs, { label: document.title, to: null }];
}

/**
 * The left rail: what this document is read among.
 *
 * A chapter of a book — bound or a record of its own — is read among the
 * book's chapters, in reading order, because that is the order it was written
 * to be read in. Every other record is read among the records of its own kind:
 * the designs among the designs, the decisions among the decisions. A document
 * that declares nothing has no siblings and no rail.
 *
 * A record that is the only one of its kind keeps its rail all the same,
 * naming itself and nothing else, with no previous and no next. The rail says
 * what this document is read among, and "the only decision" is an answer; a
 * rail that vanished would leave the reader with one region fewer and no way
 * to know why.
 */
export function railFor(pane: Pane, document: DocumentPayload): SiblingRail | null {
  const bound = bookOf(pane, document);
  const rail = bound === null ? kindRail(pane, document) : bookRail(pane, bound, document);
  if (rail === null) {
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
  const siblings = pane.records
    .filter((record) => record.kind === head.kind)
    .map((record) => siblingOf(record.path, record.title, record.status, document));
  return { title: kindTitle(head.kind), siblings, previous: null, next: null };
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
