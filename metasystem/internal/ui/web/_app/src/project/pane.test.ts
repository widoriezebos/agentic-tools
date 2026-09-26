import { describe, expect, it } from "vitest";

import type { DocumentPayload, Pane, ProjectRecord } from "./api";
import {
  ABOUT_PROJECT,
  aboutOf,
  lastAtLine,
  NOTHING_RECORDED,
  opensLine,
  pilesLine,
  STANDS_NOW,
  aboutRow,
  briefingFor,
  countLine,
  countText,
  crumbsFor,
  designNote,
  designWork,
  documentGroups,
  DEFAULT_SCOPE,
  found,
  goalGroups,
  marksDone,
  newActionFor,
  NO_SLICE_PLAN,
  nothingLine,
  noMatchLine,
  pageSections,
  projectWideLine,
  railFor,
  REFRESH,
  ROOT_GROUP,
  SCOPES,
  scopeEditable,
  scopeNote,
  scopeOf,
  shortID,
  sliceCount,
  sliceLine,
  slicePlan,
  slicePlans,
  slicesOf,
  stripActions,
  tabForKind,
  workLine,
  type Row,
  type TocEntry,
} from "./pane";

/**
 * What the briefing shows, and what the reader shows around one document,
 * from one payload.
 *
 * The fixture is the shape the server answers: four ledger goals, two of them
 * still worked under, two books whose reading order names records and binds
 * documents, a decision, four designs across the statuses, two questions, one
 * refusal, and a few documents that declare nothing.
 */

function record(fields: Partial<ProjectRecord> & Pick<ProjectRecord, "kind" | "id" | "path">): ProjectRecord {
  return {
    status: "accepted",
    goals: [],
    title: fields.id,
    home: "plans/designs",
    summary: "",
    changedAt: "2026-09-20T08:30:00Z",
    slices: [],
    ...fields,
  };
}

const intentIndex = record({
  kind: "intent", id: "intent-index", title: "The project's intent",
  path: "metasystem/docs/intent/index.md", home: "metasystem/docs/intent",
  summary: "Engineering after the shift: governing production, not building it.",
});

const doctrineIndex = record({
  kind: "doctrine", id: "doctrine-index", title: "The doctrine",
  path: "metasystem/docs/doctrine/index.md", home: "metasystem/docs/doctrine",
  summary: "A machine for letting agents build software unattended.",
});

const pane: Pane = {
  schemaVersion: 6,
  readAt: "2026-09-22T10:11:12Z",
  // The order the server answers in: the live goals first, each in id order,
  // then the concluded ones.
  goals: [
    { id: "goal-ledger", title: "goal-ledger", state: "queued",
      intent: "The ledger is the one source of open work" },
    { id: "interface-shell", title: "interface-shell", state: "claimed",
      intent: "The browser is the seat a human takes",
      sliced: { at: "2026-09-18T08:00:00Z", machine: "m1e", lineage: "coordinator" } },
    { id: "first-release", title: "first-release", state: "done", intent: "The first release shipped" },
    { id: "old-idea", title: "old-idea", state: "abandoned", intent: "An idea nobody pursued" },
  ],
  records: [
    intentIndex,
    record({
      kind: "intent", id: "intent-interface", goals: ["interface-shell"], title: "What the interface is for",
      path: "metasystem/docs/intent/interface.md", home: "metasystem/docs/intent",
      summary: "The browser is the seat a human takes.",
    }),
    doctrineIndex,
    record({
      kind: "doctrine", id: "doctrine-events", goals: ["goal-ledger"], title: "Events are the source of truth",
      path: "metasystem/docs/doctrine/events.md", home: "metasystem/docs/doctrine",
    }),
    record({
      kind: "doctrine", id: "doctrine-loose", status: "draft", goals: ["interface-shell"],
      title: "A chapter the index does not name", path: "metasystem/docs/doctrine/loose.md",
      home: "metasystem/docs/doctrine",
    }),
    record({
      kind: "decision", id: "decision-one", goals: ["goal-ledger"], title: "One binary",
      path: "metasystem/docs/decisions/0001.md", home: "metasystem/docs/decisions",
      summary: "The engine ships as one Go binary.",
    }),
    record({
      kind: "design", id: "design-pane", goals: ["interface-shell"], title: "The Project pane",
      path: "plans/designs/pane.md", summary: "The pane over the resolver.",
      slices: ["The payload carries the boundary", "The tab reads it"],
    }),
    record({
      kind: "design", id: "design-briefing", status: "draft", goals: ["interface-shell"],
      title: "The briefing", path: "plans/designs/briefing.md",
    }),
    record({
      kind: "design", id: "design-ledger", status: "done", goals: ["goal-ledger", "interface-shell"],
      title: "The ledger", path: "metasystem/plans/designs/ledger.md", home: "metasystem/plans/designs",
      slices: ["The accepted ref is fetched on a loop"],
    }),
    record({
      kind: "design", id: "design-old", status: "superseded", goals: ["interface-shell"],
      title: "The old thread", path: "plans/designs/old.md",
    }),
  ],
  intent: {
    index: intentIndex,
    chapters: [
      { path: "metasystem/docs/paper/01-the-shift.md", title: "1. The Shift", summary: "Software is no longer written by hand." },
      { id: "intent-interface", title: "2. The seat", summary: "" },
    ],
  },
  doctrine: {
    index: doctrineIndex,
    chapters: [
      { id: "doctrine-events", title: "Events are the source of truth", summary: "" },
      { id: "doctrine-absent", title: "A chapter no record declares", summary: "" },
    ],
  },
  questions: [
    { id: "Q-1", opened: "2026-09-22", question: "Where does intent live?", goals: ["interface-shell"], status: "open" },
    { id: "Q-2", opened: "2026-09-22", question: "Who accepts?", goals: ["goal-ledger"], status: "answered: decision-one" },
  ],
  problems: [{ path: "metasystem/docs/intent/index.md", line: 12, message: "the chapter names nothing" }],
  documents: [
    { path: "README.md", title: "The repository" },
    { path: "development/local.md", title: "Local rules" },
    { path: "metasystem/docs/architecture.md", title: "The engine" },
    { path: "metasystem/docs/design/principles.md", title: "Design principles" },
    { path: "metasystem/plans/g1-s1.md", title: "A historical design" },
  ],
  sittings: [],
};

