import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Backlog, Row } from "./api";
import { EditSheet } from "./EditSheet";
import { Panel } from "./Panel";

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
    // The id and the state share the eyebrow; the goal card that used to
    // repeat the intent above the Intent field is gone.
    expect(markup).toContain("ui-1 · queued, not yet approved");
    expect(markup).not.toContain("ms-act-goal");
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

/**
 * A save in flight cannot be dismissed.
 *
 * The publication goes on whether or not this sheet is on screen, so a human
 * who pressed Cancel while one was running would watch the sheet close and
 * the edit land anyway - the one outcome nothing on the page would explain.
 * The panel already guards its single way out; what this covers is the two
 * halves of that: the guard does what it says, and this sheet opts into it.
 */
function chrome(busy: boolean): string {
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <Panel
        form
        eyebrow="Queued, not yet approved"
        title="Edit goal"
        unproven=""
        refusal=""
        note=""
        busy={busy}
        act={<button type="button">Save</button>}
        onClose={() => undefined}
      >
        <p>fields</p>
      </Panel>
    </TooltipPrimitive.Provider>,
  );
}

const SOURCE = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "EditSheet.tsx"), "utf8");

/**
 * What this sheet tells the Project Partner it may write into.
 *
 * Three of its four handed-over fields are the human's own words and the fourth
 * is the goal being edited, so the three are registered as writable and the goal
 * is not. The list, the setter and the link beside each label are read from one
 * table, because a field named in one and missing from another would be a
 * suggestion offered for something nothing can write.
 */
describe("the fields the Partner may be offered words for", () => {
  it("registers the three the human writes, with the setter and the link that go with them", () => {
    expect(SOURCE).toContain('const WRITABLE_FIELDS: Record<string, keyof EditDraft> = {');
    expect(SOURCE).toContain('Intent: "intent"');
    expect(SOURCE).toContain('"Next step": "nextStep"');
    expect(SOURCE).toContain('Labels: "labels"');
    expect(SOURCE).toContain("writable={Object.keys(WRITABLE_FIELDS)}");
    expect(SOURCE).toContain("set={putWords}");
    expect(SOURCE).toContain("opening={opening}");
    for (const field of ["Intent", "Next step", "Labels"]) {
      expect(SOURCE).toContain(`<AskThePartner opening={opening} field="${field}"`);
    }
  });

  // The goal is handed over to be read. A suggestion for it would be an offer to
  // rewrite the thing being edited.
  it("does not register the goal it is editing", () => {
    expect(SOURCE).not.toContain('Goal: "');
    expect(SOURCE).not.toContain('field="Goal"');
  });

  // And the label's own row is on screen, which is where the link stands.
  it("puts each label in a row that has room for the link beside it", () => {
    const markup = sheet();
    expect(markup.match(/class="ms-act-label"/g)).toHaveLength(3);
  });
});

/**
 * Where a proposal stands, and where the caret is reported from.
 *
 * Neither can be read from static markup — there is no conversation here to
 * carry a proposal, and no document to move a caret in — so the wiring is read
 * where it is written. What it asserts is the one thing g1-s52 moved: the block
 * renders under the field's own control rather than in the drawer, and the field
 * is what tells the store where the human is writing.
 */
describe("where the Partner's proposal stands", () => {
  it("is under each writable field, inside the sheet", () => {
    for (const field of ["Intent", "Next step", "Labels"]) {
      expect(SOURCE).toContain(`<FieldProposals opening={opening} field="${field}"`);
    }
    // Under the control and above its hint, which is where the design draws it.
    expect(SOURCE).toMatch(
      /<textarea[\s\S]*?\/>\s*<FieldProposals opening=\{opening\} field="Intent"/,
    );
    expect(SOURCE).toMatch(
      /<TokenField[\s\S]*?\/>\s*<FieldProposals opening=\{opening\} field="Labels"/,
    );
  });

  it("reports the caret from each field's own row", () => {
    expect(SOURCE).toContain("const inHand = useFieldInHand(opening);");
    for (const field of ["Intent", "Next step", "Labels"]) {
      expect(SOURCE).toContain(`inHand("${field}");`);
    }
  });

  /**
   * No press does Use this and Save at once, in this step.
   *
   * Astra's two material findings are why (F1, F2): this sheet's save reads its
   * draft and its "nothing changed" guard from the render, and guards busy on its
   * own button, so a second caller that first set the words would save the
   * previous delta, or refuse, or report a save it cannot confirm. One press
   * waits for a submission path that takes the next draft explicitly and returns
   * its real outcome.
   */
  it("offers no press that uses and saves at once", () => {
    expect(SOURCE).not.toMatch(/Use and save/i);
    expect(SOURCE).not.toMatch(/Used and saved/i);
  });
});

describe("a save in flight", () => {
  // Escape and Cancel are one guarded act inside the panel, so the button's
  // own state is what says whether that guard is shut.
  it("closes nothing: the panel's one way out is shut while it is busy", () => {
    expect(chrome(true)).toMatch(/<button[^>]*disabled[^>]*>Cancel<\/button>/);
    expect(chrome(false)).not.toMatch(/<button[^>]*disabled[^>]*>Cancel<\/button>/);
  });

  // And this sheet asks for that guard. Nothing rendered statically can say
  // so - `sending` is false until a human presses Save, and there is no
  // document here to press it in - so the wiring is read where it is written.
  it("is what this sheet reports to the panel", () => {
    expect(SOURCE).toContain("busy={sending}");
  });
});
