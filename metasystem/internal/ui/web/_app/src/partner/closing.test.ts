import { describe, expect, it } from "vitest";

import type { Deposit } from "./api";
import {
  appended,
  ASK_IT,
  editable,
  askedFromSitting,
  cardsIn,
  clauseHeard,
  clauseOf,
  clausedKind,
  ENDED_WITHOUT,
  entriesIn,
  entryOf,
  FROM_THE_SITTING,
  goalsIn,
  leftOpenLine,
  missing,
  OUTCOME,
  outcomeIn,
  outcomeWritten,
  pressable,
  recordedIn,
  sectionOf,
  standingOf,
  type Entry,
} from "./sitting";

/**
 * Step 2's own rules: the case's two presses, the close's one section, and the
 * question an open question becomes.
 *
 * Three of these are the ones a human would notice if they were wrong.
 *
 * What a case's press writes is not the case. Decide records a DECISION whose
 * words are the clause and whose reason the human gave; Leave open records an
 * OPEN QUESTION in the Partner's words. Both carry the case's own mark, so the
 * record says which deposit each came from and neither press can be made twice.
 *
 * The Outcome is replaced and not appended to. Every design this project writes
 * is created with an empty Outcome heading already in it, so an append would
 * leave the heading above and the words below it under a second one — and a
 * sitting closed twice would leave two.
 *
 * And a question asked out of a sitting carries three things in one question,
 * because the register's row is one question: the words, what follows from
 * leaving it open, and the record it came out of.
 */

const THE_RECORD = "plans/designs/sessions.md";

function offered(over: Partial<Deposit> = {}): Deposit {
  return {
    kind: "case",
    text: "a person reads a long page for an hour without touching anything.",
    clause: "reading without input does not keep a session alive",
    consequence: "long readers are signed out mid-sentence until somebody rules on it",
    subject: { kind: "record", id: THE_RECORD, title: "Session limits" },
    offered: true,
    ...over,
  };
}

/** The one card, as the transcript and one reading of the record make it. */
function theCard(deposit: Deposit, source = "") {
  const cards = cardsIn([{ turn: "t1", deposits: [deposit] }], {}, THE_RECORD, recordedIn(source));
  const card = cards.at(0);
  if (card === undefined) {
    throw new Error("no card");
  }
  return card;
}

