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
const LAUNCHES = "/api/fleet/launches/";
const BOARD = "/api/board";
const LAND_NOW = "/api/fleet/land-now";
const LANE_PAUSE = "/api/fleet/lane/pause";
const LANE_RESUME = "/api/fleet/lane/resume";
const MACHINE_STOP = "/api/fleet/machines/stop";
const PROOF_LOGS = "/api/fleet/proof-logs/";

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
 * The record's own enrollment and review fields are not here: nothing on
 * this page reads them, and a machine launched from it is its human's own,
 * enrolled by the signed-in session (g1-s72).
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
  created: LaunchCreated;
  steps: LaunchStep[];
  orientation: string;
  next: LaunchNext;
  /**
   * When a human discarded this launch from the fleet page, or null. The
   * record is kept and nothing is deleted; the page draws no card for it.
   */
  discardedAt: string | null;
  /**
   * Whether the clone this launch made is still on this host, as the server
   * read the disk when it composed the page. It is not the record's own
   * field: the record cannot know what happened to a directory after it.
   */
  destinationPresent: boolean;
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

/**
 * The read of the fleet the callers of one moment share.
 *
 * The shell's rail and the Fleet page both read the fleet when they mount, and
 * both again on the same `fleet` beat, so on one page load or one beat they
 * would ask the server the same question twice. A read is shared only with the
 * callers of the same run of the page's code and let go before anything else
 * can happen, so a read asked for after an act is never answered by one that
 * started before it.
 *
 * The shared request carries no caller's signal: one caller going away does
 * not take the answer from another. Each caller's own signal still ends its
 * own wait, which is what every caller checks.
 */
export const loadFleet = sharedInTheMoment(() => request(FLEET) as Promise<Page>);

/** A read that the callers of one run of the page's code share, as above. */
export function sharedInTheMoment<T>(read: () => Promise<T>): (signal?: AbortSignal) => Promise<T> {
  let shared: Promise<T> | null = null;
  return (signal?: AbortSignal) => {
    if (shared === null) {
      shared = read();
      queueMicrotask(() => {
        shared = null;
      });
    }
    return waitFor(shared, signal);
  };
}

/** A shared read as one caller waits for it: ended early by that caller's signal. */
function waitFor<T>(read: Promise<T>, signal?: AbortSignal): Promise<T> {
  if (signal === undefined) {
    return read;
  }
  return new Promise<T>((resolve, reject) => {
    const stop = () => {
      reject(signal.reason);
    };
    if (signal.aborted) {
      stop();
      return;
    }
    signal.addEventListener("abort", stop, { once: true });
    read.then(resolve, reject).finally(() => {
      signal.removeEventListener("abort", stop);
    });
  });
}

/**
 * One seat's line on the host board, in the server's own words: what it
 * works on and how far it is, or why it cannot be believed (batch-lane
 * design D14-r2). The server classified the cards; this page judges nothing.
 */
export type BoardLine = { machine: string; text: string };

/**
 * One goal on one armed seat of this computer, as its progress card says it:
 * the stage, the round of its limit, a proof's sections done of planned, and
 * since when. `unknown` is why the card cannot be believed — a writer that is
 * dead, a stall, or a claim that moved to another machine — and such a card
 * says nothing about what the seat is doing.
 */
export type BoardGoal = {
  goal: string;
  stage?: string;
  round?: { n: number; max: number | null };
  proof?: { attempt: string; done: number; planned: number };
  batch?: string;
  since?: string;
  lastProgressAt?: string;
  unknown?: string;
};

/** One armed seat of this computer and the goals its cards name. */
export type BoardSeat = { machine: string; installation: string; unknown?: string; goals: BoardGoal[] };

/** One open question this checkout's seats asked the person. */
export type BoardQuestion = {
  id: string;
  goal: string;
  about?: string;
  machine: string;
  /** The question's first line, as the asker wrote it. */
  question: string;
  openedAt: string;
};

/**
 * The host board: every armed seat of this host with its goals, whether the
 * registry could be read at all, and the bridge's state; the landing lane;
 * the title of every goal it names; and this checkout's open questions.
 */
export type BoardPayload = {
  readable: boolean;
  reason?: string;
  bridge: string;
  seats: BoardSeat[];
  lines: BoardLine[];
  /**
   * The host's one landing lane (U12). Null where this host has none;
   * absent from a server built before the lane existed, which the panel says
   * rather than taking for "no lane".
   */
  lane?: Lane | null;
  /** Goal titles by id, for every goal the seats, the queue and the questions name. */
  titles?: Record<string, string>;
  /** How each queued goal the ledger holds as concluded ended ("done" or "abandoned"); a return of one is history. */
  ended?: Record<string, string>;
  /** This checkout's open questions; absent from a server that does not read them. */
  questions?: BoardQuestion[];
  /** Why the questions, or some of their records, could not be read; "" when every record was. */
  questionsProblem?: string;
  /** The parts of this computer's board that were not read, one line each. */
  unreadable?: string[];
};

/**
 * What runs the lane: its landing agent, started on demand when there is
 * work. Idle is normal (a ready lane with nothing to land); unready is a lane
 * that cannot run, with the reason and the fix in retry_hint.
 */
export type LaneOwnerState = "running" | "idle" | "stopped" | "unready";

/**
 * The lane's owner: its landing agent while one runs, or a person's stop, and
 * what a person does next when it cannot run.
 */
