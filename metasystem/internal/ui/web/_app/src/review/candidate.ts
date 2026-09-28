/**
 * The goal's candidate, from the room's pill (g1-s69 D3): app status, start and
 * stop --goal G, wrapped by the server as three routes. Status is read when the
 * room opens and after a human presses Run or Stop, and at no other time:
 * nothing here polls and nothing here sets a timer. Run and Stop are the human's
 * own acts under their signed-in session.
 */

const APP = "/api/app/";
const STATUS = "/status";
const START = "/start";
const STOP = "/stop";

/** What app status answers about the goal's candidate run. */
export type Candidate = {
  goal: string;
  state: string;
  readiness: string;
  address?: string;
  commit?: string;
  since?: string;
  said: string;
};

/** A candidate route refused, in its own words and under its code. */
export class CandidateError extends Error {
  readonly status: number;
  readonly code: string;
  readonly signIn: boolean;

  constructor(status: number, reason: string, code: string, signIn: boolean) {
    super(reason === "" ? `the candidate answered ${String(status)}` : reason);
    this.name = "CandidateError";
    this.status = status;
    this.code = code;
    this.signIn = signIn;
  }
}

/** The one request: a read without a body, an act with an empty one. */
async function call(resource: string, acting: boolean, signal?: AbortSignal): Promise<Candidate> {
  const response = await fetch(resource, {
    signal,
    method: acting ? "POST" : "GET",
    headers: acting ? { Accept: "application/json", "Content-Type": "application/json" } : { Accept: "application/json" },
    body: acting ? "{}" : undefined,
  });
  if (!response.ok) {
    let said: { error?: string; code?: string; signIn?: boolean } = {};
    try {
      said = (await response.json()) as typeof said;
    } catch {
      said = {};
    }
    throw new CandidateError(response.status, said.error ?? "", said.code ?? "", said.signIn === true);
  }
  return (await response.json()) as Candidate;
}

function base(goal: string): string {
  return APP + encodeURIComponent(goal);
}

export function loadCandidate(goal: string, signal?: AbortSignal): Promise<Candidate> {
  return call(`${base(goal)}${STATUS}`, false, signal);
}

export function startCandidate(goal: string): Promise<Candidate> {
  return call(`${base(goal)}${START}`, true);
}

export function stopCandidate(goal: string): Promise<Candidate> {
  return call(`${base(goal)}${STOP}`, true);
}
