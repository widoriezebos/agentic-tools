import type { Suggestion } from "./api";
import { mintKey } from "./asking";
import type { SheetDraft } from "./drafting";

/**
 * What the human may do with the words the Partner offered, and when.
 *
 * The Partner answers in words and offers text for one field of the editor the
 * human handed over. Nothing enters that field on its own: the card waits in
 * the drawer, the field says one is waiting, and Use this is the human's press.
 * This file is the whole of the rules around that press — what identifies a
 * card, which of its standings it is in, whether Use this can still be
 * done, whether Undo is still honest, and how many are waiting for one field —
 * so that every one of them can be read, and tested, without a browser.
 *
 * Two rules here are the ones a human would notice if they were wrong.
 *
 * A card belongs to one OPENING of an editor and not to the editor's name. Goal
 * A's edit sheet and goal B's are both called "Edit goal"; a suggestion asked
 * for while A was open must not be usable once A has been closed, and must not
 * become usable again because B was opened. So the sheet mints an id when it
 * mounts, the card carries it, and Use this asks for that opening by that id.
 *
 * Undo is offered only while the field still holds the suggestion's words. It
 * restores what the field held at the moment of use, which is exactly right
 * while nothing else has happened and exactly wrong once the human has typed
 * over it: that would throw away their words in the name of undoing ours.
 */

/**
 * How one opening of an editor answers the two things the store asks of it.
 *
 * The sheet supplies both, because the sheet owns the state: only it knows how
 * its fields read right now, and only it can put words into one of them. The
 * store holds the registration for as long as the sheet is on screen and asks
 * nothing of it in between — the fields are not watched and not streamed.
 */
export type Registered = {
  /** What the sheet is called, as its head says it: "Edit goal". */
  sheet: string;
  /** Its fields as they stand, for the question that carries them. */
  read: () => SheetDraft;
  /**
   * What one field holds, exactly as the sheet holds it: not trimmed, and there
   * whether or not there is anything in it.
   *
   * It is what Undo compares against, and it is why Undo does not ask the draft
   * above. A draft is what the Partner is told — its fields trimmed, its empty
   * ones left out — and a field differing from the suggestion only in the
   * whitespace around it would pass a trimmed comparison and have the human's
   * own spacing thrown away in the name of undoing ours (g1-s51 Built).
   */
  raw: (field: string) => string;
  /**
   * Put this text in that field, whole, and answer what the field held before.
   * The previous value is what Undo puts back, so it is read from the sheet at
   * the moment of the press rather than remembered from earlier.
   */
  set: (field: string, text: string) => string;
  /**
   * Put this text in that field and send the sheet, in one act, and answer what
   * the sending did — where the sheet has a submission path at all.
   *
   * It is the sheet's own, because only the sheet can build the draft it would
   * send. It takes the next draft explicitly rather than reading it back from a
   * render that has not happened yet, guards a save in flight itself, and
   * answers what it did rather than that it tried: the three things Astra's read
   * of g1-s52 named as why one press could not do both before (F1, F2). A sheet
   * that supplies none is offered Use this alone.
   */
  save?: (field: string, text: string) => Promise<Outcome>;
};

/**
 * What one submission of a sheet did, in the three answers that can be told
 * apart from this side of the wire.
 *
 * Saved is the ledger's own confirmation, and nothing else is. Refused is a no
 * with nothing behind it: this page's own validation, or a definite rejection
 * from the engine. Unresolved is everything that is neither — an answer that
 * does not say the act was refused, a transport that failed, a body nobody
 * could read — and it is the one the page must never send again by itself,
 * because the act may have landed.
 *
 * The route has no unresolved of its own: the act layer collapses every
 * publication error into a refusal, and one of its answers says the act landed
 * and must not be repeated (internal/ui/act/act.go settle). So the mapping from
 * what comes back to these three is the page's own, and it is conservative —
 * what is not confirmed and not definitely rejected is unresolved (Astra F1 on
 * g1-s56). It is written once, in src/backlog/editing.ts.
 */
