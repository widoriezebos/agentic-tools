/**
 * What a goal is called where a page names it by a line of its intent.
 *
 * Most goal records open their intent with the label "What:". It names the
 * field for whoever writes the record and says nothing to whoever reads a
 * card, so a line that names a goal starts with the words after it. The
 * server cuts the titles it sends by the same rule; this is the owner for the
 * lines a page makes from an intent itself. How much of the line shows stays
 * with each place, and the goal's own page shows the intent as it was written.
 */

const LABEL = "what:";

export function goalTitle(intent: string): string {
  const trimmed = intent.trim();
  return trimmed.slice(0, LABEL.length).toLowerCase() === LABEL ? trimmed.slice(LABEL.length).trim() : trimmed;
}
