import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Need, Proposed } from "./api";
import { ProposalBlock, ProposalSheet } from "./ProposalBlock";
import type { Acts } from "./InboxRow";
import { ANSWERED_ONCE, lineOf, linesOf, NOTHING_SENDABLE, type Applying } from "./proposals";
import type { Backlog, Budget, Row } from "../backlog/api";
import { NEEDS_ITS_BUDGET } from "../partner/proposing";

/**
 * The group worked rather than read: checkboxes on the rows, a bar over what is
 * ticked, and the sheet that bar's Apply opens.
 *
 * The sheet is the whole of Astra's S60-02. A collapsed row says the act and the
 * subject; what the act would CARRY is in the open row, so a bulk Apply over
 * collapsed rows would publish arguments nobody had read. The sheet lists every
 * ticked line whole, with each approve's budget from one read made when it
 * opened, and sends what it displayed.
 */

const now = new Date("2026-09-25T11:00:00Z");

const budget: Budget = {
  elapsedLimit: "8h",
  attemptLimit: 10,
  reservedJobMinutesLimit: 1200,
  activeJobLimit: 1,
  reviewRoundLimit: 3,
};

function goalRow(id: string, tier: number): Row {
  return {
    ref: { kind: "goal", id, revision: 3 },
    where: "live",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: `What ${id} is for.`,
    nextStep: "Read the fence.",
    concluded: "",
    origin: "main",
    priority: 2,
    sequence: 1,
    tier,
    labels: [],
    arc: "",
    pinned: "",
    blockedBy: [],
    openBlockers: [],
    holds: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-09-16T09:00:00Z",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "",
    gaps: [],
  };
}

/** The backlog as the sheet's own read answered: a budget law for tier 3 only. */
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
        outcome: "current", startedAt: "", finishedAt: "", tip: "abc1234", detail: "", message: "",
        failures: 0, cadence: "", nextAt: "", succeededAt: "", succeededTip: "abc1234",
      },
    },
    admission: { answered: true, message: "" },
    workingTree: { liveFiles: 2, archivedFiles: 0 },
    authority: { proven: true, human: "Wido", reason: "" },
    budgetDefaults: { "3": budget },
    counts: {},
    draft: { statement: "" },
    rows: [goalRow("g1-s47", 3), goalRow("g1-s52", 2)],
    closed: [],
  };
}

function need(id: string, over: Partial<Need> = {}, action: Partial<Proposed> = {}): Need {
  return {
    kind: "proposal",
    id,
    title: "The seat census answers which machines are alive",
    asked: "Not now · The seat census answers which machines are alive",
    by: "the Partner",
    since: "2026-09-25T09:00:00Z",
    deadline: "",
    silence: "it stays proposed; nothing is applied",
    recommend: "",
    where: { kind: "goal", id: "g1-s44" },
    act: "apply",
    command: "",
    row: null,
    new: true,
    words: "",
    context: "",
    owner: "",
    class: "",
    due: "",
    path: "",
    goals: [],
    ...over,
    proposal: {
      turn: "t7", index: 0, verb: "park-goal", fields: { because: "superseded by the seat inventory" },
      read: null, explanation: "the inventory covers what these were for",
      state: "waiting", words: "", version: 1,
      ...action,
    },
  };
}

const parking = need("t7/0");
const alsoParking = need(
  "t7/1",
  { title: "A machine publishes its phase with every tick", where: { kind: "goal", id: "g1-s45" } },
  { index: 1 },
);
const approving = need(
  "t8/0",
  { title: "A stopped seat says why it stopped", where: { kind: "goal", id: "g1-s47" } },
  {
    turn: "t8", verb: "approve-goal", fields: {},
    read: { intent: "A stopped seat says why it stopped.", nextStep: "Read the fence.", tier: 3, labels: [] },
    explanation: "the fence work it names has landed",
  },
);
const approvingWithoutABudget = need(
  "t8/1",
  { title: "The budget law is declared per tier", where: { kind: "goal", id: "g1-s52" } },
  {
    turn: "t8", index: 1, verb: "approve-goal", fields: {},
    read: { intent: "The budget law is declared per tier.", nextStep: "Read the law.", tier: 2, labels: [] },
    explanation: "its tier has no law and no goal has been approved yet",
  },
);

/** Nothing acts in a static render; these are the handles the rows are given. */
const acts: Acts = {
  signedIn: true,
  onSignIn: () => undefined,
  onApprove: () => undefined,
  onPark: () => undefined,
  onEdit: () => undefined,
  onReturn: () => undefined,
  onWrite: () => undefined,
  writing: "",
  refusedAt: "",
  refusal: "",
  proposals: {
    lineOf: (one) => lineOf(one),
    onApply: () => undefined,
    onDismiss: () => undefined,
    onAsk: () => undefined,
    onBulkApply: () => undefined,
    onBulkDismiss: () => undefined,
    running: false,
  },
};

/** What the page holds, as much of it as a sheet reads. */
const holding: Applying = {
  lineOf: (one) => lineOf(one),
  linesOf: (many) => linesOf(many),
  run: () => undefined,
  running: false,
  captureBudgets: () => undefined,
  budgets: {},
  noteBudgets: () => undefined,
  dismiss: () => undefined,
};

