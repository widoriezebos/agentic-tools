import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Briefing } from "./pane";
import { GoalBlock } from "./ProjectPane";
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
    expect(SOURCE).toContain("<GoalBlock briefing={briefing} ledger={ledger} onEdited={onReload} onReread={onLedger} />");

    const block = bodyOf("GoalBlock");

    expect(block).toContain("onReread: (after: Backlog) => void;");
    expect(block).toContain("onReread={onReread}");
    // The reload belongs to the one outcome that earns it. A second call to it
    // in this block is the reread taking the sheet down with the page.
    expect(block.match(/onEdited\(\)/g)).toHaveLength(1);
    expect(block).toContain("onDone={() => {\n            setEditing(false);\n            onEdited();\n          }}");
    // And the row the sheet is given is found in whatever board this block
    // holds, so once the reread has set it the sheet shows the row as the
    // ledger now has it.
    expect(block).toContain("const mine = ledger?.rows.find((row) => row.ref.id === goal.id);");
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
    // that stands — that row's own conversation is opened where it was left.
    expect(sittings).toContain("if (row.standing) {\n      showSitting(row.record.path);\n      go();\n      return;\n    }");
    // A review's row is its door and never starts anything.
    expect(sittings).toContain("reviewPath(row.record.path)");
    // One start in the whole of it, reached only past those two.
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

  function header(proposals: readonly Proposal[]): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PartnerAs held={{ proposals: cardsIn([{ turn: "t1", proposals }], {}, {}, []) }}>
            <GoalBlock
              briefing={briefing}
              ledger={null}
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
});
