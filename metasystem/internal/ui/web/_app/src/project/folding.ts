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

import { changedLines, refusalFor, sectionIn, sectionReplaced, type Compared } from "./sections";

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

/** Old and new side by side, or why the heading cannot be told apart. */
export function comparisonOf(fold: Fold): { state: "compared"; old: Compared[]; new: Compared[] } | { state: "refused"; said: string } {
  const found = sectionIn(fold.source, fold.heading);
  if (found.state !== "found") {
    return { state: "refused", said: refusalFor(fold.heading, found) };
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
