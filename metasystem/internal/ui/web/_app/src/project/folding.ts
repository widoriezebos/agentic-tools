/**
 * The section card's steps (g1-s66 D3), as a value the page holds and these
 * functions move: comparing, writing, the accepted row, done, or refused.
 *
 * The writer checks the document's revision and not the section, so a Use is
 * always made against the revision the comparison was read at, and a conflict
 * is answered by reading the design again and comparing against the words as
 * they are now: Use is offered again only on that comparison. A heading that is
 * absent or doubled is refused in words and the draft is kept. A Use that
 * wrote the section and not its accepted row is offered the row again, because
 * the design and the decisions file are two writes and only the first landed.
 */

import { cardsOf, foldAskedIn, rowFor, type DesignLoop, type DesignRow, type FoldAsked } from "./critiquing";
import { changedLines, draftRefusal, refusalFor, sectionIn, sectionReplaced, type Compared } from "./sections";

export type Fold = {
  heading: string;
  /** The Partner's words for the section, kept whatever happens to a Use. */
  text: string;
  /** The design as the comparison reads it, and the revision it was read at. */
  source: string;
  revision: string;
  phase: "comparing" | "writing" | "row" | "done" | "refused";
  said: string;
};

export const CONFLICT_SAID =
  "The design changed since it was read. The comparison is now against the design as it stands; read it again before Use.";

export function foldOpened(heading: string, text: string, source: string, revision: string): Fold {
  return { heading, text, source, revision, phase: "comparing", said: "" };
}

/** Old and new side by side, or why the heading cannot be told apart or the draft cannot be written. */
export function comparisonOf(fold: Fold): { state: "compared"; old: Compared[]; new: Compared[] } | { state: "refused"; said: string } {
  const found = sectionIn(fold.source, fold.heading);
  if (found.state !== "found") {
    return { state: "refused", said: refusalFor(fold.heading, found) };
  }
  const refused = draftRefusal(fold.text);
  if (refused !== "") {
    return { state: "refused", said: refused };
  }
  return { state: "compared", ...changedLines(found.text.replace(/\s+$/, ""), fold.text.replace(/\s+$/, "")) };
}

/** Use: the whole source with exactly the section replaced, or the refusal. */
export function foldUse(fold: Fold): { fold: Fold; save?: { source: string; revision: string } } {
  const replaced = sectionReplaced(fold.source, fold.heading, fold.text);
  if (replaced.state === "refused") {
    return { fold: { ...fold, phase: "refused", said: replaced.said } };
  }
  return { fold: { ...fold, phase: "writing", said: "" }, save: { source: replaced.source, revision: fold.revision } };
}

/** The write was refused for the revision: read again, compare again. */
export function foldConflicted(fold: Fold, source: string, revision: string): Fold {
  return { ...fold, source, revision, phase: "comparing", said: CONFLICT_SAID };
}

/** The section is written; a fold for a finding still owes its row. */
export function foldWritten(fold: Fold, owesRow: boolean): Fold {
  return { ...fold, phase: owesRow ? "row" : "done", said: "" };
}

export function foldRowFailed(fold: Fold, said: string): Fold {
  return { ...fold, phase: "row", said: `The section is written; its decision is not: ${said}. Write the decision again.` };
}

export function foldRowWritten(fold: Fold): Fold {
  return { ...fold, phase: "done", said: "" };
}

/** Whether the design already carries the section as drafted. */
export function inTheDesign(source: string, heading: string, text: string): boolean {
  const found = sectionIn(source, heading);
  return found.state === "found" && found.text.trim() === text.trim();
}

/**
 * A section the Partner drafted for this design, bound to the fold whose
 * request it answers where there is one: the request is the human's message of
 * the same turn, and it names the chain, the round, the finding and the
 * amendment. A suggestion asked by no fold owes no row.
 */
export type Offer = { id: string; heading: string; text: string; asked: FoldAsked | undefined };

type Suggested = { id: string; document?: string; section?: string; text: string; offered: boolean; mark: { dismissed: boolean } };
type Said = { turn: string; role: string; text: string };

/**
 * The sections drafted for this design, the newest per finding: two findings
 * folded into one section stand beside each other, and a later draft for the
 * same finding replaces its earlier one. One asked by no fold is the newest
 * for its heading among those.
 */
export function offersFor(design: string, offered: readonly Suggested[], messages: readonly Said[]): Offer[] {
  const asked = new Map<string, FoldAsked>();
  for (const message of messages) {
    const fold = message.role === "human" ? foldAskedIn(message.text) : null;
    if (fold !== null && fold.design === design) {
      asked.set(message.turn, fold);
    }
  }
  const latest = new Map<string, Offer>();
  for (const card of offered) {
    const heading = card.section ?? "";
    if (card.document !== design || !card.offered || card.mark.dismissed || heading === "") {
      continue;
    }
    const fold = asked.get(card.id.slice(0, card.id.lastIndexOf("#")));
    const bound = fold !== undefined && fold.heading === heading ? fold : undefined;
    const key = bound === undefined ? `heading:${heading}` : `finding:${bound.chain}:${String(bound.round)}:${bound.finding}`;
    latest.set(key, { id: card.id, heading, text: card.text, asked: bound });
  }
  return [...latest.values()];
}

/**
 * The accepted row a Use owes, for the one finding the draft was asked for, on
 * the chain the page reads, while the decisions file has no row for it.
 */
export function foldRow(loop: DesignLoop | null, asked: FoldAsked | undefined): { round: number; row: DesignRow } | undefined {
  if (loop === null || asked === undefined || asked.chain !== loop.chain) {
    return undefined;
  }
  const round = loop.rounds.find((one) => one.round === asked.round);
  const card = round === undefined ? undefined : cardsOf(round).find((one) => one.finding.id === asked.finding);
  if (card === undefined || card.row !== undefined) {
    return undefined;
  }
  return { round: card.round, row: rowFor("fold", card.finding, { reasoning: "", amendment: asked.amendment }) };
}

/**
 * The section card as the page opens it: comparing, or, where the design
 * already carries the draft and the decisions file has no row for its
 * finding, the row offered again, since a Use landed and its row did not.
 * That is read from the design and the file, so it holds after a reload.
 */
export function foldAt(offer: Offer, loop: DesignLoop | null, source: string, revision: string): Fold {
  const opened = foldOpened(offer.heading, offer.text, source, revision);
  return foldRow(loop, offer.asked) !== undefined && inTheDesign(source, offer.heading, offer.text) ? { ...opened, phase: "row" } : opened;
}
