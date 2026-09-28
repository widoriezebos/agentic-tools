import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";

import {
  ANSWERS,
  reviewStart,
  deskKey,
  deskReadKey,
  KEEP_REFUSED,
  steppingOut,
  CLEAR_REFUSED,
  reviewOutcome,
  outcomeShape,
  answerLine,
  retipped,
  reviewedOf,
  anchorOf,
  anchorsIn,
  doorLine,
  examinedLine,
  keepAfterSilence,
  keepDue,
  KEEP_AFTER_SILENCE,
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
import { appended, cardsIn, entryOf, type Entry } from "../partner/sitting";
import { recorder, type Written } from "../partner/recording";

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
    // Each thing once, and the two readings of a change told apart.
    expect(examinedLine([{ kind: "changes", since: true }, { kind: "diff", path: "a.go" }, { kind: "changes" },
      { kind: "diff", path: "a.go", since: true }])).toBe(
      "Examined: what changed since the reviewed tip in a.go; the change index; a.go, as changed; what changed since the reviewed tip",
    );
    expect(outcomeWithVerdict("clear to land", "The lock is held.", examined)).toEqual({
      text: "Verdict: clear to land\n\nExamined: the change index; owner.go:41-88\n\nThe lock is held.",
    });
    // A draft that already opens with both lines keeps them, once each.
    expect(outcomeWithVerdict("send back", "Verdict: send back\n\nExamined: a.go:1-2\n\nFix the press.", examined)).toEqual({
      text: "Verdict: send back\n\nExamined: a.go:1-2\n\nFix the press.",
    });
  });

  it("holds the verdict line exactly and never lets the Examined line go missing (Sol SOL-A-07)", () => {
    const examined = "Examined: the change index";
    // A draft that opens with the verdict line but says nothing examined gets it.
    expect(outcomeWithVerdict("clear to land", "Verdict: clear to land\n\nThe lock is held.", examined)).toEqual({
      text: "Verdict: clear to land\n\nExamined: the change index\n\nThe lock is held.",
    });
    expect(outcomeWithVerdict("clear to land", "Verdict: clear to land", examined)).toEqual({
      text: "Verdict: clear to land\n\nExamined: the change index",
    });
    // A verdict line that only starts like the chosen one is another verdict, refused in words.
    const prefix = outcomeWithVerdict("clear to land", "Verdict: clear to landing\n\nFine.", examined);
    expect(prefix).toEqual({ refusal: expect.stringContaining("Verdict: clear to landing") });
    expect("refusal" in prefix && prefix.refusal).toContain("clear to land");
    expect(outcomeWithVerdict("clear to land", "Verdict: send back\n\nFine.", examined)).toHaveProperty("refusal");
    // No desk items still says so, rather than leaving the line out.
    expect(outcomeWithVerdict("no verdict", "Ended.", "")).toEqual({
      text: "Verdict: no verdict\n\nExamined: nothing was put on the desk\n\nEnded.",
    });
  });

  it("refuses a Clear Outcome inside the recorder while the record it now reads has an unanswered finding (Sol SOL-A-02)", async () => {
    const empty = "# Review of g\n\n## Findings\n\n## Outcome\n";
    const found = appended(empty, {
      when: "2026-09-28", who: "Wido", text: "the lock is never released", clause: "owner.go:60-72",
      consequence: "a dead press holds the lock", section: "Findings", mark: "deposit:t9#0",
    }, "finding");
    const saves: string[] = [];
    // Another tab records the finding: the first write meets the moved record.
    const save = (_id: string, source: string): Promise<Written> => {
      saves.push(source);
      return Promise.reject(new Error("stale"));
    };
    const record = recorder({ id: "r.md", revision: "1", source: empty }, save,
      () => Promise.resolve({ revision: "2", source: found }), () => true);
    const outcome: Entry = { when: "2026-09-28", who: "Wido", text: "Fine.", clause: "", section: "Outcome", mark: "deposit:t3#0" };
    const shape = reviewOutcome("clear to land", "Examined: the change index");

    expect((await record.press(outcome, "outcome", "r.md", shape)).kind).toBe("conflict");
    expect(saves).toHaveLength(1);
    const again = await record.press(outcome, "outcome", "r.md", shape);
    expect(again).toEqual({ kind: "failed", reason: expect.stringContaining("the lock is never released") });
    expect("reason" in again && again.reason).toContain(CLEAR_REFUSED);
    // Nothing was written the second time.
    expect(saves).toHaveLength(1);
    // Send back is not refused for it.
    const back = await record.press(outcome, "outcome", "r.md", reviewOutcome("send back", "Examined: x"));
    expect(back.kind).toBe("conflict");
    expect(saves[1]).toContain("Verdict: send back\n\nExamined: x\n\nFine.");
  });

  it("treats an Examined line with nothing after its colon as missing (Sol SOL-A-07)", () => {
    expect(outcomeWithVerdict("send back", "Examined:\n\nFine.", "Examined: the change index")).toEqual({
      text: "Verdict: send back\n\nExamined: the change index\n\nFine.",
    });
    expect(outcomeWithVerdict("send back", "Verdict: send back\nExamined:   \nFine.", "")).toEqual({
      text: "Verdict: send back\n\nExamined: nothing was put on the desk\n\nFine.",
    });
  });

  it("records a card rebuilt after a reload under the verdict the card carries, not the page's (Sol SOL-A-02, SOL-A-07)", async () => {
    // The reload: the page holds no verdict and no marks; the card is rebuilt
    // from the conversation's messages, whose closing deposit carries its verdict.
    const carried = [{
      turn: "t3",
      deposits: [{ kind: "outcome", text: "Fine.", offered: true, verdict: "clear to land",
        subject: { kind: "record", id: "r.md", title: "Review of g" } }],
    }];
    const [card] = cardsIn(carried, {}, "r.md", new Map());
    const entry = entryOf(card, "Wido", "2026-09-28");
    const shape = outcomeShape(card, "review", []);
    expect(shape).toBeDefined();

    const empty = "# Review of g\n\n## Findings\n\n## Outcome\n";
    const open = appended(empty, {
      when: "2026-09-28", who: "Wido", text: "the lock is never released", clause: "owner.go:60-72",
      consequence: "a dead press holds the lock", section: "Findings", mark: "deposit:t9#0", answer: "unanswered",
    }, "finding");
    const saves: string[] = [];
    const save = (_id: string, source: string): Promise<Written> => {
      saves.push(source);
      return Promise.resolve({ revision: String(saves.length + 1), source });
    };
    const reread = () => Promise.reject(new Error("no reread"));
    const refused = await recorder({ id: "r.md", revision: "1", source: open }, save, reread, () => false)
      .press(entry, "outcome", "r.md", shape);
    expect(refused).toEqual({ kind: "failed", reason: expect.stringContaining(CLEAR_REFUSED) });
    expect(saves).toHaveLength(0);

    const answered = open.replace("Answer: unanswered", "Answer: fix");
    const recorded = await recorder({ id: "r.md", revision: "1", source: answered }, save, reread, () => false)
      .press(entry, "outcome", "r.md", shape);
    expect(recorded.kind).toBe("recorded");
    expect(saves).toHaveLength(1);
    const outcome = saves[0].slice(saves[0].indexOf("## Outcome"));
    expect(outcome).toContain("Verdict: clear to land\n\nExamined: nothing was put on the desk\n\nFine.");
    expect(outcome).toMatch(/Verdict: clear to land\n\nExamined: \S/u);
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

  it("keeps the words a second after the last keystroke, with no blur and no Step out", () => {
    // The human's grant of 2026-09-28 (Sol SOL-A-01): one timer, reset by every
    // keystroke, so the words typed last are kept once the typing stops.
    vi.useFakeTimers();
    try {
      const kept: string[] = [];
      let typed = "hal";
      const keep = () => {
        kept.push(typed);
      };
      let cancel = keepAfterSilence(keep);
      vi.advanceTimersByTime(KEEP_AFTER_SILENCE - 1);
      cancel();
      typed = "half a finding";
      cancel = keepAfterSilence(keep);
      vi.advanceTimersByTime(KEEP_AFTER_SILENCE - 1);
      expect(kept).toEqual([]);
      vi.advanceTimersByTime(1);
      expect(kept).toEqual(["half a finding"]);
      vi.advanceTimersByTime(10 * KEEP_AFTER_SILENCE);
      expect(kept).toEqual(["half a finding"]);
      cancel();
    } finally {
      vi.useRealTimers();
    }
    expect(KEEP_AFTER_SILENCE).toBe(1000);
  });

  it("is kept by the store a second after each change of the room, through its own keep", () => {
    // This suite mounts nothing, so the wiring is read from the store's source:
    // the effect that runs on every change of the room hands its cancel back to
    // React, and the keep it schedules is keepRoomNow, the path blur, Step out
    // and pagehide take, which reads the room as it is when the second is up.
    const store = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "..", "partner", "store.tsx"), "utf8");
    const effect = store.slice(store.indexOf("if (keepDue(keptAt.current, Date.now()))"));
    expect(effect.slice(0, effect.indexOf("}, [room, roomRecord, keepRoomNow]);"))).toContain(
      "return keepAfterSilence(() => {\n      void keepRoomNow();\n    });",
    );
    expect(store).toContain('globalThis.addEventListener("pagehide", leaving);');
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

  it("keys every desk read by the reviewed commit as well as the item, so a retip reads it again (Sol SOL-A-03)", () => {
    const now = "f".repeat(40);
    const item: DeskItem = { kind: "source", path: "owner.go", from: 1, to: 40 };
    const before = deskReadKey(item, reviewedOf(head));
    const after = deskReadKey(item, reviewedOf(retipped(head, now) ?? ""));
    expect(before).toContain(tip);
    expect(after).toContain(now);
    expect(after).not.toBe(before);
    for (const other of [{ kind: "changes" }, { kind: "diff", path: "a.go" }, { kind: "section", record: "r.md", section: "D1" }] as DeskItem[]) {
      expect(deskReadKey(other, reviewedOf(retipped(head, now) ?? ""))).not.toBe(deskReadKey(other, reviewedOf(head)));
    }
    // The strip's items keep their identity across the retip.
    expect(deskKey(item)).toBe(deskKey({ ...item }));
    // A done goal's reads are keyed by its landed commits.
    const landed = reviewedOf("- Goals: g1-s50\n- Reviewed: " + "1".repeat(40) + " (landed with Goal-Item: g1-s50)\n");
    expect(deskReadKey(item, landed)).toContain("1".repeat(40));
  });
});