export type Outcome =
  | { kind: "saved" }
  | { kind: "refused"; words: string }
  | { kind: "unresolved"; words: string };

/**
 * The two outcomes that carry words: everything a save can answer but yes.
 *
 * It is what the mapping from a failed request answers with, so that a caller
 * showing the words does not have to ask whether there are any.
 */
export type NotSaved = Extract<Outcome, { words: string }>;

/** The ledger confirmed it. */
export const SAVED: Outcome = { kind: "saved" };

/** A no, in the words of whoever said it, with nothing landed behind it. */
export function refusedSave(words: string): NotSaved {
  return { kind: "refused", words };
}

/** Neither a yes nor a no: the page rereads and never sends it again itself. */
export function unresolvedSave(words: string): NotSaved {
  return { kind: "unresolved", words };
}

/**
 * One opening's id, minted when a sheet mounts.
 *
 * It must be unique across page loads and not merely within one, and that is
 * the whole of why it is minted rather than counted. A conversation outlives the
 * page it was had on: the transcript is read back from the server on every load,
 * with its cards. A counter that started again at one would hand yesterday's
 * card the id of today's sheet, and pressing Use this on it would write into
 * whatever goal happens to be open now — which is exactly the harm a card
 * belonging to one opening exists to prevent.
 *
 * It is the turn key's own minting, because the two need the same thing: an
 * identifier this page did not have to coordinate with anybody to be sure of.
 */
export function mintOpening(): string {
  return `opening-${mintKey()}`;
}

/** What identifies one card: the turn it arrived in, and its place in it. */
export function idOf(turn: string, at: number): string {
  return `${turn}#${String(at)}`;
}

/**
 * The field in hand: which writable field last held the caret, and the opening it
 * belongs to.
 *
 * The opening travels with it because a field's name alone would follow the human
 * from one sheet to another: "Intent" is a field of the edit sheet and of the new
 * goal sheet, and a caret remembered from one is not a caret in the other.
 */
export type InHand = { opening: string; field: string };

/** Before any writable field has held the caret. */
export const NOWHERE: InHand = { opening: "", field: "" };

/** The field in hand for one opening, or "" where the caret is in another's. */
export function writingIn(writing: InHand, opening: string): string {
  return writing.opening === opening ? writing.field : "";
}

/**
 * Where a save asked for in the same press as Use this stands, or "" where no
 * save was asked for at all.
 *
 * It is kept on the card rather than in the block, because the card outlives the
 * block: a save closes the sheet, and "Used and saved" has to still be true in
 * the transcript afterwards (g1-s56 D2).
 */
export type Sent = "" | "saving" | "saved" | "refused" | "unresolved";

/** What the human has done with one suggestion. */
export type Mark = {
  /**
   * What the field held at the moment Use this was pressed, or null while it
   * has not been pressed. It is what Undo puts back.
   */
  previous: string | null;
  /** Whether the field still holds the suggestion's words. */
  holds: boolean;
  /** Folded to one line by Dismiss, and unfolded by pressing that line. */
  dismissed: boolean;
  /** Where a save asked for with the same press stands, or "" for none. */
  sent: Sent;
  /** The words that save answered with, for the two answers that carry them. */
  words: string;
};

/** Every card's mark, by the card's own id. A card with none is waiting. */
export type Marks = Readonly<Record<string, Mark>>;

const WAITING: Mark = { previous: null, holds: false, dismissed: false, sent: "", words: "" };

/** One turn's suggestions, as the transcript and the running turn carry them. */
export type Carried = { turn: string; suggestions: readonly Suggestion[] };

/**
 * Where a card stands.
 *
 * Seven standings and six cards: used and edited are one card saying two
 * different things, because what changed is not the card but whether Undo would
 * still be honest. Refused is the card that says nothing was offered at all, and
 * saved is the card whose words went into a field and into the ledger by one
 * press.
 */