function titles(rows: (Row | TocEntry)[]): string[] {
  return rows.map((row) => row.title);
}

/** One of the two books, as the whole project opens it. */
function book(id: string) {
  const found = briefingFor(pane, null).books.find((candidate) => candidate.id === id);
  if (found === undefined) {
    throw new Error(`no book ${id}`);
  }
  return found;
}

/** One document as the reading route answers it, with or without a head. */
function documentOf(id: string, head: Partial<DocumentPayload["record"]> | null): DocumentPayload {
  return {
    kind: "document", id, title: id, revision: "", source: "", owner: "app-owned", path: `/work/${id}`,
    bytes: 0, modifiedAt: "2026-09-22T09:00:00Z", readAt: "2026-09-22T10:11:12Z",
    state: "readable", reason: "",
    record:
      head === null
        ? null
        : {
            kind: "design", id: "", status: "accepted", goals: [], cites: [], affects: [],
            governs: [], supersedes: [], by: [], ...head,
          },
    referencedBy: [],
    supersededBy: [],
    headings: [],
    blocks: [],
  };
}

describe("what is on the page, as the strip of tabs names it", () => {
  it("is this page's own sections, in the order the page renders them", () => {
    expect(pageSections(briefingFor(pane, null))).toEqual([
      { id: "intent", title: "Intent", help: "intent" },
      { id: "doctrine", title: "Doctrine", help: "doctrine" },
      { id: "decisions", title: "Decisions", help: "decisions" },
      { id: "designs", title: "Designs", help: "designs" },
      { id: "questions", title: "Open questions", help: "questions" },
      { id: "sittings", title: "Sittings", help: "sittings" },
      { id: "documents", title: "Documents", help: "documents" },
    ]);
  });

  // A goal page carries neither book, and the checkout's other Markdown is
  // the project's rather than any goal's. The strip says exactly what that
  // page has, or it would offer a tab onto nothing. The goal itself is not a
  // tab: it is what the page is about, and it stands above the strip.
  it("carries only the goal's own sections on a goal page, and not the goal", () => {
    // The four names are the project's own, and none of them means the same
    // thing here: each tab's help says it is scoped to this goal.
    expect(pageSections(briefingFor(pane, "interface-shell"))).toEqual([
      { id: "decisions", title: "Decisions", help: "goal-decisions" },
      { id: "designs", title: "Designs", help: "goal-designs" },
      { id: "questions", title: "Open questions", help: "goal-questions" },
      { id: "slices", title: "Slices", help: "slices" },
    ]);
  });

  // A tab's name is one segment of the address, so it is one segment: a name
  // with a separator in it would be a second segment and a route nobody
  // registered.
  it("names every tab in one segment, on either page", () => {
    for (const goal of [null, "interface-shell"]) {
      for (const section of pageSections(briefingFor(pane, goal))) {
        expect({ id: section.id, segments: section.id.split("/").length }).toEqual({ id: section.id, segments: 1 });
        expect({ id: section.id, encoded: encodeURIComponent(section.id) }).toEqual({
          id: section.id,
          encoded: section.id,
        });
      }
    }
  });

  // The action that writes a record and the tab that shows it read from one
  // table, so no act can be offered on a tab the strip does not have.
  it("puts every kind a human can write on a tab the page carries", () => {
    const ids = pageSections(briefingFor(pane, null)).map((section) => section.id);

    for (const kind of ["intent", "doctrine", "decision", "design"]) {
      expect({ kind, tab: tabForKind(kind), carried: ids.includes(tabForKind(kind)) }).toEqual({
        kind,
        tab: tabForKind(kind),
        carried: true,
      });
    }
    expect(tabForKind("nothing")).toBe("");
  });
});

/**
 * The strip's trailing cluster, which is where contributing now happens.
 *
 * It replaced a column of four buttons that stood beside every tab and offered
 * three things the reader was not looking at. There is one act, it belongs to
 * the tab that is open, and the refresh stands beside it on every tab.
 */
describe("the one act at the trailing end of the strip", () => {
  it("is named by the tab that is open", () => {
    expect(newActionFor("intent")).toEqual({ label: "New chapter", kind: "intent" });
    expect(newActionFor("doctrine")).toEqual({ label: "New chapter", kind: "doctrine" });
    expect(newActionFor("decisions")).toEqual({ label: "New decision", kind: "decision" });
    expect(newActionFor("designs")).toEqual({ label: "New design", kind: "design" });
    expect(newActionFor("questions")).toEqual({ label: "New question", kind: null });
  });

  // Documents is the checkout's other Markdown, which this surface does not
  // write, and a slice is added in the design that lists it.
  it("is nothing at all on Documents, on Slices, and on a name no page carries", () => {
    expect(newActionFor("documents")).toBeNull();
    expect(newActionFor("slices")).toBeNull();
    expect(newActionFor("nothing-at-all")).toBeNull();
  });

  it("stands beside a refresh the strip carries on every tab of either page", () => {
    for (const goal of [null, "interface-shell"]) {
      for (const section of pageSections(briefingFor(pane, goal))) {
        const actions = stripActions(section.id);
        expect({ tab: section.id, refresh: actions.refresh }).toEqual({ tab: section.id, refresh: REFRESH });
        // Sittings writes nothing either: a sitting is started from the record
        // it is about or from the Partner's own header, and a tab that offered
        // "New sitting" would be asking which record from a page listing every
        // record there is.
        expect({ tab: section.id, writes: actions.newAction !== null }).toEqual({
          tab: section.id,
          writes: !["documents", "slices", "sittings"].includes(section.id),
        });
      }
    }
  });
});

