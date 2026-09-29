import { describe, expect, it } from "vitest";

import { ALREADY_KEPT, CARRIED_STYLE, carried, drawable, drawingId, drawingsIn, keepDrawing, keptDrawing } from "./drawings";
import { recorder } from "../partner/recording";

/**
 * Keep it (g1-s71 D3): a drawing appended once under the record's Drawings
 * section with the question that produced it as its caption, marked so a
 * reload that presses it again writes nothing; and the markup rewrite the
 * chunk parses under (D2).
 */

const REVIEW = [
  "# Review of g1-s9",
  "",
  "- Kind: review",
  "- Goals: g1-s9",
  "",
  "## Findings",
  "",
  "## Drawings",
  "",
  "## Outcome",
  "",
].join("\n");

const HANDOFF = "sequenceDiagram\n  Owner->>Holder: take the lock\n  Holder-->>Owner: done";

describe("a kept drawing", () => {
  it("goes under the Drawings section with its question as the caption", () => {
    const id = drawingId("turn-1", HANDOFF);
    const next = keptDrawing(REVIEW, { id, caption: "How does the\nhandoff work?", date: "2026-09-29", source: HANDOFF });
    expect(next).not.toBeNull();
    const text = next ?? "";
    expect(text.indexOf("## Drawings")).toBeLessThan(text.indexOf(`Asked 2026-09-29: How does the handoff work? [d:${id}]`));
    expect(text.indexOf("```mermaid")).toBeLessThan(text.indexOf("## Outcome"));
    expect(drawingsIn(text)).toEqual([{ id, caption: "How does the handoff work?", date: "2026-09-29", source: HANDOFF }]);
  });

  it("is kept once: the same drawing pressed again after a reload writes nothing", () => {
    const id = drawingId("turn-1", HANDOFF);
    const once = keptDrawing(REVIEW, { id, caption: "q", date: "2026-09-29", source: HANDOFF }) ?? "";
    expect(keptDrawing(once, { id, caption: "q", date: "2026-09-30", source: HANDOFF })).toBeNull();
    expect(ALREADY_KEPT).toContain("already keeps");
  });

  it("creates the Drawings section on the first keep in a record without one", () => {
    const design = "# A design\n\n- Kind: design\n\n## 1. What\n\nWords.\n";
    const next = keptDrawing(design, { id: "0000abcd", caption: "q", date: "2026-09-29", source: "flowchart LR\n  a --> b" }) ?? "";
    expect(next.endsWith("## Drawings\n\nAsked 2026-09-29: q [d:0000abcd]\n\n```mermaid\nflowchart LR\n  a --> b\n```\n")).toBe(true);
    expect(drawingsIn(next)).toHaveLength(1);
  });

  it("keeps a second drawing after the first, and reads both back in order", () => {
    const first = keptDrawing(REVIEW, { id: "00000001", caption: "one", date: "2026-09-29", source: "flowchart LR\n  a --> b" }) ?? "";
    const both = keptDrawing(first, { id: "00000002", caption: "two", date: "2026-09-29", source: HANDOFF }) ?? "";
    expect(drawingsIn(both).map((kept) => kept.caption)).toEqual(["one", "two"]);
    expect(both.indexOf("[d:00000002]")).toBeLessThan(both.indexOf("## Outcome"));
  });

  it("fences a source that itself carries a fence with a longer one", () => {
    const tricky = "flowchart LR\n  a[```] --> b";
    const next = keptDrawing(REVIEW, { id: "00000003", caption: "q", date: "2026-09-29", source: tricky }) ?? "";
    expect(next).toContain("````mermaid");
  });

  it("names a drawing by its turn and source, the same after a reload", () => {
    expect(drawingId("turn-1", HANDOFF)).toBe(drawingId("turn-1", HANDOFF));
    expect(drawingId("turn-2", HANDOFF)).not.toBe(drawingId("turn-1", HANDOFF));
    expect(drawingId("turn-1", HANDOFF)).toMatch(/^[0-9a-f]{8}$/u);
  });

  it("draws a mermaid fence and no other", () => {
    expect(drawable("mermaid")).toBe(true);
    expect(drawable(" Mermaid ")).toBe(true);
    expect(drawable("go")).toBe(false);
  });
});

describe("markup parsed under the policy (D2)", () => {
  it("carries every style attribute and stamps every style element, and leaves text alone", () => {
    const markup = '<svg style="max-width: 10px"><style>rect{}</style><g><text>style="x"</text><rect  style="fill:red"/></g></svg>';
    const said = carried(markup, "abc");
    expect(said).toBe(
      `<svg ${CARRIED_STYLE}="max-width: 10px"><style nonce="abc">rect{}</style><g><text>style="x"</text><rect  ${CARRIED_STYLE}="fill:red"/></g></svg>`,
    );
  });

  it("puts nothing into a nonce but the characters a nonce has", () => {
    expect(carried("<style>", 'a"b><c')).toBe('<style nonce="abc">');
  });
});

describe("Keep it through the recorder (D3)", () => {
  it("appends once, and a reload's second press over the reread record writes nothing", async () => {
    let disk = REVIEW;
    let saves = 0;
    const save = async (_id: string, source: string) => {
      saves += 1;
      disk = source;
      return { revision: `r${String(saves)}`, source };
    };
    const reread = async () => ({ revision: `r${String(saves)}`, source: disk });
    const kept = { id: drawingId("turn-7", HANDOFF), caption: "How does the handoff work?", date: "2026-09-29", source: HANDOFF };

    const first = recorder({ id: "r.md", revision: "r0", source: disk }, save, reread, () => false);
    expect((await keepDrawing(first, "r.md", kept)).said).toBe("");
    // The page is reloaded: a new recorder over the record as it now reads.
    const again = recorder({ id: "r.md", ...(await reread()) }, save, reread, () => false);
    const second = await keepDrawing(again, "r.md", kept);

    expect(second.said).toBe("");
    expect(saves).toBe(1);
    expect(drawingsIn(disk).map((one) => one.id)).toEqual([kept.id]);
  });

  it("says the recorder's refusal in words", async () => {
    const held = recorder({ id: "r.md", revision: "r0", source: REVIEW }, async () => {
      throw new Error("the disk is full");
    }, async () => ({ revision: "r0", source: REVIEW }), () => false);
    const outcome = await keepDrawing(held, "r.md", { id: "00000009", caption: "q", date: "2026-09-29", source: HANDOFF });
    expect(outcome.said).toBe("the disk is full");
  });
});
