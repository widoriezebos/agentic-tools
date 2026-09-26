import { describe, expect, it } from "vitest";

import type { Suggestion } from "./api";
import { draftOf, type Field } from "./drafting";
import {
  askLine,
  cardHead,
  cardIn,
  closedLine,
  folded,
  holding,
  idOf,
  moreLabel,
  offeredIn,
  mintOpening,
  proposalsFor,
  reach,
  refusedLine,
  standingOf,
  undoable,
  undone,
  usable,
  used,
  type Marks,
  type Offered,
  type Registered,
} from "./suggesting";

/**
 * What the human may do with the words the Partner offered.
 *
 * Two of these claims are the ones a human would notice if they were wrong, and
 * neither can be read off a screenshot: a card belongs to one OPENING of a sheet
 * rather than to the sheet's name, and Undo is offered only while the field
 * still holds the words it would undo. Both are here, whole.
 */

const OPENING = "opening-1";
const SAID = "Every refund lands within a day, with nobody touching the queue.";

function suggestion(over: Partial<Suggestion> = {}): Suggestion {
  return { opening: OPENING, editor: "Edit goal", field: "Intent", text: SAID, offered: true, ...over };
}

/** One the service refused: no opening, and the reason in the human's words. */
function refused(over: Partial<Suggestion> = {}): Suggestion {
  return suggestion({ opening: "", offered: false, reason: "Intent is not open for proposals", ...over });
}

/** One sheet's registration, which records what was written into it. */
function registered(fields: Field[], sheet = "Edit goal"): Registered & { written: [string, string][] } {
  const written: [string, string][] = [];
  const held = new Map(fields.map((field) => [field.name, field.value]));
  return {
    sheet,
    written,
    read: () => draftOf(OPENING, sheet, [...held].map(([name, value]) => ({ name, value })), [...held.keys()]),
    set: (field: string, text: string) => {
      written.push([field, text]);
      const was = held.get(field) ?? "";
      held.set(field, text);
      return was;
    },
  };
}

/** The conversation's cards, from one answer's suggestions. */
function cards(marks: Marks = {}, open: readonly string[] = [OPENING], ...offers: Suggestion[]): readonly Offered[] {
  return offeredIn([{ turn: "t1", suggestions: offers.length > 0 ? offers : [suggestion()] }], marks, open);
}

describe("a card's standing", () => {
  it("waits until the human presses something", () => {
    const card = cards()[0];
    expect(card.id).toBe(idOf("t1", 0));
    expect(card.standing).toBe("waiting");
    expect(cardHead(card)).toBe("Suggestion for Intent");
  });

  it("says it was used, and says the words are the human's once they have typed", () => {
    expect(standingOf({ previous: "was", holds: true, dismissed: false }, true)).toBe("used");
    expect(standingOf({ previous: "was", holds: false, dismissed: false }, true)).toBe("edited");
  });

  it("folds to one line when it is dismissed, whatever else is true of it", () => {
    expect(standingOf({ previous: null, holds: false, dismissed: true }, true)).toBe("dismissed");
    expect(standingOf({ previous: "was", holds: true, dismissed: true }, false)).toBe("dismissed");
  });

  it("says the sheet is closed once its opening has gone", () => {
    const card = cards({}, [])[0];
    expect(card.standing).toBe("closed");
    expect(closedLine(card)).toBe("The Edit goal sheet is closed");
  });

  /**
   * Refused wins over everything, and it has to. A suggestion the service did not
   * offer was never on the human's screen: it is not something they dismissed,
   * used, or lost to a closing sheet, and every one of those cards offers a press
   * that would be a press on nothing. What it owes them is the reason.
   */
  it("says nothing was offered, whatever else is true of it", () => {
    const card = cards({}, [OPENING], refused())[0];
    expect(card.standing).toBe("refused");
    expect(refusedLine(card)).toBe("Intent is not open for proposals");
    expect(standingOf({ previous: "was", holds: true, dismissed: true }, true, false)).toBe("refused");
    expect(standingOf({ previous: null, holds: false, dismissed: false }, false, false)).toBe("refused");
    // Nothing may be pressed on it, and it reaches no sheet: it carries no
    // opening, so there is nowhere for the words to go.
    expect(usable(card, [OPENING])).toBe(false);
    expect(undoable(card, [OPENING])).toBe(false);
    expect(reach(new Map(), card)).toBeNull();
  });

  it("says the draft was left out where that is why", () => {
    const card = cards({}, [OPENING], refused({
      reason: "the draft was left out; press Ask about this to hand it over again",
    }))[0];
    expect(refusedLine(card)).toBe("the draft was left out; press Ask about this to hand it over again");
  });

  // A record written before this build carries no reason, so the card says the
  // one true thing that is left rather than an empty line.
  it("still says something where a record carries no reason", () => {
    expect(refusedLine(refused({ reason: "" }))).toBe("Intent was not offered");
  });
});

