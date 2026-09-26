import { describe, expect, it } from "vitest";

import { draftOf, PartnerError, type Deposit } from "./api";
import {
  appended,
  cardsIn,
  clauseFrom,
  countsIn,
  draftMadeLine,
  editable,
  ELSEWHERE,
  elsewhereLine,
  entriesIn,
  entryOf,
  lineOf,
  marked,
  missing,
  recordedIn,
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
  mark: "",
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
      mark: "",
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

  /**
   * Sol's fourth finding: the identity of the deposit an entry was recorded from
   * is written into the entry, so a reload can tell an entry the record already
   * carries from a card offering to write one.
   *
   * It is written at the end of the line, and the reader takes it off again, so
   * the words the table shows are the words and not the bookkeeping.
   */
  it("carry the deposit they were recorded from, and give its words back without it", () => {
    const written = appended("## Facts\n", { ...ENTRY, mark: "deposit:t1#0" }, "fact");
    expect(written).toContain("differently from the web one [d:deposit:t1#0]\n");

    const read = entriesIn(written);
    expect(read[0].mark).toBe("deposit:t1#0");
    expect(read[0].text).toBe(ENTRY.text);
    expect(read[0].clause).toBe(ENTRY.clause);

    const held = recordedIn(written);
    expect(held.get("deposit:t1#0")?.section).toBe("Facts");
    expect(held.get("deposit:t1#0")?.text).toBe(ENTRY.text);
    expect(held.has("deposit:t1#1")).toBe(false);
  });

  it("carry no mark where a human wrote the line, and are read all the same", () => {
    const byHand = "## Facts\n\n- 2026-09-26 · Wido · a line somebody typed\n";
    expect(entriesIn(byHand)[0].mark).toBe("");
    expect(recordedIn(byHand).size).toBe(0);
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

/** The record the sitting below is on, and what every card is stamped with. */
const SUBJECT = "plans/designs/sessions.md";

function offered(over: Partial<Deposit> = {}): Deposit {
  return {
    kind: "fact",
    text: "the limit is twelve hours",
    anchor: "a.md",
    subject: { kind: "record", id: SUBJECT, title: "Session limits" },
    offered: true,
    ...over,
  };
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
    expect(standingOf(decision, marked(decision), SUBJECT)).toBe("blocked");
    expect(missing("decision", { ...marked(decision), clause: "it counts from last activity" })).toBe("");

    const fact = offered({ anchor: "" });
    expect(missing("fact", marked(fact))).toContain("anchored where it can be checked");
    expect(standingOf(fact, marked(fact), SUBJECT)).toBe("blocked");
    expect(missing("fact", { ...marked(fact), clause: "session.go:212" })).toBe("");
  });

  it("needs neither for an open question, and always needs words", () => {
    const question = offered({ kind: "question", anchor: "", consequence: "" });
    expect(missing("question", marked(question))).toBe("");
    expect(standingOf(question, marked(question), SUBJECT)).toBe("waiting");
    expect(missing("question", { ...marked(question), text: "  " })).toContain("says nothing yet");
  });

  it("stands where its mark says, and refused wins over everything", () => {
    const card = offered();
    const start = marked(card);
    expect(standingOf(card, start, SUBJECT)).toBe("waiting");
    expect(standingOf(card, { ...start, recording: true }, SUBJECT)).toBe("recording");
    expect(standingOf(card, { ...start, recorded: "Facts" }, SUBJECT)).toBe("recorded");
    expect(standingOf(card, { ...start, refusal: "the record changed" }, SUBJECT)).toBe("conflict");
    expect(standingOf(card, { ...start, dismissed: true }, SUBJECT)).toBe("dismissed");
    const none = offered({ offered: false, notOffered: "no sitting is open" });
    expect(standingOf(none, { ...marked(none), dismissed: true }, SUBJECT)).toBe("refused");
  });

  it("is identified by the turn it arrived in and its place in it", () => {
    const cards = cardsIn(
      [{ turn: "t1", deposits: [offered(), offered({ kind: "decision", reason: "r" })] }],
      {},
      SUBJECT,
      new Map(),
    );
    expect(cards.map((card) => card.id)).toEqual(["deposit:t1#0", "deposit:t1#1"]);
    expect(cards[1].kind).toBe("decision");
  });

  // The entry a press would write is composed from the words as the human now
  // has them, not from the words the Partner offered.
  it("writes the entry from the words the human now has", () => {
    const cards = cardsIn(
      [{ turn: "t1", deposits: [offered()] }],
      { "deposit:t1#0": { ...marked(offered()), text: "the limit is twelve hours, from last activity", clause: "s.go:1" } },
      SUBJECT,
      new Map(),
    );
    expect(entryOf(cards[0], "Wido", "2026-09-26")).toEqual({
      when: "2026-09-26",
      who: "Wido",
      text: "the limit is twelve hours, from last activity",
      clause: "s.go:1",
      section: "Facts",
      mark: "deposit:t1#0",
    });
  });
});

/**
 * Sol's first finding: the press used to ask only whether a deposit was offered.
 *
 * End the sitting on A, start one on B, and A's cards are still on the
 * transcript. The words on them were offered to A's record; a press that asked
 * nothing about which record they were for wrote them into B's.
 */
