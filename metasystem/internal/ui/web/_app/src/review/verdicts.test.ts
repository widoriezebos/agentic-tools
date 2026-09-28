import { describe, expect, it } from "vitest";

import {
  actingVerdict,
  candidateHref,
  correctionBrief,
  fixFindings,
  NO_FIX,
  outcomeShape,
  outcomeWithVerdict,
  pillOf,
  recordedLine,
  RETIPPED,
  reviewOutcome,
  standingOutcome,
  verdictLine,
  verdictToPerform,
} from "./room";
import { appended, cardsIn, entriesIn, entryOf, FIX, LEFT_OPEN, type Entry } from "../partner/sitting";
import { recorder, type Written } from "../partner/recording";
import type { Verdict } from "../backlog/api";

/**
 * The verdict that does something, and the running candidate (g1-s69 D1 to D3,
 * §8), as the room's own rules hold them: the Outcome bound to its tip, Record it
 * refused after a retip, the correction brief from the fix findings only, what
 * the room says once the verdict is on the goal, the lane card's line, and the
 * pill's states with the moved case naming both commits.
 */

const TIP = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b";
const MOVED = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2";
const RECORD = "metasystem/plans/reviews/review-of-g.md";

function head(tip: string): string {
  return `# Review of g\n\n- Kind: review\n- Goals: g\n- Reviewed: ${tip} (the tip of goal/g)\n\n## Findings\n\n## Outcome\n`;
}

function finding(text: string, anchor: string, answer: string, mark: string): Entry {
  return { when: "2026-09-29", who: "Wido", text, clause: anchor, consequence: "it stays wrong", section: "Findings", mark, answer };
}

function withFindings(source: string, ...found: Entry[]): string {
  return found.reduce((held, entry) => appended(held, entry, "finding"), source);
}

describe("the Outcome is bound to the tip it was drafted for", () => {
  it("writes Reviewed at under the verdict, and replaces a draft's own", () => {
    expect(outcomeWithVerdict("clear to land", "Fine.", "Examined: the change index", TIP)).toEqual({
      text: `Verdict: clear to land\n\nReviewed at: ${TIP}\n\nExamined: the change index\n\nFine.`,
    });
    expect(outcomeWithVerdict("send back", `Verdict: send back\nReviewed at: ${MOVED}\nFine.`, "Examined: x", TIP)).toEqual({
      text: `Verdict: send back\n\nReviewed at: ${TIP}\n\nExamined: x\n\nFine.`,
    });
  });

  it("refuses Record it once the record names another tip, and records it at its own", async () => {
    const saves: string[] = [];
    const save = (_id: string, source: string): Promise<Written> => {
      saves.push(source);
      return Promise.resolve({ revision: "2", source });
    };
    const outcome: Entry = { when: "2026-09-29", who: "Wido", text: "Fine.", clause: "", section: "Outcome", mark: "deposit:t3#0" };
    const retipped = recorder({ id: RECORD, revision: "1", source: head(MOVED) }, save, () => Promise.reject(new Error("unused")), () => false);
    expect(await retipped.press(outcome, "outcome", RECORD, reviewOutcome("clear to land", "Examined: x", TIP)))
      .toEqual({ kind: "failed", reason: RETIPPED });
    expect(saves).toHaveLength(0);
    expect(RETIPPED).toBe("the branch was retipped since this Outcome was drafted; press End again");

    const own = recorder({ id: RECORD, revision: "1", source: head(TIP) }, save, () => Promise.reject(new Error("unused")), () => false);
    expect((await own.press(outcome, "outcome", RECORD, reviewOutcome("clear to land", "Examined: x", TIP))).kind).toBe("recorded");
    expect(saves[0]).toContain(`Verdict: clear to land\n\nReviewed at: ${TIP}\n\nExamined: x\n\nFine.`);
  });

  it("reads the tip from the card the server stamped, so a reloaded card is bound too", () => {
    const [card] = cardsIn([{ turn: "t3", deposits: [{ kind: "outcome", text: "Fine.", offered: true, verdict: "clear to land", tip: TIP,
      subject: { kind: "record", id: RECORD, title: "Review of g" } }] }], {}, RECORD, new Map());
    const shape = outcomeShape(card, "review", []);
    const composed = shape?.(head(MOVED), entryOf(card, "Wido", "2026-09-29"));
    expect(composed).toEqual({ refusal: RETIPPED });
  });
});

