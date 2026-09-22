import { describe, expect, it } from "vitest";

import type { Heading } from "./api";
import { currentRow, DEEPEST_LEVEL, FEWEST_ROWS, outlineOf } from "./outline";

function heading(level: number, text: string): Heading {
  return { level, id: text.toLowerCase().replaceAll(" ", "-"), text };
}

describe("the contents of a document", () => {
  it("carries the headings to the third level, indented by depth", () => {
    const outline = outlineOf([
      heading(1, "Title"),
      heading(2, "First"),
      heading(3, "Detail"),
      heading(4, "Too deep"),
      heading(2, "Second"),
    ]);

    expect(outline.map((row) => row.text)).toEqual(["Title", "First", "Detail", "Second"]);
    expect(outline.map((row) => row.indent)).toEqual([0, 1, 2, 1]);
    expect(outline.map((row) => row.id)).toEqual(["title", "first", "detail", "second"]);
  });

  it("indents from the shallowest heading the document actually has", () => {
    const outline = outlineOf([heading(2, "One"), heading(3, "Two"), heading(2, "Three")]);

    expect(outline.map((row) => row.indent)).toEqual([0, 1, 0]);
  });

  it("gives a document with no shape no outline at all", () => {
    expect(outlineOf([])).toEqual([]);
    expect(outlineOf([heading(1, "Title")])).toEqual([]);
    expect(outlineOf([heading(1, "Title"), heading(2, "First")])).toEqual([]);
    expect(outlineOf([heading(4, "A"), heading(4, "B"), heading(4, "C")])).toEqual([]);
  });

  it("counts only the rows it would show", () => {
    const deep = [heading(1, "Title"), heading(2, "First"), heading(4, "Deep"), heading(5, "Deeper")];

    expect(outlineOf(deep)).toEqual([]);
    expect(DEEPEST_LEVEL).toBe(3);
    expect(FEWEST_ROWS).toBe(3);
  });

  it("leaves out a heading with no text to show", () => {
    const outline = outlineOf([heading(1, "Title"), { level: 2, id: "", text: "" }, heading(2, "First"), heading(2, "Second")]);

    expect(outline.map((row) => row.text)).toEqual(["Title", "First", "Second"]);
  });
});

describe("the row the reader is in", () => {
  const outline = outlineOf([heading(1, "Title"), heading(2, "First"), heading(2, "Second"), heading(2, "Third")]);

  it("is the first heading on screen, in the document's order", () => {
    // An observer reports whatever changed, in whatever order; the document's
    // order is what decides which of them the reader is in.
    expect(currentRow(outline, new Set(["third", "second"]), null)).toBe("second");
    expect(currentRow(outline, new Set(["title"]), null)).toBe("title");
  });

  it("keeps the mark inside a section longer than the window", () => {
    expect(currentRow(outline, new Set(), "second")).toBe("second");
  });

  it("marks nothing before anything has been seen", () => {
    expect(currentRow(outline, new Set(), null)).toBeNull();
    expect(currentRow([], new Set(["first"]), null)).toBeNull();
  });

  it("forgets a row that is not in this document's outline", () => {
    expect(currentRow(outline, new Set(), "a-heading-of-another-document")).toBeNull();
  });
});
