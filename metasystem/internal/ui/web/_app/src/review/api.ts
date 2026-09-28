/**
 * The review room's desk reads (g1-s65 D4), and the only place the room talks
 * to the server beyond the conversation's own call site.
 *
 * Three reads over one review record, each answered from the candidate's own
 * tree rather than this checkout: a source file at a range of lines with the
 * changed lines marked, the change index, and one file's hunks. Each is made
 * when a human puts something on the desk, and at no other time; nothing here
 * polls and nothing here sets a timer.
 */

const REVIEW = "/api/review/";
const SOURCE = "/source";
const CHANGES = "/changes";

/** One line of a source read, marked where the change touched it. */
export type SourceLine = { number: number; text: string; touched?: boolean };

/** One text file of the reviewed tree at a range of lines. */
export type Source = { path: string; commit: string; from: number; to: number; total: number; lines: SourceLine[] };

/** One file of the change, with its counts. */
export type ChangedFile = { path: string; added: number; deleted: number; binary?: boolean };

/** The change as a whole. `current` is the branch now; `moved` says it is not the reviewed tip. */
export type Changes = {
  goal: string;
  comparisons: { from: string; to: string }[];
  files: ChangedFile[];
  supplied: number;
  total: number;
  current: string;
  moved: boolean;
};

/** One line of a hunk. */
export type DiffLine = { kind: "context" | "added" | "deleted"; old?: number; new?: number; text: string };

/** One file's change, as hunks, across the comparisons that touched it. */
export type FileDiff = {
  path: string;
  binary?: boolean;
  parts: { from: string; to: string; hunks: { header: string; lines: DiffLine[] }[] }[];
  supplied: number;
  total: number;
};

/** A read the server refused, in its own words. */
export class ReviewError extends Error {
  readonly status: number;

  constructor(status: number, reason: string) {
    super(reason === "" ? `the review read answered ${String(status)}` : reason);
    this.name = "ReviewError";
    this.status = status;
  }
}

function base(record: string): string {
  return REVIEW + record.split("/").map((segment) => encodeURIComponent(segment)).join("/");
}

/** The one request. */
async function read<T>(resource: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(resource, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    let said = "";
    try {
      said = ((await response.json()) as { error?: string }).error ?? "";
    } catch {
      said = "";
    }
    throw new ReviewError(response.status, said);
  }
  return (await response.json()) as T;
}

/** A file of the reviewed tree from line `from` to line `to`; zero is "from the start" and "as far as allowed". */
export function loadSource(record: string, path: string, from: number, to: number, signal?: AbortSignal): Promise<Source> {
  const query = new URLSearchParams({ path });
  if (from > 0) {
    query.set("from", String(from));
  }
  if (to > 0) {
    query.set("to", String(to));
  }
  return read<Source>(`${base(record)}${SOURCE}?${query.toString()}`, signal);
}

/** The change index; with since, what changed on the branch after the reviewed tip. */
export function loadChanges(record: string, since = false, signal?: AbortSignal): Promise<Changes> {
  return read<Changes>(`${base(record)}${CHANGES}${since ? "?since=1" : ""}`, signal);
}

/** One file's hunks; with since, between the reviewed tip and the branch now. */
export function loadDiff(record: string, path: string, since = false, signal?: AbortSignal): Promise<FileDiff> {
  const query = new URLSearchParams({ path });
  if (since) {
    query.set("since", "1");
  }
  return read<FileDiff>(`${base(record)}${CHANGES}?${query.toString()}`, signal);
}
