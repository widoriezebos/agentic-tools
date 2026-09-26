import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";

import type { Row } from "./api";
import { AskAboutSheet } from "../partner/AskSheet";
import type { Field } from "../partner/drafting";
import { useOpenSheet } from "../partner/store";
import { Button } from "../shell/controls";
import { DEFAULT_MODALITY, useOpener, useWorkModal, type Modality } from "../shell/workmodal";

/**
 * The chrome every act of this section shares, before the act is made.
 *
 * A drop is a request for an application operation, never a change to a
 * status field in a browser, so the card does not move when it is dropped:
 * this panel opens, showing the exact goal, the exact values, and what will
 * be published under whose name. Only the ledger's confirmation moves a card.
 *
 * It is a panel and not a browser dialog, and it follows the Project sheet's
 * pattern exactly: a section with the dialog role, focus returned to whatever
 * opened it, Escape to close, a scrim that dims and does not close. A refusal
 * is the server's own sentence, shown here, with the fields still filled in
 * and nothing moved.
 *
 * It is one component because there are three of these acts now, and a focus
 * trap written three times is a focus trap that will be right in two places.
 *
 * How modal it is, is workmodal.tsx's. Modal for the work area — the default —
 * it renders into the work area's own layer, the scrim covers that box and no
 * more, the work behind it is inert, and focus is NOT held inside: Tab leaves
 * for the drawer and comes back, because the Project Partner beneath is meant
 * to be usable while a goal is being written. Modal for the window, which is
 * what it falls back to where there is no work area, it holds focus as it
 * always did — there is nothing beside it there to hold focus for.
 *
 * Its head carries "Ask about this", which hands the fields as they stand to
 * the Partner as a draft. Its caller says what the sheet is called, which
 * fields are its own, which of those the Partner may offer words for, and how
 * to put words into one of them. This panel writes into nothing itself: the
 * sheet owns its draft state, so the sheet owns the setter.
 */

