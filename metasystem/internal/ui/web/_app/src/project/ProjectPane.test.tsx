import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Pane } from "./api";
import type { Briefing } from "./pane";
import { GoalBlock, noPageAt } from "./ProjectPane";
import type { Backlog, Row } from "../backlog/api";
import { offersFor } from "../backlog/menu";
import type { Proposal } from "../partner/api";
import { cardsIn } from "../partner/proposing";
import { PartnerAs } from "../partner/store";

/**
 * Where the goal page puts a save nobody could confirm.
 *
 * The claim is about wiring and cannot be read from a render: the sheet's
 * unresolved outcome is an asynchronous answer from the network, and these
 * tests render to static markup and have no document to press a button in. So
 * the source is the subject, and what is asserted is the one thing that
 * decides whether the human keeps their words — which of this pane's two
 * writers the sheet's reread is handed to.
 */

const SOURCE = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "ProjectPane.tsx"), "utf8");

/**
 * The text of one function of this file, from its own `function` line to the
 * next one at the top of the file, so that a claim about what is inside
 * `GoalBlock` cannot be satisfied by something written outside it.
 */
function bodyOf(name: string): string {
  const from = SOURCE.indexOf(`function ${name}(`);
  expect(from, `${name} is a function of ProjectPane.tsx`).toBeGreaterThan(-1);
  const next = SOURCE.indexOf("\nfunction ", from + 1);
  return SOURCE.slice(from, next === -1 ? SOURCE.length : next);
}

describe("the goal page's two writers", () => {
  /**
   * `onEdited` reads the page again from the top, and reading again means a
   * loading state: it is the right thing after a save the ledger confirmed and
   * the wrong thing after one it did not, because the loading state unmounts
   * the block, the sheet inside it, the human's draft and the words saying the
   * save was not confirmed — which for a save that landed without a proof say
   * not to run it again.
   */
  it("reloads through a loading state, which is why the reread cannot go there", () => {
    const briefed = bodyOf("Briefed");

    expect(briefed).toContain("const reload = () => {");
    expect(briefed).toContain('setRead({ state: "loading" });');
  });

  /**
   * So the sheet's reread is handed the pane's other writer: the one that sets
   * the board it already read, in place. Nothing unmounts, nothing loads, and
   * the ledger is read once — by the sheet — rather than twice.
   */
  it("hands the sheet's reread the board setter, not the page reload", () => {
    expect(SOURCE).toMatch(/<GoalBlock[^>]*onEdited=\{onReload\}\s+onReread=\{onLedger\}\s+\/>/);

    const block = bodyOf("GoalBlock");

    expect(block).toContain("onReread: (after: Backlog) => void;");
    expect(block).toContain("onReread={onReread}");
    // The reload belongs to the one outcome that earns it. A second call to it
    // in this block is the reread taking the sheet down with the page.
    expect(block.match(/onEdited\(\)/g)).toHaveLength(1);
    expect(block).toContain("const done = () => {\n    setOpened(null);\n    onEdited();\n  };");
    expect(block.match(/onDone=\{done\}/g)).toHaveLength(3);
    expect(block.match(/onReread=/g)).toHaveLength(1);
    // And the row the sheet is given is found in whatever board this block
    // holds, so once the reread has set it the sheet shows the row as the
    // ledger now has it.
    expect(block).toContain(
      "const mine = ledger === null ? undefined : [...ledger.rows, ...ledger.closed].find((row) => row.ref.id === goal.id);",
    );
  });
});

/**
 * What a press on a Sittings row starts, and what it must not.
 *
 * The claim is about wiring for the same reason as the one above: the press is an
 * asynchronous start followed by a navigation, and these tests render to static
 * markup with no document to press a button in. What is asserted is the one thing
 * that decides whether a human keeps the sitting they are in — which standing the
 * guard is on.
 */
