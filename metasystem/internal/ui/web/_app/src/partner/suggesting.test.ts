import { describe, expect, it } from "vitest";

import type { Suggestion } from "./api";
import { draftOf, valueIn, type Field } from "./drafting";
import {
  answered,
  askLine,
  cardHead,
  cardIn,
  closedLine,
  FEWER,
  folded,
  holding,
  idOf,
  IN_FLIGHT,
  moreLabel,
  offeredIn,
  mintOpening,
  proposalsFor,
  reach,
  refusedLine,
  refusedSave,
  refusedSaveLine,
  SAVED,
  saving,
  standingOf,
  undoable,
  undone,
  unresolvedSave,
  unresolvedSaveLine,
  usable,
  used,
  USED_AND_SAVED,
  wasOffered,
  type Mark,
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

/**
 * One recorded before this build, as the page actually receives it.
 *
 * The flag is new in this build, so a line written earlier in the transcript
 * file does not carry it; the engine reads that line into a struct whose bool is
 * then false and writes it back out as offered: false, with no reason at all,
 * because a reason is omitted where there is none. So the legacy record reaches
 * the page looking like a refusal that will not say why.
 */
function older(): Suggestion {
  return suggestion({ offered: false });
}

/** One sheet's registration, which records what was written into it. */
function registered(fields: Field[], sheet = "Edit goal"): Registered & { written: [string, string][] } {
  const written: [string, string][] = [];
  const held = new Map(fields.map((field) => [field.name, field.value]));
  return {
    sheet,
    written,
    read: () => draftOf(OPENING, sheet, [...held].map(([name, value]) => ({ name, value })), [...held.keys()]),
    // As the sheet holds it: untrimmed, and answered for an empty field too.
    raw: (field: string) => held.get(field) ?? "",
    set: (field: string, text: string) => {
      written.push([field, text]);
      const was = held.get(field) ?? "";
      held.set(field, text);
      return was;
    },
  };
}

/** A mark as it stands after one press, spelled out so the fields are read. */
function mark(over: Partial<Mark> = {}): Mark {
  return { previous: null, holds: false, dismissed: false, sent: "", words: "", ...over };
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
    expect(standingOf(mark({ previous: "was", holds: true }), true)).toBe("used");
    expect(standingOf(mark({ previous: "was" }), true)).toBe("edited");
  });

  it("folds to one line when it is dismissed, while its sheet is open", () => {
    expect(standingOf(mark({ dismissed: true }), true)).toBe("dismissed");
  });

  /**
   * And the closing wins over the folding, which is the way round it was not.
   *
   * A card folded away before its sheet closed stayed folded afterwards, so the
   * one thing left to do with the words — copy them — appeared only if the human
   * happened to unfold a line that said nothing but "dismissed" (g1-s52 Built,
   * deferred). Nothing can be pressed on a closed opening whatever was done with
   * the card, so what it says is that the sheet is gone.
   */
  it("says the sheet is closed even where the human had folded the card away", () => {
    expect(standingOf(mark({ previous: "was", holds: true, dismissed: true }), false)).toBe("closed");
    expect(standingOf(mark({ dismissed: true }), false)).toBe("closed");
    const card = cards(folded({}, idOf("t1", 0), true), [])[0];
    expect(card.standing).toBe("closed");
    expect(closedLine(card)).toBe("The Edit goal sheet is closed");
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
    expect(standingOf(mark({ previous: "was", holds: true, dismissed: true }), true, false)).toBe("refused");
    expect(standingOf(mark({ sent: "saved" }), false, false)).toBe("refused");
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

  /**
   * A record written before this build, and why the reason is what decides.
   *
   * The service refuses nothing without saying why and says nothing about one it
   * offers, so a card with no reason was offered. Reading the flag alone made
   * every card in an older conversation a refusal whose line said "Intent was
   * not offered" of words the human had been shown and may have used.
   */
  it("reads a record written before this build as the offer it was", () => {
    expect(wasOffered(older())).toBe(true);
    expect(wasOffered(refused())).toBe(false);
    const card = cards({}, [OPENING], older())[0];
    expect(card.standing).toBe("waiting");
    expect(usable(card, [OPENING])).toBe(true);
  });

  // The floor under the line itself: a refused card shows no empty why.
  it("still says something where a refusal is asked for its line without one", () => {
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

  /**
   * What the field holds is asked of the field, whole.
   *
   * The draft the Partner is told is trimmed and drops what is empty, so a field
   * holding the suggestion's words with the human's own spacing around them reads
   * through it as the suggestion's words exactly. Undoing it would throw that
   * spacing away for a difference the comparison could not see, so the
   * registration answers raw and the store compares whole (g1-s51 Built).
   */
  it("asks the field for its raw value rather than the trimmed draft", () => {
    const sheet = registered([{ name: "Intent", value: `  ${SAID}  ` }]);
    expect(valueIn(sheet.read(), "Intent")).toBe(SAID);
    expect(sheet.raw("Intent")).toBe(`  ${SAID}  `);
    expect(sheet.raw("Intent") === SAID).toBe(false);
    // And an empty field answers, where the draft would not carry it at all.
    const empty = registered([{ name: "Next step", value: "" }]);
    expect(valueIn(empty.read(), "Next step")).toBe("");
    expect(empty.raw("Next step")).toBe("");
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
    // Unfolded, the same line folds them away again: they are shown in place,
    // under the newest, rather than in the Partner's own column.
    expect(FEWER).toBe("Fewer");
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
 * Use and save: one press, two acts, and the words say which of them landed.
 *
 * The press belongs to the sheet — it builds the next draft, sets it and sends
 * that value — and what is here is everything the card has to be able to say
 * about it afterwards. A card that said "Used and saved" of a refused request
 * would be the one thing g1-s52 refused to build (Astra F2), so saved is a
 * standing of its own and the other two answers are shown in their own words.
 */
describe("Use and save", () => {
  const id = idOf("t1", 0);

  it("puts the words in the field and says so while the save is in flight", () => {
    const marks = saving({}, id, "The board reads the ledger somehow.");
    const card = cards(marks)[0];
    expect(card.standing).toBe("used");
    expect(card.mark.sent).toBe("saving");
    expect(card.mark.previous).toBe("The board reads the ledger somehow.");
    // And no Undo, because the words are already on their way to the ledger.
    expect(undoable(card, [OPENING])).toBe(false);
    // Nor is it used a second time.
    expect(usable(card, [OPENING])).toBe(false);
  });

  it("says Used and saved on the one outcome that is a save", () => {
    const marks = answered(saving({}, id, "was"), id, SAVED);
    const card = cards(marks)[0];
    expect(card.standing).toBe("saved");
    expect(card.mark.words).toBe("");
    expect(USED_AND_SAVED).toBe("Used and saved");
    expect(undoable(card, [OPENING])).toBe(false);
  });

  /**
   * The words stay in the field and the refusal is said beside them, in the
   * engine's own sentence: the field holds what the Partner wrote, because the
   * human put it there, and the ledger holds nothing, because it said no.
   */
  it("keeps the words and says why the save was refused", () => {
    const refusal = "goal ui-1 is approved: withdraw the approval, edit it, then approve it again";
    const marks = answered(saving({}, id, "was"), id, refusedSave(refusal));
    const card = cards(marks)[0];
    expect(card.standing).toBe("used");
    expect(card.mark.sent).toBe("refused");
    expect(card.mark.words).toBe(refusal);
    expect(refusedSaveLine(refusal)).toBe(`Used; the save was refused: ${refusal}`);
    expect(undoable(card, [OPENING])).toBe(false);
  });

  it("never calls an unconfirmed save a refusal", () => {
    const said = "the act landed at tip 6984cde, but its authority proof did not: do not run it again";
    const marks = answered(saving({}, id, "was"), id, unresolvedSave(said));
    const card = cards(marks)[0];
    expect(card.mark.sent).toBe("unresolved");
    expect(unresolvedSaveLine(said)).toBe(`Used; the save was not confirmed: ${said}`);
    expect(unresolvedSaveLine(said)).not.toContain("refused");
  });

  /**
   * A save closes the sheet it saved, so the card in the transcript is the only
   * place the press is still on screen. It has to keep saying what happened
   * rather than becoming "the Edit goal sheet is closed" the instant it worked.
   */
  it("says Used and saved after the sheet it saved has closed", () => {
    const marks = answered(saving({}, id, "was"), id, SAVED);
    expect(cards(marks, [])[0].standing).toBe("saved");
    // A refused save leaves the sheet open, so that card is where it was.
    const stopped = answered(saving({}, id, "was"), id, refusedSave("no"));
    expect(cards(stopped, [])[0].standing).toBe("closed");
  });

  it("stands under its field while the sheet is still there", () => {
    const marks = answered(saving({}, id, "was"), id, SAVED);
    const standing = proposalsFor(cards(marks), OPENING, "Intent");
    expect(standing.map((card) => card.standing)).toEqual(["saved"]);
  });

  // The words a second press in the same render is answered with, kept here so
  // the sheet and the block cannot say it two different ways.
  it("answers a second press while one is in flight in one sentence", () => {
    expect(refusedSave(IN_FLIGHT)).toEqual({ kind: "refused", words: "a save is in flight" });
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
