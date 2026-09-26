import { describe, expect, it } from "vitest";

import type { Deposit } from "./api";
import {
  appended,
  cardsIn,
  clauseFrom,
  countsIn,
  entriesIn,
  entryOf,
  lineOf,
  marked,
  missing,
  sectionOf,
  standingOf,
  type Entry,
} from "./sitting";

/**
 * The four sections of a sitting's record, and the rules around the one press
 * that writes into them.
 *
 * Two of these are the rules a human would notice if they were wrong. What is
 * written has to be read back — the table is the record and not a second store,
 * so an entry that could not be parsed again would be an entry that vanished
 * from the table the moment it landed. And a record's other sections have to
 * survive the write untouched, because the write replaces the WHOLE source.
 */

const ENTRY: Entry = {
  when: "2026-09-26",
  who: "Wido",
  text: "the mobile client renews the session differently from the web one",
  clause: "internal/session/session.go:212",
  section: "Facts",
};

describe("one entry of a section", () => {
  it("is written with its date, its author, its words and its clause", () => {
    expect(lineOf(ENTRY, "fact")).toBe(
      "- 2026-09-26 · Wido · the mobile client renews the session differently from the web one\n" +
        "  - Anchor: internal/session/session.go:212\n",
    );
  });

  it("names the clause its kind carries", () => {
    expect(lineOf({ ...ENTRY, clause: "it counts from last activity", section: "Decisions" }, "decision")).toContain(
      "  - Reason: it counts from last activity\n",
    );
    expect(lineOf({ ...ENTRY, clause: "old sessions keep the old limit", section: "Open questions" }, "question")).toContain(
      "  - Consequence: old sessions keep the old limit\n",
    );
  });

  it("carries no clause line where there is no clause", () => {
    expect(lineOf({ ...ENTRY, clause: "  " }, "fact")).toBe(
      "- 2026-09-26 · Wido · the mobile client renews the session differently from the web one\n",
    );
  });

  // A record's section is a list, and a list item that carried a blank line
  // would end the item and leave the rest of the words as prose beside it.
  it("is one line however the human typed it", () => {
    const written = lineOf({ ...ENTRY, text: "the limit\n\nis twelve  hours" }, "fact");
    expect(written).toContain("· the limit is twelve hours\n");
    expect(written.split("\n").filter((one) => one !== "")).toHaveLength(2);
  });
});

describe("the record's four sections", () => {
  const source = [
    "- Kind: design",
    "- Status: draft",
    "",
    "# Session limits",
    "",
    "Some prose the sitting never touches.",
    "",
    "## Facts",
    "",
    "- 2026-09-26 · Wido · the limit is twelve hours",
    "  - Anchor: plans/intent/sessions.md",
    "",
    "## Decisions",
    "",
    "- 2026-09-26 · Wido · the limit counts from last activity",
    "  - Reason: a page nobody touched is not a session in use",
    "",
    "## Open questions",
    "",
    "- 2026-09-26 · Wido · what does the current limit protect?",
    "",
  ].join("\n");

  it("are read back with their words, their clause, who and when", () => {
    const entries = entriesIn(source);
    expect(entries).toHaveLength(3);
    expect(entries[0]).toEqual({
      when: "2026-09-26",
      who: "Wido",
      text: "the limit is twelve hours",
      clause: "plans/intent/sessions.md",
      section: "Facts",
    });
    expect(entries[1].section).toBe("Decisions");
    expect(entries[1].clause).toBe("a page nobody touched is not a session in use");
    expect(entries[2].section).toBe("Open questions");
    expect(entries[2].clause).toBe("");
  });

  it("are counted per section, and a section nobody has written is nothing", () => {
    expect(countsIn(source)).toEqual({ Facts: 1, Proposals: 0, Decisions: 1, "Open questions": 1 });
    expect(countsIn("")).toEqual({ Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0 });
  });

  // A list item anywhere else in the record is not an entry of the sitting: the
  // head's own bullets, and the prose's.
  it("do not read a list outside the four headings", () => {
    expect(entriesIn(source).some((entry) => entry.text.startsWith("Kind:"))).toBe(false);
  });

  // The whole point: what is written is read back. A round trip that lost the
  // clause or the author would be an entry that vanished from the table.
  it("read back exactly what they were written with", () => {
    const written = appended("## Facts\n", ENTRY, "fact");
    const read = entriesIn(written);
    expect(read).toHaveLength(1);
    expect(read[0]).toEqual(ENTRY);
  });
});

