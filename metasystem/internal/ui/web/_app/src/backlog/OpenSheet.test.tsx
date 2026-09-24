import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Backlog, LedgerState, Row } from "./api";
import { OpenSheet } from "./OpenSheet";

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
    <OpenSheet
      backlog={backlog(over.state ?? "read", over.proven ?? true)}
      intent={over.intent}
      onClose={() => undefined}
      onDone={() => undefined}
    />,
  );
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

  it("holds the four answers as radio groups in the kit's own words", () => {
    const markup = sheet();
    expect(markup.split('role="radiogroup"').length - 1).toBe(4);
    expect(markup).toContain("How severe could the harm be if the change is wrong?");
    expect(markup).toContain("How unfamiliar is the approach to the system and its independent examiners?");
    expect(markup).toContain("How many users or systems can it affect?");
    expect(markup).toContain("How much change has accumulated since the last broad examination of the touched area?");
    expect(markup).toContain("visible and reversible on one machine");
    expect(markup).toContain("a new law, verb, schema, seam or role");
  });

  it("holds the basis, the derived tier as text, and the override", () => {
    const markup = sheet();
    expect(markup).toContain('id="ms-open-basis"');
    expect(markup).toContain("Tier 1, the worse of severity 1 and novelty 1");
    expect(markup).toContain('id="ms-open-tier"');
    // The why appears only once a tier that is not the derived one is chosen.
    expect(markup).not.toContain('id="ms-open-why"');
  });

  it("holds the labels and the blocker behind More", () => {
    const markup = sheet();
    expect(markup).toContain(">More</span>");
    expect(markup).toContain('id="ms-open-labels"');
    expect(markup).toContain('id="ms-open-blocks"');
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
