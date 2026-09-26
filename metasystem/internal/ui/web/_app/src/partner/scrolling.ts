/**
 * Moving the conversation, and moving nothing else.
 *
 * The drawer's column sits inside a page that scrolls, inside a work area that
 * scrolls. `scrollIntoView` would move every one of those ancestors and carry
 * the work the human is looking at off the screen with it, so nothing here uses
 * it: the one scroller the conversation lives in is found, and its own
 * scrollTop is what moves.
 *
 * It is a file of its own because two things move the conversation now — the
 * transcript following the newest words down, and a field's link opening the
 * drawer at the card that belongs to it — and one of them must not have its own
 * idea of which box scrolls.
 */

/**
 * How near the end counts as being at it. A line of prose is under this, so a
 * human who has read to the bottom is followed down rather than offered a pill
 * for the pixel they are short of.
 */
export const AT_END = 24;

/** How much room is left above a card the drawer was opened at. */
const ABOVE = 12;

/** The nearest ancestor that scrolls, or null where nothing does. */
export function scrollerOf(from: Element | null): HTMLElement | null {
  let at = from?.parentElement ?? null;
  while (at !== null) {
    const overflow = globalThis.getComputedStyle(at).overflowY;
    if (overflow === "auto" || overflow === "scroll") {
      return at;
    }
    at = at.parentElement;
  }
  return null;
}

/** True while the scroller is showing the end of what is in it. */
export function atEnd(scroller: HTMLElement): boolean {
  return scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <= AT_END;
}

/**
 * Bring one element of the conversation to the top of the column it is in, and
 * move nothing outside that column. A page with no scroller — a test, or a
 * drawer that has not been laid out — moves nothing at all.
 */
export function bringUp(element: Element | null): void {
  const scroller = scrollerOf(element);
  if (scroller === null || element === null) {
    return;
  }
  scroller.scrollTop += element.getBoundingClientRect().top - scroller.getBoundingClientRect().top - ABOVE;
}
