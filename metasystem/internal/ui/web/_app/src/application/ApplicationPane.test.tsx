import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Landed, Page, Problem } from "./api";
import { Blocks } from "./ApplicationPane";
import { noNarrowing, type Narrowing } from "./application";

/**
 * What the page puts on the screen, over a payload and nothing else.
 *
 * The reading and the network belong to the pane; these are the header and the
 * three blocks, rendered as the browser would render them. Every assertion is
 * a sentence a human reads: the week's line, the row's one line, what an open
 * row holds, and the register's own words.
 */

function landed(over: Partial<Landed> = {}): Landed {
  return {
    id: "g1-s48",
    intent: "The Decisions inbox is done",
    concluded: "landed in 9017baa. The inbox groups collapse and one row opens at a time.",
    doneAt: "2026-09-24T09:00:00",
    labels: ["browser-interface"],
    arc: "",
    new: false,
    ...over,
  };
}

/** Six weeks of concluded work, so "Show earlier" has something to hide. */
const sixWeeks: Landed[] = [
  landed({ id: "g1-s48", doneAt: "2026-09-25T09:00:00", new: true }),
  landed({ id: "g1-s47", intent: "The goal editor saves a queued goal", doneAt: "2026-09-18T09:00:00", labels: ["browser-interface"] }),
  landed({ id: "g1-s42", intent: "The fleet page reads a whole chain", doneAt: "2026-09-11T09:00:00", labels: ["headless-fleet"] }),
  landed({ id: "g1-s38", intent: "The census answers which machines are alive", doneAt: "2026-09-04T09:00:00", labels: ["headless-fleet"] }),
  landed({
    id: "g1-s30", intent: "A second bundler beside the first",
    concluded: "Obsolete: self-declared duplicate holding no work",
    doneAt: "2026-08-28T09:00:00", labels: [],
  }),
  landed({ id: "g1-s24", intent: "The toolchain and the committed bundle", doneAt: "2026-08-21T09:00:00", labels: ["robustness"], arc: "covenant-harvest" }),
];

function problem(over: Partial<Problem> = {}): Problem {
  return {
    id: "KI-25",
    date: "2026-08-07",
    what: "Design-critic follow-up rounds review a stale tree. The worktree freezes at round one.",
    consequence: "Two consecutive rounds re-reported already-folded findings",
    lever: "Dispatch follow-up syncs the worktree for read-only critic roles",
    status: "OPEN",
    open: true,
    ...over,
  };
}

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: "2026-09-25T11:00:00",
    subject: "MetaSystem",
    mode: "self-hosted",
    engine: { build: "5b9d958", generation: 4, publishedAt: "2026-09-25T10:45:00" },
    counts: { landed: 429, thisMonth: 320, new: 7 },
    landed: sixWeeks,
    problems: {
      columns: ["Id", "Date", "Symptom and evidence", "Cost when it bites", "Fix direction or lever", "Status"],
      open: [problem()],
      concluded: [
        problem({
          id: "KI-2", date: "2026-08-04",
          what: "The suite's wall time grew from 2m14s to 4m38s on the same machine",
          consequence: "CI cost and slower correction loops",
          lever: "Profile the fixture set and split fast from slow",
          status: "ACCEPTED 2026-08-06 with a measured trigger", open: false,
        }),
      ],
      unread: 5,
      defects: ["row=19: wrong column count: got 3, want 6"],
      register: "metasystem/memory/known-issues.md",
    },
    docs: [
      { title: "README", path: "README.md" },
      { title: "Glossary", path: "docs/glossary.md" },
    ],
    visit: { since: "2026-09-24T11:00:00", first: false },
    ...over,
  };
}

type Shown = {
  narrowing?: Narrowing;
  openRow?: string;
  openProblem?: string;
  earlier?: boolean;
  concluded?: boolean;
};