/**
 * Project → Sittings: the records this project has sat on (g1-s55 D3).
 *
 * The rows are the payload's, so what is decided here is what a row SAYS: how
 * much of a sitting there is, when its last entry was recorded, whether one
 * stands on it now, and what pressing it will do — which a row has to say before
 * it is pressed, because the press starts a sitting.
 */
describe("what one Sittings row says", () => {
  const row = {
    record: { kind: "design", id: "design-sessions", path: "plans/designs/sessions.md", title: "Session limits" },
    counts: { facts: 2, proposals: 0, decisions: 1, questions: 1 },
    lastAt: "2026-09-26",
    standing: false,
  };

  it("counts the piles it holds, and leaves out the ones nobody wrote into", () => {
    expect(pilesLine(row.counts)).toBe("2 facts · 1 decision · 1 open question");
  });

  it("says nothing was recorded rather than counting four nothings", () => {
    expect(pilesLine({ facts: 0, proposals: 0, decisions: 0, questions: 0 })).toBe(NOTHING_RECORDED);
  });

  it("dates its last entry, and says nothing where it has none", () => {
    expect(lastAtLine(row)).toBe("last entry 2026-09-26");
    expect(lastAtLine({ ...row, lastAt: "" })).toBe("");
  });

  it("says what the press will do before it is pressed, because the press is the consent", () => {
    expect(opensLine(row, false)).toBe(
      "Open the conversation on plans/designs/sessions.md and start a sitting on it",
    );
    // A sitting already standing on it is opened and not started again.
    expect(opensLine({ ...row, standing: true }, true)).toBe(
      "Open the conversation on plans/designs/sessions.md",
    );
    // And a row nothing stands on, while a sitting stands on another record: one
    // conversation holds one sitting, so this press cannot start a second — it
    // would end the one a human is in the middle of. So it promises no start.
    expect(opensLine(row, true)).toBe(
      "Open the conversation. A sitting stands on another record, so this starts nothing",
    );
    expect(STANDS_NOW).toBe("a sitting stands on this now");
  });
});

/**
 * An empty tab says what it has none of, in its own scope. "Nothing recorded
 * yet" was said on every empty tab of every page, which told a human on a goal
 * page that a project of four hundred decisions had recorded none.
 */
describe("what an empty tab says", () => {
  it("says what this tab has none of", () => {
    expect(nothingLine("intent", false)).toBe("No chapters yet.");
    expect(nothingLine("doctrine", false)).toBe("No chapters yet.");
    expect(nothingLine("decisions", false)).toBe("No decisions yet.");
    expect(nothingLine("designs", false)).toBe("No designs yet.");
    expect(nothingLine("questions", false)).toBe("No open questions yet.");
  });

  it("says it of the goal on a goal page", () => {
    expect(nothingLine("decisions", true)).toBe("No decisions about this goal yet.");
    expect(nothingLine("designs", true)).toBe("No designs about this goal yet.");
    expect(nothingLine("questions", true)).toBe("No open questions about this goal yet.");
  });

  it("says nothing where the tab offers no act", () => {
    expect(nothingLine("documents", false)).toBe("");
    expect(nothingLine("slices", true)).toBe("");
  });
});

describe("the briefing", () => {
  it("opens each book with its own first paragraph and its reading order", () => {
    const intent = book("intent");

    expect(intent.lede).toBe("Engineering after the shift: governing production, not building it.");
    expect(intent.to).toBe("/project/doc/metasystem/docs/intent/index.md");
    expect(intent.status).toBe("accepted");
    expect(titles(intent.chapters)).toEqual(["1. The Shift", "2. The seat"]);
  });

  it("opens a bound chapter at its document and a record chapter at its own record", () => {
    const [bound, own] = book("intent").chapters;

    expect(bound.to).toBe("/project/doc/metasystem/docs/paper/01-the-shift.md");
    expect(bound.summary).toBe("Software is no longer written by hand.");
    expect(own.to).toBe("/project/doc/metasystem/docs/intent/interface.md");
    // A chapter the index left without words borrows the record's own.
    expect(own.summary).toBe("The browser is the seat a human takes.");
  });

  it("carries the records of a kind the index does not name, so none is hidden", () => {
    const doctrine = book("doctrine");

    expect(titles(doctrine.chapters)).toEqual([
      "Events are the source of truth",
      "A chapter no record declares",
      "A chapter the index does not name",
    ]);
    // A chapter naming a record no head declares opens nothing: the check verb
    // refuses it in the same breath.
    expect(doctrine.chapters[1].to).toBeNull();
  });

  it("groups designs so that what governs is open and what is finished is a run", () => {
    const designs = briefingFor(pane, null).designs;

    expect(titles(designs.open)).toEqual(["The Project pane", "The briefing"]);
    expect(designs.runs.map((run) => [run.status, run.rows.length])).toEqual([
      ["done", 1],
      ["superseded", 1],
    ]);
  });

  it("carries each row's own first words beside its title, and no chip but its status", () => {
    expect(briefingFor(pane, null).decisions).toEqual([
      {
        key: "metasystem/docs/decisions/0001.md",
        title: "One binary",
        to: "/project/doc/metasystem/docs/decisions/0001.md",
        summary: "The engine ships as one Go binary.",
        status: "accepted",
        // What the head says this record is about, carried as the head wrote
        // it: the scope control narrows by it, and the row's chip names it.
        goals: ["goal-ledger"],
        // Only a design says anything about the work it names; a decision
        // names no work, so its row carries no clause.
        note: "",
        path: "metasystem/docs/decisions/0001.md",
      },
    ]);
  });

  it("counts what is waiting and what was read", () => {
    const briefing = briefingFor(pane, null);

    expect(briefing.needsYou).toEqual({ questions: 1, designs: 1 });
    expect(briefing.checkout).toEqual({ records: 10, homes: 5, goals: 4, problems: 1 });
  });
});

