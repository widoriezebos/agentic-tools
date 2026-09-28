import { describe, expect, it } from "vitest";

import { sectionOf } from "./Desk";
import type { Block } from "../project/api";

/** A record section on the desk is the document reader's own blocks, cut at the heading (g1-s65 D4). */
describe("a record's section on the desk", () => {
  const heading = (level: number, id: string, text: string): Block => ({
    type: "heading", level, id, inlines: [{ type: "text", text }],
  });
  const paragraph = (text: string): Block => ({ type: "paragraph", inlines: [{ type: "text", text }] });
  const blocks = [
    heading(2, "4-decisions", "4. Decisions"),
    paragraph("intro"),
    heading(3, "d3-the-owner", "D3. The owner holds the lock"),
    paragraph("the lock"),
    heading(3, "d4", "D4. The desk"),
    paragraph("the desk"),
  ];

  it("is the heading and what stands under it to the next of its level", () => {
    expect(sectionOf(blocks, "D3")).toEqual([blocks[2], blocks[3]]);
    expect(sectionOf(blocks, "4-decisions")).toEqual(blocks);
    expect(sectionOf(blocks, "D9")).toEqual([]);
  });
});
