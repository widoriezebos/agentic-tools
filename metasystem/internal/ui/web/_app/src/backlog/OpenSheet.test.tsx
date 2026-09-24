import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Backlog, LedgerState, Row } from "./api";
import { OpenSheet } from "./OpenSheet";
import { SCORES } from "./opening";

/**
 * The New goal sheet as one screen.
 *
 * Three of this slice's claims are claims about what is rendered, and none of
 * them can be read from a table: that both disclosures are closed when the
 * sheet opens, that the id fills itself from the intent the sheet was given,
 * and that no banner about proof stands in front of the form. All three are
 * asserted from the markup the sheet renders.
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
    openedAt: "",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "open",
    gaps: [],
    ...over,
  };
}

function backlog(state: LedgerState = "read", proven = true): Backlog {
  return {
    schemaVersion: 1,
    observedAt: "2026-09-24T08:00:00Z",
    ledger: {
      state,
      tip: "abc1234",
      committedAt: "2026-09-24T07:00:00Z",
      freshness: { state: "current", since: "2026-09-24T07:59:00Z", detail: "" },
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
    authority: proven
      ? { proven: true, human: "Wido", reason: "" }
      : { proven: false, human: "", reason: "This interface cannot act as a human" },
    budgetDefaults: {},
    counts: {},
    draft: { statement: "" },
    rows: [row()],
    closed: [row({ ref: { kind: "goal", id: "ui-0", revision: 3 }, lane: "done" })],
  };
}

function sheet(over: { state?: LedgerState; proven?: boolean; intent?: string } = {}): string {
  return renderToStaticMarkup(
    // The pills carry the shell's tooltip, and the shell's tooltip is a Radix
    // root that needs its provider; the application's is in App.tsx.
    <TooltipPrimitive.Provider>
      <OpenSheet
        backlog={backlog(over.state ?? "read", over.proven ?? true)}
        intent={over.intent}
        onClose={() => undefined}
        onDone={() => undefined}
      />
    </TooltipPrimitive.Provider>,
  );
}

/** Every phrase the sheet actually puts on screen under a risk row. */
function chosen(markup: string): string[] {
  return [...markup.matchAll(/<p class="ms-act-chosen">([^<]*)<\/p>/g)].map((found) => found[1]);
}

describe("the head of the New goal sheet", () => {
  it("says where the goal lands and what opening it does not do", () => {
    const markup = sheet();
    expect(markup).toContain("Opens in To Do, not yet approved");
    expect(markup).toContain("New goal");
    expect(markup).not.toContain("Intake → To Do");
  });

  // An act that needs a human opens the sign-in sheet by itself, so a warning
  // in front of a form nobody has filled in yet is a warning about nothing.
  it("carries no banner about proof, proven or not", () => {
    expect(sheet({ proven: false })).not.toContain("ms-act-unproven");
    expect(sheet({ proven: true })).not.toContain("ms-act-unproven");
  });

  it("says in one muted line when the ledger cannot be read, and turns the act off", () => {
    const markup = sheet({ state: "broken" });
    expect(markup).toContain("ms-act-cannot");
    expect(markup).toContain("ledger cannot be read");
    expect(markup).toMatch(/<button[^>]*disabled[^>]*>Open goal<\/button>/);
    expect(sheet()).not.toContain("ms-act-cannot");
  });
});

describe("the three fields", () => {
  it("are the whole form, each with its own hint and an example", () => {
    const markup = sheet();
    expect(markup).toContain('id="ms-open-id"');
    expect(markup).toContain('id="ms-open-intent"');
    expect(markup).toContain('id="ms-open-nextStep"');
    expect(markup).toContain("e.g. refund-worker");
    expect(markup).toContain("what done looks like");
    // The two lines a human writes grow, so they are fields and not inputs.
    expect(markup).toMatch(/<textarea[^>]*id="ms-open-intent"/);
    expect(markup).toMatch(/<textarea[^>]*id="ms-open-nextStep"/);
  });

  // The rule the disabled button used to carry stands beside the field it is
  // about, rather than under the whole form.
  it("carry the id rule as the Id field's hint", () => {
    expect(sheet()).toContain("The short name every seat will use for this goal");
  });

  it("fill the id from the intent the sheet was opened on", () => {
    const markup = sheet({ intent: "Refunds are issued within a day" });
    expect(markup).toContain('value="refunds-are-issued-within-a-day"');
  });

  it("suggest nothing where nothing knows what the goal is for", () => {
    expect(sheet()).toMatch(/<input[^>]*id="ms-open-id"[^>]*value=""/);
  });
});

