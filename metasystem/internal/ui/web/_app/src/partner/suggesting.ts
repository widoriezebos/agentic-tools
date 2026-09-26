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
 * card, which of its five standings it is in, whether Use this can still be
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
   * Put this text in that field, whole, and answer what the field held before.
   * The previous value is what Undo puts back, so it is read from the sheet at
   * the moment of the press rather than remembered from earlier.
   */
  set: (field: string, text: string) => string;
};

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
};

/** Every card's mark, by the card's own id. A card with none is waiting. */
export type Marks = Readonly<Record<string, Mark>>;

const WAITING: Mark = { previous: null, holds: false, dismissed: false };

/** One turn's suggestions, as the transcript and the running turn carry them. */
export type Carried = { turn: string; suggestions: readonly Suggestion[] };

/**
 * Where a card stands.
 *
 * Five standings and four cards: used and edited are one card saying two
 * different things, because what changed is not the card but whether Undo would
 * still be honest.
 */
export type Standing = "waiting" | "used" | "edited" | "dismissed" | "closed";

/** One card: the suggestion, what identifies it, and where it stands. */
export type Offered = Suggestion & {
  id: string;
  /** Whether the editor opening it belongs to is still on screen. */
  open: boolean;
  mark: Mark;
  standing: Standing;
};

/**
 * Where one card stands, from its mark and whether its opening is still open.
 *
 * The order is the order a human would read them in. Dismissed first: folding
 * the card was their own act, and an act of theirs is not undone by a sheet
 * closing. Closed next: a card whose editor has gone cannot offer Use this or
 * Undo, whatever was done with it, and offering either would be offering to
 * write into something that is not there. Then used, and then waiting.
 */
export function standingOf(mark: Mark, open: boolean): Standing {
  if (mark.dismissed) {
    return "dismissed";
  }
  if (!open) {
    return "closed";
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
      cards.push({ ...suggestion, id, open: standing, mark, standing: standingOf(mark, standing) });
    });
  }
  return cards;
}

/** One card by its id, or nothing where the conversation has no such card. */
export function cardIn(offered: readonly Offered[], id: string): Offered | undefined {
  return offered.find((card) => card.id === id);
}

/**
 * How many suggestions are waiting for one field of one opening, which is what
 * the field says beside its label. A card that was used, dismissed or left
 * behind by a closed sheet is not waiting for anything.
 */
export function waitingFor(offered: readonly Offered[], opening: string, field: string): number {
  return watching(offered, opening, field).length;
}

/**
 * The newest card waiting for that field, which is the one the field's link
 * opens the drawer at, or "" where none is.
 */
export function newestFor(offered: readonly Offered[], opening: string, field: string): string {
  return watching(offered, opening, field).at(-1)?.id ?? "";
}

function watching(offered: readonly Offered[], opening: string, field: string): readonly Offered[] {
  return offered.filter(
    (card) => card.standing === "waiting" && card.opening === opening && card.field === field,
  );
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

/** Whether Undo can still be done: it was used, and the field still holds it. */
export function undoable(card: Offered | undefined, openings: readonly string[]): boolean {
  if (card === undefined || card.mark.dismissed) {
    return false;
  }
  return card.mark.previous !== null && card.mark.holds && openings.includes(card.opening);
}

/** Used: the field held this before, and it holds the suggestion now. */
export function used(marks: Marks, id: string, previous: string): Marks {
  return { ...marks, [id]: { previous, holds: true, dismissed: false } };
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

/** What a closed editor's card says instead of offering Use this. */
export function closedLine(card: Suggestion): string {
  return `The ${card.editor} sheet is closed`;
}

/** What the field says beside its label while suggestions wait for it. */
export function waitingLabel(count: number): string {
  return count === 1 ? "1 suggestion" : `${String(count)} suggestions`;
}

/** Why Undo is gone: the human typed, and what they typed stands. */
export const EDITED_SINCE = "edited since; your words stay";

/** The folded card, in one word that presses to unfold it. */
export const DISMISSED = "dismissed";
