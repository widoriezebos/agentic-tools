/**
 * The project's resources, and the only place this section talks to the
 * server.
 *
 * The two reads are made once per page view and again on Reload, and nowhere
 * else: no stream, no socket, no timer, and no window event refetches either
 * of them. The five writes are made when a human confirms one, and never on
 * their own; the save and the render below are made when a human asks for
 * them, in an editor they opened. All nine go through the one request below,
 * so there is a single place where this build reaches the network;
 * src/cuts.test.ts holds that by counting call sites, so a second request
 * written under any other name fails the guard rather than the review.
 */

const PANE = "/api/project";
const DOCUMENTS = "/api/documents/";
const RECORDS = "/api/project/records";
const QUESTIONS = "/api/project/questions";
/** The suffix both status routes share; the id is the segment before it. */
const STATUS = "/status";
/** Naming one more ledger goal on a record; the id is the segment before it. */
const GOALS = "/goals";
/** Saving one document: the read route's own id, with this after it. */
const EDIT = "/edit";
/** Rendering what is being typed. It names no document, and writes nothing. */
const PREVIEW = "/api/documents/preview";

export type DocumentState = "readable" | "unreadable" | "too-large";

/**
 * One goal of the ledger: the project's one subdivision, named by its ledger
 * id, with where it stands and why it is open. The title is the goal file's own
 * heading, which in this ledger is the id itself.
 */
export type Goal = {
  id: string;
  title: string;
  state: string;
  intent: string;
  /**
   * When slicing started on this goal and which seat started it, where the
   * record carries the boundary. Absent rather than zeroed: "slicing has not
   * started" and "slicing started at the zero instant" are not one statement.
   */
  sliced?: { at: string; machine: string; lineage: string };
};

/**
 * One declared record: what it says it is, where it lives, and its own first
 * words. The summary is a convention rather than a key, so a record that has
 * none carries the empty string.
 */
export type ProjectRecord = {
  kind: string;
  id: string;
  status: string;
  /** The ledger goals this record is about; empty is the project as a whole. */
  goals: string[];
  title: string;
  path: string;
  home: string;
  summary: string;
  /**
   * The list items this record writes under a Slices heading, as written.
   * Nothing is taken from them: there is no slice-plan owner in the engine
   * yet, so a design's own list is the whole of what is recorded.
   */
  slices: string[];
};

/**
 * One line of a book's reading order: a record named by id, or a document
 * bound by its checkout-relative path. Exactly one of the two is carried.
 */
export type Chapter = { id?: string; path?: string; title: string; summary: string };

/** A book: its index, which is a record of the kind, and its reading order. */
export type Book = { index: ProjectRecord | null; chapters: Chapter[] };

export type Question = { id: string; opened: string; question: string; goals: string[]; status: string };

/** One refusal the check verb would print, anchored where a human can act. */
export type Problem = { path: string; line: number; message: string };

/** One of the checkout's other documents, by path, with no kind claimed. */
export type DocumentFile = { path: string; title: string };

export type Pane = {
  schemaVersion: number;
  readAt: string;
  goals: Goal[];
  records: ProjectRecord[];
  intent: Book;
  doctrine: Book;
  questions: Question[];
  problems: Problem[];
  documents: DocumentFile[];
};

export type Heading = { level: number; id: string; text: string };

/** An inline node. The type names which of the rest carries anything. */
export type Inline = {
  type: string;
  text?: string;
  href?: string;
  target?: string;
  id?: string;
  src?: string;
  alt?: string;
  inlines?: Inline[];
};

export type Cell = Inline[];

export type Item = { checked: boolean | null; blocks: Block[] };

/** A block node, as the engine parsed it. No field of it is ever HTML. */
export type Block = {
  type: string;
  level?: number;
  id?: string;
  lang?: string;
  text?: string;
  ordered?: boolean;
  start?: number;
  align?: string[];
  head?: Cell[];
  rows?: Cell[][];
  inlines?: Inline[];
  blocks?: Block[];
  items?: Item[];
};

/**
 * What a record declares about itself, as data rather than as the bullet list
 * at the top of its text. A document that declares no head carries none.
 */
export type RecordHead = {
  kind: string;
  id: string;
  status: string;
  goals: string[];
  cites: string[];
  affects: string[];
  governs: string[];
  supersedes: string[];
  by: string[];
};

/** One record on the other end of a relationship: enough to show and to open. */
export type Link = { id: string; title: string; path: string; kind: string };

export type DocumentPayload = {
  kind: string;
  id: string;
  title: string;
  revision: string;
  /**
   * The file as it was read, byte for byte. It is what the editor opens, and
   * the revision above is what a save of it has to carry back. Only a
   * readable document carries it: there is no text to edit in a file that was
   * too large to read to its end, or that is not text at all.
   */
  source: string;
  owner: string;
  path: string;
  bytes: number;
  modifiedAt: string;
  readAt: string;
  state: DocumentState;
  reason: string;
  record: RecordHead | null;
  referencedBy: Link[];
  supersededBy: Link[];
  headings: Heading[];
  blocks: Block[];
};