describe("a card offered to another record", () => {
  const OTHER = "plans/designs/other.md";

  it("offers no press, and says which record it was for", () => {
    const card = offered();
    expect(standingOf(card, marked(card), OTHER)).toBe("elsewhere");
    expect(elsewhereLine(card)).toContain(SUBJECT);
    expect(elsewhereLine(card)).toContain(ELSEWHERE);
    // And its fields are nobody's to edit any more, because there is nothing
    // here to write.
    expect(editable("elsewhere")).toBe(false);
  });

  it("stands elsewhere where no sitting is open at all", () => {
    const card = offered();
    expect(standingOf(card, marked(card), "")).toBe("elsewhere");
    const unstamped = offered({ subject: undefined });
    expect(standingOf(unstamped, marked(unstamped), SUBJECT)).toBe("elsewhere");
    expect(elsewhereLine(unstamped)).toContain("another sitting");
  });

  // Recorded is the truer fact: an entry that landed in A's record landed there
  // whatever the human is sitting on now, and the card says so rather than
  // offering the words again.
  it("still says recorded where the record took it", () => {
    const card = offered();
    expect(standingOf(card, { ...marked(card), recorded: "Facts" }, OTHER)).toBe("recorded");
  });

  it("is not elsewhere on the record it was offered to", () => {
    const card = offered();
    expect(standingOf(card, marked(card), SUBJECT)).toBe("waiting");
  });
});

/**
 * Sol's fourth finding: the recorded mark lived in this browser's memory, so a
 * reload reconstructed every persisted deposit as a fresh card offering Record
 * it — for an entry the record already carried.
 */
describe("a card the record already carries", () => {
  const carried = [{ turn: "t1", deposits: [offered()] }];

  it("is recorded after a reload, with the record's own words, and offers no press", () => {
    // A reload: the browser remembers nothing about this card, and the reading
    // of the record is what it has.
    const source = appended(
      "# Session limits\n",
      { when: "2026-09-26", who: "Wido", text: "the limit is twelve hours", clause: "a.md", section: "Facts", mark: "deposit:t1#0" },
      "fact",
    );

    const cards = cardsIn(carried, {}, SUBJECT, recordedIn(source));

    expect(cards[0].standing).toBe("recorded");
    expect(cards[0].mark.recorded).toBe("Facts");
    expect(cards[0].mark.text).toBe("the limit is twelve hours");
    expect(cards[0].mark.clause).toBe("a.md");
    expect(editable(cards[0].standing)).toBe(false);
  });

  it("shows what the record says rather than what was offered or typed", () => {
    const source = appended(
      "# Session limits\n",
      {
        when: "2026-09-26",
        who: "Wido",
        text: "the limit is twelve hours, counted from last activity",
        clause: "internal/session/session.go:212",
        section: "Facts",
        mark: "deposit:t1#0",
      },
      "fact",
    );

    const cards = cardsIn(carried, { "deposit:t1#0": { ...marked(offered()), text: "half a sentence" } }, SUBJECT, recordedIn(source));

    expect(cards[0].mark.text).toBe("the limit is twelve hours, counted from last activity");
    expect(cards[0].mark.clause).toBe("internal/session/session.go:212");
    expect(cards[0].standing).toBe("recorded");
  });

  it("is waiting again only where the record does not carry it", () => {
    const cards = cardsIn(carried, {}, SUBJECT, recordedIn("## Facts\n\n- 2026-09-26 · Wido · something else\n"));
    expect(cards[0].standing).toBe("waiting");
  });
});

/**
 * Sol's fifth finding: the fields stayed editable through the write, and the
 * press had already composed its entry — so the card said "recorded" over words
 * the record never took.
 */
/**
 * Sol's third finding, on the page's side: the draft a refused Start left behind.
 *
 * The record exists in the project now. What the sheet owes the human is to say
 * so and to press Start on THAT draft next time, because the alternative is one
 * wish leaving two drafts nobody asked for.
 */
describe("a Start refused after its draft was created", () => {
  it("carries the draft's path out of the refusal", () => {
    expect(draftOf(new PartnerError(409, "a turn is running", "", "plans/designs/limits.md"))).toBe(
      "plans/designs/limits.md",
    );
    expect(draftOf(new PartnerError(400, "a sitting is for shape intent"))).toBe("");
    expect(draftOf(new Error("something else"))).toBe("");
  });

  it("says which draft was made and what pressing Start again does", () => {
    const said = draftMadeLine("plans/designs/limits.md");
    expect(said).toContain("plans/designs/limits.md");
    expect(said).toContain("opens the sitting on that record rather than making a second one");
  });
});

describe("a card whose press is in flight", () => {
  it("is not the human's to change, and neither is one the record has taken", () => {
    expect(editable("recording")).toBe(false);
    expect(editable("recorded")).toBe(false);
    expect(editable("dismissed")).toBe(false);
    expect(editable("refused")).toBe(false);
  });

  it("leaves the words editable while nothing is in flight", () => {
    expect(editable("waiting")).toBe(true);
    expect(editable("blocked")).toBe(true);
    // A refused press keeps the human's words and offers the press again, so the
    // words are theirs to change before they press it.
    expect(editable("conflict")).toBe(true);
  });
});
