import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Backlog, Row } from "./api";
import { noFilters } from "./filters";
import { Board } from "./Board";

/**
 * What the board is made of, above and below the lanes.
 *
 * Two of this change's claims are claims about where something is rendered,
 * and neither can be read from a table: the Draft lane is not a column while
 * nothing reads drafts, and the goals this build cannot place stand under the
 * lanes rather than over them. Both are asserted from the markup the board
 * renders.
 */

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "ui-1", revision: 1 },
    where: "plans/goals/ui-1.md",
    lane: "to-do",
    phase: "",
    state: "open",
    intent: "The board reads.",
    nextStep: "Take it to an end state.",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 1,
    tier: 1,
    labels: [],
    arc: "",
    pinned: "",
    blockedBy: [],
    openBlockers: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-09-22T09:00:00Z",
    doneAt: "",
    lastChangeAt: "2026-09-22T09:00:00Z",
    lastVerb: "goal open",
    gaps: [],
    ...over,
  };
}

const backlog: Backlog = {
  schemaVersion: 1,
  observedAt: "2026-09-23T10:30:03Z",
  ledger: {
    state: "read",
    tip: "2ef5d8d9c1b4a70f3e2d1c0b9a8f7e6d5c4b3a29",
    committedAt: "2026-09-23T10:12:00Z",
    freshness: { state: "current", since: "2026-09-23T10:29:41Z", detail: "already at the canonical tip" },
    stale: false,
    staleAfterSeconds: 3600,
    syncMode: "",
    stateRoot: "",
    message: "",
    problems: [],
    fetch: {
      outcome: "current",
      startedAt: "2026-09-23T10:29:40Z",
      finishedAt: "2026-09-23T10:29:41Z",
      tip: "",
      detail: "already at the canonical tip",
      message: "",
      failures: 0,
      cadence: "5m",
      nextAt: "2026-09-23T10:34:41Z",
      succeededAt: "2026-09-23T10:29:41Z",
      succeededTip: "2ef5d8d9c1b4a70f3e2d1c0b9a8f7e6d5c4b3a29",
    },
  },
  admission: { answered: true, message: "" },
  workingTree: { liveFiles: 2, archivedFiles: 0 },
  authority: { proven: false, human: "", reason: "the interface was started by an agent process" },
  budgetDefaults: {},
  counts: { "to-do": 1, unknown: 1 },
  draft: { statement: "plans/goals-drafts/ has no reader in the engine" },
  rows: [row(), row({ ref: { kind: "goal", id: "ui-2", revision: 1 }, lane: "unknown", gaps: ["no phase"] })],
  closed: [],
};

function markup(): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Board
          backlog={backlog}
          closedShown={false}
          onToggleClosed={() => undefined}
          onAct={() => undefined}
          onMoved={() => undefined}
          plans={null}
          filters={noFilters}
          window={1}
          onWindow={() => undefined}
          showing=""
          onFaded={() => undefined}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the lanes the board renders", () => {
  const board = markup();

  // The projection has no reader for plans/goals-drafts/, and a column whose
  // only content is a sentence saying so took a seventh of the board.
  it("begin with To Do, because nothing reads drafts", () => {
    expect(board).not.toContain("Draft");
    expect(board).not.toContain("has no reader in the engine");
    expect(board.indexOf("To Do")).toBeGreaterThan(-1);
    expect(board.indexOf("To Do")).toBeLessThan(board.indexOf("Ready for Work"));
  });
});

describe("what stands under the lanes", () => {
  const board = markup();

  it("is the closed items and the goals this build cannot place, in that order", () => {
    const lanes = board.indexOf('class="ms-board"');
    const below = board.indexOf('class="ms-board-below"');
    expect(below).toBeGreaterThan(lanes);
    expect(board.indexOf("closed items")).toBeGreaterThan(below);
    expect(board.indexOf("this build cannot place")).toBeGreaterThan(board.indexOf("closed items"));
  });

  it("names the count the disclosure opens to, and the goal in it", () => {
    expect(board).toContain("1 goal this build cannot place");
    expect(board).toContain("ui-2");
  });
});

describe("what no longer stands over the lanes", () => {
  // The board is lanes. What it is narrowed to and whether the page can act
  // are the toolbar's, one row above, and a banner about proof is nowhere.
  it("is the filters, the unplaced line, and the banner about proof", () => {
    const board = markup();
    expect(board).not.toContain("ms-board-filters");
    expect(board).not.toContain("ms-board-unproven");
    expect(board).not.toContain("started by an agent process");
    expect(board.indexOf('class="ms-board-frame"')).toBeLessThan(board.indexOf('class="ms-board"'));
  });
});
