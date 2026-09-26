import { describe, expect, it } from "vitest";

import { CONFLICT, recorder, type Reading, type Written } from "./recording";
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
    return { id: "plans/designs/sessions.md", revision: String(this.revision), source: this.source };
  }
}

class Stale extends Error {
  constructor() {
    super("the file changed since it was opened");
  }
}

const isStale = (error: unknown) => error instanceof Stale;

function entry(text: string, section: Entry["section"] = "Facts"): Entry {
  return { when: "2026-09-26", who: "Wido", text, clause: "a.md", section };
}

const START = "# Session limits\n\n## Facts\n\n## Decisions\n";

describe("one press", () => {
  it("writes the whole source under the reading's revision and refreshes the reading from it", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);

    const outcome = await held.press(entry("the limit is twelve hours"), "fact");

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
    const outcome = await held.press(entry("it counts from last activity", "Decisions"), "decision");
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

    const first = await held.press(entry("the limit is twelve hours"), "fact");
    const second = await held.press(entry("the mobile client renews differently"), "fact");

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
      held.press(entry("first"), "fact"),
      held.press(entry("second"), "fact"),
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
      held.press(entry("a fact"), "fact"),
      held.press(entry("a decision", "Decisions"), "decision"),
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

    const refused = await held.press(entry("the limit is twelve hours"), "fact");

    expect(refused).toMatchObject({ kind: "conflict", reason: CONFLICT });
    expect(record.asked).toHaveLength(1);
    // Nothing of theirs was written, and the record still says what it said.
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["what does the limit protect?"]);
    expect(record.rereads).toBe(1);
    // The reading is the record as it now stands, so the next press composes
    // from it — and the other person's entry survives.
    expect(held.reading().revision).toBe("7");

    const again = await held.press(entry("the limit is twelve hours"), "fact");
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

    await held.press(entry("mine"), "fact");
    await held.press(entry("mine"), "fact");

    expect(record.asked).toHaveLength(2);
    expect(record.asked[1].revision).toBe("4");
    expect(record.asked[1].source).toContain("a question");
  });

  it("does not strand the presses behind it", async () => {
    const record = new Record(START);
    const held = recorder(record.reading(), record.save, record.read, isStale);
    record.revision = 3;

    const both = await Promise.all([held.press(entry("first"), "fact"), held.press(entry("second"), "fact")]);

    // The first meets the moved record; the second is composed from the reread
    // and lands, rather than waiting for ever behind a rejected promise.
    expect(both[0].kind).toBe("conflict");
    expect(both[1].kind).toBe("recorded");
    expect(entriesIn(record.source).map((one) => one.text)).toEqual(["second"]);
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

    const outcome = await held.press(entry("the limit is twelve hours"), "fact");

    expect(outcome).toEqual({ kind: "failed", reason: "the checkout is read-only" });
    expect(record.rereads).toBe(0);
    expect(held.reading().revision).toBe("1");
  });
});
