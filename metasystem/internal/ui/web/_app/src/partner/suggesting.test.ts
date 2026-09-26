import { describe, expect, it } from "vitest";

import type { Suggestion } from "./api";
import { draftOf, type Field } from "./drafting";
import {
  cardHead,
  cardIn,
  closedLine,
  folded,
  holding,
  idOf,
  newestFor,
  offeredIn,
  mintOpening,
  reach,
  standingOf,
  undoable,
  undone,
  usable,
  used,
  waitingFor,
  waitingLabel,
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
  return { opening: OPENING, editor: "Edit goal", field: "Intent", text: SAID, ...over };
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

describe("what a field says beside its label", () => {
  it("counts the ones waiting for that field of that opening, and no others", () => {
    const offers = [
      suggestion(),
      suggestion({ text: "A second wording." }),
      suggestion({ field: "Next step", text: "Take it to an end state." }),
      suggestion({ opening: "opening-2", text: "Another goal's intent." }),
    ];
    const offered = cards({}, [OPENING, "opening-2"], ...offers);
    expect(waitingFor(offered, OPENING, "Intent")).toBe(2);
    expect(waitingFor(offered, OPENING, "Next step")).toBe(1);
    expect(waitingFor(offered, OPENING, "Labels")).toBe(0);
    expect(waitingLabel(1)).toBe("1 suggestion");
    expect(waitingLabel(2)).toBe("2 suggestions");
  });

  it("opens the drawer at the newest of them", () => {
    const offered = cards({}, [OPENING], suggestion(), suggestion({ text: "A second wording." }));
    expect(newestFor(offered, OPENING, "Intent")).toBe(idOf("t1", 1));
    expect(newestFor(offered, OPENING, "Labels")).toBe("");
  });

  it("says nothing about a suggestion that was used, dismissed, or left by a closed sheet", () => {
    const id = idOf("t1", 0);
    expect(waitingFor(cards(used({}, id, "was")), OPENING, "Intent")).toBe(0);
    expect(waitingFor(cards(folded({}, id, true)), OPENING, "Intent")).toBe(0);
    expect(waitingFor(cards({}, []), OPENING, "Intent")).toBe(0);
    // Undone, it waits again, and the field says so again.
    expect(waitingFor(cards(undone(used({}, id, "was"), id)), OPENING, "Intent")).toBe(1);
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
