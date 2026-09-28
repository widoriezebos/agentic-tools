import { describe, expect, it } from "vitest";

import {
  ANSWERS,
  answerLine,
  retipped,
  reviewedOf,
  anchorOf,
  anchorsIn,
  doorLine,
  examinedLine,
  keepDue,
  mayHaveMoved,
  NOD_LINE,
  nodded,
  onDesk,
  outcomeWithVerdict,
  parseAnchor,
  roomOf,
  roomState,
  unansweredIn,
  VERDICTS,
  withDraft,
  withoutDraft,
  type Desk,
  type DeskItem,
} from "./room";
import type { Entry } from "../partner/sitting";

/**
 * The review room's own rules (g1-s65 §3), each one a rule a human would notice
 * if it were wrong: the desk's strip, the anchors that put things on it, the
 * door line, End's refusal, the nod line, the moved tip's marks, and the room's
 * working state that survives stepping out.
 */

const source = (path: string, from = 0, to = 0): DeskItem => ({ kind: "source", path, from, to });

describe("the desk's strip", () => {
  it("keeps every item of the sitting, newest first, each once", () => {
    let desk: Desk = { items: [], current: -1 };
    desk = onDesk(desk, source("owner.go", 41, 88));
    desk = onDesk(desk, { kind: "changes" });
    desk = onDesk(desk, source("owner.go", 41, 88));
    expect(desk.items).toEqual([source("owner.go", 41, 88), { kind: "changes" }]);
    expect(desk.current).toBe(0);
  });

  it("brings an earlier item back to the front when it is pressed again", () => {
    const desk = onDesk(onDesk({ items: [], current: -1 }, { kind: "changes" }), source("a.go"));
    expect(onDesk(desk, { kind: "changes" }).items[0]).toEqual({ kind: "changes" });
  });
});

describe("anchors in the conversation", () => {
  it("reads a file and a range as a desk item", () => {
    expect(anchorsIn("The lock covers publish (internal/owner.go:60) and reconcile (internal/owner.go:72-88).")).toEqual([
      { text: "internal/owner.go:60", item: source("internal/owner.go", 60, 60) },
      { text: "internal/owner.go:72-88", item: source("internal/owner.go", 72, 88) },
    ]);
  });

  it("reads no anchor out of a sentence that names none", () => {
    expect(anchorsIn("nothing recorded covers it, version 1.2 of it")).toEqual([]);
  });

  it("writes and reads a finding's anchor as the same thing", () => {
    expect(anchorOf(source("internal/owner.go", 60, 72))).toBe("internal/owner.go:60-72");
    expect(parseAnchor("internal/owner.go:60-72")).toEqual(source("internal/owner.go", 60, 72));
    expect(anchorOf({ kind: "section", record: "plans/designs/d.md", section: "D3" })).toBe("plans/designs/d.md § D3");
    expect(parseAnchor("plans/designs/d.md § D3")).toEqual({ kind: "section", record: "plans/designs/d.md", section: "D3" });
  });
});

const finding = (text: string, answer: string, clause = "owner.go:60"): Entry => ({
  when: "2026-09-28", who: "Wido", text, clause, section: "Findings", mark: text, answer,
});

describe("the door", () => {
  const now = new Date("2026-09-28T12:00:00Z");
  it("reads the record's counts and the room's time", () => {
    expect(doorLine({ findings: 3, unanswered: 1, steppedOutAt: "2026-09-28T10:00:00Z" }, now)).toBe(
      "In review · you stepped out 2h ago · 3 findings, 1 unanswered",
    );
    expect(doorLine({ findings: 1, unanswered: 0, steppedOutAt: "" }, now)).toBe("In review · 1 finding, none unanswered");
    expect(doorLine({ findings: 0, unanswered: 0, steppedOutAt: "2026-09-28T11:59:40Z" }, now)).toBe(
      "In review · you stepped out just now · no findings yet",
    );
  });
});

describe("ending", () => {
  const entries = [finding("the press dies", "unanswered"), finding("retries forever", "accepted — bounded"),
    finding("no test kills it", "unanswered")];

  it("lists every recorded finding still unanswered, which Clear to land refuses on", () => {
    expect(unansweredIn(entries).map((entry) => entry.text)).toEqual(["the press dies", "no test kills it"]);
  });

  it("offers three ways, each with its consequence said", () => {
    expect(VERDICTS.map((verdict) => verdict.label)).toEqual(["Clear to land", "Send back", "End without a verdict"]);
    for (const verdict of VERDICTS) {
      expect(verdict.consequence).not.toBe("");
    }
  });

  it("writes the verdict as the Outcome's first line and names what was examined", () => {
    // The strip is newest first, and what was examined is said in the order it
    // was examined.
    const examined = examinedLine([source("owner.go", 41, 88), { kind: "changes" }]);
    expect(examined).toBe("Examined: the change index; owner.go:41-88");
    expect(outcomeWithVerdict("clear to land", "The lock is held.", examined)).toBe(
      "Verdict: clear to land\n\nExamined: the change index; owner.go:41-88\n\nThe lock is held.",
    );
    // A draft that already opens with the line keeps one.
    expect(outcomeWithVerdict("send back", "Verdict: send back\n\nFix the press.", "")).toBe(
      "Verdict: send back\n\nFix the press.",
    );
  });

  it("says a nod plainly when the piles are empty", () => {
    expect(nodded([])).toBe(true);
    expect(nodded(entries)).toBe(false);
    expect(NOD_LINE).toContain("nothing anyone can be held to");
  });
});