describe("the correction brief", () => {
  const source = withFindings(head(TIP),
    finding("the owner reads the wrong tree", "internal/owner.go:60", FIX, "deposit:t1#0"),
    finding("the log is noisy", "internal/log.go:3", LEFT_OPEN, "deposit:t1#1"),
    finding("a dead press holds the lock", "internal/owner.go:72-88", FIX, "deposit:t1#2"));

  it("is composed from the findings answered fix only, in the order recorded, with their anchors, under one heading naming the record and the tip", () => {
    expect(fixFindings(entriesIn(source)).map((entry) => entry.text)).toEqual(["the owner reads the wrong tree", "a dead press holds the lock"]);
    expect(correctionBrief(entriesIn(source), RECORD, TIP)).toBe(
      "# Correction brief\n\n" +
      `## The findings answered fix in ${RECORD} at 9c1f0a2b3\n\n` +
      "1. the owner reads the wrong tree (internal/owner.go:60)\n" +
      "2. a dead press holds the lock (internal/owner.go:72-88)\n",
    );
  });

  it("is nothing where no finding is answered fix, and Send back says so", () => {
    const none = withFindings(head(TIP), finding("the log is noisy", "internal/log.go:3", LEFT_OPEN, "deposit:t1#1"));
    expect(correctionBrief(entriesIn(none), RECORD, TIP)).toBe("");
    expect(NO_FIX).toContain("none is answered fix");
  });

  it("travels with a send-back once its Outcome is recorded, as the human edited it, and Clear to land carries none", () => {
    const back = { kind: "outcome", verdict: "send back" };
    expect(verdictToPerform(back, "review", RECORD, source, null)).toEqual({
      goal: "g", fixes: 2,
      asked: { record: RECORD, verdict: "send-back", work: "", brief: correctionBrief(entriesIn(source), RECORD, TIP) },
    });
    expect(verdictToPerform(back, "review", RECORD, source, "# Edited\n")?.asked.brief).toBe("# Edited\n");
    expect(verdictToPerform({ kind: "outcome", verdict: "clear to land" }, "review", RECORD, source, "# Edited\n")?.asked)
      .toEqual({ record: RECORD, verdict: "clear-to-land", work: "", brief: "" });
    // No verdict performs nothing, and neither does a sitting that shapes a record.
    expect(verdictToPerform({ kind: "outcome", verdict: "no verdict" }, "review", RECORD, source, null)).toBeNull();
    expect(verdictToPerform({ kind: "outcome", verdict: "clear to land" }, "shape a design", RECORD, source, null)).toBeNull();
    expect(actingVerdict("no verdict")).toBe("");
  });
});

describe("what the room and the card say once the verdict is on the goal", () => {
  it("says the verdict at its tip and by whom, and a send-back with its findings", () => {
    expect(recordedLine({ verdict: "clear-to-land", tip: TIP, by: "Wido" }, 0))
      .toBe("Recorded. The goal now carries your verdict: clear to land at 9c1f0a2, reviewed by Wido.");
    expect(recordedLine({ verdict: "send-back", tip: TIP, by: "Wido" }, 3))
      .toBe("Sent back with your three findings; the goal has left Review and the seat that holds it revises.");
  });

  it("reads the card's line from the goal's own history", () => {
    const base: Verdict = { verdict: "clear-to-land", by: "Wido", at: "", tip: TIP, record: RECORD, answered: false };
    expect(verdictLine(base)).toBe("reviewed by Wido · clear to land");
    expect(verdictLine({ ...base, verdict: "send-back" })).toBe("sent back by Wido · awaiting the holder");
    expect(verdictLine({ ...base, verdict: "send-back", answered: true, attempt: 3 })).toBe("attempt 3 started from your brief");
    expect(verdictLine({ ...base, verdict: "send-back", answered: true, candidates: ["discovery", "writer"] }))
      .toBe("the holder needs to know which work: discovery or writer");
    expect(verdictLine(undefined)).toBe("");
  });

  it("asks for End again where a retip leaves an Outcome card unrecorded", () => {
    expect(standingOutcome([{ kind: "outcome", standing: "waiting" }])).toBe(true);
    expect(standingOutcome([{ kind: "outcome", standing: "recorded" }, { kind: "finding", standing: "waiting" }])).toBe(false);
  });
});

describe("the candidate's pill", () => {
  const running = { goal: "g", state: "running", readiness: "answering", address: "127.0.0.1:7981", commit: TIP, said: "" };

  it("runs the reviewed tip, or says the moved case with both commits", () => {
    expect(pillOf(running, TIP)).toEqual({ state: "reviewed", words: "running the reviewed tip 9c1f0a2", address: "127.0.0.1:7981", run: false, stop: true });
    expect(pillOf({ ...running, commit: MOVED }, TIP).words).toBe("running 4d2e7b1, not the reviewed 9c1f0a2");
    expect(pillOf({ ...running, commit: MOVED }, TIP).state).toBe("moved");
  });

  it("offers Run where nothing runs, and says a start and a silent run as they are", () => {
    expect(pillOf({ goal: "g", state: "stopped", readiness: "no-probe", said: "" }, TIP)).toMatchObject({ state: "stopped", run: true, stop: false });
    expect(pillOf({ ...running, state: "starting" }, TIP)).toMatchObject({ state: "starting", run: false, stop: true });
    expect(pillOf({ ...running, readiness: "not-answering" }, TIP)).toMatchObject({ state: "not-answering", stop: true });
  });

  it("greys a goal with no launch contract, with the reason", () => {
    const pill = pillOf({ refusal: "this goal's candidate cannot run from here: this project has no launch contract", code: "no-contract" }, TIP);
    expect(pill).toEqual({ state: "no-contract", words: "this goal's candidate cannot run from here: this project has no launch contract",
      address: "", run: false, stop: false });
  });

  it("opens the address in its own origin", () => {
    expect(candidateHref("127.0.0.1:7981")).toBe("http://127.0.0.1:7981/");
    expect(candidateHref("http://127.0.0.1:7981/")).toBe("http://127.0.0.1:7981/");
    expect(candidateHref("")).toBe("");
  });
});
