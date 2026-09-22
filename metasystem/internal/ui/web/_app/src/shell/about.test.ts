import { describe, expect, it } from "vitest";

import { aboutLine } from "./about";

/**
 * The line the Project Partner's drawer carries: what the page is, and the
 * part of it being read, never the same thing twice.
 */
describe("what the drawer says a page is about", () => {
  it("names the page and the part being read", () => {
    expect(aboutLine("The Project pane", "The left rail")).toBe("The Project pane · The left rail");
  });

  it("names the page alone while no part has been read of it", () => {
    expect(aboutLine("The Project pane", "")).toBe("The Project pane");
  });

  it("names the page once where the part being read is the page's own title", () => {
    expect(aboutLine("The Project pane", "The Project pane")).toBe("The Project pane");
    expect(aboutLine("The Project pane", "  The Project pane  ")).toBe("The Project pane");
  });

  it("names the page alone for a heading that is only whitespace", () => {
    expect(aboutLine("The Project pane", "   ")).toBe("The Project pane");
  });
});
