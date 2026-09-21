import type { Heading } from "./api";

/**
 * The contents of one document.
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