describe("stepping out (Sol SOL-A-01, SOL-A-06)", () => {
  const UNSETTLED = "the previous room's answer has not settled; try again in a moment";

  it("stays in the room when the room could not be kept, and leaves once it is", async () => {
    let kept = false;
    const keep = () => Promise.resolve(kept);
    const stop = () => Promise.resolve("");
    expect(await steppingOut(false, false, stop, keep)).toEqual({ kind: "stay", said: KEEP_REFUSED });
    expect(KEEP_REFUSED).toBe("Your unfinished words could not be kept; try Step out again in a moment.");
    kept = true;
    expect(await steppingOut(false, false, stop, keep)).toEqual({ kind: "leave" });
  });

  it("during an answer, says so first, then stays while the answer has not settled, and leaves once it has", async () => {
    const keeps: string[] = [];
    const keep = () => {
      keeps.push("kept");
      return Promise.resolve(true);
    };
    let settled = false;
    const stop = () => Promise.resolve(settled ? "" : UNSETTLED);
    expect(await steppingOut(true, false, stop, keep)).toEqual({ kind: "warn" });
    expect(await steppingOut(true, true, stop, keep)).toEqual({ kind: "stay", said: UNSETTLED });
    // Nothing is kept or left while the answer runs on.
    expect(keeps).toEqual([]);
    settled = true;
    expect(await steppingOut(true, true, stop, keep)).toEqual({ kind: "leave" });
    expect(keeps).toEqual(["kept"]);
  });
});

describe("Review it after a refused open (Sol SOL-A-05)", () => {
  it("asks for the goal until a refused open names its record, then for that record", () => {
    expect(reviewStart("g1-s64", "")).toEqual({ purpose: "review", subject: { kind: "goal", id: "g1-s64", title: "g1-s64" } });
    expect(reviewStart("g1-s64", "metasystem/plans/reviews/review-of-g1-s64.md")).toEqual({
      purpose: "review",
      subject: { kind: "record", id: "metasystem/plans/reviews/review-of-g1-s64.md", title: "Review of g1-s64" },
    });
  });
});
