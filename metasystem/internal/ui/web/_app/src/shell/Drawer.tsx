import { ChevronDown, ChevronUp, Maximize2, Square } from "lucide-react";
import { useEffect, useRef } from "react";
import { useNavigate } from "react-router";

import { Composer, COMPOSER_HINT, COMPOSER_LABEL } from "./Composer";
import { Button, IconButton } from "./controls";
import { LiveDot, LiveLine } from "./LiveLine";
import { Help } from "../help/Help";
import { draftChipIn } from "../partner/attachments";
import { AttachmentChip } from "../partner/Chips";
import { FontControl } from "../partner/FontControl";
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
 * The panel is the conversation itself — what the human asked and what the
 * Partner answered, in one column at one measure — with the composer card at
 * the foot of it where the old dock had it. The context line travels with the
 * composer: closed, it is in the bar beside the field; open, it is the card's
 * own first row, over the field it is the context for. The panel has no header
 * of its own, because a line saying what the next question carries belongs to
 * the next question and not to the messages above it.
 *
 * The bar's field is one line and the panel's is not. Reaching the one-line
 * field opens the panel, which is where the writing happens, and the sentence
 * and the caret go with it.
 *
 * A handed-over draft stands beside that field too. The chip stands wherever the
 * composer is shown (g1-s52 D1): a sheet opening is the hand-over, and a drawer
 * left closed would have made the hand-over invisible and its × unreachable —
 * silent sharing, which is the one thing the master's rule about unsaved edits
 * forbids.
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

/**
 * The room one deposit card and the composer need, in pixels.
 *
 * Measured in the browser, at 1280 on the walkthrough's own sitting: the bar is
 * 48, a card with its heading, its two fields and its two buttons is 232, the
 * composer card is 174 and the panel's own padding is 28 — 482 — and this is
 * that with a line of room over it, so that a card whose fields a human has
 * opened a little is still whole. Two fifths of the work area gave the messages 149 of the 232 a
 * card needs, and a card is the one thing in here a human has to read closely
 * before they press Record it.
 *
 * Pixels, like the drawer's two minima, because it is about what fits rather
 * than about a share of anything. A number rather than a measurement taken as
 * the card arrives: the drawer asks once, for the first card, and a drawer that
 * re-measured itself at every card would be a drawer that kept taking the page.
 */
export const CARD_ROOM = 520;

/**
 * Whether a drawer standing at this many pixels asks for that room.
 *
 * The larger of the two stands, and the larger is not always the card's: a
 * drawer a human has dragged taller than a card needs asks for nothing. Nor
 * does one whose height is nothing — a drawer not on the screen has no height
 * to compare, and asking on behalf of a measurement that was never made would
 * grow a drawer nobody had opened.
 */
export function asksForRoom(standing: number): boolean {
  return standing > 0 && standing < CARD_ROOM;
}

export function Drawer({
  open,
  caret,
  onCompose,
  onToggle,
  onEscape,
  onRoom,
}: {
  open: boolean;
  caret: Caret;
  /** Writing in the closed bar: the drawer opens and the writing goes on. */
  onCompose: () => void;
  onToggle: () => void;
  onEscape: () => void;
  /**
   * Ask the work area for a taller drawer, in pixels. The shell owns the
   * height — it is one panel of the group the divider sits in — and refuses
   * where a human has said what the height is.
   */
  onRoom?: (pixels: number) => void;
}) {
  const navigate = useNavigate();
  const toggle = useRef<HTMLButtonElement | null>(null);
  const drawer = useRef<HTMLElement | null>(null);
  // Asked once, for the first card of this drawer's life — a deposit's or a
  // proposal's. A sitting deposits many cards and an answer can propose several,
  // and a drawer that grew at each of them would be a drawer that took the page
  // one card at a time.
  const asked = useRef(false);
  const {
    draft, setDraft, busy, stop, store, attachments, detach, deposits,
    proposalsLine, proposalsWaiting, showProposals,
  } = usePartner();
  // The draft a sheet handed over, where a sheet has. Only the draft: the
  // subject and a passage are chips a human made by their own press, in the
  // panel where they pressed it, and they know they are there. The draft is the
  // one attachment that appears because a sheet opened, so it is the one that
  // has to be visible — and removable — without opening anything.
  const handed = draftChipIn(attachments);

  useEffect(() => {
    if (caret === "toggle") {
      toggle.current?.focus();
    }
  }, [caret]);

  // The first deposit card asks for the room a card needs, and only where the
  // drawer has less than that: the larger of what it stands at and what one
  // card plus the composer need, which is why a drawer a human has already
  // dragged taller is left exactly as it is. A drag after this is theirs and is
  // remembered as always; the shell refuses this ask outright once they have
  // made one.
  //
  // A card of proposed acts asks for the same room, and for a stronger reason:
  // its press writes to the ledger. A human reads what an act will carry before
  // they apply it, and a drawer that showed the foot of a card of six with its
  // lines above the fold would be offering Apply over work nobody had read.
  const cards = deposits.length + proposalsWaiting;
  useEffect(() => {
    if (asked.current || !open || cards === 0 || onRoom === undefined) {
      return;
    }
    asked.current = true;
    if (asksForRoom(drawer.current?.clientHeight ?? 0)) {
      onRoom(CARD_ROOM);
    }
  }, [open, cards, onRoom]);


  return (
    <aside ref={drawer} className="ms-drawer" aria-label="Project Partner" data-open={open ? "true" : "false"}>
      <div className="ms-drawer-bar">
        <span className="ms-drawer-who">
          <span className="ms-drawer-title">Project Partner</span>
          {/* The dot alone while the drawer is open, for when the transcript
              is scrolled away from the turn that is running; closed, the bar's
              live line is the mark (g1-s74 D3b as amended). */}
          {busy && open && <LiveDot className="ms-drawer-title-dot" />}
          <Help id="partner" />
          <FontControl />
        </span>
        {!open && (
          <>
            {/* While a turn runs the field has nothing to take, so the live
                line stands in its place with the composer's own Stop beside
                it; the draft is the store's and is there again when the turn
                ends (g1-s74 D3c). */}
            {busy ? (
              <>
                <span className="ms-drawer-live">
                  <LiveLine live={store.live} />
                </span>
                <Button
                  className="ms-drawer-stop"
                  onClick={() => {
                    void stop();
                  }}
                >
                  <Square size={14} strokeWidth={1.75} aria-hidden="true" />
                  Stop
                </Button>
              </>
            ) : (
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
                  onChange={(event) => {
                    setDraft(event.target.value);
                    onCompose();
                  }}
                  onFocus={onCompose}
                />
              </>
            )}
            <Seeing />
            {/* What is waiting for this human in the conversation, counted
                across answers: a proposal they have not answered is waiting
                whether or not it is the newest thing said. Pressing it opens the
                drawer at the newest card that has one (g1-s58 D8). */}
            {proposalsLine !== "" && (
              <button
                type="button"
                className="ms-proposal-bar"
                title="Show what the Partner proposed"
                onClick={() => {
                  showProposals();
                  onToggle();
                }}
              >
                {proposalsLine}
              </button>
            )}
            {handed !== null && (
              <AttachmentChip
                attachment={handed}
                onRemove={() => {
                  detach(handed.id);
                }}
              />
            )}
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
            <div className="ms-drawer-messages">
              <Transcript />
            </div>
            <Composer onEscape={onEscape} takeCaret={caret === "panel"} />
          </>
        )}
      </div>
    </aside>
  );
}