describe("appending one entry", () => {
  const source = [
    "# Session limits",
    "",
    "Prose.",
    "",
    "## Facts",
    "",
    "- 2026-09-26 · Wido · the limit is twelve hours",
    "",
    "## Decisions",
    "",
    "- 2026-09-26 · Wido · the limit counts from last activity",
    "",
    "## Later",
    "",
    "Something after all four.",
    "",
  ].join("\n");

  it("goes at the end of its own section and touches nothing else", () => {
    const written = appended(source, { ...ENTRY, text: "a second fact" }, "fact");
    const facts = entriesIn(written).filter((entry) => entry.section === "Facts");
    expect(facts.map((entry) => entry.text)).toEqual(["the limit is twelve hours", "a second fact"]);
    // Every other line of the record is exactly where it was.
    expect(written).toContain("# Session limits\n\nProse.\n");
    expect(written).toContain("## Later\n\nSomething after all four.\n");
    expect(entriesIn(written).filter((entry) => entry.section === "Decisions")).toHaveLength(1);
  });

  it("opens the section at the foot where the record has none", () => {
    const written = appended("# A design\n\nProse.\n", ENTRY, "fact");
    expect(written).toContain("# A design\n\nProse.\n\n## Facts\n\n- 2026-09-26 · Wido");
    expect(countsIn(written)).toEqual({ Facts: 1, Proposals: 0, Decisions: 0, "Open questions": 0 });
  });

  it("writes into an empty record and into an empty section", () => {
    expect(countsIn(appended("", ENTRY, "fact"))).toEqual({
      Facts: 1, Proposals: 0, Decisions: 0, "Open questions": 0,
    });
    const empty = appended("# A design\n\n## Facts\n\n## Decisions\n", ENTRY, "fact");
    expect(entriesIn(empty)).toHaveLength(1);
    expect(empty).toContain("## Decisions");
  });

  // Two entries in a row, each composed from what the one before it wrote. It is
  // the arithmetic the recorder's queue depends on.
  it("lands both of two entries written one after the other", () => {
    const once = appended(source, { ...ENTRY, text: "first" }, "fact");
    const twice = appended(once, { ...ENTRY, text: "second" }, "fact");
    expect(entriesIn(twice).filter((entry) => entry.section === "Facts").map((entry) => entry.text)).toEqual([
      "the limit is twelve hours",
      "first",
      "second",
    ]);
  });

  it("puts each kind in its own section", () => {
    expect(sectionOf("fact")).toBe("Facts");
    expect(sectionOf("decision")).toBe("Decisions");
    expect(sectionOf("question")).toBe("Open questions");
    expect(sectionOf("proposal")).toBe("Proposals");
  });
});

/* ------------------------------------------------------------- the card -- */

function offered(over: Partial<Deposit> = {}): Deposit {
  return { kind: "fact", text: "the limit is twelve hours", anchor: "a.md", offered: true, ...over };
}

describe("a deposit card", () => {
  it("starts with the Partner's own words and the clause its kind carries", () => {
    expect(marked(offered()).text).toBe("the limit is twelve hours");
    expect(clauseFrom(offered())).toBe("a.md");
    expect(clauseFrom(offered({ kind: "decision", reason: "because" }))).toBe("because");
    expect(clauseFrom(offered({ kind: "question", consequence: "then this" }))).toBe("then this");
    expect(clauseFrom(offered({ anchor: "" }))).toBe("");
  });

  // The two the design names, judged after the human's edit: the Partner may not
  // have found the anchor, and the human may know it.
  it("refuses Record it for a decision with no reason and a fact with no anchor", () => {
    const decision = offered({ kind: "decision", reason: "" });
    expect(missing("decision", marked(decision))).toContain("the reason you gave");
    expect(standingOf(decision, marked(decision))).toBe("blocked");
    expect(missing("decision", { ...marked(decision), clause: "it counts from last activity" })).toBe("");

    const fact = offered({ anchor: "" });
    expect(missing("fact", marked(fact))).toContain("anchored where it can be checked");
    expect(standingOf(fact, marked(fact))).toBe("blocked");
    expect(missing("fact", { ...marked(fact), clause: "session.go:212" })).toBe("");
  });

  it("needs neither for an open question, and always needs words", () => {
    const question = offered({ kind: "question", anchor: "", consequence: "" });
    expect(missing("question", marked(question))).toBe("");
    expect(standingOf(question, marked(question))).toBe("waiting");
    expect(missing("question", { ...marked(question), text: "  " })).toContain("says nothing yet");
  });

  it("stands where its mark says, and refused wins over everything", () => {
    const card = offered();
    const start = marked(card);
    expect(standingOf(card, start)).toBe("waiting");
    expect(standingOf(card, { ...start, recording: true })).toBe("recording");
    expect(standingOf(card, { ...start, recorded: "Facts" })).toBe("recorded");
    expect(standingOf(card, { ...start, refusal: "the record changed" })).toBe("conflict");
    expect(standingOf(card, { ...start, dismissed: true })).toBe("dismissed");
    const none = offered({ offered: false, notOffered: "no sitting is open" });
    expect(standingOf(none, { ...marked(none), dismissed: true })).toBe("refused");
  });

  it("is identified by the turn it arrived in and its place in it", () => {
    const cards = cardsIn([{ turn: "t1", deposits: [offered(), offered({ kind: "decision", reason: "r" })] }], {});
    expect(cards.map((card) => card.id)).toEqual(["deposit:t1#0", "deposit:t1#1"]);
    expect(cards[1].kind).toBe("decision");
  });

  // The entry a press would write is composed from the words as the human now
  // has them, not from the words the Partner offered.
  it("writes the entry from the words the human now has", () => {
    const cards = cardsIn(
      [{ turn: "t1", deposits: [offered()] }],
      { "deposit:t1#0": { ...marked(offered()), text: "the limit is twelve hours, from last activity", clause: "s.go:1" } },
    );
    expect(entryOf(cards[0], "Wido", "2026-09-26")).toEqual({
      when: "2026-09-26",
      who: "Wido",
      text: "the limit is twelve hours, from last activity",
      clause: "s.go:1",
      section: "Facts",
    });
  });
});