describe("the Sittings row press", () => {
  it("opens a standing sitting where it was left, and starts one only on a row nothing stands on", () => {
    const sittings = bodyOf("Sittings");

    // A sitting is a conversation of its own (g1-s65 D16), so a start from
    // another row no longer replaces the mark a human is in the middle of: it
    // starts beside it. What still must not happen is a second start on a row
    // that stands — that row's room is opened where it was left (g1-s67 D6);
    // which row goes where is sittingRowPress's, tested in the room's tests.
    expect(sittings).toContain("const press = sittingRowPress(row);\n    if (\"go\" in press) {\n      void navigate(press.go);\n      return;\n    }");
    // The start, once it opened, goes into its room.
    expect(sittings).toContain("void navigate(sittingPath(opened));");
    // One start in the whole of it, reached only past the doors.
    expect(sittings.match(/startSitting\(/g)).toHaveLength(1);
    // And the row says which of the two the press will do before it is pressed.
    expect(sittings).toContain("title={opensLine(row, stands)}");
  });
});

describe("the chip on a goal's header", () => {
  const briefing: Briefing = {
    goal: {
      id: "g1-s44",
      title: "The seat census answers which machines are alive",
      state: "queued",
      intent: "One read of the census answers which machines are alive.",
      found: true,
      count: 3,
    },
    books: [],
    decisions: [],
    designs: { open: [], runs: [] },
    questions: [],
    slices: null,
    needsYou: { questions: 0, designs: 0 },
    checkout: { records: 0, homes: 0, goals: 0, problems: 0 },
    across: { decisions: [], designs: [], questions: [] },
    scopes: {
      decisions: { own: 0, underGoals: 0 },
      designs: { own: 0, underGoals: 0 },
      questions: { own: 0, underGoals: 0 },
    },
  };

  function proposal(over: Partial<Proposal> = {}): Proposal {
    return {
      index: 0,
      verb: "park-goal",
      goal: "g1-s44",
      title: "The seat census answers which machines are alive",
      fields: { because: "superseded by the seat inventory (g1-s42)" },
      read: null,
      why: "the inventory covers what these were for",
      offered: true,
      reason: "",
      state: "waiting",
      words: "",
      at: "2026-09-26T09:00:00Z",
      version: 1,
      ...over,
    };
  }

  function header(proposals: readonly Proposal[], ledger: Backlog | null = board({}), read = ledger !== null): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PartnerAs held={{ proposals: cardsIn([{ turn: "t1", proposals }], {}, {}, []) }}>
            <GoalBlock
              briefing={briefing}
              ledger={ledger}
              ledgerRead={read}
              onEdited={() => undefined}
              onReread={() => undefined}
            />
          </PartnerAs>
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  it("stands in the head, after the goal's own chip", () => {
    const markup = header([proposal()]);

    expect(markup).toContain('aria-label="Pause proposed on g1-s44, 1 action"');
    expect(markup.indexOf("ms-chip-proposed")).toBeGreaterThan(markup.indexOf(">queued<"));
    expect(markup.indexOf("ms-chip-proposed")).toBeLessThan(markup.indexOf("ms-project-count"));
  });

  it("is absent where nothing about this goal waits", () => {
    expect(header([])).not.toContain("ms-chip-proposed");
    expect(header([proposal({ goal: "refunds" })])).not.toContain("ms-chip-proposed");
    expect(header([proposal({ state: "applied" })])).not.toContain("ms-chip-proposed");
  });

  // The goal file in the server's working tree can lag the ledger the Backlog
  // reads, so the board's row is what says where the goal stands, whether it
  // is still open or already closed, and a board not read names no state.
  it("says where the board has the goal, and the goal file's word only where the board has no row", () => {
    const claimed = header([], board({ rows: [boardRow("g1-s44", "claimed")] }));
    expect(claimed).toContain(">claimed<");
    expect(claimed).not.toContain(">queued<");

    expect(header([], board({ closed: [boardRow("g1-s44", "done")] }))).toContain(">done<");
    expect(header([], board({ rows: [boardRow("g1-s45", "claimed")] }))).toContain(">queued<");
    expect(header([])).toContain(">queued<");
    // Not read: the read for this goal is still out, or it failed.
    for (const unread of [header([], board({ rows: [boardRow("g1-s44", "claimed")] }), false), header([], null)]) {
      expect(unread).not.toMatch(/>(claimed|queued)</);
      expect(unread).toContain("ms-briefing-title");
    }
  });
});

/** One row of the board, as far as a goal page reads it. */
function boardRow(id: string, state: string): Backlog["rows"][number] {
  return { ref: { kind: "goal", id, revision: 1 }, state, lane: "to-do" } as Backlog["rows"][number];
}

/** The board, as far as a goal page reads it: its open rows and its closed ones. */
function board(over: Partial<Pick<Backlog, "rows" | "closed">>): Backlog {
  return { rows: [], closed: [], ...over } as Backlog;
}

describe("an address this page does not have", () => {
  const book = { index: null, chapters: [] };
  const read: Pane = {
    schemaVersion: 6,
    readAt: "2026-09-22T10:11:12Z",
    goals: [{ id: "g1-s44", title: "g1-s44", state: "queued", intent: "One read of the census." }],
    records: [],
    intent: book,
    doctrine: book,
    questions: [],
    problems: [],
    documents: [],
    sittings: [],
  };

  it("is a tab that is not one of the page's own, and never an address naming none", () => {
    expect(noPageAt(read, null, null, "no-such-tab")).toBe(true);
    expect(noPageAt(read, "g1-s44", board({}), "documents")).toBe(true);

    expect(noPageAt(read, null, null, undefined)).toBe(false);
    // A tab of the page with nothing in it today is still a tab of the page.
    expect(noPageAt(read, null, null, "decisions")).toBe(false);
    expect(noPageAt(read, "g1-s44", board({}), "slices")).toBe(false);
  });

  it("is a goal neither the project's reading nor the board carries, open or closed", () => {
    expect(noPageAt(read, "no-such-goal", board({ rows: [boardRow("g1-s44", "queued")] }), undefined)).toBe(true);

    expect(noPageAt(read, "g1-s44", board({}), undefined)).toBe(false);
    expect(noPageAt(read, "g1-s50", board({ rows: [boardRow("g1-s50", "queued")] }), undefined)).toBe(false);
    expect(noPageAt(read, "g1-s51", board({ closed: [boardRow("g1-s51", "done")] }), undefined)).toBe(false);
  });

  // A board still being read, or one that could not be, has not said the goal
  // is nowhere: the page keeps what it shows while that is so.
  it("is never said of a goal before the board has been read", () => {
    expect(noPageAt(read, "no-such-goal", null, undefined)).toBe(false);
  });

  it("is answered with the not-found pane in place of the page, its tabs and its actions", () => {
    const briefed = bodyOf("Briefed");

    expect(briefed).toContain(
      'if (read.state === "read" && noPageAt(read.pane, goal, readFor === goal ? ledger : null, tab)) {\n    return <NotFoundPane />;\n  }',
    );
  });
});

/**
 * The head of a goal's page: what the goal is, the facts its row carries, and
 * the acts the backlog card offers for it, as buttons.
 */
describe("a goal page's head", () => {
  const briefing: Briefing = {
    goal: {
      id: "g1-s44",
      title: "The seat census answers which machines are alive.",
      state: "queued",
      intent: "What: The seat census answers which machines are alive. Why: nobody knows today.",
      found: true,
      count: 3,
    },
    books: [],
    decisions: [],
    designs: { open: [], runs: [] },
    questions: [],
    slices: null,
    needsYou: { questions: 0, designs: 0 },
    checkout: { records: 0, homes: 0, goals: 0, problems: 0 },
    across: { decisions: [], designs: [], questions: [] },
    scopes: {
      decisions: { own: 0, underGoals: 0 },
      designs: { own: 0, underGoals: 0 },
      questions: { own: 0, underGoals: 0 },
    },
  };

  function row(over: Partial<Row> = {}): Row {
    return {
      ref: { kind: "goal", id: "g1-s44", revision: 1 },
      where: "live",
      lane: "to-do",
      phase: "",
      state: "queued",
      intent: "What: The seat census answers which machines are alive.",
      nextStep: "Read the census once and say which machines answered, with the time each last did.",
      concluded: "",
      origin: "human",
      priority: 1,
      sequence: 9,
      tier: 2,
      labels: [],
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

  function board(rows: Row[], closed: Row[] = []): Backlog {
    return {
      schemaVersion: 1,
      observedAt: "2026-10-03T08:00:00Z",
      ledger: {
        state: "read",
        tip: "abc1234",
        committedAt: "2026-10-03T07:00:00Z",
        freshness: { state: "current", since: "2026-10-03T07:59:00Z", detail: "" },
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
      workingTree: { liveFiles: rows.length, archivedFiles: 0 },
      authority: { proven: true, human: "Wido", reason: "" },
      budgetDefaults: {},
      counts: {},
      draft: { statement: "" },
      rows,
      closed,
    };
  }

  function head(ledger: Backlog | null, about: Briefing = briefing): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PartnerAs held={{}}>
            <GoalBlock
              briefing={about}
              ledger={ledger}
              ledgerRead={ledger !== null}
              onEdited={() => undefined}
              onReread={() => undefined}
            />
          </PartnerAs>
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  /** The words a reader sees, with every tag and attribute taken out. */
  function words(markup: string): string {
    return markup.replace(/<[^>]*>/g, " ").replace(/&#x27;/g, "'").replace(/\s+/g, " ");
  }

  const neighbour = row({ ref: { kind: "goal", id: "g1-s45", revision: 1 }, sequence: 10 });

  it("is titled by what the goal is, and names its id once, above the title", () => {
    const markup = head(board([row(), neighbour]));

    expect(markup).toContain('<h2 class="ms-briefing-title">The seat census answers which machines are alive.</h2>');
    expect(markup).toContain('<p class="ms-facts-eyebrow ms-mono">g1-s44</p>');
    expect(words(markup).match(/g1-s44/g)).toHaveLength(1);
  });

  it("says what its count counts", () => {
    expect(words(head(null))).toContain("3 records about this goal");
  });

  it("says each fact the row carries under its label, and the next step whole", () => {
    const said = words(head(board([row({ claim: { machine: "m1f", lineage: "steward-seat", at: "", landingAt: "" } })])));

    expect(said).toContain("tier 2");
    expect(said).toContain("priority 1 · #9");
    expect(said).toContain("held by m1f");
    expect(said).not.toContain("steward-seat");
    expect(said).toContain("Next step Read the census once and say which machines answered, with the time each last did.");
  });

  it("leaves out a fact the row does not carry, rather than showing it empty", () => {
    const markup = head(board([row({ tier: 0, priority: 0, sequence: 0, where: "archive", nextStep: "" })]));

    expect(markup).not.toContain("ms-goal-facts");
    expect(markup).not.toContain("Next step");
    expect(words(head(board([row({ priority: 0, sequence: 0 })])))).toContain("no priority");
  });

  it("offers a button for each act the backlog card offers its row, except opening the goal", () => {
    const rows = [row(), neighbour];
    const markup = head(board(rows));
    const buttons = [...markup.matchAll(/<button type="button" class="ms-button">([^<]*)<\/button>/g)].map((found) => found[1]);
    const offered = offersFor(rows[0], rows).map((offer) => offer.label);

    expect(offered).toContain("Open goal");
    expect(buttons).toEqual(offered.filter((label) => label !== "Open goal"));
    expect(buttons).toEqual(["Ask about this", "Edit…", "Approve…", "Move down", "Prioritize…"]);
  });

  it("says in plain words why a goal a seat holds is not edited here", () => {
    const held = row({
      lane: "in-progress",
      state: "claimed",
      priority: 0,
      sequence: 0,
      claim: { machine: "m1f", lineage: "steward-seat", at: "", landingAt: "" },
    });
    const markup = head(board([held]));
    const buttons = [...markup.matchAll(/<button type="button" class="ms-button">([^<]*)<\/button>/g)].map((found) => found[1]);

    expect(buttons).toEqual(["Ask about this"]);
    expect(words(markup)).toContain("m1f is working on this goal, so it can't be edited here.");
    expect(markup).not.toContain("terminal");
  });

  it("says a goal that is over has nothing to press but a question, rather than nothing at all", () => {
    const over = row({ lane: "done", state: "done", where: "archive", priority: 0, sequence: 0 });
    const markup = head(board([neighbour], [over]));
    const buttons = [...markup.matchAll(/<button type="button" class="ms-button">([^<]*)<\/button>/g)].map((found) => found[1]);

    expect(words(markup)).toContain("tier 2");
    expect(words(markup)).toContain("Next step Read the census once");
    expect(buttons).toEqual(["Ask about this"]);
    expect(words(markup)).toContain("This goal is done, so nothing here changes it.");
  });

  it("leaves the page of a goal the ledger does not carry as it was, its count bare", () => {
    const missing: Briefing = {
      ...briefing,
      goal: { id: "g1-s99", title: "g1-s99", state: "", intent: "", found: false, count: 0 },
    };
    const markup = head(board([row(), neighbour]), missing);

    expect(markup).toBe(
      '<section class="ms-briefing-block"><p class="ms-facts-eyebrow ms-mono">g1-s99</p>' +
        '<div class="ms-briefing-head"><h2 class="ms-briefing-title">g1-s99</h2><span class="ms-project-count">0</span>' +
        '<span class="ms-briefing-act"><a class="ms-briefing-link" href="/backlog?goal=g1-s99" data-discover="true">Show on the board →</a></span></div>' +
        '<p class="ms-project-reason">The ledger carries no goal named g1-s99.</p></section>',
    );
  });

  it("keeps what it says today where the board was not read", () => {
    const markup = head(null);

    expect(markup).toContain('<h2 class="ms-briefing-title">The seat census answers which machines are alive.</h2>');
    expect(markup).not.toContain("ms-goal-acts");
    expect(markup).not.toContain("ms-goal-facts");
  });
});
