import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";

import type { Row } from "./api";
import { Button } from "../shell/controls";

/**
 * The chrome every act of this section shares, before the act is made.
 *
 * A drop is a request for an application operation, never a change to a
 * status field in a browser, so the card does not move when it is dropped:
 * this panel opens, showing the exact goal, the exact values, and what will
 * be published under whose name. Only the ledger's confirmation moves a card.
 *
 * It is a panel and not a browser dialog, and it follows the Project sheet's
 * pattern exactly: a section with the dialog role, focus held inside while it
 * is open and returned to whatever opened it, Escape to close, a scrim that
 * dims and does not close. A refusal is the server's own sentence, shown
 * here, with the fields still filled in and nothing moved.
 *
 * It is one component because there are three of these acts now, and a focus
 * trap written three times is a focus trap that will be right in two places.
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
  onClose: () => void;
  children: ReactNode;
}) {
  const panel = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const opener = globalThis.document.activeElement;
    const inside = panel.current?.querySelector<HTMLElement>(FOCUSABLE);
    (inside ?? panel.current)?.focus();
    return () => {
      if (opener instanceof HTMLElement) {
        opener.focus();
      }
    };
  }, []);

  const keys = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== "Tab" || panel.current === null) {
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

  return (
    <>
      <div className="ms-act-scrim" aria-hidden="true" />
      <section
        className="ms-act-sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby="ms-act-title"
        tabIndex={-1}
        ref={panel}
        onKeyDown={keys}
      >
        <div>
          <p className="ms-act-eyebrow">{eyebrow}</p>
          <h2 className="ms-act-title" id="ms-act-title">
            {title}
          </h2>
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
        {children}
        <div className="ms-act-foot">
          <div className="ms-act-buttons">
            {act}
            <Button onClick={onClose}>Cancel</Button>
          </div>
          <p className="ms-act-note">{note}</p>
        </div>
        {refusal !== "" && (
          <p className="ms-act-refusal" role="alert">
            {refusal}
          </p>
        )}
      </section>
    </>
  );
}
