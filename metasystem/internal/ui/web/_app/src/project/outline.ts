import type { Heading } from "./api";

/**
 * The contents of one document, and which of its rows the reader is in.
 *
 * Headings past the third level are detail rather than structure, and a
 * document with one or two headings has no shape worth a second column, so it
 * gets no outline at all rather than a list with a single row in it.
 */

export const DEEPEST_LEVEL = 3;
export const FEWEST_ROWS = 3;

export type OutlineRow = { level: number; id: string; text: string; indent: number };

export function outlineOf(headings: Heading[]): OutlineRow[] {
  const shown = headings.filter((heading) => heading.level <= DEEPEST_LEVEL && heading.text !== "");
  if (shown.length < FEWEST_ROWS) {
    return [];
  }
  const shallowest = Math.min(...shown.map((heading) => heading.level));
  return shown.map((heading) => ({
    level: heading.level,
    id: heading.id,
    text: heading.text,
    indent: heading.level - shallowest,
  }));
}

/**
 * The row the reader is in: the first heading on screen, in the document's own
 * order rather than the order an observer reported them in.
 *
 * A section longer than the window has no heading on screen at all, and the
 * mark stays where it was rather than clearing: the reader is still in that
 * section, and a mark that blinked out in the middle of one would be saying
 * something untrue about where they are.
 */
export function currentRow(
  rows: OutlineRow[],
  visible: ReadonlySet<string>,
  previous: string | null,
): string | null {
  for (const row of rows) {
    if (visible.has(row.id)) {
      return row.id;
    }
  }
  return previous !== null && rows.some((row) => row.id === previous) ? previous : null;
}