function rendered(payload: Page, shown: Shown = {}): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Blocks
          page={payload}
          narrowing={shown.narrowing ?? noNarrowing}
          openRow={shown.openRow ?? ""}
          openProblem={shown.openProblem ?? ""}
          earlier={shown.earlier ?? false}
          concluded={shown.concluded ?? false}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the header", () => {
  it("names the subject, the mode and what each of them is", () => {
    const markup = rendered(page());

    expect(markup).toContain("MetaSystem · self-hosted");
    expect(markup).toContain("The MetaSystem as the product under development");
    expect(markup).toContain("the instance doing the work is on Fleet");
  });

  it("names an adopted application without the self-hosted sentence", () => {
    const markup = rendered(page({ subject: "ledgerly", mode: "adopted" }));

    expect(markup).toContain("ledgerly · built with MetaSystem");
    expect(markup).not.toContain("the instance doing the work is on Fleet");
  });

  it("says the engine build is the last one published, and says so when there is none", () => {
    expect(rendered(page())).toContain("last published MetaSystem engine 5b9d958 · generation 4");
    expect(rendered(page({ engine: null }))).toContain("no engine build available");
  });

  it("counts the history as goals concluded", () => {
    expect(rendered(page())).toContain("429 goals concluded · 320 this month · 7 since your last visit");
  });
});

describe("what concluded", () => {
  it("is the latest four weeks, with the rest behind Show earlier", () => {
    const markup = rendered(page());

    expect(markup).toContain("Week of 21 September 2026");
    expect(markup).toContain("Week of 31 August 2026");
    // The fifth and sixth weeks are not on the page at all until asked for.
    expect(markup).not.toContain("Week of 24 August 2026");
    expect(markup).not.toContain("Week of 17 August 2026");
    expect(markup).toContain("Show earlier");
  });

  it("shows the earlier weeks when they are asked for, and stops offering them", () => {
    const markup = rendered(page(), { earlier: true });

    expect(markup).toContain("Week of 24 August 2026");
    expect(markup).toContain("Week of 17 August 2026");
    expect(markup).not.toContain("Show earlier");
  });

  // One line: the intent, the conclusion's first sentence muted, the date, and
  // a dot where it concluded since the last visit.
  it("is one line per goal, with the conclusion's first sentence and the date", () => {
    const markup = rendered(page());

    expect(markup).toContain("The Decisions inbox is done");
    expect(markup).toContain("landed in 9017baa");
    expect(markup).not.toContain("The inbox groups collapse and one row opens at a time.");
    expect(markup).toContain("2026-09-25");
    expect(markup).toContain("1 new");
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).not.toContain('aria-expanded="true"');
  });

  it("opens one row into the whole conclusion, its labels, its arc and the way to the goal", () => {
    // The row is in the sixth week, so the earlier weeks are open with it.
    const markup = rendered(page(), { openRow: "g1-s24", earlier: true });

    expect(markup).toContain("The toolchain and the committed bundle");
    expect(markup).toContain("The inbox groups collapse and one row opens at a time.");
    expect(markup).toContain("arc covenant-harvest");
    expect(markup).toContain("Open the goal");
    expect(markup).toContain('href="/backlog/goal/g1-s24"');
    expect(markup).toContain('aria-expanded="true"');
  });

  it("narrows by Find, over the id, the intent and the conclusion", () => {
    const markup = rendered(page(), { narrowing: { find: "fleet page", label: "" } });

    expect(markup).toContain("The fleet page reads a whole chain");
    expect(markup).not.toContain("The Decisions inbox is done");
    expect(markup).toContain("6 concluded · 1 shown");
    expect(markup).toContain("Clear");
  });

  it("narrows by a chip drawn from the rows shown", () => {
    const markup = rendered(page(), { narrowing: { find: "", label: "headless-fleet" } });

    expect(markup).toContain("The fleet page reads a whole chain");
    expect(markup).toContain("The census answers which machines are alive");
    expect(markup).not.toContain("The Decisions inbox is done");
    expect(markup).toContain('aria-pressed="true"');
  });

  it("says so rather than showing an empty block where nothing matches", () => {
    const markup = rendered(page(), { narrowing: { find: "nothing here matches", label: "" } });

    expect(markup).toContain("No concluded goal matches.");
  });
});

