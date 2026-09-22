import { describe, expect, it } from "vitest";

import type { Pane } from "./api";
import { areaRows, documentGroups, ROOT_GROUP, sectionsFor } from "./pane";

/**
 * What the pane shows, from one payload.
 *
 * The fixture is the shape the server answers: two declared areas, a book
 * whose reading order names a record and binds a document, a decision, two
 * designs in two homes, two questions, one refusal, and a few documents that
 * declare nothing.
 */

const pane: Pane = {
  schemaVersion: 2,
  readAt: "2026-09-22T10:11:12Z",
  areas: [
    { slug: "interface", name: "The browser workspace" },
    { slug: "goals", name: "The goal ledger" },
  ],
  records: [
    {
      kind: "intent", id: "intent-index", status: "accepted", areas: ["project"],
      title: "The project's intent", path: "metasystem/docs/intent/index.md", home: "metasystem/docs/intent",
    },
    {
      kind: "doctrine", id: "doctrine-index", status: "accepted", areas: ["project"],
      title: "The doctrine", path: "metasystem/docs/doctrine/index.md", home: "metasystem/docs/doctrine",
    },
    {
      kind: "doctrine", id: "doctrine-events", status: "accepted", areas: ["goals"],
      title: "Events are the source of truth", path: "metasystem/docs/doctrine/events.md",
      home: "metasystem/docs/doctrine",
    },
    {
      kind: "doctrine", id: "doctrine-loose", status: "draft", areas: ["interface"],
      title: "A chapter the index does not name", path: "metasystem/docs/doctrine/loose.md",
      home: "metasystem/docs/doctrine",
    },
    {
      kind: "decision", id: "decision-one", status: "accepted", areas: ["goals"],
      title: "One binary", path: "metasystem/docs/decisions/0001.md", home: "metasystem/docs/decisions",
    },
    {
      kind: "design", id: "design-pane", status: "accepted", areas: ["interface"],
      title: "The Project pane", path: "plans/designs/pane.md", home: "plans/designs",
    },
    {
      kind: "design", id: "design-ledger", status: "done", areas: ["goals", "interface"],
      title: "The ledger", path: "metasystem/plans/designs/ledger.md", home: "metasystem/plans/designs",
    },
  ],
  intent: {
    index: {
      kind: "intent", id: "intent-index", status: "accepted", areas: ["project"],
      title: "The project's intent", path: "metasystem/docs/intent/index.md", home: "metasystem/docs/intent",
    },
    chapters: [{ path: "metasystem/docs/paper/01-the-shift.md", title: "1. The Shift" }],
  },
  doctrine: {
    index: {
      kind: "doctrine", id: "doctrine-index", status: "accepted", areas: ["project"],
      title: "The doctrine", path: "metasystem/docs/doctrine/index.md", home: "metasystem/docs/doctrine",
    },
    chapters: [
      { id: "doctrine-events", title: "Events are the source of truth" },
      { id: "doctrine-absent", title: "A chapter no record declares" },
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

function titles(rows: { title: string }[]): string[] {
  return rows.map((row) => row.title);
}

function section(slug: string | null, id: string) {
  const found = sectionsFor(pane, slug).find((candidate) => candidate.id === id);
  if (found === undefined) {
    throw new Error(`no section ${id}`);
  }
  return found;
}

describe("the area tree", () => {
  it("puts the whole project first and counts what each area names", () => {
    expect(areaRows(pane)).toEqual([
      { slug: null, name: "Project", to: "/project", count: 7 },
      { slug: "interface", name: "The browser workspace", to: "/project/area/interface", count: 3 },
      { slug: "goals", name: "The goal ledger", to: "/project/area/goals", count: 3 },
    ]);
  });
});

describe("the sections", () => {
  it("are the five the design names, in its order", () => {
    expect(sectionsFor(pane, null).map((part) => part.id)).toEqual([
      "intent",
      "doctrine",
      "decisions",
      "designs",
      "questions",
    ]);
  });

  it("read a book as its index, its chapters in order, then what it does not name", () => {
    expect(titles(section(null, "doctrine").rows)).toEqual([
      "The doctrine",
      "Events are the source of truth",
      "A chapter no record declares",
      "A chapter the index does not name",
    ]);
  });

  it("open a record chapter at its record and a bound chapter at its document", () => {
    const rows = section(null, "doctrine").rows;
    expect(rows[1].to).toBe("/project/doc/metasystem/docs/doctrine/events.md");
    expect(rows[1].status).toBe("accepted");
    expect(rows[1].areas).toEqual(["goals"]);
    // A chapter no record declares is shown as the index wrote it, and opens
    // nothing: the check verb refuses it in the same breath.
    expect(rows[2].to).toBeNull();
    expect(rows[2].path).toBe("");
    expect(section(null, "intent").rows[1]).toEqual({
      key: "chapter-0",
      title: "1. The Shift",
      to: "/project/doc/metasystem/docs/paper/01-the-shift.md",
      status: "",
      areas: [],
      path: "metasystem/docs/paper/01-the-shift.md",
    });
  });

  it("list the records of a kind, in the order the payload carried them", () => {
    expect(titles(section(null, "designs").rows)).toEqual(["The Project pane", "The ledger"]);
    expect(titles(section(null, "decisions").rows)).toEqual(["One binary"]);
    expect(titles(section(null, "questions").rows)).toEqual(["Where does intent live?", "Who accepts?"]);
  });

  it("narrow to the records that name the selected area", () => {
    expect(titles(section("interface", "designs").rows)).toEqual(["The Project pane", "The ledger"]);
    expect(titles(section("goals", "designs").rows)).toEqual(["The ledger"]);
    expect(titles(section("interface", "questions").rows)).toEqual(["Where does intent live?"]);
    expect(titles(section("goals", "decisions").rows)).toEqual(["One binary"]);
    expect(section("interface", "decisions").rows).toEqual([]);
  });

  // A book belongs to the areas its index names. Under any other area only the
  // chapters that are records of that area are shown, and the bound documents,
  // which name no area of their own, stay with the index.
  it("narrow a book to the chapters that name the area", () => {
    expect(titles(section("goals", "doctrine").rows)).toEqual(["Events are the source of truth"]);
    expect(titles(section("interface", "doctrine").rows)).toEqual(["A chapter the index does not name"]);
    expect(section("interface", "intent").rows).toEqual([]);
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

  it("belong to no area, so the list does not change with the selection", () => {
    expect(documentGroups(pane.documents).length).toBe(4);
    expect(sectionsFor(pane, "interface").map((part) => part.id)).not.toContain("documents");
  });
});
