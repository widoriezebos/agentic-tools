/**
 * The application resource, and the only place this section talks to the
 * server.
 *
 * One read, made when the pane mounts and again when a human presses the
 * section's refresh — and nowhere else: no stream, no socket, no timer, and no
 * window event refetches it. src/cuts.test.ts holds that by counting call
 * sites, so a second request written under any other name fails the guard
 * rather than the review.
 *
 * The shapes below are the server's, field for field. Every list is written by
 * the composing package whether it has anything to say or not, so nothing here
 * has to tell an absent list from an empty one. The one nullable field is the
 * engine build, and it is null on purpose: a seat that has published no
 * presence record has no build to name, and the page says that in words rather
 * than showing a blank.
 */

const APPLICATION = "/api/application";

/**
 * This seat's last published engine build, from its own presence record.
 *
 * It is a record at ONE TICK and never a statement about what is running now,
 * which is why `publishedAt` travels with it.
 */
export type Engine = { build: string; generation: number; publishedAt: string };

/** The header's own line: the whole history, this month of it, and the window. */
export type Counts = { landed: number; thisMonth: number; new: number };

/** One concluded goal, in the ledger's own words. */
export type Landed = {
  id: string;
  intent: string;
  /** The one sentence written when the goal concluded, whole. */
  concluded: string;
  /** When that conclusion was written, or "" where nothing dated it. */
  doneAt: string;
  labels: string[];
  arc: string;
  /** Recorded after the start of the window below. An undated row never is. */
  new: boolean;
};

/**
 * One row of the known-issues register, by position, whatever the header
 * called the columns.
 */
export type Problem = {
  id: string;
  date: string;
  /** The third column: the symptom and its evidence, or the issue. */
  what: string;
  /** The fourth: what it costs when it bites. */
  consequence: string;
  /** The fifth, whose title differs between the two column sets. */
  lever: string;
  /** The sixth, whole, with its opening word intact. */
  status: string;
  open: boolean;
};

export type Problems = {
  /** The header's own six names, kept as supplied. */
  columns: string[];
  open: Problem[];
  concluded: Problem[];
  /** How many rows the reader refused, which the block's one line counts. */
  unread: number;
  /** Those rows in the reader's own words. */
  defects: string[];
  /** Where the register is relative to the checkout, so the link opens. */
  register: string;
};

/** One link into the Project pane's document reader. */
export type Document = { title: string; path: string };

/** The window every row's `new` was decided against. */
export type Visit = { since: string; first: boolean };

export type Page = {
  schemaVersion: number;
  readAt: string;
  subject: string;
  mode: string;
  /** Null where this seat has published no presence record. */
  engine: Engine | null;
  counts: Counts;
  landed: Landed[];
  problems: Problems;
  docs: Document[];
  visit: Visit;
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

export async function loadApplication(signal?: AbortSignal): Promise<Page> {
  const response = await fetch(APPLICATION, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new ResourceError(APPLICATION, response.status, (await reasonOf(response)).error ?? "");
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
