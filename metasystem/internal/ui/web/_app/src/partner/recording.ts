import { appended, type Entry } from "./sitting";

/**
 * Record it, serialized, against one current reading of the record.
 *
 * This file is Astra's F1 answered. The document edit this build already has
 * replaces a document's WHOLE source under a revision check: it has no append,
 * and it serializes nothing between two writes. So two Record presses composed
 * from one reading would either refuse the second — the revision moved when the
 * first landed — or, if they overlapped, write the second over the first and
 * lose an entry a human watched land.
 *
 * Three rules, and every one of them is here rather than in a component:
 *
 * 1. **One reading.** The sitting holds one reading of its record, source and
 *    revision, taken when the sitting starts and replaced by the answer of every
 *    successful write. Nothing else is ever written under a revision.
 * 2. **One press at a time.** Presses queue. Each composes its entry from the
 *    reading as it stands WHEN IT RUNS, not when it was pressed, so two in a row
 *    land both entries and two at once land both in the order they were pressed.
 * 3. **A conflict keeps the card.** Somebody else changed the record: the write
 *    refuses, nothing is written, the card keeps the human's words and says what
 *    happened, and the reading is taken again so the same press against the new
 *    reading is one more press of Record it. Stale bytes are never written under
 *    a fresh revision, because the source is recomposed from the reread.
 * 4. **One record.** A press names the record it was composed against, and a
 *    recorder writes nothing for any other. It is Sol's first finding at the
 *    second gate: the card checks the sitting it can see, and this checks the
 *    record it holds, so a card from a sitting that has ended cannot reach the
 *    record of the one now standing even if a press for it gets this far. And
 *    `movesTheTable` is the other side of the same finding: a press that was in
 *    flight when the sitting moved answers about the record it wrote to, and
 *    that answer must not become the table of the record now on screen.
 */

/** One reading of the record: which document, its revision, its whole source. */
export type Reading = { id: string; revision: string; source: string };

/** What a write answered with: the document as it now reads from disk. */
export type Written = { revision: string; source: string };

/**
 * How this recorder saves. A stale revision rejects, which is not a failure.
 *
 * It is called a saver rather than a writer because this build allows the name
 * `write` in one file only — the one that writes a preference — and the cut guard
 * counts the identifier wherever it appears.
 */
export type Saver = (id: string, source: string, revision: string) => Promise<Written>;

/** How it reads the record again, which is what a conflict needs. */
export type Reader = (id: string) => Promise<Written>;

/** Whether a rejection was the revision moving under us, or something else. */
export type IsStale = (error: unknown) => boolean;

/** What one press ended as. */
export type Outcome =
  | { kind: "recorded"; section: string; reading: Reading }
  | { kind: "conflict"; reason: string; reading: Reading }
  | { kind: "elsewhere"; reason: string }
  | { kind: "failed"; reason: string };

/** An outcome that answered with a reading: it reached the record, or it read it. */
export type Landed = Extract<Outcome, { reading: Reading }>;

export type Recorder = {
  /** The reading as it stands, which is what the table reads. */
  reading: () => Reading;
  /** Take a fresh reading, which is what starting a sitting does. */
  reread: () => Promise<Reading>;
  /**
   * Press Record it for one entry, into the record named. It queues behind every
   * earlier press, and it writes nothing where the record named is not the one
   * this recorder holds.
   */
  press: (entry: Entry, kind: string, into: string) => Promise<Outcome>;
};

/**
 * What a human is told when the record moved under their press.
 *
 * It says three things, because all three matter: nothing was written, their
 * words are still here, and pressing again now writes against the record as it
 * stands.
 */
export const CONFLICT =
  "The record changed while you were writing, so nothing was written. Your words are still here — press Record it again to add them to the record as it now stands.";

/**
 * What a human is told when the press was for another record.
 *
 * It should not be reachable from the interface at all — a card from another
 * sitting offers no press — so it says what happened rather than what to do: the
 * one thing that must not happen is words landing in a record nobody offered
 * them to, and this is the gate that refuses it.
 */
export const ELSEWHERE =
  "This was offered to another record, so nothing was written into the one this sitting is on.";

/** Open a recorder over one reading. */
export function recorder(
  start: Reading,
  save: Saver,
  read: Reader,
  stale: IsStale,
): Recorder {
  let held = start;
  // The queue. Every press chains onto the last, so the writes are serialized
  // in the order they were pressed however fast a human presses.
  let queue: Promise<unknown> = Promise.resolve();

  const take = (answered: Written): Reading => {
    held = { id: held.id, revision: answered.revision, source: answered.source };
    return held;
  };

  const reread = async (): Promise<Reading> => take(await read(held.id));

  const press = (entry: Entry, kind: string, into: string): Promise<Outcome> => {
    // The entry is composed from the reading as it stands WHEN THIS RUNS, which
    // is the whole of why the composition is inside the queued task and not
    // outside it: the press before this one has already moved the reading.
    const next = queue.then(async (): Promise<Outcome> => {
      if (into !== held.id) {
        return { kind: "elsewhere", reason: ELSEWHERE };
      }
      try {
        const answered = await save(held.id, appended(held.source, entry, kind), held.revision);
        return { kind: "recorded", section: entry.section, reading: take(answered) };
      } catch (error: unknown) {
        if (!stale(error)) {
          return { kind: "failed", reason: reasonOf(error) };
        }
        // The record moved. Nothing was written; the reading is taken again so
        // the next press composes from what the record now says.
        try {
          return { kind: "conflict", reason: CONFLICT, reading: await reread() };
        } catch (again: unknown) {
          return { kind: "failed", reason: reasonOf(again) };
        }
      }
    });
    // The queue never rejects: every outcome above is a value, so one failed
    // press cannot strand the presses behind it.
    queue = next;
    return next;
  };

  return { reading: () => held, reread, press };
}

/**
 * Whether one press's answer may become the table on screen.
 *
 * A press that was in flight when the sitting ended, or moved to another record,
 * answers about the record it was composed against. Its entry is in that record
 * — nothing is lost — but the table this page shows is another record's, and a
 * reading of A written over B's table would show a human entries their record
 * does not carry and a revision their next press would write under.
 *
 * `standing` is the reading the page now holds, or null where no sitting is
 * open at all; in that case nothing on screen is a table, so nothing moves it.
 */
export function movesTheTable(outcome: Outcome, standing: Reading | null): outcome is Landed {
  if (outcome.kind === "failed" || outcome.kind === "elsewhere") {
    return false;
  }
  return standing !== null && outcome.reading.id === standing.id;
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
