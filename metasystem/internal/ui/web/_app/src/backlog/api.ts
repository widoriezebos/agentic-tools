/**
 * The backlog resource, and the second place this build talks to the server.
 *
 * It is read when the pane mounts and again on Refresh, and nowhere else: no
 * stream, no socket, no timer, and no window event refetches it. Freshness is
 * the server's — its own loop keeps the accepted ref at the canonical tip — so
 * the page never has to poll to stay current. src/cuts.test.ts holds the call
 * sites to a written list, so a request added under any other name fails the
 * guard rather than the review.
 */

import type { LaneId } from "./lanes";

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
  claim?: Claim;
  waiting?: Waiting;
  abandoned?: Abandoned;
  fence?: Fence;
  sliced: boolean;
  decomposed: boolean;
  openedAt: string;
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
  counts: Partial<Record<LaneId, number>>;
  draft: { statement: string };
  rows: Row[];
  closed: Row[];
};

export async function loadBacklog(signal?: AbortSignal): Promise<Backlog> {
  return answerOf(await fetch("/api/backlog", { signal, headers: { Accept: "application/json" } }));
}

/**
 * The backlog the server answered with, or the reason its response is not an
 * answer. Every ledger state is a 200 carrying why, so a status outside the
 * successful range means the engine itself could not answer and the status is
 * what a human acts on.
 */
export async function answerOf(response: Response): Promise<Backlog> {
  if (!response.ok) {
    throw new Error(`/api/backlog answered ${String(response.status)}`);
  }
  return (await response.json()) as Backlog;
}
