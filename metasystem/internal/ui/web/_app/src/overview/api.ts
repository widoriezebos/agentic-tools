/**
 * The overview resource, and the only place this section talks to the server.
 *
 * One read, made when the pane mounts and again when a human presses the
 * section's refresh, and nowhere else: no stream, no socket, no timer, and no
 * window event refetches it. src/cuts.test.ts holds that by counting call
 * sites, so a second request written under any other name fails the guard
 * rather than the review.
 *
 * The shapes below are the server's, field for field. Every one of them is
 * written by the composing package whether it has anything to say or not, so
 * nothing here has to tell an absent field from an empty one.
 */

const OVERVIEW = "/api/overview";

/**
 * Where one row opens, said as what kind of thing it is rather than as an
 * address. The server owns no routes; src/overview/overview.ts turns a Where
 * into a destination this build actually serves.
 */
export type Where = { kind: string; id: string };

/** One thing a human can open, with the one fact that says why it is here. */
export type Item = { id: string; title: string; note: string; at: string; where: Where };

/** A capped list with the whole count beside it. */
export type Group = { count: number; items: Item[] };

export type NeedsYou = {
  approvals: Group;
  questions: Group;
  drafts: Group;
  designs: Group;
  alerts: Group;
  /** True when nothing proves a human on this seat. */
  signIn: boolean;
  /** The five counts added up. It does not count the sign-in row. */
  total: number;
};

export type Changed = {
  concluded: Group;
  moved: Group;
  records: Group;
  messages: number;
  total: number;
};

export type Seat = { machine: string; lineage: string };

export type Claimed = { id: string; title: string; seat: Seat; phase: string; at: string };

export type Waiting = { count: number; id: string; title: string; reason: string; since: string };

/** One count of the strip: a lane of the board, or Done today. */
export type Lane = { id: string; count: number };

export type Work = { inProgress: Claimed[]; next: Item[]; waiting: Waiting; lanes: Lane[] };

export type Book = { chapters: number; summary: string };

export type Tally = { total: number; drafts: number };

export type Progress = { id: string; title: string; path: string; done: number; goals: number };

export type Designs = { total: number; done: number; inFlight: number; progress: Progress[] };

/**
 * One kind of record counted by what it is about: the project's own — the ones
 * whose head names no goal — and the ones that name at least one. A record
 * naming three goals is one record here and not three.
 */
export type Scope = { own: number; underGoals: number };

/** The three kinds the Project page's scope control narrows, counted both ways. */
export type Scoped = { decisions: Scope; designs: Scope; questions: Scope };

export type Memory = {
  intent: Book;
  doctrine: Book;
  decisions: Tally;
  designs: Designs;
  questions: number;
  /** The same three kinds by scope, which is what the tiles show. */
  scoped: Scoped;
};

/**
 * How current the server judged its own fetch loop to be. It is the same
 * judgement the board's sync chip carries, made once on the server, so the
 * two surfaces cannot disagree about the same loop.
 */
export type FreshnessState = "current" | "behind" | "failed";

export type Health = {
  ok: boolean;
  syncedAt: string;
  freshness: FreshnessState;
  problems: Group;
};

export type Page = {
  schemaVersion: number;
  readAt: string;
  /** The start of the window "what changed" is read over. */
  since: string;
  /** True when that window is a first visit's day rather than a marker. */
  first: boolean;
  needsYou: NeedsYou;
  changed: Changed;
  work: Work;
  memory: Memory;
  health: Health;
};

/** A response that was not what was asked for, with the server's own words. */
export class ResourceError extends Error {
  readonly status: number;

  constructor(resource: string, status: number, reason = "") {
    super(reason === "" ? `${resource} answered ${String(status)}` : reason);
    this.name = "ResourceError";
    this.status = status;
  }
}

type Refusal = { error?: string };

export async function loadOverview(signal?: AbortSignal): Promise<Page> {
  const response = await fetch(OVERVIEW, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new ResourceError(OVERVIEW, response.status, (await reasonOf(response)).error ?? "");
  }
  return (await response.json()) as Page;
}

/** A body that is not the refusal shape says nothing, which is not an error. */
async function reasonOf(response: Response): Promise<Refusal> {
  try {
    return (await response.json()) as Refusal;
  } catch {
    return {};
  }
}

/** What went wrong, in one line a human can act on. */
export function failureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