/** Everything that can hold focus inside the panel, in the order it is met. */
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function Panel({
  eyebrow,
  title,
  goal,
  unproven,
  refusal,
  note,
  act,
  aside,
  form = false,
  modal = DEFAULT_MODALITY,
  sheetName,
  fields = [],
  opening,
  writable = [],
  set,
  busy = false,
  onClose,
  children,
}: {
  /** Where the act is coming from and going to, above the title. */
  eyebrow: string;
  title: string;
  /** The goal being acted on, where there is one; an open has none yet. */
  goal?: Row;
  /** Why this server cannot act, or "" when it can. */
  unproven: string;
  /** What the server refused, in its own words, or "". */
  refusal: string;
  /** What the act will do, or why the button is disabled. */
  note: string;
  /** The primary button, which the sheet owns because it knows the act. */
  act: ReactNode;
  /** One line under the head, where the sheet has something to say there. */
  aside?: ReactNode;
  /**
   * The one-screen shape: the body between the head and the foot scrolls and
   * the foot stays where it is, so the button that ends a form is never below
   * the fold. A confirmation is short enough to scroll whole and takes the
   * default; a form a human fills takes this. The refusal moves with the
   * foot, because the sentence explaining why nothing happened belongs beside
   * the button that did not work.
   */
  form?: boolean;
  /** How much of the window this is modal for. The work area, by default. */
  modal?: Modality;
  /** What the capture calls this sheet while it is open. Its title, by default. */
  sheetName?: string;
  /**
   * Work is in flight that closing would not stop.
   *
   * A sheet that publishes one act per goal goes on publishing whether or not
   * it is on screen, so while it is busy this panel cannot be dismissed:
   * Cancel is disabled and Escape does nothing. The scrim never closed
   * anything. The sheet says when it is busy; this only stops a human
   * dismissing the one place that says so.
   */
  busy?: boolean;
  /** The fields "Ask about this" hands over, in the order the sheet asks them. */
  fields?: Field[];
  /**
   * The one opening this sheet is, minted by the sheet when it mounted.
   *
   * A suggestion belongs to an opening and not to a sheet's name: goal A's edit
   * sheet and goal B's are both "Edit goal", and words prepared while A stood
   * must not be usable once it has closed, nor woken by B. A sheet that offers
   * nothing writable need not mint one.
   */
  opening?: string;
  /**
   * Which of those fields the Partner may be offered words for, including the
   * ones with nothing in them yet, in the order the sheet asks them. A
   * confirmation supplies none, and so does a sheet whose values are chosen
   * from a menu rather than written.
   */
  writable?: string[];
  /**
   * Put words in one of those fields, whole, and answer what it held before.
   * It is the sheet's own, because the sheet owns the draft.
   */
  set?: (field: string, text: string) => string;
  onClose: () => void;
  children: ReactNode;
}) {
  // Declared first, so that its cleanup is first: the work area is live again
  // before the caret is handed back to whatever opened this.
  const host = useWorkModal(modal);
  useOpenSheet(sheetName ?? title);
  const panel = useRef<HTMLElement | null>(null);
  // Read while it still has the caret: covering the work area takes the caret
  // off whatever had it, and an effect reads the document too late.
  const opener = useOpener();

  useEffect(() => {
    const inside = panel.current?.querySelector<HTMLElement>(FOCUSABLE);
    (inside ?? panel.current)?.focus();
    return () => {
      opener.current?.focus();
    };
  }, [opener]);

  // The one way out, guarded once so Escape and Cancel cannot disagree.
  const dismiss = () => {
    if (!busy) {
      onClose();
    }
  };

  const keys = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      dismiss();
      return;
    }
    // Focus is held inside only while this is modal for the whole window.
    // Over the work area the Partner beneath is live, and Tab is how a human
    // reaches it.
    if (event.key !== "Tab" || panel.current === null || host !== null) {
      return;
    }
    const stops = [...panel.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
    const first = stops.at(0);
    const last = stops.at(-1);
    if (first === undefined || last === undefined) {
      return;
    }
    const at = globalThis.document.activeElement;
    if (event.shiftKey && at === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && at === last) {
      event.preventDefault();
      first.focus();
    }
  };

  const sheet = (
    <>
      <div className="ms-act-scrim" aria-hidden="true" />
      <section
        className={form ? "ms-act-sheet ms-act-sheet--form" : "ms-act-sheet"}
        role="dialog"
        // Only a sheet that blocks the whole window is modal to a reader: one
        // that leaves the drawer live has to say so, or a screen reader would
        // be told the Partner beneath it is not there.
        aria-modal={host === null}
        aria-labelledby="ms-act-title"
        tabIndex={-1}
        ref={panel}
        onKeyDown={keys}
      >
        <div className="ms-act-head">
          <div>
            <p className="ms-act-eyebrow">{eyebrow}</p>
            <h2 className="ms-act-title" id="ms-act-title">
              {title}
            </h2>
          </div>
          {fields.length > 0 && (
            <AskAboutSheet
              sheet={sheetName ?? title}
              fields={fields}
              opening={opening}
              writable={writable}
              set={set}
            />
          )}
        </div>
        {goal !== undefined && (
          <div className="ms-act-goal">
            <p className="ms-act-goal-id ms-mono">{goal.ref.id}</p>
            <p className="ms-act-goal-intent">{goal.intent}</p>
          </div>
        )}
        {unproven !== "" && (
          <p className="ms-act-unproven" role="alert">
            {unproven}
          </p>
        )}
        {aside}
        {form ? <div className="ms-act-body">{children}</div> : children}
        <div className="ms-act-foot">
          {form && refusal !== "" && (
            <p className="ms-act-refusal" role="alert">
              {refusal}
            </p>
          )}
          <div className="ms-act-buttons">
            {act}
            <Button disabled={busy} onClick={dismiss}>
              Cancel
            </Button>
          </div>
          {note !== "" && <p className="ms-act-note">{note}</p>}
        </div>
        {!form && refusal !== "" && (
          <p className="ms-act-refusal" role="alert">
            {refusal}
          </p>
        )}
      </section>
    </>
  );

  // Over the work area the sheet renders into the work area's own layer, so
  // the scrim is that box and the drawer beneath is outside it.
  return host === null ? sheet : createPortal(sheet, host);
}
