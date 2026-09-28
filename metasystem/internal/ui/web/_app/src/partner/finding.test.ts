import { describe, expect, it } from "vitest";

import { recorder, type Written } from "./recording";
import {
  ACCEPTED,
  answered,
  answerOf,
  appended,
  cardHead,
  entriesIn,
  FIX,
  followUp,
  LEFT_OPEN,
  lineOf,
  missing,
  pilesOf,
  sectionOf,
  UNANSWERED,
  type Entry,
} from "./sitting";

/**
 * The review's finding (g1-s65 D8), as the record carries it: an entry of the
 * Findings pile with its anchor, its consequence and an Answer line that says
 * `unanswered` from the moment Record it lands (Astra S65-01), and the four
 * answers that rewrite that one line by the entry's mark and nothing else.
 */

const REVIEW = [
  "# Review of g1-s64",
  "",
  "- Kind: review",
  "- Goals: g1-s64",
  "",
  "## Facts",
  "",
  "## Findings",
  "",
  "## Decisions",
  "",
  "## Open questions",
  "",
  "## Drawings",
  "",
  "## Outcome",
  "",
].join("\n");

const FINDING: Entry = {
  when: "2026-09-28",
  who: "Wido",
  text: "nothing recorded covers a press that dies between publish and reconcile",
  clause: "internal/owner.go:60-72",
  consequence: "a dead press leaves the lock held until restart",
  section: "Findings",
  mark: "deposit:t1#0",
};

describe("the finding's entry", () => {
  it("lands in the Findings pile with its anchor, its consequence and Answer: unanswered", () => {
    expect(sectionOf("finding")).toBe("Findings");
    expect(cardHead("finding")).toBe("A finding");
    expect(lineOf(FINDING, "finding")).toBe(
      "- 2026-09-28 · Wido · nothing recorded covers a press that dies between publish and reconcile [d:deposit:t1#0]\n" +
        "  - Anchor: internal/owner.go:60-72\n" +
        "  - Consequence: a dead press leaves the lock held until restart\n" +
        "  - Answer: unanswered\n",
    );
    const written = appended(REVIEW, FINDING, "finding");
    const [read] = entriesIn(written).filter((entry) => entry.section === "Findings");
    expect(read.text).toBe(FINDING.text);
    expect(read.clause).toBe("internal/owner.go:60-72");
    expect(read.consequence).toBe("a dead press leaves the lock held until restart");
    expect(read.answer).toBe(UNANSWERED);
    expect(read.mark).toBe("deposit:t1#0");
    // Everything outside the pile is exactly as it was.
    expect(written.slice(0, written.indexOf("## Findings"))).toBe(REVIEW.slice(0, REVIEW.indexOf("## Findings")));
    expect(written.slice(written.indexOf("## Decisions"))).toBe(REVIEW.slice(REVIEW.indexOf("## Decisions")));
  });

  it("is a pile of a review's and of no other record's", () => {
    expect(pilesOf("review")).toEqual(["Facts", "Findings", "Decisions", "Open questions"]);
    expect(pilesOf("shape a design")).toEqual(["Facts", "Proposals", "Decisions", "Open questions"]);
  });

  it("needs its words and its anchor before it is recorded", () => {
    const blank = { text: "", clause: "a.go:1", recording: false, recorded: "", refusal: "", dismissed: false };
    expect(missing("finding", blank)).not.toBe("");
    expect(missing("finding", { ...blank, text: "it dies", clause: "" })).toBe(
      "A finding is anchored where it sits. Write the anchor before recording it.",
    );
    expect(missing("finding", { ...blank, text: "it dies" })).toBe("");
  });
});

describe("the four answers", () => {
  const two = appended(appended(REVIEW, FINDING, "finding"),
    { ...FINDING, text: "no test kills the press", mark: "local-1", clause: "internal/owner_test.go:1" }, "finding");

  it("each say their consequence in the words the record keeps", () => {
    expect(FIX).toBe("fix — waits for Send back");
    expect(followUp("g2-s01")).toBe("follow-up — goal g2-s01");
    expect(ACCEPTED("the lease bounds the retry")).toBe("accepted — the lease bounds the retry");
    expect(LEFT_OPEN).toBe("left open");
    expect(answerOf("accepted — the lease bounds the retry")).toBe("accepted");
    expect(answerOf("follow-up — goal g2-s01")).toBe("follow-up");
    expect(answerOf(UNANSWERED)).toBe("unanswered");
  });

  it("rewrite one entry's Answer line by its mark, and nothing else", () => {
    const after = answered(two, "local-1", FIX);
    expect(after).not.toBeNull();
    const findings = entriesIn(after ?? "").filter((entry) => entry.section === "Findings");
    expect(findings.map((entry) => entry.answer)).toEqual([UNANSWERED, FIX]);
    // One line changed, and it is that line.
    const before = two.split("\n");
    const now = (after ?? "").split("\n");
    expect(now.length).toBe(before.length);
    expect(now.filter((line, at) => line !== before[at])).toEqual([`  - Answer: ${FIX}`]);
  });

  it("gives an entry that has none an Answer line under its clauses", () => {
    const bare = two.replace("  - Answer: unanswered\n", "");
    const after = answered(bare, "deposit:t1#0", LEFT_OPEN) ?? "";
    expect(entriesIn(after).find((entry) => entry.mark === "deposit:t1#0")?.answer).toBe(LEFT_OPEN);
    expect(after).toContain("  - Consequence: a dead press leaves the lock held until restart\n  - Answer: left open\n");
  });

  it("answers null for a mark the record does not carry", () => {
    expect(answered(two, "deposit:nowhere#9", FIX)).toBeNull();
  });
});

describe("the recorder's second composition", () => {
  it("rewrites the Answer line under the reading's revision, serialized with the presses", async () => {
    let held = { revision: "1", source: appended(REVIEW, FINDING, "finding") };
    const saves: string[] = [];
    const save = (_id: string, source: string, revision: string): Promise<Written> => {
      saves.push(revision);
      if (revision !== held.revision) {
        return Promise.reject(new Error("stale"));
      }
      held = { revision: String(Number(held.revision) + 1), source };
      return Promise.resolve(held);
    };
    const record = recorder({ id: "r.md", ...held }, save, () => Promise.resolve(held), () => false);

    const pressed = record.press({ ...FINDING, text: "second", mark: "local-2" }, "finding", "r.md");
    const answeredNow = record.answer("deposit:t1#0", FIX, "r.md");
    const [first, second] = await Promise.all([pressed, answeredNow]);

    expect(first.kind).toBe("recorded");
    expect(second.kind).toBe("recorded");
    expect(saves).toEqual(["1", "2"]);
    const findings = entriesIn(held.source).filter((entry) => entry.section === "Findings");
    expect(findings.map((entry) => [entry.mark, entry.answer])).toEqual([
      ["deposit:t1#0", FIX],
      ["local-2", UNANSWERED],
    ]);
    expect((await record.answer("deposit:nowhere#1", FIX, "r.md")).kind).toBe("failed");
    expect((await record.answer("deposit:t1#0", FIX, "another.md")).kind).toBe("elsewhere");
  });
});