describe("a goal page", () => {
  it("opens with the goal's own id, state and intent, from the ledger", () => {
    expect(briefingFor(pane, "interface-shell").goal).toEqual({
      id: "interface-shell",
      title: "interface-shell",
      state: "claimed",
      intent: "The browser is the seat a human takes",
      found: true,
      count: 6,
    });
  });

  it("says so when the ledger carries no such goal, rather than inventing one", () => {
    expect(briefingFor(pane, "nowhere").goal).toEqual({
      id: "nowhere", title: "nowhere", state: "", intent: "", found: false, count: 0,
    });
  });

  it("carries no book at all: the intent and the doctrine are the project's", () => {
    expect(briefingFor(pane, "interface-shell").books).toEqual([]);
    expect(briefingFor(pane, null).books.map((one) => one.id)).toEqual(["intent", "doctrine"]);
  });

  it("narrows every section to the records whose goals name it", () => {
    expect(titles(briefingFor(pane, "goal-ledger").decisions)).toEqual(["One binary"]);
    expect(briefingFor(pane, "interface-shell").decisions).toEqual([]);
    expect(titles(briefingFor(pane, "interface-shell").questions)).toEqual(["Where does intent live?"]);
    expect(titles(briefingFor(pane, "interface-shell").designs.open)).toEqual(["The Project pane", "The briefing"]);
    // A record about two goals is shown under both, because it is about both.
    expect(titles(briefingFor(pane, "goal-ledger").designs.runs[0].rows)).toEqual(["The ledger"]);
    expect(titles(briefingFor(pane, "interface-shell").designs.runs[0].rows)).toEqual(["The ledger"]);
  });

  it("shows a record about the whole project on the project page and on no goal page", () => {
    expect(titles(briefingFor(pane, null).questions)).toEqual(["Where does intent live?", "Who accepts?"]);
    expect(titles(briefingFor(pane, "first-release").designs.open)).toEqual([]);
  });
});

/**
 * A design's own words against the ledger's own states.
 *
 * The fixture is a ledger of three goals — one still worked under, two that
 * landed — and four designs over them: one part of the way there, one whose
 * work is all in, one that named no work at all, and one that already says it
 * is done.
 */
describe("what a design's work has done", () => {
  const ledger: Pane = {
    ...pane,
    goals: [
      { id: "live-one", title: "live-one", state: "claimed", intent: "Still being worked" },
      { id: "landed-one", title: "landed-one", state: "done", intent: "Landed first" },
      { id: "landed-two", title: "landed-two", state: "done", intent: "Landed second" },
    ],
    records: [
      record({
        kind: "design", id: "design-part", title: "Part of the way",
        goals: ["landed-one", "landed-two", "live-one"], path: "plans/designs/part.md",
      }),
      record({
        kind: "design", id: "design-landed", title: "All of it in",
        goals: ["landed-one", "landed-two"], path: "plans/designs/landed.md",
      }),
      record({ kind: "design", id: "design-standing", title: "A standing design", path: "plans/designs/standing.md" }),
      record({
        kind: "design", id: "design-shipped", status: "done", title: "Already done",
        goals: ["landed-one"], path: "plans/designs/shipped.md",
      }),
    ],
  };

  it("counts the goals a design names that have landed", () => {
    expect(workLine(designWork(ledger, ["landed-one", "landed-two", "live-one"]))).toBe("2 of 3 goals done");
    expect(workLine(designWork(ledger, ["live-one"]))).toBe("0 of 1 goal done");
  });

  it("says so as a whole once every one of them is in", () => {
    expect(workLine(designWork(ledger, ["landed-one", "landed-two"]))).toBe("all 2 goals done");
    expect(workLine(designWork(ledger, ["landed-one"]))).toBe("all 1 goal done");
  });

  it("derives nothing from a design that names no goals", () => {
    const work = designWork(ledger, []);
    expect(work).toEqual({ goals: [], done: 0, landed: false });
    expect(workLine(work)).toBe("");
    expect(marksDone(work, "accepted")).toBe(false);
  });

  it("names each goal as a link to its own page, with the ledger's word for it", () => {
    expect(designWork(ledger, ["landed-one"]).goals).toEqual([
      { id: "landed-one", name: "landed-one", to: "/backlog/goal/landed-one", state: "done", done: true },
    ]);
  });

  it("counts a goal the ledger does not carry as work that has not landed", () => {
    const work = designWork(ledger, ["landed-one", "nowhere"]);
    expect(workLine(work)).toBe("1 of 2 goals done");
    expect(work.goals.map((goal) => [goal.id, goal.state, goal.done])).toEqual([
      ["landed-one", "done", true],
      ["nowhere", "", false],
    ]);
    expect(marksDone(work, "accepted")).toBe(false);
  });

  it("reads them in the ledger's order, with the ones it does not carry last", () => {
    expect(designWork(ledger, ["nowhere", "landed-two", "live-one"]).goals.map((goal) => goal.id)).toEqual([
      "live-one",
      "landed-two",
      "nowhere",
    ]);
  });

  it("offers to mark a design done only when it named work, all of it landed, and it has not said so", () => {
    expect(marksDone(designWork(ledger, ["landed-one", "landed-two"]), "accepted")).toBe(true);
    expect(marksDone(designWork(ledger, ["landed-one", "live-one"]), "accepted")).toBe(false);
    expect(marksDone(designWork(ledger, ["landed-one"]), "done")).toBe(false);
  });

  it("puts the count on the design's row, and nothing on a standing or a done one", () => {
    const designs = briefingFor(ledger, null).designs;
    expect(designs.open.map((row) => [row.title, row.note])).toEqual([
      ["Part of the way", "2 of 3 goals done"],
      ["All of it in", "all 2 goals done"],
      ["A standing design", ""],
    ]);
    expect(designs.runs.map((run) => run.rows.map((row) => [row.title, row.note]))).toEqual([
      [["Already done", ""]],
    ]);
  });

  it("says nothing on a kind that names no work of its own", () => {
    expect(designNote(ledger, record({ kind: "decision", id: "d", path: "docs/decisions/1.md", goals: ["live-one"] }))).toBe(
      "",
    );
  });
});

