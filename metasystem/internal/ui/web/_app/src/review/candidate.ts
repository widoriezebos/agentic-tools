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

/**
 * One request of Try it, as it is sent: a read without a body, an act with an
 * empty one; each names the commit the page shows, where it shows one (RF-04,
 * fix round 3 F-1): the server runs exactly that version, and refuses one that
 * is not the goal's, however the review has moved since the page read it.
 */
export function candidateRequest(resource: string, acting: boolean, commit: string): { url: string; method: string; body?: string } {
  const named = !acting && commit !== "" ? `?commit=${encodeURIComponent(commit)}` : "";
  return {
    url: `${resource}${named}`,
    method: acting ? "POST" : "GET",
    body: acting ? JSON.stringify(commit === "" ? {} : { commit }) : undefined,
  };
}

/** The one request, sent. */
async function call(resource: string, acting: boolean, commit: string, signal?: AbortSignal): Promise<Candidate> {
  const asked = candidateRequest(resource, acting, commit);
  const response = await fetch(asked.url, {
    signal,
    method: asked.method,
    headers: acting ? { Accept: "application/json", "Content-Type": "application/json" } : { Accept: "application/json" },
    body: asked.body,
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

/** The routes' own address for one goal's run. */
export function candidateRoute(goal: string, action: "status" | "start" | "stop"): string {
  return `${base(goal)}${action === "status" ? STATUS : action === "start" ? START : STOP}`;
}

export function loadCandidate(goal: string, commit = "", signal?: AbortSignal): Promise<Candidate> {
  return call(`${base(goal)}${STATUS}`, false, commit, signal);
}

export function startCandidate(goal: string, commit = ""): Promise<Candidate> {
  return call(`${base(goal)}${START}`, true, commit);
}

export function stopCandidate(goal: string, commit = ""): Promise<Candidate> {
  return call(`${base(goal)}${STOP}`, true, commit);
}
