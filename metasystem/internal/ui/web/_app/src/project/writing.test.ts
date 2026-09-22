import { describe, expect, it } from "vitest";

import { fileFor, incomplete, noteFor, pageFor, slugOf, type Draft } from "./writing";

/**
 * What the sheet promises before it writes.
 *
 * The slug cases are the server's own, in
 * internal/ui/project/write_test.go: the two sides compose the same file name
 * from the same title, and a case that passed on one side and not the other
 * would be a sheet that named a file the server never wrote.
 */

function draft(over: Partial<Draft> = {}): Draft {
  return { kind: "decision", title: "One binary", goals: ["ledger-sync"], affects: [], cites: [], ...over };
}

describe("the file name a title yields", () => {
  it("is the server's own rule", () => {
    expect(slugOf("One binary")).toBe("one-binary");
    expect(slugOf("  The engine's *one* binary — really!  ")).toBe("the-engine-s-one-binary-really");
    expect(slugOf("g1-s22 Project, step 2")).toBe("g1-s22-project-step-2");
    expect(slugOf("··· !!! ···")).toBe("");
  });

  it("stops at sixty characters, and never on a hyphen", () => {
    const long = slugOf("The Project Partner is a drawer along the bottom of the page and more");
    expect(long).toBe("the-project-partner-is-a-drawer-along-the-bottom-of-the-page");
    expect(long.length).toBeLessThanOrEqual(60);
    expect(slugOf("a".repeat(59) + " b")).toBe("a".repeat(59));
  });
});

describe("the page as it will be written", () => {
  /*
   * Byte for byte, and the same page TestThePageIsWrittenByteForByte pins in
   * internal/ui/project/write_test.go, with the id line standing in for the
   * one the server mints. Two sides that drifted would be a sheet promising a
   * file nobody wrote.
   */
  it("is the head the grammar requires, the references, and the kind's own sections", () => {
    expect(
      pageFor(draft({ kind: "decision", title: "One binary", goals: ["ledger-sync"], cites: ["design-ledger"], affects: ["design-reading"] })),
    ).toBe(
      [
        "# One binary",
        "",
        "- Kind: decision",
        "- Id: (a fresh id, minted when this is written)",
        "- Status: draft",
        "- Goals: ledger-sync",
        "- Cites: design-ledger",
        "- Affects: design-reading",
        "",
        "## Context",
        "",
        "## Decision",
        "",
        "## Consequences",
        "",
      ].join("\n"),
    );
  });

  it("leaves a key out rather than declaring it empty", () => {
    expect(pageFor(draft())).not.toContain("Cites");
    expect(pageFor(draft())).not.toContain("Affects");
  });

  it("writes no Goals line at all for a record about the project as a whole", () => {
    expect(pageFor(draft({ goals: [] }))).not.toContain("Goals");
    expect(pageFor(draft({ goals: ["ledger-sync", "reading-pane"] }))).toContain(
      "- Goals: ledger-sync reading-pane\n",
    );
  });

  it("gives each kind its own empty sections", () => {
    expect(pageFor(draft({ kind: "design" }))).toContain("## Outcome\n\n## Scope\n\n## What changes\n\n## Verification");
    expect(pageFor(draft({ kind: "intent" }))).toContain("## Users\n\n## Outcomes\n\n## Constraints\n\n## Open questions");
  });

  it("collapses the title's whitespace, as the server does", () => {
    expect(pageFor(draft({ title: "  One   binary  " }))).toContain("# One binary\n");
  });
});

describe("what the sheet says it will do", () => {
  it("names the home and the file, for each kind", () => {
    expect(fileFor(draft({ kind: "decision" }))).toBe("docs/decisions/one-binary.md");
    expect(fileFor(draft({ kind: "design" }))).toBe("plans/designs/one-binary.md");
    expect(fileFor(draft({ kind: "intent" }))).toBe("docs/intent/one-binary.md");
  });

  it("says where the file will be, with a fresh id and a draft status", () => {
    expect(noteFor(draft())).toBe(
      "Writes docs/decisions/one-binary.md with a fresh id and Status: draft, then opens it.",
    );
  });

  it("says that an intent chapter joins the index's reading order", () => {
    expect(noteFor(draft({ kind: "intent" }))).toContain("listed in the intent index's reading order");
  });
});

describe("what a draft needs before it can be written", () => {
  it("is nothing, when it has a title", () => {
    expect(incomplete(draft())).toBe("");
  });

  it("is a title", () => {
    expect(incomplete(draft({ title: "   " }))).toBe("A record needs a title.");
  });

  it("is a title with a letter or a digit in it", () => {
    expect(incomplete(draft({ title: "!!!" }))).toContain("yields no file name");
  });

  it("is not a goal: a record about the project as a whole names none", () => {
    expect(incomplete(draft({ goals: [] }))).toBe("");
  });
});