describe("a case at the edge", () => {
  it("lands on no pile by itself, and offers neither of its clauses as an entry", () => {
    const card = theCard(offered());
    // Its own clause on the card is the consequence, because that is what its own
    // press records: Leave open writes it with the question.
    expect(clauseOf("case")).toBe("Consequence");
    expect(card.mark.clause).toBe("long readers are signed out mid-sentence until somebody rules on it");
    expect(card.standing).toBe("waiting");
    // It needs nothing before a press, because neither press records the case.
    expect(missing("case", card.mark)).toBe("");
  });

  it("opens the Decide sheet with the clause the Partner heard, or with the case itself", () => {
    expect(clauseHeard(offered())).toBe("reading without input does not keep a session alive");
    expect(clauseHeard(offered({ clause: "  " }))).toBe(
      "a person reads a long page for an hour without touching anything.",
    );
  });

  it("records a decision from Decide: the clause as its words, the human's reason, and the case's mark", () => {
    const card = theCard(offered());
    const entry = entryOf(card, "Wido", "2026-09-26", {
      kind: "decision",
      text: "reading without input does not keep a session alive",
      clause: "a page nobody has touched for an hour is not a session in use",
    });
    expect(entry.section).toBe("Decisions");
    expect(entry.mark).toBe(card.id);
    const written = appended("# Session limits\n", entry, "decision");
    expect(written).toContain("## Decisions");
    expect(written).toContain("2026-09-26 · Wido · reading without input does not keep a session alive");
    expect(written).toContain("  - Reason: a page nobody has touched for an hour is not a session in use");
    // The record is what says the card was recorded, so the same card reads back
    // as recorded in the pile the press put it in.
    expect(theCard(offered(), written).standing).toBe("recorded");
    expect(theCard(offered(), written).mark.recorded).toBe("Decisions");
  });

  it("records an open question from Leave open: the case's words and the consequence it gave", () => {
    const card = theCard(offered());
    const entry = entryOf(card, "Wido", "2026-09-26", {
      kind: "question",
      text: card.mark.text,
      clause: card.mark.clause,
    });
    expect(entry.section).toBe("Open questions");
    expect(entry.mark).toBe(card.id);
    const written = appended("# Session limits\n", entry, "question");
    expect(written).toContain("## Open questions");
    expect(written).toContain("· a person reads a long page for an hour without touching anything.");
    expect(written).toContain("  - Consequence: long readers are signed out mid-sentence");
    expect(theCard(offered(), written).mark.recorded).toBe("Open questions");
  });

  it("says what Leave open will do before it is pressed", () => {
    expect(leftOpenLine("long readers are signed out")).toContain("Records this case as an open question");
    expect(leftOpenLine("long readers are signed out")).toContain(
      "Its consequence is recorded with it: long readers are signed out",
    );
    expect(leftOpenLine("  ")).toBe("Records this case as an open question, in the Partner's words.");
  });

  it("is decided once: a decision already in the record offers no second press", () => {
    const card = theCard(offered());
    const written = appended(
      "# Session limits\n",
      entryOf(card, "Wido", "2026-09-26", { kind: "decision", text: "the clause", clause: "the reason" }),
      "decision",
    );
    const again = theCard(offered(), written);
    expect(again.standing).toBe("recorded");
    // And what it shows is the record's own words rather than the case's.
    expect(again.mark.text).toBe("the clause");
    expect(again.mark.clause).toBe("the reason");
  });
});

const CLOSING = "The limit counts from last activity.\n\nOpen questions: what the twelve hours protected.";

function outcomeCard(source = ""): Entry {
  const card = theCard(offered({ kind: "outcome", text: CLOSING, clause: undefined, consequence: undefined }), source);
  return entryOf(card, "Wido", "2026-09-26");
}

/** An outcome drafted the way the Partner drafts one: prose, under its own sub-headings. */
const SUBBED =
  "The limit counts from last activity.\n\n### Constraints\n\nThe mobile client renews on its own clock.\n\n" +
  "### What is left open\n\nWhether anything the twelve hours protected is lost.";

function subbedCard(source = ""): Entry {
  const card = theCard(offered({ kind: "outcome", text: SUBBED, clause: undefined, consequence: undefined }), source);
  return entryOf(card, "Wido", "2026-09-26");
}

