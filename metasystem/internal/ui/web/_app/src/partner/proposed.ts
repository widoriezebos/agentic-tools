import { verbWord, type Card, type Line } from "./proposing";

/**
 * What a goal's own row says about the acts the Partner proposed on it.
 *
 * A proposal is made in the conversation and answered there, and until g1-s61
 * the only place it appeared outside the drawer was the Decisions inbox. The
 * human's day is not spent in either: it is spent on the board, triaging, and a
 * goal whose Partner said something about it last night looked exactly like a
 * goal it had never seen. So the subject's own row says so, before the row is
 * opened.
 *
 * Everything in this file is a function of what the conversation already holds.
 * There is no reader, no payload field and no request: the store folds the
 * snapshot and the stream's beats, and the three rows read the same cards the
 * card in the drawer reads. Which is why the chip leaves the moment a line is
 * applied or dismissed, on every page at once, without anybody re-reading
 * anything (g1-s61 D1).
 *
 * The four states here are the four the inbox's own reader keeps
 * (internal/ui/partner/proposals.go, `Unsettled`): waiting, applying, refused
 * and unresolved. Every one of them is a choice still on the human — the first
 * two to answer or to recover, the last two to try again or to put away — and a
 * line that was applied or dismissed is a record rather than a choice, so it
 * says nothing on a row.
 */

/** One line still waiting on the human, with the card in the transcript it is on. */
export type Proposed = Line & {
  /** The answer's card, which is what a press opens the drawer at. */
  card: string;
};

/**
 * Every act the Partner proposed on one goal that still waits on this human,
 * newest answer first.
 *
 * Newest first because that is the order the press needs: the chip opens the
 * drawer at the newest such line's card, and a human who has been proposed two
 * things about one goal wants the one that was said last. Inside one answer the
 * lines keep the order they were proposed in, as the card shows them.
 *
 * A line the service never offered is not here. It is on the card with the
 * reason it was refused admission, and it is not something the human can answer:
 * the inbox leaves it out for the same reason (g1-s60, as built).
 */
export function proposedFor(cards: readonly Card[], goal: string): readonly Proposed[] {
  const found: Proposed[] = [];
  if (goal === "") {
    return found;
  }
  for (let at = cards.length - 1; at >= 0; at -= 1) {
    for (const line of cards[at].lines) {
      if (line.goal === goal && waitsOnTheHuman(line)) {
        found.push({ ...line, card: cards[at].id });
      }
    }
  }
  return found;
}

/**
 * True for a line that is still a choice rather than a record.
 *
 * Not `settledState`: that one answers what nothing more will happen to by
 * itself, and a refused line is exactly that — nothing more will happen to it —
 * while still being a choice the human has to make. A refusal is a no with the
 * engine's own sentence behind it, and Try again and Dismiss are both live.
 */
export function waitsOnTheHuman(line: Line): boolean {
  return line.offered && line.state !== "applied" && line.state !== "dismissed";
}

/* --------------------------------------------------------------- the words -- */

/** What the chip says, and whether it says it in the danger colour. */
export type ChipWords = { words: string; danger: boolean };

export const PROPOSED = "proposed";
export const REFUSED = "refused";
export const UNRESOLVED = "unresolved";

/**
 * The chip's words for these lines, or null where there are none and the row
 * shows no chip at all.
 *
 * One line is named by the act: the word the button on the page that offers that
 * act uses, so the row, the card and the page say one thing (g1-s58 D4). Several
 * are a count, because four verbs on one row is a row nobody reads and the card
 * is one press away.
 *
 * A refused or an unresolved line is in the danger colour, because those wait on
 * the human too and what they wait for is not a yes: one is a no with a reason,
 * and the other is an act whose outcome nobody knows. A count over lines that
 * include one of those carries the colour as well — the alarm belongs to the row
 * whether or not the alarming line happens to be the newest.
 */
export function chipWords(lines: readonly Proposed[]): ChipWords | null {
  if (lines.length === 0) {
    return null;
  }
  const danger = lines.some(alarming);
  if (lines.length > 1) {
    return { words: `${String(lines.length)} ${PROPOSED}`, danger };
  }
  const line = lines[0];
  if (line.state === "refused") {
    return { words: `${verbWord(line.verb)} ${REFUSED}`, danger: true };
  }
  if (line.state === "unresolved") {
    return { words: UNRESOLVED, danger: true };
  }
  return { words: `${verbWord(line.verb)} ${PROPOSED}`, danger };
}

/** True for a line whose state is the row's alarm rather than its offer. */
function alarming(line: Proposed): boolean {
  return line.state === "refused" || line.state === "unresolved";
}

/**
 * The chip's accessible name: what it says, the goal it is about, and how many
 * actions are behind it.
 *
 * The visible words are short because they stand in a row of chips; the name is
 * the whole sentence, because a human reading the page through a screen reader
 * hears the chip out of the row it is in and "2 proposed" alone names nothing.
 */
export function chipName(goal: string, lines: readonly Proposed[]): string {
  const said = chipWords(lines);
  if (said === null) {
    return "";
  }
  const many = lines.length;
  return `${said.words} on ${goal}, ${String(many)} action${many === 1 ? "" : "s"}`;
}
