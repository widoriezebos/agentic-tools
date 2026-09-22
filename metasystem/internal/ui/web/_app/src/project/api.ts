/**
 * The two project resources, and the only place this section talks to the
 * server.
 *
 * Each is read once per page view and again on Reload, and nowhere else: no
 * stream, no socket, no timer, and no window event refetches either of them.
 * src/cuts.test.ts holds that by counting call sites, so a second request
 * written under any other name fails the guard rather than the review.
 */

const PANE = "/api/project";
const DOCUMENTS = "/api/documents/";

export type DocumentState = "readable" | "unreadable" | "too-large";

/** One slug the intent index declares, with the name it reads by. */
export type Area = { slug: string; name: string };

/**
 * One declared record: what it says it is, where it lives, and its own first
 * words. The summary is a convention rather than a key, so a record that has
 * none carries the empty string.
 */
export type ProjectRecord = {
  kind: string;
  id: string;
  status: string;
  areas: string[];
  title: string;
  path: string;
  home: string;
  summary: string;
};

/**
 * One line of a book's reading order: a record named by id, or a document
 * bound by its checkout-relative path. Exactly one of the two is carried.
 */
export type Chapter = { id?: string; path?: string; title: string; summary: string };

/** A book: its index, which is a record of the kind, and its reading order. */
export type Book = { index: ProjectRecord | null; chapters: Chapter[] };

export type Question = { id: string; opened: string; question: string; areas: string[]; status: string };

/** One refusal the check verb would print, anchored where a human can act. */
export type Problem = { path: string; line: number; message: string };

/** One of the checkout's other documents, by path, with no kind claimed. */
export type DocumentFile = { path: string; title: string };

export type Pane = {
  schemaVersion: number;
  readAt: string;
  areas: Area[];
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
  areas: string[];
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

/** A response that was not what was asked for, with the status it carried. */
export class ResourceError extends Error {
  readonly status: number;

  constructor(resource: string, status: number) {
    super(`${resource} answered ${String(status)}`);
    this.name = "ResourceError";
    this.status = status;
  }
}

/** A document this checkout does not serve, which has its own card. */
export function isNotFound(error: unknown): boolean {
  return error instanceof ResourceError && error.status === 404;
}

async function getJSON<T>(resource: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(resource, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new ResourceError(resource, response.status);
  }
  return (await response.json()) as T;
}

export async function loadPane(signal?: AbortSignal): Promise<Pane> {
  return getJSON<Pane>(PANE, signal);
}

export async function loadDocument(id: string, signal?: AbortSignal): Promise<DocumentPayload> {
  return getJSON<DocumentPayload>(DOCUMENTS + encodeSegments(id), signal);
}

/** Each segment is encoded on its own, so the separators stay separators. */
export function encodeSegments(id: string): string {
  return id
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");
}

/** What went wrong, in one line a human can act on. */
export function failureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
