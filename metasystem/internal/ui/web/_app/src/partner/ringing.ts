/**
 * "Show me" is a hover.
 *
 * A goal link in an answer marks the one card on the board that is that goal,
 * for as long as the pointer or the caret is on it, and marks nothing else.
 * Six mentioned goals mean six links and no rings until one is hovered.
 *
 * It is written on the element rather than carried through a store, because
 * the drawer and the board are two trees over one page and the mark is a
 * hover: a state that lived in a provider would re-render the whole board
 * twice for every link a pointer crosses. The board already names its cards —
 * every card and every list row carries the goal it is — so this finds the one
 * it means and puts a class on it.
 *
 * The class is not the landing's own, deliberately. The landing's ring fades
 * by an animation whose end takes it off again; a hover has to stay up for as
 * long as the hover does, so it is the same ring without the fade.
 */

/** What the marked card wears. The rule is in backlog.css with the rest. */
export const RINGED = "ms-goal-ringed";

/** The attribute every card and row carries, naming the goal it shows. */
export const GOAL_ATTRIBUTE = "data-goal";

/** Mark the one visible card for this goal, and answer the way to unmark it. */
export function ring(goal: string): () => void {
  const marked = cardsFor(goal);
  for (const card of marked) {
    card.classList.add(RINGED);
  }
  return () => {
    for (const card of marked) {
      card.classList.remove(RINGED);
    }
  };
}

/** Every element on this page showing that goal, which is one or none. */
function cardsFor(goal: string): Element[] {
  if (goal === "") {
    return [];
  }
  return [...globalThis.document.querySelectorAll(`[${GOAL_ATTRIBUTE}="${cssValue(goal)}"]`)];
}

/**
 * A goal id inside an attribute selector. Ledger ids are letters, digits and
 * dashes, but the selector is built from a value this page did not mint, so
 * the two characters that would end the string early are refused rather than
 * escaped: a goal named with one of them rings nothing, and the link still
 * navigates.
 */
function cssValue(goal: string): string {
  return /^[A-Za-z0-9_\-.]+$/.test(goal) ? goal : "";
}