describe("what a record is about", () => {
  it("names each goal as a link to its own page", () => {
    expect(aboutOf(pane, ["interface-shell", "goal-ledger"])).toEqual([
      { id: "interface-shell", name: "interface-shell", to: "/backlog/goal/interface-shell", known: true },
      { id: "goal-ledger", name: "goal-ledger", to: "/backlog/goal/goal-ledger", known: true },
    ]);
  });

  it("still names a goal the ledger does not carry, and says it is not known", () => {
    expect(aboutOf(pane, ["nowhere"])).toEqual([
      { id: "nowhere", name: "nowhere", to: "/backlog/goal/nowhere", known: false },
    ]);
  });

  it("says nothing for a record about the project as a whole", () => {
    expect(aboutOf(pane, [])).toEqual([]);
  });
});

describe("the trail above a document", () => {
  it("names the section, the kind and the title of a record", () => {
    const document = documentOf("plans/designs/pane.md", {
      kind: "design", id: "design-pane", goals: ["interface-shell"],
    });

    expect(crumbsFor(pane, document)).toEqual([
      { label: "Project", to: "/project" },
      { label: "Designs", to: null },
      { label: "plans/designs/pane.md", to: null },
    ]);
  });

  it("reads the same for a record about the project as a whole", () => {
    const document = documentOf("metasystem/docs/decisions/0001.md", {
      kind: "decision", id: "decision-one", goals: [],
    });

    expect(crumbsFor(pane, document)[1]).toEqual({ label: "Decisions", to: null });
  });

  it("names the book a bound chapter belongs to", () => {
    const document = documentOf("metasystem/docs/paper/01-the-shift.md", null);

    expect(crumbsFor(pane, document)).toEqual([
      { label: "Project", to: "/project" },
      { label: "Intent", to: "/project/doc/metasystem/docs/intent/index.md" },
      { label: "metasystem/docs/paper/01-the-shift.md", to: null },
    ]);
  });

  it("says Documents for a file that declares nothing and no book names", () => {
    expect(crumbsFor(pane, documentOf("README.md", null))[1]).toEqual({ label: "Documents", to: null });
  });
});

describe("the left rail", () => {
  it("reads a chapter among the book's chapters, in reading order", () => {
    const rail = railFor(pane, documentOf("metasystem/docs/paper/01-the-shift.md", null));

    expect(rail?.title).toBe("Intent");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual([
      "The project's intent",
      "1. The Shift",
      "2. The seat",
    ]);
    expect(rail?.siblings.filter((sibling) => sibling.current).map((sibling) => sibling.title)).toEqual([
      "1. The Shift",
    ]);
    expect(rail?.previous?.title).toBe("The project's intent");
    expect(rail?.next?.title).toBe("2. The seat");
  });

  it("reads a record among the records of its own kind, whatever it is about", () => {
    const rail = railFor(pane, documentOf("plans/designs/pane.md", {
      kind: "design", id: "design-pane", goals: ["interface-shell"],
    }));

    expect(rail?.title).toBe("Designs");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual([
      "The Project pane",
      "The briefing",
      "The ledger",
      "The old thread",
    ]);
    expect(rail?.previous).toBeNull();
    expect(rail?.next?.title).toBe("The briefing");
  });

  it("reads the one record of its kind among itself", () => {
    const rail = railFor(pane, documentOf("metasystem/docs/decisions/0001.md", {
      kind: "decision", id: "decision-one", goals: ["goal-ledger"],
    }));

    // The only decision in the checkout. The rail still says what this is read
    // among, marks it, and offers nowhere to step: a record with no siblings
    // keeps its region, or the reading would slide into the rail's column.
    expect(rail?.title).toBe("Decisions");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual(["One binary"]);
    expect(rail?.siblings.map((sibling) => sibling.current)).toEqual([true]);
    expect(rail?.previous).toBeNull();
    expect(rail?.next).toBeNull();
  });

  it("gives a document that declares nothing no rail", () => {
    expect(railFor(pane, documentOf("README.md", null))).toBeNull();
  });
});

describe("an id in the reader", () => {
  it("keeps its ends, which is what tells two of them apart", () => {
    expect(shortID("01M348YTJ2688CC90GYDBYGJB5")).toBe("01M348…GJB5");
  });

  it("leaves an id short enough to read alone", () => {
    expect(shortID("Q-1")).toBe("Q-1");
  });
});

