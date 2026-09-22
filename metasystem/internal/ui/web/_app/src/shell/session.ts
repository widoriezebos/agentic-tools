/**
 * Who the server is acting as, and the two acts that change it.
 *
 * It is read once with the workspace, and again only when a sign-in or a
 * sign-out answers with a new one: no stream, no socket, no timer, and no
 * window event refetches it, because a session changes when a human changes
 * it and at no other moment. The one exception a session has — its hours
 * running out — is not polled for either: the server says so when an act
 * reaches it, and the page reads that answer.
 *
 * All three requests go through the one call below, so there is a single
 * place in this build that reaches the network from here; src/cuts.test.ts
 * holds the call sites to a written list.
 *
 * Nothing about the session is kept in this browser but the cookie the server
 * set, which no script here can read: it is HttpOnly, and everything the page
 * shows comes from the answers below.
 */

const SESSION = "/api/session";
const SIGN_IN = "/api/session/sign-in";
const SIGN_OUT = "/api/session/sign-out";

/** How a request is proving a human, or that it is not. */
export type Source = "code" | "terminal" | "none";

/** What every session answer carries, signed in or not. */
export type SessionState = {
  human: string;
  signedIn: boolean;
  /** When this session stops acting, or "" for one that does not expire. */
  until: string;
  source: Source;
};

/** What the page holds while it reads, and after it has. */
export type SessionStatus =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "known"; session: SessionState };

/**
 * A sign-in the server refused, with the code it refused under.
 *
 * The sentence is the server's own, because the server is the only thing that
 * knows whether a code was wrong, already spent, or arriving too fast.
 */
export class SignInError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, reason = "", code = "") {
    super(reason === "" ? `signing in answered ${String(status)}` : reason);
    this.name = "SignInError";
    this.status = status;
    this.code = code;
  }
}

type Refusal = { error?: string; code?: string };

async function ask(resource: string, body?: unknown, signal?: AbortSignal): Promise<SessionState> {
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
    let refusal: Refusal = {};
    try {
      refusal = (await response.json()) as Refusal;
    } catch {
      refusal = {};
    }
    throw new SignInError(response.status, refusal.error ?? "", refusal.code ?? "");
  }
  return (await response.json()) as SessionState;
}

export async function loadSession(signal?: AbortSignal): Promise<SessionState> {
  return ask(SESSION, undefined, signal);
}

/**
 * Signing in with this seat's one-time code. The handle travels only where the
 * server said it does not know one; where it does, what is sent is ignored and
 * the configured name is what acts.
 */
export async function signIn(code: string, human: string): Promise<SessionState> {
  return ask(SIGN_IN, human === "" ? { code } : { code, human });
}

export async function signOut(): Promise<SessionState> {
  return ask(SIGN_OUT, {});
}

/**
 * What the identity control in the header shows.
 *
 * The handle and the qualifier are two pieces rather than one sentence
 * because the header has to narrow: on a phone the qualifier folds away and
 * the handle stays, since the handle is what identifies the act and the clock
 * only says how long it has.
 */
export type Control =
  | { kind: "loading" }
  | { kind: "sign-in" }
  | { kind: "signed-in"; who: string; qualifier: string; canSignOut: boolean };

/**
 * The control, from the state.
 *
 * A session that could not be read offers the sign-in anyway: the remedy for
 * not knowing is the same as the remedy for not being signed in, and an
 * identity control that renders nothing is one a human cannot use.
 */
export function controlFor(status: SessionStatus): Control {
  if (status.state === "loading") {
    return { kind: "loading" };
  }
  if (status.state === "failed" || !status.session.signedIn) {
    return { kind: "sign-in" };
  }
  const { human, source, until } = status.session;
  if (source === "terminal") {
    return { kind: "signed-in", who: human, qualifier: "terminal", canSignOut: false };
  }
  return { kind: "signed-in", who: human, qualifier: `until ${clock(until)}`, canSignOut: true };
}

/** The local wall clock to the minute, which is how long a session has left. */
export function clock(stamp: string): string {
  if (stamp === "") {
    return "unknown";
  }
  const at = new Date(stamp);
  if (Number.isNaN(at.getTime())) {
    return "unknown";
  }
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

/**
 * Whether the sheet has to ask for a name.
 *
 * It asks once, and only on a seat that has named nobody: a seat with a human
 * of its own never takes one from a browser, so offering the field there would
 * be offering a choice that is not one.
 */
export function needsAHandle(status: SessionStatus): boolean {
  return status.state === "known" && status.session.human === "";
}

/** Six digits and nothing else is a code this seat could accept. */
export function isCode(typed: string): boolean {
  return /^[0-9]{6}$/.test(typed.trim());
}

/** What signing in is for, in one line. */
export const SIGN_IN_NOTE =
  "Your acts are published as you, and the ledger records this browser session beside each one.";

/** Why the button is disabled, or the empty string when it is not. */
export function blockedForSignIn(code: string, handle: string, needsHandle: boolean): string {
  if (needsHandle && handle.trim() === "") {
    return "This seat does not know who you are yet: give the name your acts are recorded under.";
  }
  if (needsHandle && /\s/.test(handle.trim())) {
    return "A name is one word: the ledger records it beside every act.";
  }
  if (!isCode(code)) {
    return "A one-time code is six digits.";
  }
  return "";
}

