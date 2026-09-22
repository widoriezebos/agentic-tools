/**
 * The backlog resource and its two acts, and the second place this build
 * talks to the server.
 *
 * The read is made when the pane mounts and again on Refresh, and nowhere
 * else: no stream, no socket, no timer, and no window event refetches it.
 * Freshness is the server's — its own loop keeps the accepted ref at the
 * canonical tip — so the page never has to poll to stay current.
 *
 * The two writes are made when a human confirms one in the sheet, and never
 * on their own. They are the two drag-and-drop moves that are real acts: To
 * Do to Ready for Work is goal approve, and back again is goal unapprove.
 * Each answers with the backlog as the ledger now stands, so the board moves
 * a card because the ledger moved rather than because the browser asked.
 *
 * All three go through the one request below, so there is a single place in
 * this build that reaches the network from here; src/cuts.test.ts holds the
 * call sites to a written list, so a request added under any other name fails
 * the guard rather than the review.
 */

import type { LaneId } from "./lanes";
import type { NewGoal } from "./opening";

const BACKLOG = "/api/backlog";
/** The collection: a body posted here opens a goal. */
const OPEN = "/api/backlog/goals";
/** The same resource with one goal beneath it, where the acts on one live. */
const GOALS = "/api/backlog/goals/";
const APPROVE = "/approve";
const WITHDRAW = "/withdraw";
const PRIORITY = "/priority";

/** What could be read of the accepted ledger. */
export type LedgerState = "read" | "absent" | "no-ledger" | "broken" | "unreadable" | "refused";

/** What the server's last fetch of the canonical branch did. */
export type FetchOutcome = "never" | "running" | "advanced" | "current" | "failed";

export type Reference = { kind: string; id: string; revision: number };

export type Approval = {
  by: string;
  at: string;
  authority: string;
  reviewBy: string;
  expired: boolean;
  expiredWhy: string;
};

export type Claim = { machine: string; lineage: string; at: string; landingAt: string };

export type Waiting = { reason: string; since: string; by: string; from: string; blocker: string };

export type Abandoned = { by: string; at: string; because: string };

export type Fence = { reason: string; closedAt: string };

/**
 * The complete limit tuple a goal is worked under. There is no partial form:
 * the four limits and the review rounds travel together or not at all,
 * because the machinery never invents a value a human did not confirm.
 */
export type Budget = {
  elapsedLimit: string;
  attemptLimit: number;
  reservedJobMinutesLimit: number;
  activeJobLimit: number;
  reviewRoundLimit: number;
};

/**
 * Whether a drop on this board would act, and as whom.
 *
 * The server proves this once, when it starts, from the ancestry of the
 * process that started it. A server an agent started carries the reason
 * instead, and the board says so before anything is dragged.
 */
export type Authority = { proven: boolean; human: string; reason: string };

export type Row = {
  ref: Reference;
  where: string;
  lane: LaneId;
  phase: string;
  state: string;
  intent: string;
  nextStep: string;
  concluded: string;
  origin: string;
  priority: number;
  sequence: number;
  tier: number;
  labels: string[];
  arc: string;
  pinned: string;
  blockedBy: string[];
  openBlockers: string[];
  approved?: Approval;
  /** The tuple the record carries, where it carries one. */
  budget?: Budget;
  claim?: Claim;
  waiting?: Waiting;
  abandoned?: Abandoned;
  fence?: Fence;
  sliced: boolean;
  decomposed: boolean;
  openedAt: string;
  /**
   * When this goal's own conclusion was recorded, or "" where nothing
   * recorded one. It is the goal's History speaking, not its state, which is
   * why a goal can be done and carry no date: the Done lane's window says so
   * rather than guessing.
   */
  doneAt: string;
  lastChangeAt: string;
  lastVerb: string;
  gaps: string[];
};

export type FetchClause = {
  outcome: FetchOutcome;
  startedAt: string;
  finishedAt: string;
  tip: string;
  detail: string;
  message: string;
  failures: number;
  cadence: string;
  nextAt: string;
};

