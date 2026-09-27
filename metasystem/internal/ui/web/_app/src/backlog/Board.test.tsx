import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Backlog, Row } from "./api";
import { noFilters } from "./filters";
import { Board } from "./Board";
import type { Proposal } from "../partner/api";
import { cardsIn } from "../partner/proposing";
import { PartnerAs } from "../partner/store";

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
    holds: [],
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

function markup(proposals: readonly Proposal[] = []): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ proposals: cardsIn([{ turn: "t1", proposals }], {}, {}, []) }}>
          <Board
            backlog={backlog}
            view="board"
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
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

/** One act the Partner proposed, in the shape the snapshot carries it. */
function proposal(over: Partial<Proposal> = {}): Proposal {
  return {
    index: 0,
    verb: "park-goal",
    goal: "ui-1",
    title: "The board reads.",
    fields: { because: "superseded by the seat inventory (g1-s42)" },
    read: null,
    why: "the inventory covers what this was for",
    offered: true,
    reason: "",
    state: "waiting",
    words: "",
    at: "2026-09-26T09:00:00Z",
    version: 1,
    ...over,
  };
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

/**
 * What the Partner proposed about a goal, on that goal's own card.
 *
 * The board is where a morning of triage happens, and until this a card said
 * nothing about a proposal waiting on its goal. The chip is last in the card's
 * chips — after the facts the ledger holds — and it is the one of them that is a
 * control: it opens the conversation at the line (g1-s61 D2).
 */
describe("the chip on a board card", () => {
  it("stands last in the card's chips, named for the goal and the count", () => {
    const board = markup([proposal()]);

    expect(board).toContain('aria-label="Not now proposed on ui-1, 1 action"');
    expect(board).toContain("Not now proposed");
    expect(board.indexOf("ms-chip-proposed")).toBeGreaterThan(board.indexOf(">tier 1<"));
  });

  it("is one chip per card, whatever a card was proposed, and only on that card", () => {
    const board = markup([proposal(), proposal({ index: 1, verb: "edit-goal" })]);

    expect(board.match(/ms-chip-proposed/g)).toHaveLength(1);
    expect(board).toContain('aria-label="2 proposed on ui-1, 2 actions"');
    expect(board).toContain(">2 proposed</button>");
  });

  it("wears the danger colour where the line was refused", () => {
    const board = markup([proposal({ state: "refused", words: "goal ui-1 is claimed by m2a" })]);

    expect(board).toContain("ms-chip-proposed--wrong");
    expect(board).toContain("Not now refused");
  });

  it("is on no card where nothing was proposed, which is the ordinary board", () => {
    expect(markup()).not.toContain("ms-chip-proposed");
    expect(markup([proposal({ goal: "no-such-goal" })])).not.toContain("ms-chip-proposed");
    expect(markup([proposal({ state: "dismissed" })])).not.toContain("ms-chip-proposed");
  });
});
