import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Backlog, Row } from "./api";
import { EditSheet } from "./EditSheet";

/**
 * The edit sheet as one screen.
 *
 * The claims here are claims about what is rendered and cannot be read from a
 * table: that the three fields open filled in from the row rather than empty,
 * that the button is disabled while nothing has changed, and that the sheet
 * says what it will publish and what it will leave alone.
 */

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "ui-1", revision: 1 },
    where: "plans/goals/ui-1.md",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: "The board reads the ledger.",
    nextStep: "Take it to a working end state.",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 1,
    tier: 1,
    labels: ["board", "ui"],
    arc: "",
    pinned: "",
    blockedBy: [],
    holds: [],
    openBlockers: [],
    sliced: false,
    decomposed: false,
    openedAt: "",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "open",
    gaps: [],
    ...over,
  };
}

function backlog(): Backlog {
  return {
    schemaVersion: 1,
    observedAt: "2026-09-25T08:00:00Z",
    ledger: {
      state: "read",
      tip: "abc1234",
      committedAt: "2026-09-25T07:00:00Z",
      freshness: { state: "current", since: "2026-09-25T07:59:00Z", detail: "" },
      stale: false,
      staleAfterSeconds: 900,
      syncMode: "",
      stateRoot: "",
      message: "",
      problems: [],
      fetch: {
        outcome: "current",
        startedAt: "",
        finishedAt: "",
        tip: "abc1234",
        detail: "",
        message: "",
        failures: 0,
        cadence: "",
        nextAt: "",
        succeededAt: "",
        succeededTip: "abc1234",
      },
    },
    admission: { answered: true, message: "" },
    workingTree: { liveFiles: 1, archivedFiles: 0 },
    authority: { proven: true, human: "Wido", reason: "" },
    budgetDefaults: {},
    counts: {},
    draft: { statement: "" },
    rows: [row()],
    closed: [],
  };
}

function sheet(goal: Row = row()): string {
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <EditSheet goal={goal} backlog={backlog()} onClose={() => undefined} onDone={() => undefined} />
    </TooltipPrimitive.Provider>,
  );
}

describe("the edit sheet", () => {
  it("opens filled in from the row, all three fields", () => {
    const markup = sheet();
    expect(markup).toContain("The board reads the ledger.");
    expect(markup).toContain("Take it to a working end state.");
    // The labels arrive as the chips the token field makes of them.
    expect(markup).toContain("board");
    expect(markup).toContain("ui");
  });

  it("names the goal it is about, and says it is not yet approved", () => {
    const markup = sheet();
    expect(markup).toContain("ui-1");
    expect(markup).toContain("Queued, not yet approved");
    expect(markup).toContain("Edit goal");
  });

  it("will not save a sheet in which nothing has changed", () => {
    const markup = sheet();
    expect(markup).toContain("Nothing has changed yet.");
    // The primary button is disabled while there is nothing to send.
    expect(markup).toMatch(/<button[^>]*disabled[^>]*>Save<\/button>/);
  });

  it("quotes the intake rules under the two lines rather than inventing its own", () => {
    const markup = sheet();
    expect(markup).toContain("What done looks like: the outcome, not the work.");
    expect(markup).toContain("never a script of the how");
  });
});
