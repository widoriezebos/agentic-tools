import { useEffect, useState } from "react";

import type { Heading } from "./api";

/**
 * The contents of a page, and which of its rows the reader is in.
 *
 * For a document that is its headings: headings past the third level are
 * detail rather than structure, and a document with one or two headings has no
 * shape worth a second column, so it gets no outline at all rather than a list
 * with a single row in it. For the briefing it is the sections the page
 * renders. Both are anchors on the page being read, so both follow the reader
 * through the one hook below.
 */

export const DEEPEST_LEVEL = 3;
export const FEWEST_ROWS = 3;

/** Anything the reader can be inside: a heading, or a section of a page. */
export type Anchor = { id: string };

export type OutlineRow = Anchor & { level: number; text: string; indent: number };

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
  rows: readonly Anchor[],
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

/**
 * Which row the reader is in, observed rather than timed.
 *
 * An IntersectionObserver reports each anchor as it enters and leaves, and the
 * row is decided from the set on screen, in the page's own order. There is no
 * timer and no scroll handler: the browser tells this hook when something
 * changed, and nothing else wakes it. The key is what the rows belong to — a
 * document's id, a goal's — so that opening another one starts again rather
 * than keeping the mark the last one left.
 */
export function useReadingRow(rows: readonly Anchor[], key: string): string | null {
  const [current, setCurrent] = useState<string | null>(null);

  useEffect(() => {
    setCurrent(null);
    if (rows.length === 0 || typeof IntersectionObserver !== "function") {
      return;
    }
    const visible = new Set<string>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            visible.add(entry.target.id);
          } else {
            visible.delete(entry.target.id);
          }
        }
        setCurrent((previous) => currentRow(rows, visible, previous));
      },
      // The bottom margin keeps the mark on the section being read rather than
      // on whichever anchor happens to be at the foot of the window.
      { rootMargin: "0px 0px -60% 0px" },
    );
    for (const row of rows) {
      const anchor = globalThis.document.getElementById(row.id);
      if (anchor !== null) {
        observer.observe(anchor);
      }
    }
    return () => {
      observer.disconnect();
    };
  }, [rows, key]);

  return current;
}
