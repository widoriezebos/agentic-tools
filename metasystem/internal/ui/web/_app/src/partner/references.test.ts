import { describe, expect, it } from "vitest";

import type { Index } from "./api";
import { namesIn, runsIn, type Run } from "./references";

/**
 * A name in an answer links only where it points at one thing.
 *
 * Astra's eighth finding is the rule: stable ids link, titles link only when
 * they are unique among the records, and a name this workspace does not carry
 * stays text — because a link that refuses is worse than no link.
 */

const index: Index = {
  goals: ["g1-s23", "g1-s26", "run"],
  records: [
    { id: "01M36T7", path: "plans/designs/g1-s26.md", title: "When a design has shipped", kind: "design" },
    { id: "01M348Y", path: "plans/designs/reader.md", title: "Reading", kind: "design" },
    { id: "01M348Z", path: "docs/reading.md", title: "Reading", kind: "design" },
  ],
};

const names = namesIn(index);

function linked(text: string): string[] {
  return runsIn(text, names)
    .filter((run): run is Extract<Run, { reference: unknown }> => "reference" in run)
    .map((run) => run.reference.text);
}

function where(text: string): string[] {
  return runsIn(text, names)
    .filter((run): run is Extract<Run, { reference: unknown }> => "reference" in run)
    .map((run) => run.reference.to);
}

describe("what becomes a link", () => {
  it("is a goal id, wherever it stands on its own", () => {
    expect(linked("g1-s23 is waiting on g1-s26.")).toEqual(["g1-s23", "g1-s26"]);
    expect(where("g1-s23 is waiting.")).toEqual(["/backlog?goal=g1-s23"]);
  });

  it("is a record's path and a record's id", () => {
    expect(linked("See plans/designs/reader.md, whose id is 01M348Y.")).toEqual([
      "plans/designs/reader.md",
      "01M348Y",
    ]);
    expect(where("plans/designs/reader.md")).toEqual(["/project/doc/plans/designs/reader.md"]);
  });

  it("is a title only where it names one record and not two", () => {
    // "When a design has shipped" is one record; "Reading" is two, so it is
    // two things an answer's own words cannot tell apart, and neither links.
    expect(linked("When a design has shipped says so.")).toEqual(["When a design has shipped"]);
    expect(linked("Reading says so.")).toEqual([]);
  });

  it("is nothing this workspace does not carry", () => {
    expect(linked("g9-s99 and plans/designs/nowhere.md are not here.")).toEqual([]);
  });
});

describe("where a name does not stand on its own", () => {
  // A goal really called "run" must not light up inside "running", and a path
  // must not match inside a longer one.
  it("is not a link inside a longer word or path", () => {
    expect(linked("running is not a goal")).toEqual([]);
    expect(linked("run is a goal")).toEqual(["run"]);
    expect(linked("docs/reading.md.bak")).toEqual([]);
    expect(linked("g1-s230 is not g1-s23")).toEqual(["g1-s23"]);
  });

  it("keeps every word around it, in order", () => {
    const runs = runsIn("Look at g1-s23 next.", names);
    expect(runs).toEqual([
      { text: "Look at " },
      { reference: { kind: "goal", id: "g1-s23", text: "g1-s23", to: "/backlog?goal=g1-s23" } },
      { text: " next." },
    ]);
  });

  it("prefers the longest name at a position, so a path wins over what is in it", () => {
    expect(linked("plans/designs/g1-s26.md")).toEqual(["plans/designs/g1-s26.md"]);
  });
});

describe("an answer that names many things", () => {
  it("links every one of them, and marks none of them by itself", () => {
    const said = "g1-s23, g1-s26 and run are all here.";
    expect(linked(said)).toEqual(["g1-s23", "g1-s26", "run"]);
  });

  it("resolves nothing at all against an index this build has not read", () => {
    const none = namesIn({ goals: null, records: null });
    expect(runsIn("g1-s23 is waiting.", none)).toEqual([{ text: "g1-s23 is waiting." }]);
  });
});