describe("known problems", () => {
  it("lists the open rows first, one line each, and not the concluded ones", () => {
    const markup = rendered(page());

    expect(markup).toContain("KI-25");
    expect(markup).toContain("Design-critic follow-up rounds review a stale tree");
    expect(markup).not.toContain("Two consecutive rounds re-reported already-folded findings");
    expect(markup).not.toContain("The suite's wall time grew from 2m14s to 4m38s on the same machine");
    expect(markup).toContain("Show concluded (1)");
  });

  it("opens one row into the cost, the lever and the whole status", () => {
    const markup = rendered(page(), { openProblem: "KI-25" });

    expect(markup).toContain("Cost when it bites");
    expect(markup).toContain("Two consecutive rounds re-reported already-folded findings");
    expect(markup).toContain("Fix direction or lever");
    expect(markup).toContain("Dispatch follow-up syncs the worktree for read-only critic roles");
    expect(markup).toContain("Status");
  });

  // Adoption ships a second column set, and the fifth column is a different
  // question in it: an open row labelled "Fix direction or lever" over a
  // "Reopen when" cell would be the page telling the human that the register
  // says something it does not.
  it("labels an open row with its own register's column names", () => {
    const adopted = page({
      problems: {
        ...page().problems,
        columns: ["Id", "Date", "Issue", "Consequence", "Reopen when", "Status"],
      },
    });
    const markup = rendered(adopted, { openProblem: "KI-25" });

    expect(markup).toContain(">Consequence</span>");
    expect(markup).toContain(">Reopen when</span>");
    expect(markup).not.toContain("Cost when it bites");
    expect(markup).not.toContain("Fix direction or lever");
  });

  // A register with no table in it names no columns, and the block falls back
  // to its own names rather than drawing an empty label.
  it("falls back to its own names where the register carried no header", () => {
    const headless = page({ problems: { ...page().problems, columns: [] } });
    const markup = rendered(headless, { openProblem: "KI-25" });

    expect(markup).toContain(">Cost when it bites</span>");
    expect(markup).toContain(">Fix direction or lever</span>");
    expect(markup).toContain(">Status</span>");
  });

  // An accepted limitation still exists, so its word travels onto the line.
  it("shows the concluded rows when they are asked for, with the status word kept visible", () => {
    const markup = rendered(page(), { concluded: true });

    expect(markup).toContain("The suite&#x27;s wall time grew from 2m14s to 4m38s on the same machine");
    expect(markup).toContain(">ACCEPTED</span>");
    expect(markup).not.toContain("Show concluded (1)");
  });

  it("counts the rows it could not read and puts the register one click away", () => {
    const markup = rendered(page());

    expect(markup).toContain("5 issues could not be read as rows");
    expect(markup).toContain("Open the register");
    expect(markup).toContain('href="/project/doc/metasystem/memory/known-issues.md"');
  });

  it("says nothing about unread rows where every row was read", () => {
    const empty = page();
    const markup = rendered(page({ problems: { ...empty.problems, unread: 0 } }));

    expect(markup).not.toContain("could not be read");
    expect(markup).not.toContain("Open the register");
  });
});

describe("what it is", () => {
  it("is one line of links into the reader, and renders no document itself", () => {
    const markup = rendered(page());

    expect(markup).toContain("What it is");
    expect(markup).toContain('href="/project/doc/README.md"');
    expect(markup).toContain('href="/project/doc/docs/glossary.md"');
  });

  it("says so where this checkout carries none of them", () => {
    const markup = rendered(page({ docs: [] }));

    expect(markup).toContain("This checkout carries none of the documents this block links.");
  });
});
