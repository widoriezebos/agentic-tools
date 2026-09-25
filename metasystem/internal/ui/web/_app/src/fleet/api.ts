/**
 * The fleet resource, and the only place this section talks to the server.
 *
 * One read, made when the pane mounts, again when a human presses the
 * section's refresh, and again when the server says a presence attempt
 * finished — which arrives on the one stream this build holds open and is not
 * a request this file makes. src/cuts.test.ts holds that by counting call
 * sites, so a second request written under any other name fails the guard
 * rather than the review.
 *
 * The shapes below are the server's, field for field. Every one of them is
 * written by the composing package whether it has anything to say or not, so
 * nothing here has to tell an absent field from an empty one.
 */

import type { LaneId } from "../backlog/lanes";

const FLEET = "/api/fleet";

/** What this seat concludes about one machine, from its presence record. */
export type Standing = "reachable" | "unreachable" | "unknown";

/**
 * Where the presence copy this page was composed from came from, and what the
 * server's own fetch owner has managed so far.
 *
 * Attempt, success and failure are three fields because they are three facts:
 * a failure leaves the last success and the refs it brought standing, and a
 * success that brought nothing is not the same as never having fetched.
 */
export type Copy = {
  source: string;
  attemptedAt: string;
  succeededAt: string;
  failedAt: string;
  problem: string;
};

/** The one accepted tip every holder on this page was read from. */
export type Claims = { tip: string; unavailable: string };

/** One line of the steward's last recorded health verdict. */
export type Role = { role: string; status: string; reason: string };

/**
 * That verdict, with the instant it was recorded at. It is a past observation
 * and the page says so: `observedAt` is what "last recorded" is measured
 * from, and `problem` names a file that could not be read at all.
 */
export type Health = { state: string; observedAt: string; problem: string; roles: Role[] };

/** What this machine last managed to publish about itself. */
export type Publication = {
  lastAttemptAt: string;
  lastSuccessAt: string;
  lastOutcome: string;
  rung: number;
  detail: string;
};

/**
 * The newest delegate chain one machine has in flight. `startedAt` is null on
 * a reservation that has not begun, which the row says rather than hiding.
 */
export type Running = {
  job: string;
  role: string;
  round: number;
  goal: string;
  startedAt: string | null;
};

/** The seat this interface is running on. */
export type ThisSeat = {
  machine: string;
  noNickname: boolean;
  /** armed, not armed, stale, or unreadable — a word and never a boolean. */
  armed: string;
  health: Health | null;
  publication: Publication | null;
  publicationProblem: string;
  running: Running | null;
  runningProblem: string;
};

/** One goal a machine holds, with the flag its holder's standing raises. */
export type Held = {
  goal: string;
  title: string;
  /**
   * The projection's own lane id; the board's lane register titles it, so
   * there is one owner for what a lane is called.
   */
  lane: LaneId;
  machine: string;
  standing: Standing;
  since: string;
  flag: string;
};

/** One row of the fleet. */
export type Machine = {
  machine: string;
  standing: Standing;
  reason: string;
  ageSeconds: number | null;
  /** The record's own tickAt, whatever the standing. */
  seen: string;
  since: string;
  running: Running | null;
  engine: string;
  generation: number;
  holds: Held[];
  this: boolean;
};

export type Page = {
  schemaVersion: number;
  readAt: string;
  copy: Copy;
  claims: Claims;
  this: ThisSeat;
  needsYou: Held[];
  machines: Machine[];
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

export async function loadFleet(signal?: AbortSignal): Promise<Page> {
  const response = await fetch(FLEET, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new ResourceError(FLEET, response.status, (await reasonOf(response)).error ?? "");
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
