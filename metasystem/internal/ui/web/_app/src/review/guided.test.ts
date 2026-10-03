import { describe, expect, it } from "vitest";

import {
  pillOf,
  recordedLine,
  retippedAfresh,
  reviewedOf,
  roomFindingsOf,
  verdictToPerform,
  changedSince,
  verdictRefused,
  writeRefused,
  VERSION_CHANGED,
  VERSION_MOVED,
  nodPlan,
  verdictAnswers,
} from "./room";
import type { Deposit } from "../partner/api";
import { CONFLICT } from "../partner/recording";
import { cardsIn, decidedSource, EARLIER, entriesIn, entryOf, FIX, lineOf, recordedIn } from "../partner/sitting";

/**
 * The guided review's reading of the record and its acts (review-findings-
 * read-as-decisions §3, folded with Astra's round 1): which findings stand and
 * which are about an earlier version, the record moved on to the current
 * version with nothing carried forward by itself (RF-03), the verdict bound to
 * the version and the saved record the person saw (RF-02), Try it at the
 * reviewed version (RF-04), and the banner after the verdict, in words.
 */

const TIP = "a1".repeat(20);
const NOW = "c2".repeat(20);
const RECORD = "plans/reviews/review-of-g1-s21.md";
const SUBJECT = { kind: "record", id: RECORD, title: "Review of g1-s21" };

function head(tip: string): string {
  return `# Review of g1-s21\n\n- Kind: review\n- Goals: g1-s21\n- Reviewed: ${tip} (the tip of goal/g1-s21)\n\n## Facts\n\n## Findings\n\n## Outcome\n`;
}

function deposit(over: Partial<Deposit> = {}): Deposit {
  return {
    kind: "finding", text: "nothing tries a press that dies at owner.go:21-24", anchor: "internal/owner/owner.go:21-24",
    consequence: "the lock stays held", severity: "blocks", title: "A press that dies halfway leaves the ledger locked.",
    why: "The next press waits forever.", recommend: "must-fix", reason: "Release the lock on every way out.",
    subject: SUBJECT, offered: true, ...over,
  };
}

describe("the findings a room reads", () => {
  it("are the Partner's offered findings, with the answer the record carries, and the record's own", () => {
    const cards = cardsIn([{ turn: "t1", deposits: [deposit(), deposit({ title: "Only the happy path is tested.", severity: "fix" })] }],
      {}, RECORD, new Map());
    const recorded = head(TIP).replace("## Findings\n", `## Findings\n\n${lineOf({
      when: "2026-10-03", who: "Wido", text: "A press that dies halfway leaves the ledger locked.", clause: "internal/owner/owner.go:21-24",
      section: "Findings", mark: "deposit:t1#0", severity: "blocks", why: "The next press waits forever.", recommends: "must-fix",
      reason: "Release the lock on every way out.", answer: FIX,
    }, "finding")}- 2026-10-03 · Wido · My own: the log says nothing. [d:local-1]\n  - Answer: ${FIX}\n`);
    const read = roomFindingsOf(cardsIn([{ turn: "t1", deposits: [deposit(), deposit({ title: "Only the happy path is tested.", severity: "fix" })] }],
      {}, RECORD, recordedIn(recorded)), entriesIn(recorded), []);
    expect(cards).toHaveLength(2);
    expect(read.current.map((one) => [one.title, one.recorded, one.answer])).toEqual([
      ["A press that dies halfway leaves the ledger locked.", true, FIX],
      ["Only the happy path is tested.", false, ""],
      ["My own: the log says nothing.", true, FIX],
    ]);
    expect(read.current[0].evidence).toBe("nothing tries a press that dies at owner.go:21-24");
    expect(read.earlier).toEqual([]);
  });

  it("are each one once: the same finding offered again, as a resumed session offers it, is the one it was", () => {
    const again = cardsIn([{ turn: "t1", deposits: [deposit()] }, { turn: "t2", deposits: [deposit({ title: " a press that dies halfway leaves the ledger locked. " })] }],
      {}, RECORD, new Map());
    expect(roomFindingsOf(again, [], []).current.map((one) => one.id)).toEqual(["deposit:t1#0"]);
    const recorded = head(TIP).replace("## Findings\n", `## Findings\n\n- 2026-10-03 · Wido · A press that dies halfway leaves the ledger locked. [d:deposit:t2#0]\n  - Answer: ${FIX}\n`);
    const kept = roomFindingsOf(cardsIn([{ turn: "t1", deposits: [deposit()] }, { turn: "t2", deposits: [deposit()] }], {}, RECORD, recordedIn(recorded)),
      entriesIn(recorded), []);
    expect(kept.current.map((one) => [one.id, one.answer])).toEqual([["deposit:t2#0", FIX]]);
  });

  it("keep two different findings that share a title apart, each with its own decision (fix round 1, F-1)", () => {
    const two = cardsIn([{ turn: "t1", deposits: [
      deposit(),
      deposit({ text: "the reconcile path also dies at owner.go:40 without releasing", anchor: "internal/owner/owner.go:40",
        severity: "fix", recommend: "fix-later" }),
    ] }], {}, RECORD, new Map());
    const read = roomFindingsOf(two, [], []).current;
    expect(read.map((one) => [one.id, one.severity])).toEqual([["deposit:t1#0", "blocks"], ["deposit:t1#1", "fix"]]);
    // And two the record carries, by hand, under one title and at two places.
    const hand = head(TIP).replace("## Findings\n", "## Findings\n\n- 2026-10-03 · Wido · One title. [d:local-1]\n  - Anchor: a.go:1\n" +
      `  - Answer: ${FIX}\n- 2026-10-03 · Wido · One title. [d:local-2]\n  - Anchor: b.go:2\n  - Answer: ${FIX}\n`);
    expect(roomFindingsOf([], entriesIn(hand), []).current.map((one) => one.id)).toEqual(["local-1", "local-2"]);
  });

  it("keep two deposited findings with one title and one text apart where they sit in two places (fix round 4)", () => {
    const two = cardsIn([{ turn: "t1", deposits: [
      deposit(),
      deposit({ anchor: "internal/owner/owner.go:40-44" }),
      deposit({ severity: "fix", recommend: "fix-later" }),
    ] }], {}, RECORD, new Map());
    expect(roomFindingsOf(two, [], []).current.map((one) => one.id)).toEqual(["deposit:t1#0", "deposit:t1#1", "deposit:t1#2"]);
  });

  it("leave a finding not offered, or offered to another sitting, out", () => {
    const cards = cardsIn([{ turn: "t1", deposits: [deposit({ offered: false, notOffered: "no" }),
      deposit({ subject: { kind: "record", id: "elsewhere.md", title: "" } })] }], {}, RECORD, new Map());
    expect(roomFindingsOf(cards, [], []).current).toEqual([]);
  });

  it("mark as earlier what was raised before this version's opening, and what the record moved there (RF-03)", () => {
    const cards = cardsIn([{ turn: "t1", deposits: [deposit()] }, { turn: "t3", deposits: [deposit({ title: "Raised again." })] }],
      {}, RECORD, new Map());
    const read = roomFindingsOf(cards, [], ["t1"]);
    expect(read.earlier.map((one) => one.title)).toEqual(["A press that dies halfway leaves the ledger locked."]);
    expect(read.current.map((one) => one.title)).toEqual(["Raised again."]);
  });
});

