import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { CandidateError } from "./candidate";
import { correctionBrief, pressSignedIn, strandedVerdict, type ToPerform } from "./room";
import { PendingVerdict } from "./Verdict";
import { BacklogError, type Verdict } from "../backlog/api";
import { appended, entriesIn, FIX, LEFT_OPEN, type Entry } from "../partner/sitting";

/**
 * Sol's fix round on the verdicts (g1-s69): a press refused for want of a
 * sign-in opens the sheet and is pressed again once signed in (SOL-S69-03), and
 * a recorded Outcome whose verdict never reached the goal is offered again on
 * load, with the brief the human drafted (SOL-S69-04).
 */

const TIP = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b";
const MOVED = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2";
const RECORD = "metasystem/plans/reviews/review-of-g.md";

function finding(text: string, anchor: string, answer: string, mark: string): Entry {
  return { when: "2026-09-29", who: "Wido", text, clause: anchor, consequence: "it stays wrong", section: "Findings", mark, answer };
}

/** A review record with two findings, one answered fix, and the Outcome End drafted and Record it wrote. */
function recorded(verdict: string, reviewedAt: string): string {
  const head = `# Review of g\n\n- Kind: review\n- Goals: g\n- Reviewed: ${TIP} (the tip of goal/g)\n\n## Findings\n\n## Outcome\n`;
  const found = [
    finding("the owner reads the wrong tree", "internal/owner.go:60", FIX, "deposit:t1#0"),
    finding("the log is noisy", "internal/log.go:3", LEFT_OPEN, "deposit:t1#1"),
  ].reduce((held, entry) => appended(held, entry, "finding"), head);
  return `${found}\nVerdict: ${verdict}\n\nReviewed at: ${reviewedAt}\n\nExamined: internal/owner.go:60\n\n` +
    "- Recorded from the sitting · 2026-09-29 · Wido [d:deposit:t3#0]\n";
}

describe("a verdict the record carries and the goal does not", () => {
  const source = recorded("send back", TIP);
  const drafted = correctionBrief(entriesIn(source), RECORD, TIP);

  it("is offered again on load with the brief the human edited, else the one composed from the fix findings", () => {
    expect(strandedVerdict(source, RECORD, undefined, "# Edited brief\n")).toEqual({
      goal: "g", fixes: 1, asked: { record: RECORD, verdict: "send-back", work: "", brief: "# Edited brief\n" },
    });
    expect(strandedVerdict(source, RECORD, undefined, null)?.asked.brief).toBe(drafted);
    expect(drafted).toContain("the owner reads the wrong tree");
    // A verdict on the goal from another record, or at another tip, is not this one.
    const other: Verdict = { verdict: "send-back", by: "Wido", at: "", tip: MOVED, record: "plans/reviews/review-of-g.md", answered: false };
    expect(strandedVerdict(source, RECORD, other, null)).not.toBeNull();
    expect(strandedVerdict(recorded("clear to land", TIP), RECORD, undefined, "# Edited brief\n")?.asked)
      .toEqual({ record: RECORD, verdict: "clear-to-land", work: "", brief: "" });
  });

  it("is nothing where the goal's history carries the reviewed line for this record and tip", () => {
    const line: Verdict = { verdict: "send-back", by: "Wido", at: "", tip: TIP, record: "plans/reviews/review-of-g.md", answered: false };
    expect(strandedVerdict(source, RECORD, line, null)).toBeNull();
    expect(strandedVerdict(source, RECORD, { ...line, record: RECORD }, null)).toBeNull();
  });

  it("is nothing where the Outcome carries no acting verdict, or was drafted for another tip", () => {
    expect(strandedVerdict(recorded("no verdict", TIP), RECORD, undefined, null)).toBeNull();
    expect(strandedVerdict(recorded("send back", MOVED), RECORD, undefined, null)).toBeNull();
    expect(strandedVerdict(`# Review of g\n\n- Goals: g\n- Reviewed: ${TIP} (the tip of goal/g)\n\n## Outcome\n`, RECORD, undefined, null)).toBeNull();
  });

  it("shows the pending verdict with its drafted brief and the press that records it on the goal", () => {
    const pending = strandedVerdict(source, RECORD, undefined, "# Edited brief\n\n1. Read the reviewed tree.\n") as ToPerform;
    const markup = renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PendingVerdict pending={pending} busy={false} onPress={() => undefined} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
    expect(markup).toContain("The Outcome is recorded, and the verdict is not yet on the goal: send back.");
    expect(markup).toContain("1. Read the reviewed tree.");
    expect(markup).toMatch(/<button[^>]*>Record the verdict on the goal</u);
  });
});

/** The act as a room's press drives it, with the sheet the page would open. */
function driving(refusals: unknown[]) {
  let acts = 0;
  const signIns: (() => Promise<void>)[] = [];
  const done: string[] = [];
  const refused: unknown[] = [];
  const ports = {
    act: () => {
      const refusal = refusals[acts];
      acts += 1;
      return refusal === undefined ? Promise.resolve("on the goal") : Promise.reject(refusal);
    },
    done: (answer: string) => {
      done.push(answer);
    },
    refused: (error: unknown) => {
      refused.push(error);
    },
    signIn: (again: () => Promise<void>) => {
      signIns.push(again);
    },
  };
  return { ports, signIns, done, refused, acts: () => acts };
}

describe("a press refused for want of a sign-in", () => {
  const cases: [string, () => Error][] = [
    ["a verdict press", () => new BacklogError("/api/backlog/goals/g/review", 401, "nobody is signed in here", "unproven", true)],
    ["a Run press", () => new CandidateError(401, "nobody is signed in here", "unproven", true)],
  ];

  for (const [name, refusal] of cases) {
    it(`${name} opens the sign-in sheet and, after a sign-in, is pressed again once`, async () => {
      const driven = driving([refusal()]);
      await pressSignedIn(driven.ports);
      expect(driven.acts()).toBe(1);
      expect(driven.signIns).toHaveLength(1);
      expect(driven.done).toHaveLength(0);
      await driven.signIns[0]();
      expect(driven.acts()).toBe(2);
      expect(driven.done).toEqual(["on the goal"]);
      expect(driven.signIns).toHaveLength(1);
    });

    it(`${name} refused for sign-in again after the sign-in is not pressed a third time`, async () => {
      const driven = driving([refusal(), refusal()]);
      await pressSignedIn(driven.ports);
      await driven.signIns[0]();
      expect(driven.acts()).toBe(2);
      expect(driven.signIns).toHaveLength(1);
      expect(driven.refused).toHaveLength(2);
    });
  }

  it("any other refusal opens no sheet and is said", async () => {
    const driven = driving([new BacklogError("/api/backlog/goals/g/review", 409, "goal g is not waiting to land", "engine")]);
    await pressSignedIn(driven.ports);
    expect(driven.signIns).toHaveLength(0);
    expect(driven.refused).toHaveLength(1);
  });
});
