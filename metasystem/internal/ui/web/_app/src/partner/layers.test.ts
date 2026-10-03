import { describe, expect, it } from "vitest";

import type { Deposit } from "./api";
import {
  answerOf,
  cardsIn,
  decidedSource,
  entriesIn,
  entryOf,
  FIX,
  lineOf,
  NOT_A_PROBLEM,
  recordedIn,
  type Entry,
} from "./sitting";

/**
 * A finding read as a decision (review-findings-read-as-decisions §3): the
 * record carries four lines beside its anchor and its answer — how much it
 * matters, why, what the reviewer recommends and the reason — so a reload and
 * the board read the same card, and one press writes a finding with the
 * decision the person made on it.
 */

const REVIEW = "# Review of g1-s21\n\n- Kind: review\n- Goals: g1-s21\n\n## Findings\n\n## Decisions\n";

const LAYERED: Entry = {
  when: "2026-10-03",
  who: "Wido",
  text: "A press that dies halfway leaves the ledger locked.",
  clause: "internal/owner/owner.go:21-24",
  consequence: "the lock stays held",
  severity: "blocks",
  why: "The next press waits forever.",
  recommends: "must-fix",
  reason: "Release the lock on every way out.",
  answer: FIX,
  section: "Findings",
  mark: "deposit:t1#0",
};

const DEPOSIT: Deposit = {
  kind: "finding",
  text: "nothing tries a press that dies at owner.go:21-24 after e2f9c92",
  anchor: "internal/owner/owner.go:21-24",
  consequence: "the lock stays held",
  severity: "blocks",
  title: "A press that dies halfway leaves the ledger locked.",
  why: "The next press waits forever.",
  recommend: "must-fix",
  reason: "Release the lock on every way out.",
  subject: { kind: "record", id: "plans/reviews/review-of-g1-s21.md", title: "Review of g1-s21" },
  offered: true,
};

describe("a finding's record lines", () => {
  it("carry its severity, why, recommendation and reason beside its anchor and its answer", () => {
    expect(lineOf(LAYERED, "finding")).toBe(
      "- 2026-10-03 · Wido · A press that dies halfway leaves the ledger locked. [d:deposit:t1#0]\n" +
        "  - Anchor: internal/owner/owner.go:21-24\n" +
        "  - Consequence: the lock stays held\n" +
        "  - Severity: blocks\n" +
        "  - Why: The next press waits forever.\n" +
        "  - Recommends: must-fix\n" +
        "  - Reason: Release the lock on every way out.\n" +
        `  - Answer: ${FIX}\n`,
    );
  });

  it("are read back whole, so a reload reads the same card", () => {
    const [read] = entriesIn(`${REVIEW.replace("## Findings\n", `## Findings\n\n${lineOf(LAYERED, "finding")}`)}`);
    expect(read).toEqual(LAYERED);
  });

  it("read a decision's Reason line as its reason still, outside the findings", () => {
    const source = "## Decisions\n\n- 2026-10-03 · Wido · one lock [d:x]\n  - Reason: simpler\n";
    expect(entriesIn(source)[0]).toMatchObject({ clause: "simpler", section: "Decisions" });
    expect(entriesIn(source)[0].reason).toBeUndefined();
  });
});

describe("a finding the Partner offered", () => {
  it("is written under its title, with its layers, and the evidence stays its own", () => {
    const [card] = cardsIn([{ turn: "t1", deposits: [DEPOSIT] }], {}, DEPOSIT.subject?.id ?? "", new Map());
    const entry = entryOf(card, "Wido", "2026-10-03");
    expect(entry).toMatchObject({
      text: "A press that dies halfway leaves the ledger locked.",
      clause: "internal/owner/owner.go:21-24",
      severity: "blocks",
      why: "The next press waits forever.",
      recommends: "must-fix",
      reason: "Release the lock on every way out.",
    });
  });

  it("is recorded with the person's decision in one write, and a recorded one has its answer rewritten", () => {
    const [card] = cardsIn([{ turn: "t1", deposits: [DEPOSIT] }], {}, DEPOSIT.subject?.id ?? "", new Map());
    const entry = entryOf(card, "Wido", "2026-10-03");
    const once = decidedSource(REVIEW, card.id, NOT_A_PROBLEM("the lock is released by the next begin"), entry);
    expect(once).not.toBeNull();
    const read = recordedIn(once ?? "").get(card.id);
    expect(answerOf(read?.answer ?? "")).toBe("not a problem");
    expect(read?.answer).toBe("not a problem — the lock is released by the next begin");
    const twice = decidedSource(once ?? "", card.id, FIX, entry);
    expect(entriesIn(twice ?? "").filter((one) => one.section === "Findings")).toHaveLength(1);
    expect(recordedIn(twice ?? "").get(card.id)?.answer).toBe(FIX);
    expect(decidedSource(REVIEW, "deposit:none#0", FIX, undefined)).toBeNull();
  });
});