describe("a send-back's decisions through the next version (fix round 1, F-2)", () => {
  it("keeps a follow-up the reviewer recommended, recorded with the send-back, once the record moves on", () => {
    const cards = cardsIn([{ turn: "t1", deposits: [deposit(), deposit({ text: "only the happy path is tested", title: "Only the happy path is tested.",
      severity: "fix", recommend: "fix-later", anchor: "internal/owner/owner_test.go:5-13" })] }], {}, RECORD, new Map());
    const read = roomFindingsOf(cards, [], []).current;
    let source = head(TIP);
    for (const one of verdictAnswers("send back", nodPlan(read), "", { [read[1].id]: "tests-for-failures" })) {
      const card = cards.find((each) => each.id === one.id);
      source = decidedSource(source, one.id, one.answer, card === undefined ? undefined : entryOf(card, "Wido", "2026-10-03")) ?? source;
    }
    const earlier = entriesIn(retippedAfresh(source, NOW) ?? "").filter((one) => one.section === EARLIER);
    expect(earlier.map((one) => [one.text, one.answer])).toEqual([
      ["A press that dies halfway leaves the ledger locked.", `${FIX} (as the reviewer recommended)`],
      ["Only the happy path is tested.", "follow-up — goal tests-for-failures"],
    ]);
  });
});

describe("Review the current version (RF-03)", () => {
  it("moves the record to the current version and its findings to Earlier findings, answers and all", () => {
    const decided = head(TIP).replace("## Findings\n", `## Findings\n\n- 2026-10-03 · Wido · A finding. [d:deposit:t1#0]\n  - Severity: blocks\n  - Answer: ${FIX}\n`);
    const moved = retippedAfresh(decided, NOW) ?? "";
    expect(reviewedOf(moved).tip).toBe(NOW);
    expect(reviewedOf(moved).previously).toEqual([TIP]);
    const entries = entriesIn(moved);
    expect(entries.filter((one) => one.section === "Findings")).toEqual([]);
    expect(entries.filter((one) => one.section === EARLIER).map((one) => [one.text, one.answer, one.severity]))
      .toEqual([["A finding.", FIX, "blocks"]]);
    // A deposit the record moved stays recorded, so it is not offered again as new.
    expect(recordedIn(moved).has("deposit:t1#0")).toBe(true);
    // Moved again, the earlier ones gather under the one section.
    const twice = retippedAfresh(moved.replace("## Findings\n", "## Findings\n\n- 2026-10-04 · Wido · Another. [d:deposit:t5#0]\n  - Answer: unanswered\n"), TIP) ?? "";
    expect(entriesIn(twice).filter((one) => one.section === EARLIER).map((one) => one.text)).toEqual(["A finding.", "Another."]);
    expect(retippedAfresh("# no head\n", NOW)).toBeNull();
  });
});