describe("the documents", () => {
  it("group by the first two segments of a path, and never by more", () => {
    expect(documentGroups(pane.documents)).toEqual([
      { id: "", title: ROOT_GROUP, files: [{ path: "README.md", title: "The repository" }] },
      {
        id: "development",
        title: "development",
        files: [{ path: "development/local.md", title: "Local rules" }],
      },
      {
        id: "metasystem/docs",
        title: "metasystem/docs",
        files: [
          { path: "metasystem/docs/architecture.md", title: "The engine" },
          { path: "metasystem/docs/design/principles.md", title: "Design principles" },
        ],
      },
      {
        id: "metasystem/plans",
        title: "metasystem/plans",
        files: [{ path: "metasystem/plans/g1-s1.md", title: "A historical design" }],
      },
    ]);
  });
});

describe("a goal's slice plan", () => {
  // Two records that already exist, and no third: the goal's own slicing
  // boundary, and the list each governing design wrote. There is no
  // slice-plan owner in the engine, which is why this is a read.
  it("is the goal's boundary and the slices each governing design lists", () => {
    const plan = slicePlan(pane, "interface-shell");

    expect(plan.started).toEqual({ at: "2026-09-18T08:00:00Z", machine: "m1e", lineage: "coordinator" });
    expect(plan.designs.map((design) => design.title)).toEqual(["The Project pane", "The ledger"]);
    expect(plan.designs[0].slices).toEqual(["The payload carries the boundary", "The tab reads it"]);
    expect(plan.designs[0].to).toBe("/project/doc/plans/designs/pane.md");
    expect(plan.designs[0].status).toBe("accepted");
    expect(sliceCount(plan)).toBe(3);
  });

  // A design that governs the goal and wrote no Slices section is not part of
  // the plan: there is nothing recorded there to read, and listing it as an
  // empty plan would say the design chose to record none.
  it("leaves out a governing design that recorded no slices", () => {
    expect(slicePlan(pane, "interface-shell").designs.map((design) => design.title)).not.toContain("The briefing");
  });

  it("is empty, and started nowhere, for a goal nobody sliced or planned", () => {
    const plan = slicePlan(pane, "goal-ledger");

    expect(plan.started).toBeNull();
    // design-ledger names this goal too, and lists a slice under it.
    expect(plan.designs.map((design) => design.title)).toEqual(["The ledger"]);
    expect(slicePlan(pane, "old-idea")).toEqual({ started: null, designs: [] });
    expect(sliceCount(slicePlan(pane, "old-idea"))).toBe(0);
  });

  // The line the tab shows when nothing is recorded names the gap and where
  // its owner comes from, rather than implying a plan that is merely empty.
  it("names the missing owner in one line", () => {
    expect(NO_SLICE_PLAN).toBe("No slice plan is recorded; the slice-plan owner arrives with gate 5 (master)");
  });

  // The whole project has no one plan: a tab showing every design's list at
  // once would be a list of lists.
  it("belongs to a goal page and not to the project", () => {
    expect(briefingFor(pane, null).slices).toBeNull();
    expect(briefingFor(pane, "interface-shell").slices).not.toBeNull();
  });

  // One derivation answers the card and the tab, so the two cannot say
  // different things about the same goal.
  it("is built once for a whole board of cards", () => {
    const plans = slicePlans(pane, ["interface-shell", "goal-ledger", "old-idea"]);

    expect([...plans.keys()]).toEqual(["interface-shell", "goal-ledger", "old-idea"]);
    expect(plans.get("interface-shell")).toEqual(slicePlan(pane, "interface-shell"));
    expect(plans.get("old-idea")).toEqual({ started: null, designs: [] });
  });

  // Whichever parts are known, and nothing when neither is. No state per
  // slice and none in total: the ledger records that slicing began and
  // nothing finer, and reading one out of prose is a reconstruction.
  it("says on a card what is planned and when slicing began, and nothing else", () => {
    expect(sliceLine(3, "18/09/2026")).toBe("3 slices planned · slicing started 18/09/2026");
    expect(sliceLine(1, "18/09/2026")).toBe("1 slice planned · slicing started 18/09/2026");
    expect(sliceLine(3, "")).toBe("3 slices planned");
    expect(sliceLine(0, "18/09/2026")).toBe("slicing started 18/09/2026");
    expect(sliceLine(0, "")).toBe("");
  });

  it("flattens a plan into slices that each name the design listing them", () => {
    expect(slicesOf(slicePlan(pane, "interface-shell"))).toEqual([
      {
        key: "plans/designs/pane.md-0",
        text: "The payload carries the boundary",
        title: "The Project pane",
        to: "/project/doc/plans/designs/pane.md",
      },
      {
        key: "plans/designs/pane.md-1",
        text: "The tab reads it",
        title: "The Project pane",
        to: "/project/doc/plans/designs/pane.md",
      },
      {
        key: "metasystem/plans/designs/ledger.md-0",
        text: "The accepted ref is fetched on a loop",
        title: "The ledger",
        to: "/project/doc/metasystem/plans/designs/ledger.md",
      },
    ]);
    expect(slicesOf(slicePlan(pane, "old-idea"))).toEqual([]);
  });
});


/* ------------------------------------------------- scope is a filter -- */

/**
 * The same project with records on both sides of the line: three decisions,
 * four designs and three open questions, some naming goals and some naming
 * none, so every scope has something in it and no count is the same number
 * twice.
 *
 * It is a fixture of its own rather than more rows on the one above, because
 * the one above pins what every other reading of this module answers and a
 * row added to it would move a dozen counts that have nothing to do with
 * scope.
 */
