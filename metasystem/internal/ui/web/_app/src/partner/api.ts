/**
 * The Project Partner's resources, and the only place this section talks to
 * the server.
 *
 * Three requests go through the one request below: the conversation, which is
 * read when the page loads and again on every reconnect of the one stream; a
 * send, which happens when a human presses Send; and a stop, which happens
 * when they press Stop. Nothing here polls, nothing here sets a timer, and the
 * turn's own beats arrive on the notification stream rather than from here —
 * src/cuts.test.ts holds that by counting call sites.
 *
 * A complete answer is rendered through the reader's own preview route, which
 * is project/api.ts's call site and not a second one here: there is one
 * Markdown parser in this application and it is the server's.
 */

const PARTNER = "/api/partner";
const TURNS = "/api/partner/turns";
/** Stopping the running turn: the turn's id, with this after it. */
const STOP = "/stop";

/** One column of the board as the page is showing it. */
export type Lane = { id: string; title: string; total: number; goals: string[] };

/** Where a human was when they asked, as the page knows it. */
export type Page = {
  section: string;
  path: string;
  tab?: string;
  kind?: string;
  subject?: string;
  title?: string;
  revision?: string;
  filters?: string[];
  /** The board as the page is showing it, lane by lane, after its filters. */
  lanes?: Lane[];
  /** What the open project tab lists, by each row's own key. */
  records?: string[];
  label?: string;
};

export type Outcome = "complete" | "stopped" | "failed" | "refused";

export type Message = {
  id: string;
  turn: string;
  role: "human" | "partner";
  text: string;
  at: string;
  outcome?: Outcome;
  detail?: string;
  activity?: string[];
  key?: string;
  page?: Page;
};

/** Everything the page needs to render the conversation from cold. */
export type Snapshot = {
  runtime: string;
  model: string;
  human: string;
  busy: boolean;
  turn: string;
  partial: string;
  partialSeq: number;
  activity: string[] | null;
  readOnly: string;
  messages: Message[] | null;
};

export type EventKind = "text" | "activity" | "done" | "error" | "stopped";

/** One beat of a running turn, as the stream carries it. */
export type PartnerEvent = {
  turn: string;
  seq: number;
  kind: EventKind;
  text: string;
  at: string;
};

/**
 * A response that was not what was asked for. The Partner's refusals carry
 * more than a sentence: a send while busy is a state the page shows rather
 * than an error, and a runtime that will not start carries the line that
 * installs it.
 */
export class PartnerError extends Error {
  readonly status: number;
  readonly install: string;

  constructor(status: number, reason: string, install = "") {
    super(reason === "" ? `the Partner answered ${String(status)}` : reason);
    this.name = "PartnerError";
    this.status = status;
    this.install = install;
  }
}

/** True when the server refused because a turn is already running. */
export function isBusy(error: unknown): boolean {
  return error instanceof PartnerError && error.status === 409;
}

type Refusal = { error?: string; install?: string };

/**
 * The one request. A body makes it a write, and a write is a POST of JSON;
 * everything else is a read.
 */
async function request<T>(resource: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const sending = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: sending ? "POST" : "GET",
    headers: sending
      ? { Accept: "application/json", "Content-Type": "application/json" }
      : { Accept: "application/json" },
    body: sending ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const refusal = await reasonOf(response);
    throw new PartnerError(response.status, refusal.error ?? "", refusal.install ?? "");
  }
  return (await response.json()) as T;
}

async function reasonOf(response: Response): Promise<Refusal> {
  try {
    return (await response.json()) as Refusal;
  } catch {
    return {};
  }
}

/** The conversation as it stands: read on load and on every reconnect. */
export async function loadPartner(signal?: AbortSignal): Promise<Snapshot> {
  return request<Snapshot>(PARTNER, undefined, signal);
}

/**
 * Ask one question. The key is the page's own, so the same send twice is the
 * same turn once and a retry after a lost answer never asks twice.
 */
export async function sendTurn(key: string, text: string, about: Page): Promise<{ turn: string }> {
  return request<{ turn: string }>(TURNS, { key, text, about });
}

/** Stop the running turn, and read back what it settled as. */
export async function stopTurn(turn: string): Promise<Snapshot> {
  return request<Snapshot>(`${TURNS}/${encodeURIComponent(turn)}${STOP}`, {});
}