describe("what the sitting came to", () => {
  it("is written as the record's own Outcome section, with the deposit's mark", () => {
    const written = outcomeWritten("# Session limits\n\n- Kind: design\n", outcomeCard());
    expect(written).toContain("## Outcome");
    expect(written).toContain("The limit counts from last activity.");
    expect(written).toContain("Open questions: what the twelve hours protected.");
    expect(written).toContain(`- ${FROM_THE_SITTING} · 2026-09-26 · Wido [d:deposit:t1#0]`);
    // Its paragraphs survive: an outcome is a section and not one line.
    expect(written.split("\n").filter((one) => one.trim() !== "").length).toBeGreaterThan(4);
  });

  it("carries no clause, because it is a section and not an entry", () => {
    expect(clausedKind("outcome")).toBe(false);
    expect(clausedKind("fact")).toBe(true);
    expect(sectionOf("outcome")).toBe(OUTCOME);
  });

  it("replaces the Outcome the record already has, and leaves every other section alone", () => {
    const before =
      "# Session limits\n\n- Kind: design\n\n## Outcome\n\nWhat an earlier sitting decided.\n\n" +
      `- ${FROM_THE_SITTING} · 2026-09-20 · Wido [d:deposit:old#0]\n\n` +
      "## Scope\n\nWhat this does not touch.\n\n## Facts\n\n- 2026-09-24 · Wido · a fact [d:deposit:t0#0]\n";
    const written = outcomeWritten(before, outcomeCard());
    expect(written.match(/## Outcome/g)).toHaveLength(1);
    expect(written).not.toContain("What an earlier sitting decided.");
    expect(written).not.toContain("deposit:old#0");
    expect(written).toContain("## Scope\n\nWhat this does not touch.");
    expect(written).toContain("- 2026-09-24 · Wido · a fact [d:deposit:t0#0]");
    // The table is untouched: the Outcome is not one of the four piles.
    expect(entriesIn(written).filter((entry) => entry.section === "Facts")).toHaveLength(1);
    expect(entriesIn(written).some((entry) => entry.section === OUTCOME)).toBe(false);
  });

  it("keeps the heading's own level where the record spells it differently", () => {
    const written = outcomeWritten("# It\n\n### Outcome\n\nOld words.\n", outcomeCard());
    // The heading line is the record's own, whole: a close that rewrote it at
    // level two would move the section in the record's outline.
    expect(written.split("\n").filter((one) => one.trim().endsWith("Outcome"))).toEqual(["### Outcome"]);
    expect(written).not.toContain("Old words.");
  });

  it("is read back as the record has it, so a reload offers no second Outcome", () => {
    const written = outcomeWritten("# Session limits\n", outcomeCard());
    const read = outcomeIn(written);
    expect(read).not.toBeNull();
    expect(read?.section).toBe(OUTCOME);
    expect(read?.text).toBe(CLOSING);
    expect(read?.when).toBe("2026-09-26");
    expect(read?.who).toBe("Wido");
    expect(read?.mark).toBe("deposit:t1#0");
    // Which is what makes the card recorded: the record says so.
    expect(recordedIn(written).get("deposit:t1#0")?.section).toBe(OUTCOME);
    const card = theCard(offered({ kind: "outcome", text: CLOSING }), written);
    expect(card.standing).toBe("recorded");
    expect(card.mark.recorded).toBe(OUTCOME);
  });

  it("reads an Outcome a human wrote by hand, and claims no deposit wrote it", () => {
    const read = outcomeIn("# It\n\n## Outcome\n\nWhat I decided, typed here myself.\n");
    expect(read?.text).toBe("What I decided, typed here myself.");
    expect(read?.mark).toBe("");
    expect(recordedIn("# It\n\n## Outcome\n\nWhat I decided, typed here myself.\n").size).toBe(0);
    expect(outcomeIn("# It\n\n## Scope\n\nNo outcome at all.\n")).toBeNull();
  });

  it("keeps a sub-heading the outcome carries inside the outcome, foot and all", () => {
    // The Partner drafts the close as prose with its own sub-headings — the
    // constraints, what is left open — and the line that says the close was
    // recorded is BELOW them. A section cut at the first sub-heading would leave
    // that foot outside it: the card would show as unrecorded on a reload, and
    // the next close would leave the tail of this one under its sub-heading.
    const written = outcomeWritten(
      "# Session limits\n\n- Kind: design\n\n## Outcome\n\n## Scope\n\nWhat this does not touch.\n",
      subbedCard(),
    );
    const read = outcomeIn(written);
    expect(read?.text).toBe(SUBBED);
    expect(read?.mark).toBe("deposit:t1#0");
    // Which is what makes the card recorded: the record says so.
    expect(recordedIn(written).get("deposit:t1#0")?.section).toBe(OUTCOME);
    // And the section that follows is still its own.
    expect(written).toContain("## Scope\n\nWhat this does not touch.");

    // Replaced whole on the next close: sub-headings, prose and foot.
    const again = outcomeWritten(written, outcomeCard());
    expect(again.match(/## Outcome/g)).toHaveLength(1);
    expect(again).not.toContain("### Constraints");
    expect(again).not.toContain("on its own clock");
    expect(again).toContain("The limit counts from last activity.");
    expect(again).toContain("## Scope\n\nWhat this does not touch.");
    expect(outcomeIn(again)?.text).toBe(CLOSING);
  });

  it("says what ending without recording left behind", () => {
    expect(ENDED_WITHOUT).toContain("no outcome was written into the record");
    expect(ENDED_WITHOUT).toContain("Everything you recorded during it is still there");
  });
});

describe("an open question asked out of a sitting", () => {
  const entry: Entry = {
    when: "2026-09-25",
    who: "Wido",
    text: "what the current twelve-hour limit protects",
    clause: "the new limit is chosen without knowing what the old one was for",
    section: "Open questions",
    mark: "deposit:t2#0",
  };

  it("is one question carrying its words, its consequence and the record it came from", () => {
    expect(askedFromSitting(entry, THE_RECORD)).toBe(
      "what the current twelve-hour limit protects — " +
        "leaving it open: the new limit is chosen without knowing what the old one was for — " +
        `from the sitting on ${THE_RECORD}`,
    );
  });

  it("carries the source even where the Partner attached no consequence", () => {
    expect(askedFromSitting({ ...entry, clause: "" }, THE_RECORD)).toBe(
      `what the current twelve-hour limit protects — from the sitting on ${THE_RECORD}`,
    );
  });

  it("is one line, because the register's row is one cell", () => {
    const said = askedFromSitting({ ...entry, text: "what\nthe limit\tprotects" }, THE_RECORD);
    expect(said.includes("\n")).toBe(false);
    expect(said).toContain("what the limit protects");
  });

  it("is scoped by the ledger goals the record's own head names", () => {
    const head =
      "# Session limits\n\n- Kind: design\n- Id: design-sessions\n- Status: draft\n" +
      "- Goals: browser-interface session-limits\n\n## Facts\n\n- 2026-09-26 · Wido · a fact\n";
    expect(goalsIn(head)).toEqual(["browser-interface", "session-limits"]);
  });

  it("is about the project as a whole where the record names no goal", () => {
    expect(goalsIn("# It\n\n- Kind: design\n- Id: x\n- Status: draft\n\n## Facts\n")).toEqual([]);
    // A "Goals:" line inside a section is not the head's, so it is not read.
    expect(goalsIn("# It\n\n- Kind: design\n\n## Facts\n\n- Goals: not-a-scope\n")).toEqual([]);
  });

  it("is what the press on the table is called", () => {
    expect(ASK_IT).toBe("Ask it");
  });
});

/**
 * A card the record moved under is pressed again, without an edit.
 *
 * It is the one rule of step 2 that fixes something step 1 offered and did not
 * do: the press was shown on a card in conflict and then admitted nothing,
 * because only a waiting card was admitted and the only way back to waiting was
 * to edit a field. A human happy with their words had nothing to edit, and the
 * button did nothing at all.
 */
describe("a card left in conflict", () => {
  it("is pressed again as it stands, and so is a case", () => {
    expect(pressable("conflict")).toBe(true);
    expect(pressable("waiting")).toBe(true);
  });

  it("is the only state besides waiting that a press is admitted from", () => {
    for (const standing of ["refused", "dismissed", "recorded", "elsewhere", "recording", "blocked"] as const) {
      expect({ standing, pressable: pressable(standing) }).toEqual({ standing, pressable: false });
    }
  });

  it("keeps the words the human wrote, because they are what the next press writes", () => {
    const card = theCard(offered({ kind: "fact", text: "the limit is twelve hours", anchor: "sessions.md:14" }));
    const conflicted = { ...card, mark: { ...card.mark, refusal: "The record changed while you were writing" } };
    expect(standingOf(conflicted, conflicted.mark, THE_RECORD)).toBe("conflict");
    // And the fields are still theirs, so the words can be changed as well as
    // pressed: what is refused is a card in flight, not a card refused.
    expect(editable("conflict")).toBe(true);
  });
});
