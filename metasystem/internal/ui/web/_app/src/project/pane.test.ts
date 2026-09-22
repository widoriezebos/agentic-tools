import { describe, expect, it } from "vitest";

import type { DocumentPayload, Pane, ProjectRecord } from "./api";
import {
  areaRows,
  briefingFor,
  crumbsFor,
  documentGroups,
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
 * The fixture is the shape the server answers: two declared areas, two books
 * whose reading order names records and binds documents, a decision, four
 * designs across the statuses, two questions, one refusal, and a few documents
 * that declare nothing.
 */

function record(fields: Partial<ProjectRecord> & Pick<ProjectRecord, "kind" | "id" | "path">): ProjectRecord {
  return {
    status: "accepted",
    areas: [],
    title: fields.id,
    home: "plans/designs",
    summary: "",
    ...fields,
  };
}

const intentIndex = record({
  kind: "intent", id: "intent-index", areas: ["project"], title: "The project's intent",
  path: "metasystem/docs/intent/index.md", home: "metasystem/docs/intent",
  summary: "Engineering after the shift: governing production, not building it.",
});

const doctrineIndex = record({
  kind: "doctrine", id: "doctrine-index", areas: ["project"], title: "The doctrine",
  path: "metasystem/docs/doctrine/index.md", home: "metasystem/docs/doctrine",
  summary: "A machine for letting agents build software unattended.",
});

const pane: Pane = {
  schemaVersion: 3,
  readAt: "2026-09-22T10:11:12Z",
  areas: [
    { slug: "interface", name: "The browser workspace" },
    { slug: "goals", name: "The goal ledger" },
  ],
  records: [
    intentIndex,
    record({
      kind: "intent", id: "intent-interface", areas: ["interface"], title: "What the interface is for",
      path: "metasystem/docs/intent/interface.md", home: "metasystem/docs/intent",
      summary: "The browser is the seat a human takes.",
    }),
    doctrineIndex,
    record({
      kind: "doctrine", id: "doctrine-events", areas: ["goals"], title: "Events are the source of truth",
      path: "metasystem/docs/doctrine/events.md", home: "metasystem/docs/doctrine",
    }),
    record({
      kind: "doctrine", id: "doctrine-loose", status: "draft", areas: ["interface"],
      title: "A chapter the index does not name", path: "metasystem/docs/doctrine/loose.md",
      home: "metasystem/docs/doctrine",
    }),
    record({
      kind: "decision", id: "decision-one", areas: ["goals"], title: "One binary",
      path: "metasystem/docs/decisions/0001.md", home: "metasystem/docs/decisions",
      summary: "The engine ships as one Go binary.",
    }),
    record({
      kind: "design", id: "design-pane", areas: ["interface"], title: "The Project pane",
      path: "plans/designs/pane.md", summary: "The pane over the resolver.",
    }),
    record({
      kind: "design", id: "design-briefing", status: "draft", areas: ["interface"],
      title: "The briefing", path: "plans/designs/briefing.md",
    }),
    record({
      kind: "design", id: "design-ledger", status: "done", areas: ["goals", "interface"],
      title: "The ledger", path: "metasystem/plans/designs/ledger.md", home: "metasystem/plans/designs",
    }),
    record({
      kind: "design", id: "design-old", status: "superseded", areas: ["interface"],
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
    { id: "Q-1", opened: "2026-09-22", question: "Where does intent live?", areas: ["interface"], status: "open" },
    { id: "Q-2", opened: "2026-09-22", question: "Who accepts?", areas: ["goals"], status: "answered: decision-one" },
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

/** One of the two books, scoped to an area. */
function scoped(slug: string, id: string) {
  const found = briefingFor(pane, slug).areaBooks.find((candidate) => candidate.id === id);
  if (found === undefined) {
    throw new Error(`no book ${id} under ${slug}`);
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
            kind: "design", id: "", status: "accepted", areas: [], cites: [], affects: [],
            governs: [], supersedes: [], by: [], ...head,
          },
    referencedBy: [],
    supersededBy: [],
    headings: [],
    blocks: [],
  };
}

describe("the area tree", () => {
  it("puts the whole project first and counts what each area names", () => {
    expect(areaRows(pane)).toEqual([
      { slug: null, name: "Project", to: "/project", count: 10 },
      { slug: "interface", name: "The browser workspace", to: "/project/area/interface", count: 6 },
      { slug: "goals", name: "The goal ledger", to: "/project/area/goals", count: 3 },
    ]);
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

  it("carries each row's own first words beside its title", () => {
    expect(briefingFor(pane, null).decisions).toEqual([
      {
        key: "metasystem/docs/decisions/0001.md",
        title: "One binary",
        to: "/project/doc/metasystem/docs/decisions/0001.md",
        summary: "The engine ships as one Go binary.",
        status: "accepted",
        areas: ["goals"],
        path: "metasystem/docs/decisions/0001.md",
      },
    ]);
  });

  it("counts what is waiting and what was read", () => {
    const briefing = briefingFor(pane, null);

    expect(briefing.needsYou).toEqual({ questions: 1, designs: 1 });
    expect(briefing.checkout).toEqual({ records: 10, homes: 5, areas: 2, problems: 1 });
  });
});

describe("an area page", () => {
  it("opens by naming the area and counting what names it", () => {
    expect(briefingFor(pane, "interface").area).toEqual({
      slug: "interface",
      name: "The browser workspace",
      declared: true,
      count: 6,
    });
    expect(briefingFor(pane, "nowhere").area?.declared).toBe(false);
  });

  it("does not repeat a project-wide book, and scopes each to the area", () => {
    expect(briefingFor(pane, "interface").books).toEqual([]);
    expect(titles(scoped("interface", "intent").rows)).toEqual(["2. The seat"]);
    expect(titles(scoped("interface", "doctrine").rows)).toEqual(["A chapter the index does not name"]);
    expect(titles(scoped("goals", "doctrine").rows)).toEqual(["Events are the source of truth"]);
  });

  it("says in one line when the area has no chapter, rather than Nothing recorded", () => {
    const intent = scoped("goals", "intent");

    expect(intent.rows).toEqual([]);
    expect(intent.none).toBe("No intent chapter for The goal ledger yet; the project-wide intent applies.");
  });

  it("leaves the area chip off every row, because every row is in this area", () => {
    const briefing = briefingFor(pane, "interface");

    for (const row of [...briefing.decisions, ...briefing.designs.open, ...briefing.questions]) {
      expect({ title: row.title, areas: row.areas }).toEqual({ title: row.title, areas: [] });
    }
    expect(briefingFor(pane, null).designs.open[0].areas).toEqual(["interface"]);
  });

  it("narrows every section to the records that name the area", () => {
    expect(titles(briefingFor(pane, "goals").decisions)).toEqual(["One binary"]);
    expect(briefingFor(pane, "interface").decisions).toEqual([]);
    expect(titles(briefingFor(pane, "interface").questions)).toEqual(["Where does intent live?"]);
    expect(titles(briefingFor(pane, "interface").designs.open)).toEqual(["The Project pane", "The briefing"]);
  });
});

describe("the trail above a document", () => {
  it("names the area, the kind and the title of a record", () => {
    const document = documentOf("plans/designs/pane.md", { kind: "design", id: "design-pane", areas: ["interface"] });

    expect(crumbsFor(pane, document)).toEqual([
      { label: "Project", to: "/project" },
      { label: "The browser workspace", to: "/project/area/interface" },
      { label: "Designs", to: null },
      { label: "plans/designs/pane.md", to: null },
    ]);
  });

  it("says Everything for a record that names no declared area", () => {
    const document = documentOf("metasystem/docs/decisions/0001.md", {
      kind: "decision", id: "decision-one", areas: ["project"],
    });

    expect(crumbsFor(pane, document)[1]).toEqual({ label: "Everything", to: "/project" });
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

  it("reads a record among the records of its kind in its areas", () => {
    const rail = railFor(pane, documentOf("plans/designs/pane.md", { kind: "design", id: "design-pane", areas: ["interface"] }));

    expect(rail?.title).toBe("Designs · The browser workspace");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual([
      "The Project pane",
      "The briefing",
      "The ledger",
      "The old thread",
    ]);
    expect(rail?.previous).toBeNull();
    expect(rail?.next?.title).toBe("The briefing");
  });

  it("reads a record that names no declared area among every record of its kind", () => {
    const rail = railFor(pane, documentOf("metasystem/docs/decisions/0001.md", {
      kind: "decision", id: "decision-one", areas: ["project"],
    }));

    // The only decision in the checkout. The rail still says what this is read
    // among, marks it, and offers nowhere to step: a record with no siblings
    // keeps its region, or the reading would slide into the rail's column.
    expect(rail?.title).toBe("Decisions · Everything");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual(["One binary"]);
    expect(rail?.siblings.map((sibling) => sibling.current)).toEqual([true]);
    expect(rail?.previous).toBeNull();
    expect(rail?.next).toBeNull();
  });

  it("reads the one record of its kind in its area among itself", () => {
    const rail = railFor(pane, documentOf("metasystem/docs/decisions/0001.md", {
      kind: "decision", id: "decision-one", areas: ["goals"],
    }));

    expect(rail?.title).toBe("Decisions · The goal ledger");
    expect(rail?.siblings.map((sibling) => sibling.title)).toEqual(["One binary"]);
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
