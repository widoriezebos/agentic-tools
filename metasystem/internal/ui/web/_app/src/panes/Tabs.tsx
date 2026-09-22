import { useId, useRef, type KeyboardEvent, type ReactNode } from "react";

import "./tabs.css";

/**
 * Tabs: one section of a page, and only one, at a time.
 *
 * The strip under the header used to be an index into a page that carried
 * every section at once, and an anchor that moved the scroll is not a tab: it
 * marked where a reader had got to rather than choosing what they were
 * reading. These are tabs. Selecting one shows that section and hides the
 * others, nothing scrolls, and the strip says which one is open.
 *
 * The semantics are the ARIA pattern's, because a strip that looks like tabs
 * and announces itself as a list of links is a strip that lies to anyone not
 * looking at it: one tablist, a tab per section carrying what it controls,
 * and one panel named by the tab that opened it. Focus roves — one tab stop
 * for the whole strip, and Left, Right, Home and End move within it — so the
 * keyboard reaches the strip in one press and leaves it in one more.
 *
 * Which tab is open is the page's, not this component's: it comes from the
 * address, so a tab can be linked and reloaded, and this component is told
 * what to show and says when a human asked for another.
 */

/** One tab: what it is called, and the section it opens. */
export type Tab = { id: string; title: string; panel: ReactNode };

/**
 * Where a key takes the focus, as an index into the strip, or null for a key
 * the strip does not answer to.
 *
 * The two arrows wrap, which is the pattern's own rule: a strip is a ring, and
 * a human holding an arrow key at the end of it expects the other end rather
 * than a dead press.
 */
export function tabAfter(key: string, at: number, count: number): number | null {
  if (count === 0) {
    return null;
  }
  switch (key) {
    case "ArrowLeft":
      return (at - 1 + count) % count;
    case "ArrowRight":
      return (at + 1) % count;
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
}

/**
 * Which tab a page opens on: what the address names, then what the browser
 * remembers, then the first.
 *
 * The address wins, because a link to a tab is a link to that tab and a
 * preference from last week must not outrank what a human just opened. A name
 * the page has no tab for is answered with the first tab rather than with the
 * remembered one: the address asked for something this page does not have, and
 * falling through to a remembered tab would answer a wrong address with a
 * surprise. An address naming no tab at all has asked for nothing, and that is
 * where the remembered tab is the right answer.
 */
export function tabShown(
  tabs: readonly { id: string }[],
  named: string | undefined,
  remembered: string | null,
): string {
  if (tabs.length === 0) {
    return "";
  }
  const first = tabs[0].id;
  const carried = (candidate: string) => tabs.some((tab) => tab.id === candidate);
  if (named !== undefined && named !== "") {
    return carried(named) ? named : first;
  }
  return remembered !== null && carried(remembered) ? remembered : first;
}

/**
 * The strip and the open panel, as two siblings rather than one block, so a
 * page can run the strip across its whole width and keep the panel in the
 * column it reads in. A page that wants them stacked gets that by placing
 * nothing between them.
 */
export function Tabs({
  label,
  tabs,
  selected,
  onSelect,
  panelClassName,
}: {
  /** What the strip is, for anyone who cannot see that it is a strip. */
  label: string;
  tabs: readonly Tab[];
  selected: string;
  onSelect: (id: string) => void;
  /** What the page calls its reading column, where the panel is that column. */
  panelClassName?: string;
}) {
  const named = useId();
  const buttons = useRef(new Map<string, HTMLButtonElement>());

  if (tabs.length === 0) {
    return null;
  }

  const at = tabs.findIndex((tab) => tab.id === selected);
  const open = tabs[at < 0 ? 0 : at];
  const tabId = (id: string) => `${named}-tab-${id}`;
  const panelId = (id: string) => `${named}-panel-${id}`;

  // The strip is one tab stop, so the arrows have to carry the focus as well
  // as the selection: the tab that arrives is the tab that is open, which is
  // what makes a strip of six sections one press wide rather than six.
  const move = (event: KeyboardEvent<HTMLDivElement>) => {
    const next = tabAfter(event.key, at < 0 ? 0 : at, tabs.length);
    if (next === null) {
      return;
    }
    event.preventDefault();
    const moved = tabs[next];
    onSelect(moved.id);
    buttons.current.get(moved.id)?.focus();
  };

  return (
    <>
      <div className="ms-tabs-strip">
        <div className="ms-tabs-list" role="tablist" aria-label={label} onKeyDown={move}>
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={tabId(tab.id)}
              className="ms-tab"
              aria-selected={tab.id === open.id}
              aria-controls={panelId(tab.id)}
              tabIndex={tab.id === open.id ? 0 : -1}
              ref={(element) => {
                if (element !== null) {
                  buttons.current.set(tab.id, element);
                }
                return () => {
                  buttons.current.delete(tab.id);
                };
              }}
              onClick={() => {
                onSelect(tab.id);
              }}
            >
              {tab.title}
            </button>
          ))}
        </div>
      </div>
      {/* The panel is a tab stop of its own: a section whose rows are all
          links would be reachable without it, and one that is a paragraph
          would not, and which of the two a page has is not this component's
          to know. */}
      <div
        className={panelClassName === undefined ? "ms-tabs-panel" : `ms-tabs-panel ${panelClassName}`}
        role="tabpanel"
        id={panelId(open.id)}
        aria-labelledby={tabId(open.id)}
        tabIndex={0}
      >
        {open.panel}
      </div>
    </>
  );
}
