import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { CircleHelp } from "lucide-react";
import { useRef, useState } from "react";

import "./help.css";
import { HELP, type HelpId } from "./terms";

/**
 * The help control: a question mark beside a name, and what that name is for.
 *
 * It is the universal circled question mark, because that is the mark every
 * interface uses for "explain this" and an interface that invents its own
 * teaches it to nobody. The Decisions section gave the same mark up for a
 * gavel so that this one can mean only this.
 *
 * Hovering or reaching it with the keyboard shows the explanation after the
 * shell's tooltip delay; clicking pins it, and a second click, Escape, or a
 * press anywhere outside lets it go. The pin is what makes it work on a
 * touchscreen, which has no hover to give — a tap is the whole gesture.
 *
 * It never takes the click of what it sits beside. A help icon in a lane head
 * or on a card is inside something draggable and clickable, so the press and
 * the click stop where they are: the row underneath never hears them, and the
 * icon is never a drag handle.
 */

/**
 * Only one explanation stays pinned.
 *
 * A press on one help icon never reaches another's dismiss layer, because the
 * icon stops the event before the document sees it. So the icon that pins asks
 * whatever was pinned before it to let go, and clicking along a row of them
 * reads as moving from one explanation to the next rather than as piling them
 * up.
 */
let releasePinned: (() => void) | null = null;

export function Help({
  id,
  /** Where the strip owns the Tab order and lends this icon one stop of it. */
  tabIndex,
}: {
  id: HelpId;
  tabIndex?: number;
}) {
  const { term, text } = HELP[id];
  const [shown, setShown] = useState(false);
  const [pinned, setPinned] = useState(false);
  // The pin is read inside handlers that run in the same event as the click
  // that set it — Radix closes its own tooltip on pointerdown and on click —
  // so it is held where those handlers can see it now rather than next render.
  const isPinned = useRef(false);

  const pin = (next: boolean) => {
    if (next) {
      releasePinned?.();
      releasePinned = () => {
        pin(false);
      };
    } else if (isPinned.current) {
      releasePinned = null;
    }
    isPinned.current = next;
    setPinned(next);
    setShown(next);
  };

  return (
    <TooltipPrimitive.Root
      open={shown}
      onOpenChange={(open) => {
        // Hover and focus open and close it; a pinned explanation ignores
        // them, and is let go by the three gestures that mean "done".
        if (isPinned.current) {
          return;
        }
        setShown(open);
      }}
    >
      <TooltipPrimitive.Trigger asChild>
        <button
          type="button"
          className="ms-help"
          aria-label={`What is ${term}?`}
          aria-expanded={pinned}
          draggable={false}
          tabIndex={tabIndex}
          onPointerDown={(event) => {
            event.stopPropagation();
          }}
          onClick={(event) => {
            event.stopPropagation();
            pin(!isPinned.current);
          }}
        >
          <CircleHelp size={14} strokeWidth={1.75} aria-hidden="true" />
        </button>
      </TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          className="ms-help-popover"
          sideOffset={6}
          collisionPadding={8}
          onEscapeKeyDown={() => {
            pin(false);
          }}
          onPointerDownOutside={() => {
            pin(false);
          }}
        >
          <span className="ms-help-term">{term}</span>
          <span className="ms-help-text">{text}</span>
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
