import { StickyNote } from "lucide-react";
import { useEffect, useRef } from "react";

import { useStickies } from "./store";
import { IconButton } from "../shell/controls";

/**
 * The notepad's button, in the header's right cluster beside the bell.
 *
 * It is one control and one number. The number is how many stickies are open —
 * not how many are new, which is the bell's question: a notepad has nothing
 * arriving on it, and what a human wants to know at a glance is how much they
 * still have to do.
 *
 * It sits beside the bell because the two are the same kind of thing in the
 * same place: a small standing count a human reads without stopping. The bell
 * comes first, because it is the one that changes on its own.
 *
 * The badge carries its own name, so what a screen reader hears is "3 open"
 * rather than the digit three, and the button's own name stays "Stickies"
 * whether anything is open or not.
 */

/** Past this the exact number stops mattering and the badge stops growing. */
const TOO_MANY = 99;

export function StickiesButton() {
  const { notepad, panelIsOpen, openPanel } = useStickies();
  const holder = useRef<HTMLSpanElement>(null);
  const wasOpen = useRef(false);

  // Closing the panel puts the keyboard back where it came from, exactly as
  // the bell's does: the sheet's own restore leaves focus on the document
  // body, because the element that had it went with the sheet.
  useEffect(() => {
    if (wasOpen.current && !panelIsOpen) {
      holder.current?.querySelector("button")?.focus();
    }
    wasOpen.current = panelIsOpen;
  }, [panelIsOpen]);

  const open = notepad.counts.open;

  return (
    <span className="ms-stickies-button" ref={holder}>
      <IconButton
        label="Stickies"
        aria-expanded={panelIsOpen}
        aria-controls="stickies-panel"
        onClick={() => {
          openPanel();
        }}
      >
        <StickyNote size={16} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
      {open > 0 && (
        <span className="ms-stickies-count" role="status" aria-label={`${String(open)} open`}>
          {open > TOO_MANY ? `${String(TOO_MANY)}+` : open}
        </span>
      )}
    </span>
  );
}
