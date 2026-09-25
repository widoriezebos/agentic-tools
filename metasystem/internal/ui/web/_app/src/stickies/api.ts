/**
 * The notepad's four routes, and the only place this section talks to the
 * server.
 *
 * Four requests go through the one request below: the read, when the page
 * loads, and the three acts, when a human makes one. Nothing here polls and
 * nothing here sets a timer — a sticky changes when its owner changes it, and
 * its owner is the human at this keyboard.
 *
 * Every act answers the WHOLE notepad rather than the sticky it touched, so
 * the panel, the badge and the page's own block are one list and cannot come
 * to three opinions about the order or the counts.
 *
 * src/cuts.test.ts holds the call site to a written list, so a request added
 * under any other name fails the guard rather than the review.
 */

const STICKIES = "/api/stickies";
const ONE = "/api/stickies/";
/** Removing one for good: the sticky's id, with this after it. */
const REMOVE = "/remove";

/** What a sticky is about: a ledger goal, or a document by its own path. */
export type About = { kind: "goal" | "record"; id: string };

/** One sticky, as the server keeps it. */
export type Sticky = {
  id: string;
  text: string;
  about: About[];
  createdAt: string;
  updatedAt: string;
  /** "" while it is open; when it was struck off, once it has been. */
  doneAt: string;
};

/**
 * The whole notepad: whose it is, every sticky in the order the panel shows
 * them, and the two counts.
 *
 * `human` is "" on a seat that knows nobody. That is not an error — the seat
 * keeps a notepad of its own — and it is what the panel says "sign in so your
 * stickies are yours" over.
 */
export type Notepad = {
  schemaVersion: number;
  human: string;
  stickies: Sticky[];
  counts: { open: number; done: number };
};

/** What an edit changes. A field left out is a field nobody is changing. */
export type Change = { text?: string; about?: About[]; done?: boolean };

export class NotepadError extends Error {
  readonly status: number;

  constructor(status: number, reason: string) {
    super(reason === "" ? `the notepad answered ${String(status)}` : reason);
    this.name = "NotepadError";
    this.status = status;
  }
}

/** The one request. A body makes it a write, and a write is a POST of JSON. */
async function request(resource: string, body?: unknown, signal?: AbortSignal): Promise<Notepad> {
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
    throw new NotepadError(response.status, await reasonOf(response));
  }
  return (await response.json()) as Notepad;
}

async function reasonOf(response: Response): Promise<string> {
  try {
    const refusal = (await response.json()) as { error?: string };
    return refusal.error ?? "";
  } catch {
    return "";
  }
}

/** This human's notepad as it stands. */
export async function loadStickies(signal?: AbortSignal): Promise<Notepad> {
  return request(STICKIES, undefined, signal);
}

/** Write one, and read back the whole notepad. */
export async function addSticky(text: string, about: About[]): Promise<Notepad> {
  return request(STICKIES, { text, about });
}

/** Change one: its text, what it is about, or whether it is done. */
export async function changeSticky(id: string, change: Change): Promise<Notepad> {
  return request(ONE + encodeURIComponent(id), change);
}

/** Take one off this notepad for good. */
export async function removeSticky(id: string): Promise<Notepad> {
  return request(`${ONE}${encodeURIComponent(id)}${REMOVE}`, {});
}

/** What went wrong, in the server's own words where it gave any. */
export function failureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