/**
 * The id a sheet mints, and why a count would not do.
 *
 * A conversation outlives the page it was had on: the transcript is read back
 * from the server on every load, cards and all. An id that started again at one
 * would hand yesterday's card the id of today's sheet, and Use this on it would
 * write into whatever goal happens to be open now.
 */
describe("the id one opening is", () => {
  it("is different every time a sheet mounts, and across page loads", () => {
    const minted = new Set([mintOpening(), mintOpening(), mintOpening()]);
    expect(minted.size).toBe(3);
    for (const id of minted) {
      expect(id.startsWith("opening-")).toBe(true);
      expect(id.length).toBeGreaterThan(12);
    }
  });
});

describe("Use this", () => {
  it("reaches the setter of its own opening, and puts the words in that field", () => {
    const sheet = registered([{ name: "Intent", value: "The board reads the ledger somehow." }]);
    const openings = new Map<string, Registered>([[OPENING, sheet]]);
    const card = cards()[0];
    expect(usable(card, [...openings.keys()])).toBe(true);
    const reached = reach(openings, card);
    expect(reached).not.toBeNull();
    const previous = reached?.set(card.field, card.text);
    expect(sheet.written).toEqual([["Intent", SAID]]);
    expect(previous).toBe("The board reads the ledger somehow.");
  });

  /**
   * F1, both halves. A suggestion belongs to the one opening it was asked from:
   * once that sheet has closed there is nothing to write into, and a sheet of
   * the same name opened again over another goal is a different opening — so
   * pressing Use this there must not overwrite that goal's field.
   */
  it("is refused on a closed opening, and on a sheet of the same name opened again", () => {
    const card = cards()[0];
    expect(usable(card, [])).toBe(false);
    expect(reach(new Map(), card)).toBeNull();

    const reopened = registered([{ name: "Intent", value: "Another goal's intent." }]);
    const openings = new Map<string, Registered>([["opening-2", reopened]]);
    expect(usable(card, [...openings.keys()])).toBe(false);
    expect(reach(openings, card)).toBeNull();
    expect(reopened.written).toEqual([]);
  });

  it("is not pressed twice: a card already used is undone before it is used again", () => {
    const marks = used({}, idOf("t1", 0), "was");
    expect(usable(cards(marks)[0], [OPENING])).toBe(false);
    expect(usable(cards(undone(marks, idOf("t1", 0)))[0], [OPENING])).toBe(true);
  });

  it("is not offered on a folded card", () => {
    expect(usable(cards(folded({}, idOf("t1", 0), true))[0], [OPENING])).toBe(false);
  });
});

describe("Undo", () => {
  const id = idOf("t1", 0);

  it("puts back what the field held at the moment of use", () => {
    const sheet = registered([{ name: "Intent", value: "The board reads the ledger somehow." }]);
    const openings = new Map<string, Registered>([[OPENING, sheet]]);
    const previous = reach(openings, cards()[0])?.set("Intent", SAID) ?? "";
    const marks = used({}, id, previous);
    const card = cards(marks)[0];
    expect(undoable(card, [...openings.keys()])).toBe(true);
    reach(openings, card)?.set(card.field, card.mark.previous ?? "");
    expect(sheet.written.at(-1)).toEqual(["Intent", "The board reads the ledger somehow."]);
    // And the card waits again, because the field holds the human's own words.
    expect(cards(undone(marks, id))[0].standing).toBe("waiting");
  });

  /**
   * F2. Typing after a suggestion arrived is never overwritten by it: once the
   * field no longer holds the words, Undo is gone and the card says so.
   */
  it("is gone once the human has typed over the words, and the card says why", () => {
    let marks = used({}, id, "was");
    expect(undoable(cards(marks)[0], [OPENING])).toBe(true);
    marks = holding(marks, cards(marks), OPENING, "Intent", "the human's own second thought");
    expect(undoable(cards(marks)[0], [OPENING])).toBe(false);
    expect(cards(marks)[0].standing).toBe("edited");
    // And typing the words back is the words back: the card is honest either way.
    marks = holding(marks, cards(marks), OPENING, "Intent", SAID);
    expect(cards(marks)[0].standing).toBe("used");
  });

  it("changes nothing when the field's answer is the one it already had", () => {
    const marks = used({}, id, "was");
    expect(holding(marks, cards(marks), OPENING, "Intent", SAID)).toBe(marks);
    expect(holding(marks, cards(marks), OPENING, "Next step", "anything")).toBe(marks);
  });

  it("is refused once the opening has gone, however the card was left", () => {
    const marks = used({}, id, "was");
    expect(undoable(cards(marks, [])[0], [])).toBe(false);
  });
});

