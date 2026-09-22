import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import { approveGoal, withdrawGoal, type Authority, type Backlog, type Row } from "./api";
import {
  approveNote,
  blockedFor,
  budgetOf,
  draftOf,
  prefillFor,
  withdrawNote,
  type BudgetDraft,
  type BudgetSource,
  type Move,
} from "./moves";
import { Button } from "../shell/controls";
import { failureMessage } from "../shell/workspace";

/**
 * The act, before it is made.
 *
 * A drop is a request for an application operation, never a change to a
 * status field in a browser, so the card does not move when it is dropped:
 * this panel opens, showing the exact goal, the exact budget, and what will
 * be published under whose name. Only the ledger's confirmation moves a card.
 *
 * It is a panel and not a browser dialog, and it follows the Project sheet's
 * pattern exactly: a section with the dialog role, focus held inside while it
 * is open and returned to whatever opened it, Escape to close, a scrim that
 * dims and does not close. A refusal is the server's own sentence, shown
 * here, with the fields still filled in and nothing moved.
 */

/** Everything that can hold focus inside the panel, in the order it is met. */
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** What the sheet was opened to do. */
export type Request = { move: Move; goal: Row };

/** Where a prefilled budget came from, in the words the sheet uses. */
const SOURCES: Record<BudgetSource, string> = {
  goal: "the tuple this goal already carries",
  project: "the project's budget law for this goal's tier",
  "last-approved": "the goal approved most recently",
  none: "nothing: this project declares no budget law and no goal has been approved yet",
};

export function ActSheet({
  request,
  backlog,
  onClose,
  onDone,
}: {
  request: Request;
  backlog: Backlog;
  onClose: () => void;
  /** The ledger as it stands after the act, which is what moves the card. */
  onDone: (moved: Backlog) => void;
}) {
  const panel = useRef<HTMLElement | null>(null);
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const prefill = prefillFor(request.goal, backlog.budgetDefaults, backlog.rows);
  const [draft, setDraft] = useState<BudgetDraft>(() => draftOf(prefill.budget));
  const [reason, setReason] = useState("");
  const authority: Authority = backlog.authority;
  const blocked = blockedFor(request.move, authority.proven, authority.reason, draft);

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

  const send = () => {
    const budget = budgetOf(draft);
    if (request.move === "approve" && budget === null) {
      return;
    }
    setSending(true);
    setRefusal("");
    const acting =
      request.move === "approve"
        ? approveGoal(request.goal.ref.id, budget as NonNullable<typeof budget>)
        : withdrawGoal(request.goal.ref.id, reason.trim());
    acting
      .then((moved) => {
        setSending(false);
        onDone(moved);
      })
      .catch((error: unknown) => {
        setSending(false);
        setRefusal(failureMessage(error));
      });
  };

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
          <p className="ms-act-eyebrow">
            {request.move === "approve" ? "To Do → Ready for Work" : "Ready for Work → To Do"}
          </p>
          <h2 className="ms-act-title" id="ms-act-title">
            {request.move === "approve" ? "Approve" : "Withdraw approval"}
          </h2>
        </div>
        <div className="ms-act-goal">
          <p className="ms-act-goal-id ms-mono">{request.goal.ref.id}</p>
          <p className="ms-act-goal-intent">{request.goal.intent}</p>
        </div>
        {!authority.proven && (
          <p className="ms-act-unproven" role="alert">
            {authority.reason}
          </p>
        )}
        {request.move === "approve" ? (
          <BudgetFields draft={draft} source={prefill.source} onChange={setDraft} />
        ) : (
          <div className="ms-act-field">
            <label htmlFor="ms-act-reason">Reason (optional)</label>
            <input
              id="ms-act-reason"
              type="text"
              value={reason}
              onChange={(event) => {
                setReason(event.target.value);
              }}
            />
          </div>
        )}
        <div className="ms-act-foot">
          <div className="ms-act-buttons">
            <Button primary disabled={blocked !== "" || sending} onClick={send}>
              {request.move === "approve" ? "Approve" : "Withdraw approval"}
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </div>
          <p className="ms-act-note">
            {blocked === "" ? noteFor(request.move, authority.human) : blocked}
          </p>
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

function noteFor(move: Move, human: string): string {
  return move === "approve" ? approveNote(human) : withdrawNote;
}

/**
 * The five limits, every one of them shown and every one of them confirmed.
 * The sheet says where the numbers came from, because a value a human did not
 * choose is one they have to be able to recognize as not theirs.
 */
function BudgetFields({
  draft,
  source,
  onChange,
}: {
  draft: BudgetDraft;
  source: BudgetSource;
  onChange: (draft: BudgetDraft) => void;
}) {
  const field = (
    id: keyof BudgetDraft,
    label: string,
    hint: string,
    mode: "text" | "numeric",
  ) => (
    <div className="ms-act-field" key={id}>
      <label htmlFor={`ms-act-${id}`}>{label}</label>
      <input
        id={`ms-act-${id}`}
        type="text"
        inputMode={mode}
        value={draft[id]}
        placeholder={hint}
        onChange={(event) => {
          onChange({ ...draft, [id]: event.target.value });
        }}
      />
    </div>
  );
  return (
    <>
      <p className="ms-act-source">Prefilled from {SOURCES[source]}.</p>
      <div className="ms-act-budget">
        {field("elapsedLimit", "Elapsed limit", "4h, or 1d2h", "text")}
        {field("attemptLimit", "Attempts", "6", "numeric")}
        {field("reservedJobMinutesLimit", "Reserved job minutes", "720", "numeric")}
        {field("activeJobLimit", "Active jobs", "1", "numeric")}
        {field("reviewRoundLimit", "Review rounds", "2", "numeric")}
      </div>
    </>
  );
}