export type Standing = "refused" | "saved" | "waiting" | "used" | "edited" | "dismissed" | "closed";

/** One card: the suggestion, what identifies it, and where it stands. */
export type Offered = Suggestion & {
  id: string;
  /** Whether the editor opening it belongs to is still on screen. */
  open: boolean;
  mark: Mark;
  standing: Standing;
};

/**
 * Where one card stands, from its mark, whether its opening is still open, and
 * whether it was offered at all.
 *
 * The order is the order a human would read them in. Refused first, and it wins
 * over everything: a suggestion the service did not offer is not something the
 * human dismissed, used or lost to a closing sheet — it never reached them, and
 * the only true thing to say about it is why. Saved next, and it outranks the
 * closing sheet on purpose: a save closes the sheet it saved, so a card that
 * became "the Edit goal sheet is closed" the instant its words landed would be
 * telling a human the one thing they least need to hear about the act they just
 * made (g1-s56 D2). Closed after that: a card whose editor has gone cannot
 * offer Use this or Undo, and offering either would be offering to write into
 * something that is not there — which is also why closed outranks dismissed
 * now, so that a card folded away before its sheet closed offers Copy rather
 * than staying folded over nothing (g1-s52 Built). Then dismissed, then used,
 * then waiting.
 */
export function standingOf(mark: Mark, open: boolean, offered = true): Standing {
  if (!offered) {
    return "refused";
  }
  if (mark.sent === "saved") {
    return "saved";
  }
  if (!open) {
    return "closed";
  }
  if (mark.dismissed) {
    return "dismissed";
  }
  if (mark.previous === null) {
    return "waiting";
  }
  return mark.holds ? "used" : "edited";
}

/** Every card this conversation carries, oldest first. */
export function offeredIn(carried: readonly Carried[], marks: Marks, open: readonly string[]): readonly Offered[] {
  const cards: Offered[] = [];
  for (const one of carried) {
    one.suggestions.forEach((suggestion, at) => {
      const id = idOf(one.turn, at);
      const mark = marks[id] ?? WAITING;
      const standing = open.includes(suggestion.opening);
      cards.push({
        ...suggestion,
        id,
        open: standing,
        mark,
        standing: standingOf(mark, standing, wasOffered(suggestion)),
      });
    });
  }
  return cards;
}

/**
 * Whether one suggestion was offered, read from its reason and not its flag.
 *
 * The service refuses nothing without saying why (internal/ui/partner/service.go
 * admit, notOffered), and it sets no reason on one it offers, so the reason is
 * what identifies a refusal. The flag alone could not: a suggestion recorded
 * before this build carries neither field, and reading the flag made every card
 * in an older conversation a refusal that said "Intent was not offered" of words
 * the human had been shown and may have used.
 */
export function wasOffered(card: Suggestion): boolean {
  return (card.reason ?? "").trim() === "";
}

/** One card by its id, or nothing where the conversation has no such card. */
export function cardIn(offered: readonly Offered[], id: string): Offered | undefined {
  return offered.find((card) => card.id === id);
}

/**
 * Every proposal standing under one field of one opening, newest first.
 *
 * It is what the block beside that field renders from, and it is why the drawer's
 * height stopped mattering: the words the Partner offered belong where the human
 * is writing, not in a column whose default height hides a card behind the
 * composer (g1-s52 §1, D2).
 *
 * Four standings belong here and three do not. Waiting is the offer; used and
 * edited are the same card still saying what happened and, while Undo is honest,
 * offering it; saved is the press that did both, and it says so here as well as
 * on the card, for as long as the sheet it saved is still on screen. Dismissed
 * was the human folding it away, and a block that kept showing it would be
 * refusing their act; closed cannot happen at all, because a closed opening is a
 * sheet that is not on screen to render this; and refused was never offered for
 * a field, so it belongs only where the answer is read.
 */
