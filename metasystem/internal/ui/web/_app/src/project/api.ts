/**
 * The two project resources, and the only place this section talks to the
 * server.
 *
 * Each is read once per page view and again on Reload, and nowhere else: no
 * stream, no socket, no timer, and no window event refetches either of them.
 * src/cuts.test.ts holds that by counting call sites, so a second request
 * written under any other name fails the guard rather than the review.
 */

const THREAD = "/api/project";
const DOCUMENTS = "/api/documents/";

export type SubsectionState = "recorded" | "not-recorded" | "not-projected";
export type DocumentState = "readable" | "unreadable" | "too-large";

/** One document in the catalogue, named the way the resolver names it. */
export type DocumentEntry = {
  kind: string;
  id: string;
  title: string;
  owner: string;
  bytes: number;
  modifiedAt: string;
  state: DocumentState;
  reason: string;
};

export type Group = { id: string; title: string; documents: DocumentEntry[] };

export type CovenantIdentity = { name: string; entryPoint: string; sourcePaths: string[] };
export type Requirement = { id: string; ref: string; proof: string };
export type Battery = { command: string; metric: string; direction: string; threshold: string };
export type Budget = { metric: string; bound: number; direction: string };
export type Guard = { name: string; command: string; cadence: number; floor: number };

export type Covenant = {
  path: string;
  error?: string;
  identity?: CovenantIdentity;
  requirements?: Requirement[];
  battery?: Battery;
  budgets?: Budget[];
  guards?: Guard[];
  guardrails?: string[];
};

export type Purpose = { path: string; text: string };

export type Subsection = {
  id: string;
  title: string;
  state: SubsectionState;
  lookedFor: string[];
  covenant: Covenant | null;
  purpose: Purpose | null;
  groups: Group[];
};

export type Thread = { schemaVersion: number; readAt: string; subsections: Subsection[] };

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

export async function loadThread(signal?: AbortSignal): Promise<Thread> {
  return getJSON<Thread>(THREAD, signal);
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
