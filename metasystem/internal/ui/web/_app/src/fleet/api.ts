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
const LAUNCH = "/api/fleet/launch";

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
  /**
   * The last fetch's own failure, kept while the refs it brought before still
   * stand — or a namespace this seat could not read at all, which displaces
   * it, and which leaves the machine list empty because nothing was read.
   */
  problem: string;
  /**
   * The last write of the file the Partner's tool reads, where it failed. Its
   * own field because it is its own fact: this page's copy is fine and the
   * tool's reading of it is not.
   */
  metadataProblem: string;
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

/**
 * What a job is a phase of. `roundLimit` is null for a build, which shows its
 * round without a denominator because a build has no round limit of its own;
 * for a critic it is the limit its chain's root froze.
 */
export type Phase = { role: string; round: number; roundLimit: number | null };

/**
 * The job in hand. `capEndsAt` is when the minutes it reserved run out — the
 * recorded deadline, or the start plus the cap — and it is the one instant
 * anything on this page looks forward to. It names the cap and never
 * completion.
 */
export type WorkingJob = {
  id: string;
  role: string;
  status: string;
  startedAt: string | null;
  capMinutes: number | null;
  capEndsAt: string | null;
};

/**
 * The goal's box, as the engine's own projection counts it.
 *
 * Every number is nullable and `problem` is why. A projection that could not
 * be made carries its reason and no figures, because zeros would say the goal
 * has spent nothing — which is the one thing an unknown projection does not
 * know. The whole box is null where the goal carries none at all.
 */
export type Box = {
  attempts: number | null;
  attemptLimit: number | null;
  reservedMinutes: number | null;
  reservedMinutesLimit: number | null;
  problem: string;
};

/** One job of the chain. It carries its cap and never a consumed charge. */
export type ChainMember = {
  job: string;
  role: string;
  round: number;
  status: string;
  startedAt: string | null;
  endedAt: string | null;
  capMinutes: number | null;
};

/** One thing a machine is doing, whole: what the row's disclosure opens to. */
export type Working = {
  goal: string;
  phase: Phase;
  job: WorkingJob;
  box: Box | null;
  chain: ChainMember[];
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
  /**
   * What this machine is doing, in the detail the row opens to.
   *
   * It is a list because this seat's row is not like the others. A machine
   * elsewhere carries what its presence record published, which is one chain;
   * on this host the server reads the job records itself and sends every job
   * in flight, newest first. Empty is idle.
   */
  working: Working[];
  /**
   * The local jobs reader's own explanation, carried rather than shown as
   * idle. Only this seat's row can have one.
   */
  workingProblem: string;
};

/** One step of a launch, as the verb recorded it. */
export type LaunchStep = {
  step: string;
  /** pending, done, skipped, failed or armed — a word and never a boolean. */
  outcome: string;
  at: string;
  /** The owner's own sentence, where the step refused or has something to say. */
  words: string;
};

/** The two commands a human has for a machine that joined. */
export type LaunchNext = { session: string; stop: string };

/** What a launch made, and so what a retry of it may reuse. */
export type LaunchCreated = { destination: boolean; nickname: boolean; evidenceRoot: boolean };

/**
 * One launch, as the verb's own record carries it.
 *
 * The human's authorization is not a field here and never was: the word
 * reaches the arming verb's argument list and nothing else, which is why a
 * retry that has to enroll asks for it again.
 */
export type Launch = {
  schemaVersion: number;
  launch: string;
  machine: string;
  destination: string;
  clonedCommit: string;
  builtStamp: string;
  process: { pid: number; startedAt: number };
  startedAt: string;
  endedAt: string | null;
  /**
   * starting, running, done, armed or failed. `starting` is a launch this
   * server wrote down whose verb has not yet named its own process; `armed`
   * is a machine that is up and has not been seen publishing yet, and is not
   * a failure.
   */
  outcome: string;
  reviewBy: string;
  created: LaunchCreated;
  steps: LaunchStep[];
  orientation: string;
  next: LaunchNext;
};

/** The two facts a destination is proposed from, which only the server knows. */
export type Launching = { parent: string; repository: string };

export type Page = {
  schemaVersion: number;
  readAt: string;
  copy: Copy;
  claims: Claims;
  this: ThisSeat;
  needsYou: Held[];
  machines: Machine[];
  launches: Launch[];
  launching: Launching;
};

/** What the sheet sends, or what a retry sends. */
export type LaunchRequest = {
  machine?: string;
  destination?: string;
  word: string;
  reviewBy: string;
  /**
   * The day this browser is on, as a plain YYYY-MM-DD. The server judges the
   * review date against it rather than against its own day: the two differ
   * for several hours of every day, and the date a human answered is the one
   * that was on their screen.
   */
  today: string;
  resume?: string;
};

/** A response that was not what was asked for, with the server's own words. */
export class ResourceError extends Error {
  readonly status: number;
  /** The code the act refused under, where it refused under one. */
  readonly code: string;
  /**
   * Whether the remedy is in this page rather than in a terminal: the server
   * found no signed-in human behind the act. The page opens the sign-in sheet
   * on it, exactly as the board's acts do.
   */
  readonly signIn: boolean;

  constructor(resource: string, status: number, reason = "", code = "", signIn = false) {
    super(reason === "" ? `${resource} answered ${String(status)}` : reason);
    this.name = "ResourceError";
    this.status = status;
    this.code = code;
    this.signIn = signIn;
  }
}

type Refusal = { error?: string; code?: string; signIn?: boolean };

/**
 * The one request. A body makes it the act, and an act is a POST of JSON;
 * without one it is the read.
 *
 * Both go through here for the reason the backlog's do: there is exactly one
 * place in this file that reaches the network, so a second way to the server
 * cannot be added without the cut guard seeing it.
 */
async function request(resource: string, body?: unknown, signal?: AbortSignal): Promise<unknown> {
  const acting = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: acting ? "POST" : "GET",
    headers: acting
      ? { Accept: "application/json", "Content-Type": "application/json" }
      : { Accept: "application/json" },
    body: acting ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const refusal = await reasonOf(response);
    throw new ResourceError(resource, response.status, refusal.error ?? "", refusal.code ?? "", refusal.signIn === true);
  }
  return await response.json();
}

export async function loadFleet(signal?: AbortSignal): Promise<Page> {
  return (await request(FLEET, undefined, signal)) as Page;
}

/**
 * The one act this section makes: a machine of this fleet joins on this host.
 *
 * It answers 202 with the record the server wrote before anything ran, so the
 * card has something to draw at once. Everything after that reaches the page
 * through the read above, which re-runs on the `fleet` event the record's own
 * changes cause.
 */
export async function launchMachine(asked: LaunchRequest, signal?: AbortSignal): Promise<Launch> {
  return (await request(LAUNCH, asked, signal)) as Launch;
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
