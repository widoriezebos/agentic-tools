import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from "react";

import "./tabs.css";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";

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

/**
 * One tab: what it is called, the section it opens, and the term beside it.
 *
 * A tab's name is one of the words this project uses in its own way —
 * Doctrine, Slices — so the strip carries the explanation of each. The help is
 * a button and cannot nest inside the tab's own button, so the two stand side
 * by side in a wrapper the strip's semantics see straight through.
 */
export type Tab = { id: string; title: string; panel: ReactNode; help?: HelpId };

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
  action,
  panelClassName,
}: {
  /** What the strip is, for anyone who cannot see that it is a strip. */
  label: string;
  tabs: readonly Tab[];
  selected: string;
  onSelect: (id: string) => void;
  /**
   * The one act the open tab offers, at the trailing end of the strip.
   *
   * It stands outside the tablist, which is what it is: a tab chooses what is
   * read and this writes something new, and a control inside the list would be
   * a tab the arrows landed on that opened no section. Outside it, it is one
   * ordinary tab stop after the strip's own.
   */
  action?: ReactNode;
  /** What the page calls its reading column, where the panel is that column. */
  panelClassName?: string;
}) {
  const named = useId();
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const list = useRef<HTMLDivElement | null>(null);

  const at = tabs.findIndex((tab) => tab.id === selected);
  const openID = tabs.length === 0 ? "" : tabs[at < 0 ? 0 : at].id;

  // A strip narrower than its tabs scrolls sideways, and the tab that is open
  // is the one that has to be on the screen: it arrives from an address
  // somebody was sent, or after the arrows moved it, and neither should leave
  // it over the edge. Only the strip's own scroll moves; the page does not.
  useEffect(() => {
    const strip = list.current;
    const button = buttons.current.get(openID);
    if (strip === null || button === undefined) {
      return;
    }
    const left = button.offsetLeft;
    const right = left + button.offsetWidth;
    if (left < strip.scrollLeft) {
      strip.scrollLeft = left;
    } else if (right > strip.scrollLeft + strip.clientWidth) {
      strip.scrollLeft = right - strip.clientWidth;
    }
  }, [openID]);

  if (tabs.length === 0) {
    return null;
  }

  const open = tabs[at < 0 ? 0 : at];
  const tabId = (id: string) => `${named}-tab-${id}`;
  const panelId = (id: string) => `${named}-panel-${id}`;

  // The strip is one tab stop, so the arrows have to carry the focus as well
  // as the selection: the tab that arrives is the tab that is open, which is
  // what makes a strip of six sections one press wide rather than six.
  const move = (event: KeyboardEvent<HTMLDivElement>) => {
    // The arrows belong to the tabs. The help icons stand in the strip too,
    // and an arrow pressed while one of them has the caret must not change
    // which section is open.
    if (!(event.target instanceof Element) || event.target.getAttribute("role") !== "tab") {
      return;
    }
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
        <div className="ms-tabs-list" role="tablist" aria-label={label} ref={list} onKeyDown={move}>
          {/* The wrapper is presentation and nothing else: the strip still
              owns tabs, and the help beside each one is a button that could
              not have been nested inside it. Only the open tab's help is in
              the Tab order, so the strip costs the keyboard one extra stop
              rather than one per section. */}
          {tabs.map((tab) => (
            <span key={tab.id} role="presentation" className="ms-tab-slot">
              <button
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
              {tab.help !== undefined && <Help id={tab.help} tabIndex={tab.id === open.id ? 0 : -1} />}
            </span>
          ))}
        </div>
        {action !== undefined && action !== null && <div className="ms-tabs-action">{action}</div>}
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
