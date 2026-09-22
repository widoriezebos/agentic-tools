import { describe, expect, it } from "vitest";

import type { DocumentPayload, Pane, ProjectRecord } from "./api";
import {
  aboutOf,
  briefingFor,
  crumbsFor,
  documentGroups,
  pageSections,
  railFor,
  ROOT_GROUP,
  shortID,
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
  schemaVersion: 4,
  readAt: "2026-09-22T10:11:12Z",
  // The order the server answers in: the live goals first, each in id order,
  // then the concluded ones.
  goals: [
    { id: "goal-ledger", title: "goal-ledger", state: "queued",
      intent: "The ledger is the one source of open work" },
    { id: "interface-shell", title: "interface-shell", state: "claimed",
      intent: "The browser is the seat a human takes" },
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
    }),
    record({
      kind: "design", id: "design-briefing", status: "draft", goals: ["interface-shell"],
      title: "The briefing", path: "plans/designs/briefing.md",
    }),
    record({
      kind: "design", id: "design-ledger", status: "done", goals: ["goal-ledger", "interface-shell"],
      title: "The ledger", path: "metasystem/plans/designs/ledger.md", home: "metasystem/plans/designs",
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
    kind: "document", id, title: id, revision: "", owner: "app-owned", path: `/work/${id}`,
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
      { id: "intent", title: "Intent" },
      { id: "doctrine", title: "Doctrine" },
      { id: "decisions", title: "Decisions" },
      { id: "designs", title: "Designs" },
      { id: "questions", title: "Open questions" },
      { id: "documents", title: "Documents" },
    ]);
  });

  // A goal page opens with the goal itself and carries neither book, and the
  // checkout's other Markdown is the project's rather than any goal's. The
  // strip says exactly what that page has, or it would offer anchors to
  // sections that are not on it.
  it("names the goal first on a goal page, and no section that page does not render", () => {
    expect(pageSections(briefingFor(pane, "interface-shell"))).toEqual([
      { id: "goal", title: "interface-shell" },
      { id: "decisions", title: "Decisions" },
      { id: "designs", title: "Designs" },
      { id: "questions", title: "Open questions" },
    ]);
  });

  // Every tab is an anchor on the page, never an address: the ledger's goals
  // are the Backlog's, and this strip navigates nowhere.
  it("carries no route at all, on either page", () => {
    for (const goal of [null, "interface-shell"]) {
      for (const section of pageSections(briefingFor(pane, goal))) {
        expect({ id: section.id, slash: section.id.includes("/") }).toEqual({ id: section.id, slash: false });
      }
    }
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