describe("what is disclosed", () => {
  it("is closed when the sheet opens, both of it", () => {
    const markup = sheet();
    expect(markup).toContain("<details class=\"ms-act-disclosure\">");
    expect(markup).not.toContain("<details class=\"ms-act-disclosure\" open");
    // Two disclosures and no more: the risk answers, and More.
    expect(markup.split('<details class="ms-act-disclosure"').length - 1).toBe(2);
  });

  // The sheet opens the risk disclosure once, when all three answers are in
  // and the act is waiting on the basis inside it. Two of three is not that
  // moment, so a sheet opened from a design's page is still closed.
  it("stays closed while the three answers are not all in", () => {
    const markup = sheet({ intent: "Refunds are issued within a day" });
    expect(markup).not.toContain('<details class="ms-act-disclosure" open');
  });

  it("names the tier the answers derive on the row that opens it", () => {
    expect(sheet()).toContain("Tier 1 · from the four answers below");
  });

  it("holds the four answers as one row each, as radio groups, in the kit's words", () => {
    const markup = sheet();
    expect(markup.split('role="radiogroup"').length - 1).toBe(4);
    expect(markup.split('class="ms-act-score-line"').length - 1).toBe(4);
    expect(markup).toContain("How severe could the harm be if the change is wrong?");
    expect(markup).toContain("How unfamiliar is the approach to the system and its independent examiners?");
    expect(markup).toContain("How many users or systems can it affect?");
    expect(markup).toContain("How much change has accumulated since the last broad examination of the touched area?");
    // Three pills per row, and the pill is the number: what it means is under
    // the row, in the tooltip, and in the accessible name.
    expect(markup.split('class="ms-act-pill"').length - 1).toBe(12);
    expect(markup).toContain('aria-label="2, new logic inside an existing owner"');
  });

  // One phrase per row, and the one that was chosen: eleven of the twelve
  // sentences are off the screen until a human asks for them.
  it("puts one phrase on screen per row, and it is the chosen stop's", () => {
    const shown = chosen(sheet());
    expect(shown).toEqual(SCORES.map((score) => score.stops[0]));
    expect(shown).toHaveLength(4);
    // The other two of a scale are nowhere in the visible lines.
    expect(shown).not.toContain(SCORES[0].stops[1]);
    expect(shown).not.toContain(SCORES[0].stops[2]);
  });

  it("holds the why-these-answers line, and calls it that", () => {
    const markup = sheet();
    expect(markup).toContain('id="ms-open-basis"');
    expect(markup).toContain(">Why these answers</label>");
    expect(markup).toContain("One line saying why those four answers are the answers.");
    // The field the request carries is unchanged; only its name is new.
    expect(markup).not.toContain(">Basis</label>");
  });

  // The tier is derived, so it is a statement and not a fifth question. The
  // select is a link away, for the case the engine allows and a human rarely
  // wants.
  it("states the tier and keeps the override behind a link", () => {
    const markup = sheet();
    expect(markup).toContain("Tier 1, from severity 1 and novelty 1.");
    expect(markup).toContain("Record a different tier…");
    expect(markup).not.toContain('id="ms-open-tier"');
    expect(markup).not.toContain('id="ms-open-why"');
    expect(markup).not.toContain("Keep the derived tier");
  });

  it("holds the labels and both directions of the relation behind More", () => {
    const markup = sheet();
    expect(markup).toContain(">More</span>");
    expect(markup).toContain('id="ms-open-labels"');
    expect(markup).toContain('id="ms-open-blocks"');
    expect(markup).toContain('id="ms-open-blockedBy"');
  });

  // No field there is free text any more: all three are comboboxes over what
  // this page has already loaded, and none is the plain box it used to be.
  it("asks for both dependency directions and the labels through the shell fields", () => {
    const markup = sheet();
    expect(markup.split('role="combobox"').length - 1).toBe(3);
    expect(markup).toContain('aria-controls="ms-open-blocks-list"');
    expect(markup).toContain('aria-controls="ms-open-blockedBy-list"');
    expect(markup).toContain('aria-controls="ms-open-labels-list"');
    expect(markup).not.toMatch(/<input[^>]*id="ms-open-blocks"[^>]*value="[^"]/);
    // The list is a human's own request: nothing is open when the sheet is.
    expect(markup).not.toContain('role="listbox"');
  });

  // The consequence of naming a dependency, said before it is named rather
  // than after the act. Two directions read almost the same, so each names
  // the goal that ends up waiting.
  it("says who waits, under each of the two fields that records it", () => {
    const markup = sheet();
    expect(markup).toContain("The chosen goals wait for this one.");
    expect(markup).toContain("This goal waits for the chosen ones and parks until they are done.");
  });
});

describe("the foot", () => {
  it("is outside the body that scrolls, so the act is never below the fold", () => {
    const markup = sheet();
    const body = markup.indexOf('class="ms-act-body"');
    const foot = markup.indexOf('class="ms-act-foot"');
    expect(body).toBeGreaterThan(-1);
    expect(foot).toBeGreaterThan(body);
    expect(markup).toContain("Open goal");
    expect(markup).toContain("Cancel");
  });

  it("says what the act is waiting on, one field at a time", () => {
    expect(sheet()).toContain("A goal is named by one id");
  });
});