export function proposalsFor(
  offered: readonly Offered[],
  opening: string,
  field: string,
): readonly Offered[] {
  return offered
    .filter(
      (card) =>
        card.opening === opening &&
        card.field === field &&
        (card.standing === "waiting" ||
          card.standing === "used" ||
          card.standing === "edited" ||
          card.standing === "saved"),
    )
    .reverse();
}

/* --------------------------------------------------------- the four presses -- */

/**
 * The registration a press on this card would reach: the card's own opening, by
 * its id, and no other.
 *
 * This is where F1 is answered. A closed sheet has taken its registration back,
 * so a press reaches nothing; a sheet of the same name opened again over another
 * goal registered a different opening, so a press reaches nothing there either.
 * Neither is a special case in the store — they are the same absence.
 */
export function reach(
  openings: ReadonlyMap<string, Registered>,
  card: Offered | undefined,
): Registered | null {
  if (card === undefined) {
    return null;
  }
  return openings.get(card.opening) ?? null;
}

/**
 * Whether Use this can still be done, and it is the opening's absence that
 * usually answers no: a card whose sheet has been closed, or whose sheet's name
 * has been opened again as a new opening, has no setter to call. A card already
 * used is not used again; the human undoes it first.
 */
export function usable(card: Offered | undefined, openings: readonly string[]): boolean {
  if (card === undefined || card.mark.dismissed) {
    return false;
  }
  return card.mark.previous === null && openings.includes(card.opening);
}

/**
 * Whether Undo can still be done: it was used, the field still holds it, and no
 * save was asked for in the same press.
 *
 * The last clause is why one press that saves does not offer an undo. What Undo
 * puts back is a field's words, and a save has already carried the other words
 * past this page: an undo of the field alone would leave the sheet and the
 * ledger saying different things, and an undo of the act is not something this
 * interface has (g1-s56 D2).
 */
export function undoable(card: Offered | undefined, openings: readonly string[]): boolean {
  if (card === undefined || card.mark.dismissed || card.mark.sent !== "") {
    return false;
  }
  return card.mark.previous !== null && card.mark.holds && openings.includes(card.opening);
}

/** Used: the field held this before, and it holds the suggestion now. */
export function used(marks: Marks, id: string, previous: string): Marks {
  return { ...marks, [id]: { previous, holds: true, dismissed: false, sent: "", words: "" } };
}

/**
 * Used, with a save asked for in the same press: the words are in the field and
 * the sheet is sending them.
 *
 * It is written before the sending answers, because it is already true — the
 * field holds the Partner's words — and because what it withholds matters: while
 * a save is in flight the card offers no Undo, so nothing can put the human's
 * old words back into a field whose new ones are already on their way.
 */
export function saving(marks: Marks, id: string, previous: string): Marks {
  return { ...marks, [id]: { previous, holds: true, dismissed: false, sent: "saving", words: "" } };
}

/** What that save answered, on the card that asked for it. */
export function answered(marks: Marks, id: string, outcome: Outcome): Marks {
  const mark = marks[id] ?? WAITING;
  return {
    ...marks,
    [id]: { ...mark, sent: outcome.kind, words: outcome.kind === "saved" ? "" : outcome.words },
  };
}

/** Undone: the card waits again, because the field holds the human's words. */
export function undone(marks: Marks, id: string): Marks {
  return { ...marks, [id]: { ...WAITING, dismissed: marks[id]?.dismissed ?? false } };
}

/** Folded to one line, or unfolded by pressing that line. */
export function folded(marks: Marks, id: string, dismissed: boolean): Marks {
  return { ...marks, [id]: { ...(marks[id] ?? WAITING), dismissed } };
}

/**
 * A field says what it now holds.
 *
 * It is how the card learns that the human has typed over the words it offered.
 * Nothing else changes: a card that was not used has nothing to lose, and a
 * value that still matches leaves the marks exactly as they were, so a
 * keystroke that changes no answer re-renders nothing.
 */
