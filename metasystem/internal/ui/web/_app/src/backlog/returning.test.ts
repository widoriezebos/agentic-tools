import { describe, expect, it } from "vitest";

import { noFilters, type Filters } from "./filters";
import { appliedLine, askedFor, returnAddress } from "./returning";

/**
 * Going back to the board a question was asked from.
 *
 * Astra's ninth finding is the rule: the chip must reopen the set and the
 * viewpoint that made "these" mean something, say what it applied, and name
 * what it could not — never present today's page as the old one.
 */

const narrowed: Filters = { text: "sign", priority: "", tier: "1", seat: "=m1e", arc: "" };

describe("the address a capture returns to", () => {
  it("carries the view, the window and every filter that narrowed the board", () => {
    const address = returnAddress("board", narrowed, 7, "g1-s23");
    const query = new URLSearchParams(address.slice(address.indexOf("?")));
    expect(address.startsWith("/backlog?")).toBe(true);
    expect(query.get("goal")).toBe("g1-s23");
    expect(query.get("view")).toBe("board");
    expect(query.get("window")).toBe("7");
    expect(query.get("text")).toBe("sign");
    expect(query.get("tier")).toBe("1");
    expect(query.get("seat")).toBe("=m1e");
    expect(query.get("priority")).toBeNull();
  });

  it("is the view and the window alone where nothing narrowed it", () => {
    const address = returnAddress("list", noFilters, null);
    expect(address).toBe("/backlog?view=list&window=all");
  });
});

describe("what an arrival asks for", () => {
  it("reads back exactly what the address carried", () => {
    const asked = askedFor(new URLSearchParams(returnAddress("board", narrowed, 7, "g1-s23").slice(9)));
    expect(asked.view).toBe("board");
    expect(asked.reach).toBe(7);
    expect(asked.goal).toBe("g1-s23");
    expect(asked.filters).toEqual(narrowed);
  });

  it("round-trips a board nobody narrowed as no filters, not as today's", () => {
    const asked = askedFor(new URLSearchParams(returnAddress("board", noFilters, 1).slice(9)));
    expect(asked.filters).toEqual(noFilters);
    expect(asked.applied).toContain("no filters");
  });

  it("leaves alone everything an address does not name", () => {
    const asked = askedFor(new URLSearchParams("goal=g1-s23"));
    expect(asked.view).toBeNull();
    expect(asked.filters).toBeNull();
    expect(asked.reach).toBeNull();
    expect(asked.goal).toBe("g1-s23");
    expect(asked.applied).toEqual([]);
  });

  it("ignores a view, a window and a rank the board does not have", () => {
    const asked = askedFor(new URLSearchParams("view=galaxy&window=nine&tier=9"));
    expect(asked.view).toBeNull();
    expect(asked.reach).toBeNull();
    expect(asked.filters).toBeNull();
  });
});

describe("what the page then says it did", () => {
  it("names the view, the window and the filters it applied", () => {
    const asked = askedFor(new URLSearchParams(returnAddress("board", narrowed, 7, "g1-s23").slice(9)));
    const line = appliedLine(asked, "");
    expect(line).toContain("Showing this board as it was: the board");
    expect(line).toContain("Done reaching back 7 days");
    expect(line).toContain('filters text "sign", tier 1, seat m1e');
  });

  // A subject that is no longer there is said, never silently replaced by
  // whatever this board happens to hold today.
  it("names a subject that is no longer on the board", () => {
    const asked = askedFor(new URLSearchParams(returnAddress("board", noFilters, 1, "g1-s23").slice(9)));
    expect(appliedLine(asked, "g1-s23 is no longer on this board.")).toContain("no longer on this board");
  });

  it("says nothing for an arrival that asked for nothing", () => {
    expect(appliedLine(askedFor(new URLSearchParams("")), "")).toBe("");
  });
});
