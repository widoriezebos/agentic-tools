import { describe, expect, it } from "vitest";

import { CONFLICT, ELSEWHERE, movesTheTable, recorder, type Reading, type Written } from "./recording";
import { countsIn, entriesIn, type Entry } from "./sitting";

/**
 * Record it, against one reading, one press at a time.
 *
 * This file is Astra's F1. The document edit replaces a whole source under a
 * revision check: it has no append, and it serializes nothing. So the two things
 * proved here are the two a human would lose without them — two presses in a row
 * landing BOTH entries, and two overlapping presses landing both rather than the
 * second writing over the first.
 *
 * Nothing here waits on a clock. The fake record below hands back the promise
 * each write is waiting on, so a test decides the order things settle in.
 */

/**
 * A record that behaves the way the route does: it refuses a write whose
 * revision is not the one it holds, and it answers a write it took with the
 * document as it now reads.
 */
class Record {
  source: string;
  revision = 1;
  /** Every write it was asked for, in the order it was asked. */
  asked: { source: string; revision: string }[] = [];
  /** How many times it was read again. */
  rereads = 0;

  constructor(source: string) {
    this.source = source;
  }

  save = (id: string, source: string, revision: string): Promise<Written> => {
    this.asked.push({ source, revision });
    if (revision !== String(this.revision)) {
      return Promise.reject(new Stale());
    }
    this.source = source;
    this.revision += 1;
    return Promise.resolve({ revision: String(this.revision), source });
  };

  read = (): Promise<Written> => {
    this.rereads += 1;
    return Promise.resolve({ revision: String(this.revision), source: this.source });
  };

  reading(): Reading {
    return { id: SUBJECT, revision: String(this.revision), source: this.source };
  }
}

class Stale extends Error {
  constructor() {
    super("the file changed since it was opened");
  }
}

const isStale = (error: unknown) => error instanceof Stale;

/** The record every press below is for, and the one this recorder holds. */
const SUBJECT = "plans/designs/sessions.md";

let minted = 0;

/**
 * One entry, with an identity of its own: each call mints a fresh mark, because
 * the mark is what says which deposit an entry was recorded from and two entries
 * of one press would be the one thing it exists to prevent.
 */
function entry(text: string, section: Entry["section"] = "Facts"): Entry {
  minted += 1;
  return { when: "2026-09-26", who: "Wido", text, clause: "a.md", section, mark: `deposit:t#${String(minted)}` };
}

const START = "# Session limits\n\n## Facts\n\n## Decisions\n";

describe("one press", () => {
  it("writes the whole source under the reading's revision and refreshes the reading from it", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);

    const outcome = await held.press(entry("the limit is twelve hours"), "fact", SUBJECT);

    expect(outcome.kind).toBe("recorded");
    expect(record.asked).toHaveLength(1);
    expect(record.asked[0].revision).toBe("1");
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["the limit is twelve hours"]);
    // The reading came from the write, so the next press is against what the
    // record now says rather than against what it said when the sitting began.
    expect(held.reading().revision).toBe("2");
    expect(held.reading().source).toBe(record.source);
  });

  it("names the section the record took it into", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    const outcome = await held.press(entry("it counts from last activity", "Decisions"), "decision", SUBJECT);
    expect(outcome).toMatchObject({ kind: "recorded", section: "Decisions" });
  });
});

describe("two presses", () => {
  // Both entries land. Without the reading being refreshed from the first write,
  // the second would be refused; without the composition happening inside the
  // queue, it would be composed from a source that has lost the first entry.
  it("in a row land both entries", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);

    const first = await held.press(entry("the limit is twelve hours"), "fact", SUBJECT);
    const second = await held.press(entry("the mobile client renews differently"), "fact", SUBJECT);

    expect([first.kind, second.kind]).toEqual(["recorded", "recorded"]);
    expect(entriesIn(record.source).map((one) => one.text)).toEqual([
      "the limit is twelve hours",
      "the mobile client renews differently",
    ]);
    expect(record.rereads).toBe(0);
  });

  // The one that used to lose an entry. Both are pressed before either has
  // settled; the queue makes the second compose from what the first wrote.
  it("at once are serialized, and neither is lost", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);

    const both = await Promise.all([
      held.press(entry("first"), "fact", SUBJECT),
      held.press(entry("second"), "fact", SUBJECT),
    ]);

    expect(both.map((one) => one.kind)).toEqual(["recorded", "recorded"]);
    expect(record.asked.map((one) => one.revision)).toEqual(["1", "2"]);
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["first", "second"]);
    expect(countsIn(record.source).Facts).toBe(2);
  });

  it("of different kinds land in their own sections", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    await Promise.all([
      held.press(entry("a fact"), "fact", SUBJECT),
      held.press(entry("a decision", "Decisions"), "decision", SUBJECT),
    ]);
    expect(countsIn(record.source)).toEqual({ Facts: 1, Proposals: 0, Decisions: 1, "Open questions": 0 });
  });
});

