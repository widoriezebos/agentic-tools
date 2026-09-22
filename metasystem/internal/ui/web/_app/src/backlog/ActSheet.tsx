import { useState } from "react";

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
import { Panel } from "./Panel";
import { Button } from "../shell/controls";
import { failureMessage } from "../shell/workspace";

/**
 * Approving and withdrawing, before the act is made.
 *
 * The panel's chrome — the dialog role, the focus held inside it, Escape, the
 * scrim — is Panel's, shared with the two acts that arrived after this one.
 * What is here is the part only this act knows: which of the two verbs is
 * being published, the complete budget one of them carries, and what each one
 * promises.
 */

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
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const prefill = prefillFor(request.goal, backlog.budgetDefaults, backlog.rows);
  const [draft, setDraft] = useState<BudgetDraft>(() => draftOf(prefill.budget));
  const [reason, setReason] = useState("");
  const authority: Authority = backlog.authority;
  const blocked = blockedFor(request.move, authority.proven, authority.reason, draft);
  const approving = request.move === "approve";

  const send = () => {
    const budget = budgetOf(draft);
    if (approving && budget === null) {
      return;
    }
    setSending(true);
    setRefusal("");
    const acting = approving
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

  return (
    <Panel
      eyebrow={approving ? "To Do → Ready for Work" : "Ready for Work → To Do"}
      title={approving ? "Approve" : "Withdraw approval"}
      goal={request.goal}
      unproven={authority.proven ? "" : authority.reason}
      refusal={refusal}
      note={blocked === "" ? noteFor(request.move, authority.human) : blocked}
      onClose={onClose}
      act={
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          {approving ? "Approve" : "Withdraw approval"}
        </Button>
      }
    >
      {approving ? (
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
    </Panel>
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
