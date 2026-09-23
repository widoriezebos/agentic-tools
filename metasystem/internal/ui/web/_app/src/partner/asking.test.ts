import { describe, expect, it } from "vitest";

import { keyFor, mintKey, pageOf } from "./asking";

/**
 * What a question carries, and the key it is asked under.
 *
 * The page context is the answer to Astra's third finding: a label cannot be
 * looked up, so what travels is the subject's identity and the revision the
 * page is displaying. A field the page does not know is not sent at all —
 * an empty string would be a claim that the page knows the subject has none.
 */

describe("what a question carries", () => {
  it("names the section and the address of the page it was asked from", () => {
    expect(pageOf("/backlog", {})).toEqual({ section: "Backlog", path: "/backlog" });
  });

  it("names a goal by its ledger id, with the tab that is open", () => {
    expect(pageOf("/backlog/goal/g1-s28/plan", { kind: "goal", subject: "g1-s28", title: "g1-s28", tab: "Plan" })).toEqual({
      section: "Backlog",
      path: "/backlog/goal/g1-s28/plan",
      kind: "goal",
      subject: "g1-s28",
      title: "g1-s28",
      tab: "Plan",
    });
  });

  it("names a document by its id and the revision the page is showing", () => {
    expect(
      pageOf("/project/doc/records/designs/d1.md", {
        kind: "document",
        subject: "records/designs/d1.md",
        title: "A design",
        revision: "sha1:abc",
      }),
    ).toEqual({
      section: "Project",
      path: "/project/doc/records/designs/d1.md",
      kind: "document",
      subject: "records/designs/d1.md",
      title: "A design",
      revision: "sha1:abc",
    });
  });

  it("names what the board is narrowed to, and nothing when it is not", () => {
    expect(pageOf("/backlog", { filters: ["priority 2", "seat m1e"] }).filters).toEqual(["priority 2", "seat m1e"]);
    expect(pageOf("/backlog", { filters: [] }).filters).toBeUndefined();
  });

  it("sends no field the page does not know", () => {
    expect(pageOf("/overview", { kind: "", subject: "", revision: "" })).toEqual({
      section: "Overview",
      path: "/overview",
    });
  });

  it("says nothing about a section it has no name for", () => {
    expect(pageOf("/nowhere", {})).toEqual({ section: "", path: "/nowhere" });
  });
});

describe("the turn key", () => {
  it("is minted once and kept while nothing has been accepted", () => {
    let minted = 0;
    const mint = () => {
      minted += 1;
      return `k-${String(minted)}`;
    };
    const first = keyFor("", mint);
    expect(first).toBe("k-1");
    // The send was refused, so the page still holds the key: pressing Send
    // again is this turn again rather than a second one.
    expect(keyFor(first, mint)).toBe("k-1");
    expect(minted).toBe(1);
    // Accepted, so the page lets it go and the next question mints its own.
    expect(keyFor("", mint)).toBe("k-2");
  });

  it("is different every time it is minted", () => {
    const keys = new Set([mintKey(), mintKey(), mintKey()]);
    expect(keys.size).toBe(3);
    for (const key of keys) {
      expect(key.length).toBeGreaterThan(8);
    }
  });
});