describe("a record that moved", () => {
  // Nothing is written, the card keeps its words, the reading is taken again,
  // and pressing once more is one more press against what the record now says.
  it("refuses the press, rereads, and the same press then lands", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    // Somebody else saved the record: its revision has moved past the reading.
    record.revision = 7;
    record.source = `${START}\n## Open questions\n\n- 2026-09-26 · Someone · what does the limit protect?\n`;

    const refused = await held.press(entry("the limit is twelve hours"), "fact", SUBJECT);

    expect(refused).toMatchObject({ kind: "conflict", reason: CONFLICT });
    expect(record.asked).toHaveLength(1);
    // Nothing of theirs was written, and the record still says what it said.
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["what does the limit protect?"]);
    expect(record.rereads).toBe(1);
    // The reading is the record as it now stands, so the next press composes
    // from it — and the other person's entry survives.
    expect(held.reading().revision).toBe("7");

    const again = await held.press(entry("the limit is twelve hours"), "fact", SUBJECT);
    expect(again.kind).toBe("recorded");
    expect(entriesIn(record.source).map((one) => one.text)).toEqual([
      "the limit is twelve hours",
      "what does the limit protect?",
    ]);
  });

  // Stale bytes are never written under a fresh revision: the source that goes
  // is composed from the reread, so the write carries the other person's entry.
  it("is never written over with the source the press was composed from", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    record.revision = 4;
    record.source = `${START}\n## Open questions\n\n- 2026-09-26 · Someone · a question\n`;

    await held.press(entry("mine"), "fact", SUBJECT);
    await held.press(entry("mine"), "fact", SUBJECT);

    expect(record.asked).toHaveLength(2);
    expect(record.asked[1].revision).toBe("4");
    expect(record.asked[1].source).toContain("a question");
  });

  it("does not strand the presses behind it", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    record.revision = 3;

    const both = await Promise.all([
      held.press(entry("first"), "fact", SUBJECT),
      held.press(entry("second"), "fact", SUBJECT),
    ]);

    // The first meets the moved record; the second is composed from the reread
    // and lands, rather than waiting for ever behind a rejected promise.
    expect(both[0].kind).toBe("conflict");
    expect(both[1].kind).toBe("recorded");
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["second"]);
  });
});

/**
 * Sol's first finding, at the recorder's own gate.
 *
 * A card offered in a sitting on A stays on the transcript after that sitting
 * ends. The card refuses the press itself, but the recorder is the gate that has
 * to hold whatever a page does: a press for another record writes nothing here,
 * so words offered to A can never reach B's record.
 */
describe("a press for another record", () => {
  it("writes nothing, and says so", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);

    const refused = await held.press(entry("A's words"), "fact", "plans/designs/other.md");

    expect(refused).toEqual({ kind: "elsewhere", reason: ELSEWHERE });
    expect(record.asked).toHaveLength(0);
    expect(record.rereads).toBe(0);
    expect(entriesIn(record.source)).toHaveLength(0);
    // And it leaves the reading where it was, so the press behind it is still
    // against the record this recorder holds.
    expect(held.reading().revision).toBe("1");
    const mine = await held.press(entry("mine"), "fact", SUBJECT);
    expect(mine.kind).toBe("recorded");
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["mine"]);
  });

  /**
   * And the other half of the same finding: a press that was in flight when the
   * sitting moved answers about the record it wrote to. Its entry is in that
   * record and nothing is lost — but the table on screen is another record's, so
   * that answer must not become it.
   */
  it("and a late answer from the record before does not move the table on screen", async () => {
    const before = new Record(START);
    const landed = await recorder(before.reading(), before.save, before.read, isStale)
      .press(entry("A's fact"), "fact", SUBJECT);

    expect(landed.kind).toBe("recorded");
    // The sitting has moved: the reading on screen is another record's.
    const now: Reading = { id: "plans/designs/other.md", revision: "1", source: START };
    expect(movesTheTable(landed, now)).toBe(false);
    // The sitting has ended: there is no table at all.
    expect(movesTheTable(landed, null)).toBe(false);
    // And on the record it was pressed on, it moves the table, so the guard is
    // saying where the boundary is rather than refusing everything.
    expect(movesTheTable(landed, before.reading())).toBe(true);
  });

  it("and a refusal moves no table at all", () => {
    const standing: Reading = { id: SUBJECT, revision: "1", source: START };
    expect(movesTheTable({ kind: "elsewhere", reason: ELSEWHERE }, standing)).toBe(false);
    expect(movesTheTable({ kind: "failed", reason: "the checkout is read-only" }, standing)).toBe(false);
    // A conflict answered with a reading of this record, which the table takes:
    // it is what the next press composes from.
    expect(movesTheTable({ kind: "conflict", reason: CONFLICT, reading: standing }, standing)).toBe(true);
  });
});

describe("a write that failed for another reason", () => {
  it("is reported as a failure and nothing is reread", async () => {
    const record = new Record(START);
    const held = recorder(
      record.reading(),
      () => Promise.reject(new Error("the checkout is read-only")),
      record.read,
      isStale,
    );

    const outcome = await held.press(entry("the limit is twelve hours"), "fact", SUBJECT);

    expect(outcome).toEqual({ kind: "failed", reason: "the checkout is read-only" });
    expect(record.rereads).toBe(0);
    expect(held.reading().revision).toBe("1");
  });
});