export type Ledger = {
  state: LedgerState;
  tip: string;
  committedAt: string;
  stale: boolean;
  staleAfterSeconds: number;
  syncMode: string;
  stateRoot: string;
  message: string;
  problems: string[];
  fetch: FetchClause;
};

export type WorkingTree = { liveFiles: number | null; archivedFiles: number | null };

export type Backlog = {
  schemaVersion: number;
  observedAt: string;
  ledger: Ledger;
  admission: { answered: boolean; message: string };
  workingTree: WorkingTree;
  authority: Authority;
  /** The project's budget law, by tier, which an approval prefills from. */
  budgetDefaults: Partial<Record<string, Budget>>;
  counts: Partial<Record<LaneId, number>>;
  draft: { statement: string };
  rows: Row[];
  closed: Row[];
};

/**
 * A refusal the server explained, with the code it refused under.
 *
 * The sentence is the engine's own, because the engine is the only thing that
 * knows why an act was not made, and that sentence is what a human acts on. A
 * refusal that explained nothing carries the resource and the status, which
 * is all there is to say.
 */
export class BacklogError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(resource: string, status: number, reason = "", code = "") {
    super(reason === "" ? `${resource} answered ${String(status)}` : reason);
    this.name = "BacklogError";
    this.status = status;
    this.code = code;
  }
}

/** What a refusal's body carries, where the server sent one. */
type Refusal = { error?: string; code?: string };

/**
 * The one request. A body makes it an act, and an act is a POST of JSON;
 * without one it is the read. Every answer, act or read, is a backlog: the
 * two acts answer with the ledger as it stands after them, which is the whole
 * reason the board may move a card.
 */
async function request(resource: string, body?: unknown, signal?: AbortSignal): Promise<Backlog> {
  const acting = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: acting ? "POST" : "GET",
    headers: acting
      ? { Accept: "application/json", "Content-Type": "application/json" }
      : { Accept: "application/json" },
    body: acting ? JSON.stringify(body) : undefined,
  });
  return answerOf(resource, response);
}

/**
 * The backlog the server answered with, or the reason its response is not an
 * answer. Every ledger state is a 200 carrying why, so a status outside the
 * successful range means the engine itself could not answer, or refused the
 * act, and what it said is what a human acts on.
 */
export async function answerOf(resource: string, response: Response): Promise<Backlog> {
  if (!response.ok) {
    const refusal = await reasonOf(response);
    throw new BacklogError(resource, response.status, refusal.error ?? "", refusal.code ?? "");
  }
  return (await response.json()) as Backlog;
}

/** A body that is not the refusal shape says nothing, which is not an error. */
async function reasonOf(response: Response): Promise<Refusal> {
  try {
    return (await response.json()) as Refusal;
  } catch {
    return {};
  }
}

export async function loadBacklog(signal?: AbortSignal): Promise<Backlog> {
  return request(BACKLOG, undefined, signal);
}

/**
 * goal approve, for one goal, under the complete budget the human confirmed.
 * Nothing about who is approving travels: the server acts as the human it
 * proved at boot, and a name in a body would authorize nothing.
 */
export async function approveGoal(id: string, budget: Budget): Promise<Backlog> {
  return request(`${GOALS}${encodeURIComponent(id)}${APPROVE}`, budget);
}

/** goal unapprove, for one goal, with the reason the human gave. */
export async function withdrawGoal(id: string, reason: string): Promise<Backlog> {
  return request(`${GOALS}${encodeURIComponent(id)}${WITHDRAW}`, { reason });
}

/**
 * goal set-priority, for one goal: the band, and the one-based position in
 * it. A null sequence appends, which is what the engine does when the command
 * edge is given no --sequence, and is a different request from position 1.
 */
export async function rankGoal(id: string, priority: number, sequence: number | null): Promise<Backlog> {
  return request(`${GOALS}${encodeURIComponent(id)}${PRIORITY}`, { priority, sequence });
}

/** goal open, for one new goal, under origin human. */
export async function openGoal(asked: NewGoal): Promise<Backlog> {
  return request(OPEN, asked);
}