/**
 * A response that was not what was asked for.
 *
 * A refusal the server explained carries its own sentence, because that
 * sentence is what a human acts on; a response that explained nothing carries
 * the resource and the status, which is all there is to say. Problems are the
 * check verb's own refusals, where a write was refused over what it would have
 * written.
 */
export class ResourceError extends Error {
  readonly status: number;
  readonly problems: Problem[];

  constructor(resource: string, status: number, reason = "", problems: Problem[] = []) {
    super(reason === "" ? `${resource} answered ${String(status)}` : reason);
    this.name = "ResourceError";
    this.status = status;
    this.problems = problems;
  }
}

/** A document this checkout does not serve, which has its own card. */
export function isNotFound(error: unknown): boolean {
  return error instanceof ResourceError && error.status === 404;
}

/** What a refusal's body carries, where the server sent one. */
type Refusal = { error?: string; problems?: Problem[] };

/**
 * The one request. A body makes it a write, and a write is a POST of JSON;
 * everything else is a read. A refusal is read for its own words before it is
 * thrown, and a refusal that carries none is thrown with the status it had.
 */
async function request<T>(resource: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const sending = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: sending ? "POST" : "GET",
    headers: sending
      ? { Accept: "application/json", "Content-Type": "application/json" }
      : { Accept: "application/json" },
    body: sending ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const refusal = await reasonOf(response);
    throw new ResourceError(resource, response.status, refusal.error ?? "", refusal.problems ?? []);
  }
  return (await response.json()) as T;
}

/** A body that is not the refusal shape says nothing, which is not an error. */
async function reasonOf(response: Response): Promise<Refusal> {
  try {
    return (await response.json()) as Refusal;
  } catch {
    return {};
  }
}

export async function loadPane(signal?: AbortSignal): Promise<Pane> {
  return request<Pane>(PANE, undefined, signal);
}

export async function loadDocument(id: string, signal?: AbortSignal): Promise<DocumentPayload> {
  return request<DocumentPayload>(DOCUMENTS + encodeSegments(id), undefined, signal);
}

/* -------------------------------------------------------- the five writes -- */

/**
 * What a human asked the interface to create. Everything else about the
 * record — its id, its status, its file name, its home, its body — is the
 * server's to decide, and none of it is in this object: there is no path here
 * because no path travels.
 */
export type NewRecord = { kind: string; title: string; goals: string[]; affects?: string[]; cites?: string[] };

export type NewQuestion = { question: string; goals: string[] };

/** What a write produced, re-read through the resolver before it answered. */
export type Written = { record: ProjectRecord; path: string; absolute: string };

export type Asked = { question: Question };

export async function createRecord(asked: NewRecord): Promise<Written> {
  return request<Written>(RECORDS, asked);
}

export async function setRecordStatus(id: string, status: string): Promise<Written> {
  return request<Written>(`${RECORDS}/${encodeURIComponent(id)}${STATUS}`, { status });
}

export async function askQuestion(asked: NewQuestion): Promise<Asked> {
  return request<Asked>(QUESTIONS, asked);
}

export async function setQuestionStatus(id: string, status: string): Promise<Asked> {
  return request<Asked>(`${QUESTIONS}/${encodeURIComponent(id)}${STATUS}`, { status });
}

/**
 * Name one more ledger goal on a record's head. The goal is one the machine
 * just minted, never one a human typed, and the answer is the document as it
 * now reads from disk so the page re-renders from the file.
 */
export async function addRecordGoal(id: string, goal: string): Promise<DocumentPayload> {
  return request<DocumentPayload>(`${RECORDS}/${encodeURIComponent(id)}${GOALS}`, { add: goal });
}

/* ------------------------------------------ editing one document in place -- */

/** What a render answers with: the blocks, and nothing a file would carry. */
export type Preview = { blocks: Block[] };

/**
 * Save one document. The revision is the one the read handed over, and a file
 * that no longer hashes to it is refused with 409 rather than written over.
 * The answer is the document as it now reads from disk.
 */
export async function editDocument(id: string, source: string, revision: string): Promise<DocumentPayload> {
  return request<DocumentPayload>(`${DOCUMENTS}${encodeSegments(id)}${EDIT}`, { source, revision });
}

/** Render what is being typed, through the engine's own parser. */
export async function previewDocument(source: string): Promise<Preview> {
  return request<Preview>(PREVIEW, { source });
}

/**
 * Each segment is encoded on its own, so the separators stay separators. It is
 * this file's own: the one caller outside it was the link that handed a path to
 * a local editor, and this page edits the document itself now.
 */
function encodeSegments(id: string): string {
  return id
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");
}

/** What went wrong, in one line a human can act on. */
export function failureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