function block(needs: readonly Need[], selected: readonly string[]): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <ProposalBlock
          needs={needs}
          selected={selected}
          onSelect={() => undefined}
          opened=""
          onOpen={() => undefined}
          acts={acts}
          now={now}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function sheet(needs: readonly Need[]): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <ProposalSheet
          needs={needs}
          backlog={backlog()}
          holding={holding}
          onClose={() => undefined}
          onApply={() => undefined}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the group's rows and its bar", () => {
  it("gives every row a checkbox and no button", () => {
    const markup = block([parking, alsoParking], []);

    expect(markup.match(/type="checkbox"/g)).toHaveLength(3); // the two rows, and Select all
    expect(markup).toContain("Select all shown");
    expect(markup).not.toContain(">Apply</button>");
  });

  it("puts the bar over what is ticked, and not there at all until something is", () => {
    const quiet = block([parking, alsoParking], []);
    const ticked = block([parking, alsoParking], ["t7/0", "t7/1"]);

    expect(quiet).not.toContain("ms-decisions-bar");
    expect(ticked).toContain('<span class="ms-decisions-bar-count">2 selected</span>');
    expect(ticked).toContain(">Apply</button>");
    expect(ticked).toContain(">Dismiss</button>");
    expect(ticked).toContain(">Clear</button>");
  });

  it("counts only the ticked rows this group carries", () => {
    // A human who ticked rows of the queue and came here sees this group's own
    // count: a bar over rows nobody can see is the one thing it must never be.
    const markup = block([parking], ["t7/0", "g1-s40", "g1-s41"]);

    expect(markup).toContain('<span class="ms-decisions-bar-count">1 selected</span>');
  });
});

describe("the sheet the bar's Apply opens", () => {
  it("lists every ticked line whole, in the card's own words", () => {
    const markup = sheet([parking, alsoParking]);

    expect(markup).toContain('<span class="ms-proposal-word">Not now</span>');
    expect(markup).toContain("The seat census answers which machines are alive");
    expect(markup).toContain("A machine publishes its phase with every tick");
    // Every argument the act will carry, and the Partner's own words with it.
    expect(markup).toContain('<span class="ms-proposal-label">Reason</span>');
    expect(markup).toContain("superseded by the seat inventory");
    expect(markup).toContain("The Partner: the inventory covers what these were for");
    expect(markup).toContain("Apply 2 actions");
  });

  it("shows each approve's budget from its own one read, with the source", () => {
    const markup = sheet([approving, parking]);

    expect(markup).toContain('<span class="ms-proposal-label">Budget</span>');
    expect(markup).toContain("8h elapsed · 10 attempts · 1200 reserved job-minutes · 1 active jobs · 3 review rounds");
    expect(markup).toContain("budget law for this goal");
    // And the whole of what that approval would authorise.
    expect(markup).toContain("A stopped seat says why it stopped.");
    expect(markup).toContain("Read the fence.");
  });

  it("lists an approve with no tuple, names it, and leaves it out of the press", () => {
    const markup = sheet([parking, approvingWithoutABudget]);

    expect(markup).toContain(NEEDS_ITS_BUDGET);
    expect(markup).toContain("ms-decisions-planned--out");
    // One of the two would be sent, and the head counts what would be sent.
    expect(markup).toContain("Apply 1 action");
  });

  it("refuses the press where nothing it lists can be sent", () => {
    const markup = sheet([approvingWithoutABudget]);

    expect(markup).toContain(NOTHING_SENDABLE);
    expect(markup).toMatch(/<button[^>]*disabled[^>]*>Apply<\/button>/);
  });

  it("says what the run will do, in the run's own rule", () => {
    const markup = sheet([parking, alsoParking]);

    expect(markup).toContain("One act per line, in this order, over 2 actions.");
    expect(markup).toContain("A refusal is passed and the run goes on");
    expect(markup).toContain("an answer that does not say what happened stops it");
  });
});

/**
 * A line somebody has already answered is not part of a bulk press.
 *
 * Trying one again is a retry, and a retry is one line at a time: the human
 * reads what happened, checks the goal, and presses Try again on that row. Bulk
 * Try again is a later slice, and a bar that quietly did it would be that slice
 * without the reading it needs.
 */
describe("what the sheet leaves out", () => {
  const refused = need(
    "t7/2",
    { title: "The fleet page reads a seat's whole chain", where: { kind: "goal", id: "g1-s46" } },
    { index: 2, state: "refused", words: "goal g1-s46 is claimed by m2a", version: 3 },
  );

  it("lists a line already answered, names it, and leaves it out of the press", () => {
    const markup = sheet([parking, refused]);

    expect(markup).toContain(ANSWERED_ONCE);
    expect(markup).toContain("ms-decisions-planned--out");
    expect(markup).toContain("Apply 1 action");
  });

  it("refuses the press where every ticked line has been answered", () => {
    const markup = sheet([refused]);

    expect(markup).toContain(NOTHING_SENDABLE);
    expect(markup).toMatch(/<button[^>]*disabled[^>]*>Apply<\/button>/);
  });
});