const scoped: Pane = {
  ...pane,
  records: [
    record({
      kind: "decision", id: "decision-wide", title: "One binary",
      path: "metasystem/docs/decisions/0001.md", home: "metasystem/docs/decisions",
      summary: "The engine ships as one Go binary.",
    }),
    record({
      kind: "decision", id: "decision-also-wide", title: "Loopback only",
      path: "metasystem/docs/decisions/0002.md", home: "metasystem/docs/decisions",
    }),
    record({
      kind: "decision", id: "decision-under", goals: ["goal-ledger"], title: "The accepted ref",
      path: "metasystem/docs/decisions/0003.md", home: "metasystem/docs/decisions",
    }),
    record({
      kind: "design", id: "design-wide", title: "The application shell",
      path: "plans/designs/shell.md",
    }),
    record({
      kind: "design", id: "design-pane", goals: ["interface-shell"], title: "The Project pane",
      path: "plans/designs/pane.md",
    }),
    record({
      kind: "design", id: "design-both", goals: ["goal-ledger", "interface-shell"],
      title: "The ledger", path: "plans/designs/ledger.md",
    }),
    record({
      kind: "design", id: "design-unknown", goals: ["no-such-goal"], title: "A design naming a goal nobody planted",
      path: "plans/designs/nowhere.md",
    }),
  ],
  questions: [
    { id: "Q-1", opened: "2026-09-22", question: "Where does intent live?", goals: [], status: "open" },
    { id: "Q-2", opened: "2026-09-22", question: "Who accepts?", goals: ["goal-ledger"], status: "open" },
    { id: "Q-3", opened: "2026-09-22", question: "Is seat the word?", goals: ["interface-shell"], status: "open" },
  ],
};

describe("the scope a page is showing", () => {
  it("offers the three in the order the control does, and defaults to the project's own", () => {
    expect(SCOPES.map((one) => one.id)).toEqual(["project", "goals", "all"]);
    expect(SCOPES.map((one) => one.title)).toEqual(["Project", "Goals", "All"]);
    expect(DEFAULT_SCOPE).toBe("project");
  });

  // A browser holding a word from a build that offered a fourth scope has no
  // preference this build can honour, and gets the default rather than a page
  // that shows nothing.
  it("reads a remembered word, and answers a word it does not know with the default", () => {
    expect(scopeOf("goals")).toBe("goals");
    expect(scopeOf("all")).toBe("all");
    expect(scopeOf("everything")).toBe(DEFAULT_SCOPE);
    expect(scopeOf(null)).toBe(DEFAULT_SCOPE);
  });

  it("opens on the records whose head names no goal", () => {
    const briefing = briefingFor(scoped, null, "project");
    expect(titles(briefing.decisions)).toEqual(["One binary", "Loopback only"]);
    expect(titles(briefing.designs.open)).toEqual(["The application shell"]);
    expect(titles(briefing.questions)).toEqual(["Where does intent live?"]);
  });

  it("shows the goal-scoped ones under Goals, and both under All", () => {
    expect(titles(briefingFor(scoped, null, "goals").decisions)).toEqual(["The accepted ref"]);
    expect(titles(briefingFor(scoped, null, "all").decisions)).toEqual([
      "One binary",
      "Loopback only",
      "The accepted ref",
    ]);
  });

  // The counts are of records and not of namings: a design about two goals is
  // one design under goals, and it stands under each of the two only where
  // the grouping puts it.
  it("counts each kind both ways, over the whole project, whatever is being shown", () => {
    for (const showing of SCOPES) {
      const briefing = briefingFor(scoped, null, showing.id);
      expect({ showing: showing.id, ...briefing.scopes.decisions }).toEqual({
        showing: showing.id, own: 2, underGoals: 1,
      });
      expect({ showing: showing.id, ...briefing.scopes.designs }).toEqual({
        showing: showing.id, own: 1, underGoals: 3,
      });
      expect({ showing: showing.id, ...briefing.scopes.questions }).toEqual({
        showing: showing.id, own: 1, underGoals: 2,
      });
    }
  });

  // The head of a tab is one sentence and not two numbers with a gap between
  // them: twelve of these, and forty-three more that this view is not showing.
  it("says what it is showing and what it is leaving out, in one line", () => {
    expect(scopeNote("project", { own: 12, underGoals: 43 })).toBe("43 under goals");
    expect(countLine("Designs", 12, scopeNote("project", { own: 12, underGoals: 43 }))).toBe(
      "Designs 12 · 43 under goals",
    );
    expect(countText(12, "43 under goals")).toBe("12 · 43 under goals");
  });

  // All leaves nothing out, and neither does a scope whose other side is
  // empty: "0 under goals" is furniture rather than a fact.
  it("says nothing where there is nothing left out", () => {
    expect(scopeNote("all", { own: 12, underGoals: 43 })).toBe("");
    expect(scopeNote("project", { own: 12, underGoals: 0 })).toBe("");
    expect(scopeNote("goals", { own: 0, underGoals: 43 })).toBe("");
    expect(scopeNote("goals", { own: 12, underGoals: 43 })).toBe("12 project-wide");
    expect(countLine("Designs", 12, "")).toBe("Designs 12");
  });

  // Narrowing the view does not answer a question: what is waiting is what is
  // waiting in this project, and the control is about what is being read.
  it("counts what needs a human over everything, not over the scope shown", () => {
    for (const showing of SCOPES) {
      expect({ showing: showing.id, ...briefingFor(scoped, null, showing.id).needsYou }).toEqual({
        showing: showing.id, questions: 3, designs: 4,
      });
    }
  });

  // A goal page has no scope: it is one goal's records by definition, and the
  // records that name no goal are not among the records that name this one.
  it("narrows the project's page and leaves a goal page alone", () => {
    for (const showing of SCOPES) {
      expect(titles(briefingFor(scoped, "goal-ledger", showing.id).decisions)).toEqual(["The accepted ref"]);
    }
  });
});