describe("the verdict the room performs", () => {
  it("names the version and the saved record the person decided on (RF-02)", () => {
    const performed = verdictToPerform({ kind: "outcome", verdict: "clear to land" }, "review", RECORD, head(TIP), null, "blob:7f3e");
    expect(performed?.asked).toEqual({ record: RECORD, verdict: "clear-to-land", brief: "", work: "", tip: TIP, revision: "blob:7f3e" });
  });

  it("is said in words once it is on the goal", () => {
    expect(recordedLine({ verdict: "clear-to-land", tip: TIP, by: "Wido" }, 0, "g1-s21", ["tests-for-failures"])).toBe(
      "Recorded on g1-s21 as your verdict: looks good, land it. The seat lands it on its next turn. 1 follow-up opened: tests-for-failures.",
    );
    expect(recordedLine({ verdict: "clear-to-land", tip: TIP, by: "Wido" }, 0, "g1-s21", [])).toBe(
      "Recorded on g1-s21 as your verdict: looks good, land it. The seat lands it on its next turn.",
    );
    expect(recordedLine({ verdict: "send-back", tip: TIP, by: "Wido" }, 2, "g1-s21", [])).toBe(
      "Recorded on g1-s21 as your verdict: send it back. The builder gets your 2 must-fix decisions as a correction, and the goal leaves Review until it comes back fixed.",
    );
  });
});

describe("Try it", () => {
  const running = { goal: "g1-s21", state: "running", readiness: "answering", address: "127.0.0.1:7981", commit: TIP, said: "" };
  it("says what Start does and where the version runs, in words (RF-04)", () => {
    expect(pillOf({ goal: "g1-s21", state: "stopped", readiness: "no-probe", said: "" }, TIP)).toMatchObject({
      state: "stopped", run: true, stop: false,
      words: "Start runs this exact version on this computer, on a port of its own, so you can click through it. It stays up until you press Stop.",
    });
    expect(pillOf(running, TIP)).toMatchObject({ state: "reviewed", words: "Running on port 7981.", stop: true });
    expect(pillOf({ ...running, commit: NOW }, TIP).words).toBe(
      "Running on port 7981, but not the version you are reviewing: what runs is another version of this goal.",
    );
    for (const said of [pillOf(running, TIP).words, pillOf(null, TIP).words, pillOf({ ...running, state: "starting" }, TIP).words]) {
      expect(said).not.toMatch(/candidate|tip|[0-9a-f]{7}/u);
    }
  });
});

describe("a verdict refused because a newer version exists (fix round 1, F-3)", () => {
  it("says so in words with the one act, and a changed review as before", () => {
    expect(verdictRefused(VERSION_MOVED, head(TIP), head(TIP))).toBe(
      "Your verdict was not recorded: a newer version of this goal exists, and a verdict counts only for the version " +
        "that would land. The page shows it now: press Review the current version, then decide.",
    );
    expect(verdictRefused(VERSION_CHANGED, head(TIP), head(NOW))).toBe(changedSince(head(TIP), head(NOW)));
  });
});

describe("a verdict refused because the review changed (RF-02)", () => {
  it("says what changed: another version, or the findings another room added", () => {
    const before = head(TIP);
    expect(changedSince(before, head(NOW))).toBe(
      "Your verdict was not recorded: this review now names another version than the one you decided on. It shows that version now; decide again.",
    );
    const added = before.replace("## Findings\n", "## Findings\n\n- 2026-10-03 · Ann · The log says nothing. [d:local-9]\n  - Answer: unanswered\n");
    expect(changedSince(before, added)).toBe(
      "Your verdict was not recorded: this review changed after you decided. Added since: The log says nothing. It shows the review as it stands now; decide again.",
    );
    expect(changedSince(before, `${before}\nmore words\n`)).toBe(
      "Your verdict was not recorded: this review changed after you decided. It shows the review as it stands now; decide again.",
    );
  });
});

describe("a write the record moved under (fix round 2, N-2)", () => {
  it("says the press there is now, not Record it", () => {
    const conflict = { kind: "conflict" as const, reason: CONFLICT, reading: { id: RECORD, revision: "r2", source: head(TIP) } };
    expect(writeRefused(conflict, "verdict")).toBe(
      "The review changed while your verdict was being written, so nothing was written. Give your verdict again: it is given on the review as it now stands.",
    );
    expect(writeRefused(conflict, "decision")).toBe(
      "The review changed while your decision was being written, so nothing was written. Press your decision again: it is recorded on the review as it now stands.",
    );
    expect(writeRefused({ kind: "failed", reason: "the disk is full" }, "verdict")).toBe("the disk is full");
    expect(writeRefused(conflict, "verdict")).not.toContain("Record it");
  });
});
