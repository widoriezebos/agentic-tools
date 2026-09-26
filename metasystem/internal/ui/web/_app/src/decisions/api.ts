/**
 * The decisions resource, and the only place this section talks to the server.
 *
 * One read, made when the pane mounts, again when a human presses the
 * section's refresh, and again after an act completes — and nowhere else: no
 * stream, no socket, no timer, and no window event refetches it.
 * src/cuts.test.ts holds that by counting call sites, so a second request
 * written under any other name fails the guard rather than the review.
 *
 * The one other resource this page reads is the backlog's, through the board's
 * own client: the approve and withdraw sheet needs the whole backlog payload
 * beside the row it is opened on — the authority, the budget law and the rows
 * it prefills from — and there is one client for that in this build.
 *
 * The shapes below are the server's, field for field. Every one of them is
 * written by the composing package whether it has anything to say or not, so
 * nothing here has to tell an absent field from an empty one.
 */

import type { Row } from "../backlog/api";

const DECISIONS = "/api/decisions";

/** Where one row opens, said as what kind of thing it is, never as an address. */
export type Where = { kind: string; id: string };

/**
 * What a row asks this human to do, where this interface has the act.
 *
 * Two are the board's. The other two are this page's own, admitted from a
 * signed-in browser under R-125-m1u: park is "Not now", and unpark returns a
 * paused goal to the queue.
 */
export type Act = "approve" | "withdraw" | "park" | "unpark" | "";

/** One thing that is waiting on a human. */
export type Need = {
  kind: string;
  id: string;
  title: string;
  /** What is being asked, in one sentence, from the record. */
  asked: string;
  by: string;
  since: string;
  /** The instant the record says this must be answered by, or "". */
  deadline: string;
  /** What the machinery does if this human does nothing. */
  silence: string;
  /** The asker's own recommendation, where the record carries one. */
  recommend: string;
  where: Where;
  act: Act;
  /** The terminal command that makes this decision, where that is the way. */
  command: string;
  /** The whole backlog row, for the rows whose act is the board's sheet. */
  row: Row | null;
  /**
   * Whether this row was recorded after the start of the window below.
   *
   * The server decides it, because the server is what read the dates and what
   * knows when this human was last here. It is by recorded dates and is honest
   * about them rather than exact: an instant is compared as an instant, a
   * calendar date counts from the window's own day, and a row nothing dated is
   * never new.
   */
  new: boolean;
  /**
   * The register row a ruling review names: what was actually ruled, why, who
   * owns it, and the schedule. Every other kind carries them empty.
   */
  words: string;
  context: string;
  owner: string;
  class: string;
  due: string;
  /** Where the record this row is about lives, relative to the checkout. */
  path: string;
  /** The goals a landed design named, with where each one stands. */
  goals: GoalState[];
};

/** One goal a record names, and where the ledger says it stands. */
export type GoalState = { id: string; state: string };

/**
 * The window a page's "new" was decided against: the end of this human's
 * previous visit to THIS page, or a day back on a first one.
 *
 * It travels so the page can say what it means by new rather than leaving a
 * reader to infer a boundary from the dots.
 */
export type Visit = {
  /** The start of the window in RFC3339, or "" where there was none. */
  since: string;
  /** Whether the window is a first visit's day rather than a previous visit. */
  first: boolean;
};

/** One row of the rulings register, whole. */
export type Ruling = {
  id: string;
  date: string;
  words: string;
  context: string;
  owner: string;
  class: string;
  due: string;
  event: string;
  /** The review condition exactly as the register wrote it. */
  condition: string;
  /** A valid due date at or before the day this was read. */
  duePassed: boolean;
  /** The ledger goals these words name verbatim. */
  mentions: string[];
};

/** One decided thing that is not a ruling. */
export type Item = { id: string; title: string; note: string; at: string; where: Where };

/** One approval on record, with the whole row it was recorded on. */
export type Approved = {
  id: string;
  title: string;
  by: string;
  at: string;
  authority: string;
  expired: boolean;
  row: Row;
};

/** One goal a person paused, with the whole of what they said. */
export type NotNow = {
  id: string;
  title: string;
  by: string;
  at: string;
  /** The reason the park recorded, which is the whole of why. */
  because: string;
  /** The goal this park waits for, where a human directed it at one. */
  blocker: string;
  where: Where;
};

export type Decided = {
  rulings: Ruling[];
  /** The register's broken rows, in the steward's own words. */
  defects: string[];
  decisions: Item[];
  answered: Item[];
  approved: Approved[];
  /** Every park a person made, newest first. */
  notNow: NotNow[];
};

/**
 * The figures the page shows: the whole inbox, its two blocks, and the whole
 * register. `asked` and `waiting` always sum to `needsYou` — the page splits
 * the one list the server composed rather than reading two.
 */
export type Counts = { needsYou: number; asked: number; waiting: number; rulings: number };

export type Page = {
  schemaVersion: number;
  readAt: string;
  /** True when nothing proves a human on this seat. */
  signIn: boolean;
  needsYou: Need[];
  decided: Decided;
  counts: Counts;
  /** The window every row's `new` was decided against. */
  visit: Visit;
  /**
   * Where the rulings register is, relative to the checkout — which is what
   * the document reader opens a path against. Every register destination in
   * this payload names it.
   */
  register: string;
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

export async function loadDecisions(signal?: AbortSignal): Promise<Page> {
  const response = await fetch(DECISIONS, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new ResourceError(DECISIONS, response.status, (await reasonOf(response)).error ?? "");
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
