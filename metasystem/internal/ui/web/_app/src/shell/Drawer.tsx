import { ChevronDown, ChevronUp, Maximize2 } from "lucide-react";
import { useEffect, useRef } from "react";
import { useNavigate } from "react-router";

import { Composer, COMPOSER_HINT, COMPOSER_LABEL } from "./Composer";
import { IconButton } from "./controls";
import { Help } from "../help/Help";
import { Chips } from "../partner/Chips";
import { Seeing } from "../partner/Seeing";
import { Transcript } from "../partner/Transcript";
import { usePartner } from "../partner/store";

/**
 * The Project Partner, as a drawer along the bottom of the work area.
 *
 * It was a column on the right, and it took a third of the width from the one
 * thing the work area is for. Along the bottom it takes forty-eight pixels and
 * names the page above it: who it is, where to type, what it is about, and the
 * way to open it. Opening lifts a panel over the foot of the page; the page
 * above keeps its width and keeps scrolling.
 *
 * The panel is the conversation itself — what the human asked, what the
 * Partner answered, and what it did on the way — with the composer at the foot
 * of it where the old dock had it. The context line travels with the composer:
 * closed, it is in the bar beside the field; open, it is the panel's own
 * header, over the messages it is the context for.
 *
 * The bar's field is one line and the panel's is not. Reaching the one-line
 * field opens the panel, which is where the writing happens, and the sentence
 * and the caret go with it.
 *
 * It arrives closed. The bar is the whole of what an unasked-for collaborator
 * owes the page; the panel is what a human opens, and what this build then
 * remembers for them.
 *
 * The element id and the class names are the kit's own, and stay; what a human
 * reads says Project Partner.
 */

/**
 * Where the caret belongs once the drawer has opened or closed.
 *
 * Opening and closing replace the whole drawer, so whatever had the caret is
 * gone by the time the new one is on screen and the shell has to say where it
 * goes: to the panel's composer for a human who was already writing, and back
 * to the toggle for one who pressed it or pressed Escape. Nowhere is the first
 * render, where nothing has been touched yet.
 */
export type Caret = "none" | "toggle" | "panel";

export function Drawer({
  open,
  caret,
  onCompose,
  onToggle,
  onEscape,
}: {
  open: boolean;
  caret: Caret;
  /** Writing in the closed bar: the drawer opens and the writing goes on. */
  onCompose: () => void;
  onToggle: () => void;
  onEscape: () => void;
}) {
  const navigate = useNavigate();
  const toggle = useRef<HTMLButtonElement | null>(null);
  const { draft, setDraft, busy, store } = usePartner();
  // A conversation nobody has started yet is the first minute: the page's own
  // three questions, and the one sentence that teaches the gesture.
  const first = store.messages.length === 0;

  useEffect(() => {
    if (caret === "toggle") {
      toggle.current?.focus();
    }
  }, [caret]);


  return (
    <aside className="ms-drawer" aria-label="Project Partner" data-open={open ? "true" : "false"}>
      <div className="ms-drawer-bar">
        <span className="ms-drawer-who">
          <span className="ms-drawer-title">Project Partner</span>
          <Help id="partner" />
        </span>
        {!open && (
          <>
            <label className="ms-visually-hidden" htmlFor="drawer-composer">
              {COMPOSER_LABEL}
            </label>
            <input
              id="drawer-composer"
              className="ms-drawer-field"
              type="text"
              placeholder={COMPOSER_HINT}
              value={draft}
              disabled={busy}
              onChange={(event) => {
                setDraft(event.target.value);
                onCompose();
              }}
              onFocus={onCompose}
            />
            <Seeing />
          </>
        )}
        <span className="ms-drawer-actions">
          <IconButton
            label="Expand the Project Partner view"
            onClick={() => {
              void navigate("/brain");
            }}
          >
            <Maximize2 size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <button
            type="button"
            ref={toggle}
            className="ms-drawer-toggle"
            aria-expanded={open}
            aria-controls="brain-dock"
            onClick={onToggle}
          >
            {open ? "Close" : "Open"}
            {open ? (
              <ChevronDown size={14} strokeWidth={1.75} aria-hidden="true" />
            ) : (
              <ChevronUp size={14} strokeWidth={1.75} aria-hidden="true" />
            )}
          </button>
        </span>
      </div>
      {/* The panel is the element the toggle says it controls, so it is here
          whether or not it is shown; what is in it belongs to an open drawer,
          and a composer nobody can see is a composer that must never be given
          the caret. */}
      <div className="ms-drawer-panel" id="brain-dock" hidden={!open}>
        {open && (
          <>
            <div className="ms-drawer-head">
              <Seeing />
            </div>
            <div className="ms-drawer-messages">
              <Transcript />
            </div>
            <Chips first={first} />
            <Composer onEscape={onEscape} takeCaret={caret === "panel"} />
          </>
        )}
      </div>
    </aside>
  );
}