/**
 * What stands under one field of one sheet.
 *
 * It is the whole of where g1-s52 moved the offer to: the words a human is
 * deciding about render under the field they are for, so the block has to pick
 * exactly that field's proposals, newest first, and nothing else's.
 */
describe("the proposals under a field", () => {
  it("are that field's, of that opening, newest first", () => {
    const offers = [
      suggestion(),
      suggestion({ text: "A second wording." }),
      suggestion({ field: "Next step", text: "Take it to an end state." }),
      suggestion({ opening: "opening-2", text: "Another goal's intent." }),
    ];
    const offered = cards({}, [OPENING, "opening-2"], ...offers);
    expect(proposalsFor(offered, OPENING, "Intent").map((card) => card.id)).toEqual([
      idOf("t1", 1),
      idOf("t1", 0),
    ]);
    expect(proposalsFor(offered, OPENING, "Next step").map((card) => card.text)).toEqual([
      "Take it to an end state.",
    ]);
    expect(proposalsFor(offered, OPENING, "Labels")).toEqual([]);
    expect(moreLabel(1)).toBe("1 more");
    expect(moreLabel(3)).toBe("3 more");
  });

  /**
   * Three standings belong here and three do not. A used one still stands, saying
   * what happened and offering Undo while that is honest; a dismissed one does
   * not, because folding it away was the human's own act; and one that was never
   * offered does not, because it was refused for a field and belongs where the
   * answer is read.
   */
  it("keep a used one and drop what the human folded away or was never offered", () => {
    const id = idOf("t1", 0);
    expect(proposalsFor(cards(used({}, id, "was")), OPENING, "Intent")).toHaveLength(1);
    expect(proposalsFor(cards(used({}, id, "was")), OPENING, "Intent")[0].standing).toBe("used");
    // Typed over: it still stands, and it says the human's words stay.
    const edited = holding(used({}, id, "was"), cards(used({}, id, "was")), OPENING, "Intent", "mine");
    expect(proposalsFor(cards(edited), OPENING, "Intent")[0].standing).toBe("edited");
    expect(proposalsFor(cards(folded({}, id, true)), OPENING, "Intent")).toEqual([]);
    expect(proposalsFor(cards({}, [OPENING], refused()), OPENING, "Intent")).toEqual([]);
    // Undone, it waits again, and the block offers it again.
    expect(proposalsFor(cards(undone(used({}, id, "was"), id)), OPENING, "Intent")[0].standing).toBe(
      "waiting",
    );
  });

  // A closed opening cannot reach here at all: the sheet that would render the
  // block is not on screen. The assertion is that it is filtered out anyway,
  // because a block that trusted its caller would offer Use this into nothing.
  it("drop a card whose sheet has closed", () => {
    expect(proposalsFor(cards({}, []), OPENING, "Intent")).toEqual([]);
  });
});

/**
 * What the field's own link puts in the composer.
 *
 * An empty field is asked for at all rather than asked to be bettered: "a better
 * Next step" is a strange thing to ask for a next step nobody has written.
 */
describe("Ask the Partner", () => {
  it("asks for a better one where there is one, and for one at all where there is not", () => {
    expect(askLine("Intent", "The board reads the ledger.")).toBe("Suggest a better Intent");
    expect(askLine("Next step", "")).toBe("Suggest a Next step");
    expect(askLine("Next step", "   ")).toBe("Suggest a Next step");
  });
});

describe("the cards of a conversation", () => {
  it("are identified by the turn they arrived in and their place in it", () => {
    const offered = offeredIn(
      [
        { turn: "t1", suggestions: [suggestion()] },
        { turn: "t2", suggestions: [suggestion({ field: "Labels", text: "board ui" })] },
      ],
      {},
      [OPENING],
    );
    expect(offered.map((card) => card.id)).toEqual([idOf("t1", 0), idOf("t2", 0)]);
    expect(cardIn(offered, idOf("t2", 0))?.field).toBe("Labels");
    expect(cardIn(offered, "nothing like it")).toBeUndefined();
  });
});