export function holding(marks: Marks, offered: readonly Offered[], opening: string, field: string, value: string): Marks {
  let next = marks;
  for (const card of offered) {
    if (card.opening !== opening || card.field !== field || card.mark.previous === null) {
      continue;
    }
    const holds = value === card.text;
    if (holds === card.mark.holds) {
      continue;
    }
    next = { ...next, [card.id]: { ...card.mark, holds } };
  }
  return next;
}

/* ------------------------------------------------------------- the words -- */

/** What the card heads itself with: the field the words are for. */
export function cardHead(card: Suggestion): string {
  return `Suggestion for ${card.field}`;
}

/** What the block beside a field heads itself with. */
export const PROPOSES = "The Partner proposes";

/** What a card heads itself with when nothing was offered at all. */
export const NOT_OFFERED = "Not offered";

/** What a closed editor's card says instead of offering Use this. */
export function closedLine(card: Suggestion): string {
  return `The ${card.editor} sheet is closed`;
}

/**
 * Why nothing was offered, as the card says it: the service's own reason.
 *
 * The reason is what makes a card a refusal, so a refused card has one. The
 * plain sentence is the floor under a caller that asked for the line anyway,
 * so that a card cannot show an empty why.
 */
export function refusedLine(card: Suggestion & { reason?: string }): string {
  const said = (card.reason ?? "").trim();
  return said === "" ? `${card.field} was not offered` : said;
}

/** What stands under a proposal while older ones for that field are behind it. */
export function moreLabel(count: number): string {
  return `${String(count)} more`;
}

/** The same line, once they are unfolded: pressing it folds them away again. */
export const FEWER = "Fewer";

/** What the older proposals head themselves with, once they are unfolded. */
export const PROPOSED_EARLIER = "The Partner proposed earlier";

/** The second button under a proposal, where the sheet can send what it holds. */
export const USE_AND_SAVE = "Use and save";

/** What the card and the block say when one press did both, and it landed. */
export const USED_AND_SAVED = "Used and saved";

/** What they say in between: the words are in, and the sheet is sending them. */
export const SAVING = "Used · saving…";

/** A second press while one save is in flight, in the words it is answered in. */
export const IN_FLIGHT = "a save is in flight";

/**
 * The words went in and the save was refused: what the block says, with the
 * refusal's own sentence after it.
 *
 * The two halves are said separately because they are separately true. The
 * field holds the Partner's words — nobody is taking them back — and the ledger
 * holds nothing, for the reason the engine gave. Save at the foot of the sheet
 * is still there, under its normal validation, which is what a human does next.
 */
export function refusedSaveLine(words: string): string {
  return `Used; the save was refused: ${words}`;
}

/**
 * The words went in and nobody can say whether the save landed.
 *
 * It never says refused, because that would be a claim about the ledger this
 * page cannot make, and it never invites another press: the page has reread
 * what the ledger says and sends nothing again by itself (g1-s56 D1).
 */
export function unresolvedSaveLine(words: string): string {
  return `Used; the save was not confirmed: ${words}`;
}

/**
 * What the field's own link puts in the composer: a request for that field, in
 * the words a human would have typed.
 *
 * A field with something in it is asked to be bettered; an empty one is asked
 * for at all, because "a better Next step" is a strange thing to ask for a next
 * step nobody has written. It is put in the composer and not sent: the request
 * is still the human's to finish (g1-s52 D4).
 */
export function askLine(field: string, value: string): string {
  return value.trim() === "" ? `Suggest a ${field}` : `Suggest a better ${field}`;
}

/** The field's own link, which is one name in one place. */
export const ASK_THE_PARTNER = "Ask the Partner";

/** Why Undo is gone: the human typed, and what they typed stands. */
export const EDITED_SINCE = "edited since; your words stay";

/** The folded card, in one word that presses to unfold it. */
export const DISMISSED = "dismissed";