describe("a moved tip", () => {
  it("marks the findings anchored in a file that changed, and no other", () => {
    const changed = ["internal/owner.go"];
    expect(mayHaveMoved(finding("a", "unanswered", "internal/owner.go:60"), changed)).toBe(true);
    expect(mayHaveMoved(finding("b", "unanswered", "internal/other.go:3"), changed)).toBe(false);
  });
});

describe("the room's working state", () => {
  it("keeps the unfinished words by card and lets them go when the card is recorded or dismissed", () => {
    let drafts = withDraft({}, "deposit:t#0", { text: "the press dies", clause: "owner.go:60" });
    drafts = withDraft(drafts, "local-1", { text: "half", clause: "", kind: "finding" });
    expect(Object.keys(drafts)).toEqual(["deposit:t#0", "local-1"]);
    expect(Object.keys(withoutDraft(drafts, "deposit:t#0"))).toEqual(["local-1"]);
  });

  it("is written whole and read back whole", () => {
    const desk: Desk = { items: [source("owner.go", 1, 40)], current: 0 };
    const drafts = withDraft({}, "local-1", { text: "half", clause: "owner.go:3", kind: "finding" });
    const kept = roomState(desk, "board", drafts);
    expect(roomOf({ desk: kept.desk, face: kept.face, drafts: kept.drafts, at: "2026-09-28T10:00:00Z" })).toEqual({
      desk, face: "board", drafts,
    });
    expect(roomOf(null)).toEqual({ desk: { items: [], current: -1 }, face: "desk", drafts: {} });
  });

  it("writes at most once a second while typing, and always on leaving", () => {
    expect(keepDue(0, 500)).toBe(true);
    expect(keepDue(1000, 1500)).toBe(false);
    expect(keepDue(1000, 2000)).toBe(true);
  });
});

describe("the four answers' own refusals", () => {
  it("writes a follow-up only once its goal is open, and an acceptance only with a reason", () => {
    expect(answerLine("fix", "")).toEqual({ line: "fix — waits for Send back" });
    expect(answerLine("left open", "")).toEqual({ line: "left open" });
    expect(answerLine("follow-up", "")).toEqual({ refusal: "A follow-up is written once its goal is open." });
    expect(answerLine("follow-up", "g2-s01")).toEqual({ line: "follow-up — goal g2-s01" });
    expect(answerLine("accepted", "  ")).toEqual({
      refusal: "An accepted risk is recorded with your reason. Write the reason before accepting it.",
    });
    expect(answerLine("accepted", "the lease bounds it")).toEqual({ line: "accepted — the lease bounds it" });
  });

  it("says each answer's consequence before it is pressed", () => {
    expect(ANSWERS.map((one) => one.label)).toEqual(["Fix in this goal", "Follow-up goal", "Accept, with reason", "Leave open"]);
    for (const one of ANSWERS) {
      expect(one.consequence).not.toBe("");
    }
  });
});

describe("the record's head", () => {
  const tip = "e".repeat(40);
  const head = `# Review of g1-s64\n\n- Kind: review\n- Goals: g1-s64\n- Reviewed: ${tip} (the tip of goal/g1-s64)\n\n## Findings\n`;

  it("names the goal and the tip it was reviewed at", () => {
    expect(reviewedOf(head)).toEqual({ goal: "g1-s64", tip, landed: [], previously: [] });
    expect(reviewedOf("- Goals: g1-s50\n- Reviewed: " + "1".repeat(40) + " (landed with Goal-Item: g1-s50)\n").landed)
      .toEqual(["1".repeat(40)]);
  });

  it("moves to the new tip only on the human's press, keeping the old one as Previously", () => {
    const now = "f".repeat(40);
    const moved = retipped(head, now) ?? "";
    expect(moved).toContain(`- Reviewed: ${now} (the tip of goal/g1-s64)\n- Previously: ${tip}\n`);
    expect(reviewedOf(moved)).toEqual({ goal: "g1-s64", tip: now, landed: [], previously: [tip] });
    const again = retipped(moved, "a".repeat(40)) ?? "";
    expect(reviewedOf(again).previously).toEqual([tip, now]);
    // Everything below the head is untouched.
    expect(again.slice(again.indexOf("## Findings"))).toBe("## Findings\n");
    expect(retipped("# no head\n", now)).toBeNull();
  });
});