describe("the goal-scoped records, grouped under their goals", () => {
  const groups = goalGroups(scoped, briefingFor(scoped, null, "goals").across.designs);

  // The ledger's order, which is every other listing of goals in this
  // interface; a goal the ledger does not carry keeps its place at the end
  // under the id it is named by, rather than being dropped with the records
  // that name it.
  it("is in the ledger's own order, with what the ledger does not carry last", () => {
    expect(groups.map((group) => group.id)).toEqual(["goal-ledger", "interface-shell", "no-such-goal"]);
    expect(groups.map((group) => group.to)).toEqual([
      "/backlog/goal/goal-ledger",
      "/backlog/goal/interface-shell",
      "/backlog/goal/no-such-goal",
    ]);
  });

  // A record about two goals is about each of them: filing it under the first
  // would hide it from the human who came to the second.
  it("puts a record under each goal it names", () => {
    expect(titles(groups[0].rows)).toEqual(["The ledger"]);
    expect(titles(groups[1].rows)).toEqual(["The Project pane", "The ledger"]);
  });

  it("carries no group for a goal nothing on this tab is about", () => {
    const decisions = goalGroups(scoped, briefingFor(scoped, null, "goals").across.decisions);
    expect(decisions.map((group) => group.id)).toEqual(["goal-ledger"]);
    expect(goalGroups(scoped, [])).toEqual([]);
  });
});

describe("Find, which crosses every scope", () => {
  // The rows Find searches are the whole tab's and not the view's: a human who
  // remembers a design and not which goal it named should not have to guess
  // the scope before they can look for it.
  const everyDesign = briefingFor(scoped, null, "project").across.designs;

  it("reaches rows the control is not showing", () => {
    expect(titles(found(everyDesign, "ledger"))).toEqual(["The ledger"]);
    expect(titles(found(everyDesign, "pane"))).toEqual(["The Project pane"]);
  });

  it("looks in the title, the first words, the path and the goals a row names", () => {
    expect(titles(found(everyDesign, "interface-shell"))).toEqual(["The Project pane", "The ledger"]);
    expect(titles(found(everyDesign, "plans/designs/shell.md"))).toEqual(["The application shell"]);
  });

  // Every word has to be somewhere, in any order and in any of them, and case
  // is nothing: a human typing either is typing what they remember.
  it("wants every word, in any order, in any case", () => {
    expect(titles(found(everyDesign, "LEDGER the"))).toEqual(["The ledger"]);
    expect(titles(found(everyDesign, "ledger pane"))).toEqual([]);
  });

  it("is not a search at all when nothing was typed", () => {
    expect(found(everyDesign, "").length).toBe(everyDesign.length);
    expect(found(everyDesign, "   ").length).toBe(everyDesign.length);
  });

  it("says what found nothing, in the words that were typed", () => {
    expect(noMatchLine("  wobble ")).toBe("Nothing on this tab matches “wobble”, in any scope.");
  });
});

describe("the line a goal page's tab ends with", () => {
  it("counts the project's own records of that tab's kind, and leads to them", () => {
    expect(projectWideLine("decisions", 12)).toBe("12 project-wide decisions →");
    expect(projectWideLine("designs", 1)).toBe("1 project-wide design →");
    expect(projectWideLine("questions", 4)).toBe("4 project-wide open questions →");
  });

  // Nothing to say is said with nothing, and a tab with no such line is a tab
  // whose kind has none: neither is a line saying "0".
  it("says nothing where there is none, and nothing on a tab that has no such kind", () => {
    expect(projectWideLine("designs", 0)).toBe("");
    expect(projectWideLine("documents", 12)).toBe("");
    expect(projectWideLine("intent", 12)).toBe("");
  });

  it("is built from the counts the briefing carries on the goal page itself", () => {
    const briefing = briefingFor(scoped, "goal-ledger", "project");
    expect(projectWideLine("decisions", briefing.scopes.decisions.own)).toBe("2 project-wide decisions →");
  });
});

describe("what a record says it is about, on its own page", () => {
  it("is the project as a whole where the head names no goal", () => {
    expect(aboutRow(scoped, [])).toEqual({ project: true, goals: [], named: false });
    expect(ABOUT_PROJECT).toBe("this project");
  });

  // One goal gets its title beside its id, because there is room for it and
  // because an id alone does not say what the record is about. Several do not:
  // three ids and three titles is a paragraph in a facts row.
  it("names one goal with its title, and several by their ids alone", () => {
    const one = aboutRow(scoped, ["goal-ledger"]);
    expect({ project: one.project, named: one.named, ids: one.goals.map((goal) => goal.id) }).toEqual({
      project: false, named: true, ids: ["goal-ledger"],
    });
    const two = aboutRow(scoped, ["goal-ledger", "interface-shell"]);
    expect({ project: two.project, named: two.named, ids: two.goals.map((goal) => goal.id) }).toEqual({
      project: false, named: false, ids: ["goal-ledger", "interface-shell"],
    });
  });

  // A chapter of either book is about the project as a whole by definition,
  // and the grammar refuses a Goals line on one: an Edit that could only ever
  // be refused is a control that lies.
  it("is editable on every kind but the two books", () => {
    expect(scopeEditable("decision")).toBe(true);
    expect(scopeEditable("design")).toBe(true);
    expect(scopeEditable("intent")).toBe(false);
    expect(scopeEditable("doctrine")).toBe(false);
    expect(scopeEditable("")).toBe(false);
  });
});