export type LaneOwner = {
  state: LaneOwnerState;
  pid: number | null;
  since: string | null;
  last_exit: string | null;
  stopped_by: string | null;
  stopped_because?: string;
  retry_hint: string | null;
};

/**
 * One hand-in of the plain lane's queue and what became of it: waiting,
 * returned (with why), superseded by a newer hand-in of its goal, or landed.
 */
export type LaneEntry = {
  goal: string;
  branch: string;
  sha: string;
  seat: string;
  at: string;
  state: "waiting" | "returned" | "superseded" | "landed" | (string & {});
  reason?: string;
  returned_at?: string;
  /** The hand-in's one plain sentence of what it delivers, from its seat. */
  delivered?: string;
  /** When a landed hand-in reached main: the time of the push that brought it. */
  landed_at?: string;
};

/** The lane's proof recorded running; died is one that ended without a result. */
export type LaneRunningProof = {
  tree: string;
  commit?: string;
  since: string;
  attempt: string;
  log?: string;
  state: "running" | "died" | (string & {});
  /** The waiting hand-ins this proof's commit holds, by goal. */
  goals?: string[];
};

/** The newest proof the lane recorded. */
export type LaneProof = {
  tree: string;
  commit: string;
  result: "green" | "red" | (string & {});
  log: string;
  at: string;
  attempt?: string;
  reason?: string;
};

/** The newest push the lane made to main. */
export type LanePush = { old: string; commit: string; tree: string; at: string };

/**
 * The landing lane, field for field as /api/board carries it: landing status
 * --json's data. The plain lane's fields are absent from a server built
 * before they joined the board.
 */
export type Lane = {
  root: string | null;
  registered_by: string | null;
  registered_at: string | null;
  owner: LaneOwner;
  /**
   * Why the landing agent would run now, read the way the lane's keeper reads
   * it ("queued", "proof-finished").
   */
  wake?: LaneWake | null;
  summary: string;
  paused?: boolean;
  agent_alive?: boolean;
  queue?: LaneEntry[];
  running_proof?: LaneRunningProof | null;
  last_proof?: LaneProof | null;
  last_push?: LanePush | null;
  /** The lane's records this read could not read, one sentence each. */
  problems?: string[];
  /** Why this computer's lane registration could not be read, where it could not. */
  unreadable?: string;
};

export type LaneWake = { reasons: string[]; unread: string[] };

/**
 * What `metasystem landing run` answered, as its one-result envelope says
 * it: the outcome, line 1 (summary) and line 2 (next), null where it names
 * none. Pause, Resume and Stop answer their own verbs' envelopes in the same
 * shape.
 */
export type LandNowAnswer = {
  outcome: string;
  summary: string;
  next: { argv: string[]; reason: string } | null;
};

/**
 * The host board, read beside the fleet: when the pane mounts, on Refresh,
 * and on the same `fleet` event and reconnect the fleet re-reads on.
 */
export async function loadBoard(signal?: AbortSignal): Promise<BoardPayload> {
  return (await request(BOARD, undefined, signal)) as BoardPayload;
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

/**
 * Land now, from the landing lane card: the server runs `metasystem landing
 * run` once under the signed-in session and answers the verb's envelope,
 * success or refusal alike. A press while an agent runs is the verb's success
 * that started nothing.
 */
export async function landNow(signal?: AbortSignal): Promise<LandNowAnswer> {
  return (await request(LAND_NOW, {}, signal)) as LandNowAnswer;
}

/**
 * Pause and Resume, from the landing lane's heading and Needs you, and Stop,
 * from a stuck seat's item and an opened row (fleet-panel-ux step 2, slice
 * 2b): the server runs `metasystem landing stop`, `landing start` or `machine
 * stop` once, as the signed-in person, and answers the verb's envelope as Land
 * now does. A repeat is the verb's success that changed nothing. Stop is never
 * run for the machine serving this page: the server refuses it in two lines.
 */
export async function pauseLane(signal?: AbortSignal): Promise<LandNowAnswer> {
  return (await request(LANE_PAUSE, {}, signal)) as LandNowAnswer;
}

export async function resumeLane(signal?: AbortSignal): Promise<LandNowAnswer> {
  return (await request(LANE_RESUME, {}, signal)) as LandNowAnswer;
}

export async function stopMachine(machine: string, signal?: AbortSignal): Promise<LandNowAnswer> {
  return (await request(MACHINE_STOP, { machine }, signal)) as LandNowAnswer;
}

/**
 * A stopped launch put out of sight, or a joined machine's card dismissed:
 * the server marks the record and deletes nothing. It is idempotent, and it
 * answers the record as it now reads.
 */
export async function discardLaunch(id: string, signal?: AbortSignal): Promise<Launch> {
  return (await request(discardAddress(id), {}, signal)) as Launch;
}

/** Where one launch's discard is posted: its own address beneath the fleet. */
export function discardAddress(id: string): string {
  return `${LAUNCHES}${encodeURIComponent(id)}/discard`;
}

/**
 * Where one landing proof's log is read, whole, as text: its attempt beneath
 * the lane's proof logs. The server finds the log the lane's records name for
 * that attempt; the page never names a path. It is a link the person opens in
 * a tab of its own, not a request this file makes.
 */
export function proofLogAddress(attempt: string): string {
  return `${PROOF_LOGS}${encodeURIComponent(attempt)}`;
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
