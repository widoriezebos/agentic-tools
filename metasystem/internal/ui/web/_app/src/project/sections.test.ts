import { describe, expect, it } from "vitest";

import {
  changedLines,
  headingsOf,
  nextStepFor,
  outcomeParagraph,
  sectionIn,
  sectionReplaced,
} from "./sections";

/**
 * One section of a design, found by its heading and replaced whole (g1-s66 D3):
 * a heading identifies a section only when it occurs exactly once, and nothing
 * outside the section is touched.
 */

const DESIGN = [
  "# The loop",
  "",
  "- Kind: design",
  "",
  "## 3. The room",
  "",
  "The room, as it was.",
  "",
  "### A detail",
  "",
  "Kept with its section.",
  "",
  "## 4. Decisions",
  "",
  "- D1. Old words.",
  "",
  "```",
  "## 4. Decisions",
  "```",
  "",
  "## 5. Step 1, the smallest thing that works",
  "",
  "D1 to D5.",
  "",
  "## Outcome",
  "",
  "Build the loop from the room so a design goes to critique and back with one press.",
  "It closes by the rules.",
  "",
  "The rest is later.",
  "",
].join("\n");

describe("a section", () => {
  it("is found by its heading, to the next heading of its level, sub-headings kept", () => {
    const found = sectionIn(DESIGN, "3. The room");
    expect(found.state).toBe("found");
    if (found.state === "found") {
      expect(found.text).toBe("## 3. The room\n\nThe room, as it was.\n\n### A detail\n\nKept with its section.\n");
    }
  });

  it("ignores a heading inside a code fence", () => {
    expect(sectionIn(DESIGN, "4. Decisions").state).toBe("found");
  });

  it("is refused when absent or when its heading occurs twice", () => {
    expect(sectionIn(DESIGN, "9. Nowhere")).toEqual({ state: "absent" });
    const twice = `${DESIGN}\n## 3. The room\n\nAgain.\n`;
    expect(sectionIn(twice, "3. The room")).toEqual({ state: "duplicate", count: 2 });
  });

  it("is replaced exactly, and nothing else moves", () => {
    const replaced = sectionReplaced(DESIGN, "3. The room", "## 3. The room\n\nThe room, continued.\n\n### A detail\n\nKept with its section.\n");
    expect(replaced).toEqual({ state: "replaced", source: DESIGN.replace("The room, as it was.", "The room, continued.") });
  });

  it("keeps the document's heading line when the draft carries none", () => {
    const replaced = sectionReplaced(DESIGN, "3. The room", "Only the body.");
    expect(replaced.state === "replaced" && replaced.source.includes("## 3. The room\n\nOnly the body.\n\n## 4. Decisions")).toBe(true);
  });

  it("refuses in words, keeping nothing written, for an absent or duplicate heading", () => {
    expect(sectionReplaced(DESIGN, "9. Nowhere", "x")).toEqual({
      state: "refused",
      said: "The heading “9. Nowhere” is not in this document as it stands; nothing was written and the draft is kept.",
    });
    expect(sectionReplaced(`${DESIGN}\n## 4. Decisions\n`, "4. Decisions", "x")).toEqual({
      state: "refused",
      said: "The heading “4. Decisions” occurs 2 times in this document, so the section cannot be told apart; nothing was written and the draft is kept.",
    });
  });

  it("lists the headings a finding can be folded into", () => {
    expect(headingsOf(DESIGN)).toEqual([
      "The loop", "3. The room", "A detail", "4. Decisions", "5. Step 1, the smallest thing that works", "Outcome",
    ]);
  });
});

describe("old and new side by side", () => {
  it("marks the lines that changed on each side", () => {
    const compared = changedLines("a\nb\nc", "a\nB\nc\nd");
    expect(compared.old).toEqual([
      { text: "a", changed: false }, { text: "b", changed: true }, { text: "c", changed: false },
    ]);
    expect(compared.new).toEqual([
      { text: "a", changed: false }, { text: "B", changed: true }, { text: "c", changed: false }, { text: "d", changed: true },
    ]);
  });
});

describe("a goal from the design's outcome (D5)", () => {
  it("takes the intent from the Outcome's first paragraph, on one line", () => {
    expect(outcomeParagraph(DESIGN)).toBe(
      "Build the loop from the room so a design goes to critique and back with one press. It closes by the rules.",
    );
    expect(outcomeParagraph("# x\n\n## Outcome\n\n")).toBe("");
  });

  it("says where to continue from, naming the section that holds step 1", () => {
    expect(nextStepFor("plans/designs/g1-s66.md", DESIGN)).toBe(
      "Continue from the record plans/designs/g1-s66.md: build step 1 as its §5 says",
    );
    expect(nextStepFor("plans/designs/x.md", "# x\n")).toBe("Continue from the record plans/designs/x.md: build step 1 as it says");
  });
});
