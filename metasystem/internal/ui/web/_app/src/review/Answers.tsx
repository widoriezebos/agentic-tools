import { createContext, useEffect, useId, useState } from "react";

import { ANSWERS, parseAnchor, type AnswerKind } from "./room";
import { loadBacklog, type Backlog } from "../backlog/api";
import { OpenSheet } from "../backlog/OpenSheet";
import { usePartner } from "../partner/store";
import { answerOf } from "../partner/sitting";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { Trouble } from "../shell/Trouble";

/**
 * The files that changed on the branch since the tip the review names, while
 * the room says the tip moved: a finding anchored in one of them carries "may
 * have moved" on its card and on the board until the new tip is reviewed (D9).
 */
export const MovedFiles = createContext<readonly string[]>([]);

/**
 * A recorded finding's four answers (g1-s65 D8), on its card and on the board.
 *
 * Each press says its consequence beside it, before it is pressed, and each one
 * rewrites that finding's Answer line in the record through the recorder, by the
 * entry's mark: the door's counts and End's refusal read the record, so an
 * answer is a fact of the record and not of this browser (Astra S65-01).
 *
 * Two of the four ask for something first. Follow-up goal opens the New goal
 * sheet with the finding as its prefilled intent, and the answer is written only
 * once the goal has been opened, naming it. Accept opens a small sheet whose
 * reason is required; its words are kept with the room's drafts while it is open.
 */
export function FindingAnswers({ mark, text, answer, moved }: { mark: string; text: string; answer: string; moved: boolean }) {
  const { answerFinding, accepting, noteAccepting, keepRoomNow } = usePartner();
  const [refusal, setRefusal] = useState("");
  const [busy, setBusy] = useState(false);
  const [opening, setOpening] = useState(false);
  const reason = accepting[mark];

  const press = async (kind: AnswerKind, detail: string) => {
    setBusy(true);
    setRefusal("");
    try {
      setRefusal(await answerFinding(mark, kind, detail));
    } finally {
      setBusy(false);
    }
  };

  const now = answerOf(answer);
  return (
    <div className="ms-finding-answers">
      <p className={`ms-finding-answer ms-finding-answer--${now.replace(/\s+/gu, "-")}`}>
        <span className="ms-deposit-label">Answer</span>
        {answer === "" ? "unanswered" : answer}
        {moved && <span className="ms-finding-moved">may have moved</span>}
      </p>
      <ul className="ms-finding-presses">
        {ANSWERS.map((one) => (
          <li key={one.answer} className="ms-finding-press">
            <Button
              disabled={busy}
              primary={one.answer === "fix" && now === "unanswered"}
              onClick={() => {
                if (one.answer === "follow-up") {
                  setOpening(true);
                  return;
                }
                if (one.answer === "accepted") {
                  noteAccepting(mark, reason ?? "");
                  return;
                }
                void press(one.answer, "");
              }}
            >
              {one.label}
            </Button>
            <span className="ms-finding-consequence">{one.consequence}</span>
          </li>
        ))}
      </ul>
      {refusal !== "" && (
        <Trouble text={refusal} role="status" variant="small" />
      )}
      {opening && (
        <FollowUp
          intent={text}
          onClose={() => {
            setOpening(false);
          }}
          onOpened={(goal) => {
            setOpening(false);
            void press("follow-up", goal);
          }}
        />
      )}
      <Sheet
        open={reason !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            noteAccepting(mark, null);
          }
        }}
        side="right"
        label="Accept this finding"
        title="Accept, with reason"
        closeLabel="Close without accepting"
        bodyClassName="ms-sitting-sheet ms-review-sheet"
        sheetName="Accept, with reason"
      >
        <AcceptSheet
          text={text}
          reason={reason ?? ""}
          busy={busy}
          refusal={refusal}
          onReason={(said) => {
            noteAccepting(mark, said);
          }}
          onLeave={() => {
            void keepRoomNow();
          }}
          onAccept={() => {
            void press("accepted", reason ?? "");
          }}
        />
      </Sheet>
    </div>
  );
}

function AcceptSheet({
  text,
  reason,
  busy,
  refusal,
  onReason,
  onLeave,
  onAccept,
}: {
  text: string;
  reason: string;
  busy: boolean;
  refusal: string;
  onReason: (said: string) => void;
  onLeave: () => void;
  onAccept: () => void;
}) {
  const field = useId();
  return (
    <>
      <p className="ms-sitting-said">
        {"Accepting records the risk with your reason on this finding\u2019s own line. " +
          "The reason is yours, and the finding is not accepted without one."}
      </p>
      <p className="ms-deposit-text">{text}</p>
      <label className="ms-sitting-label" htmlFor={field}>
        Your reason
      </label>
      <textarea
        id={field}
        className="ms-deposit-field"
        rows={3}
        value={reason}
        onChange={(event) => {
          onReason(event.target.value);
        }}
        onBlur={onLeave}
      />
      {reason.trim() === "" && (
        <p className="ms-deposit-needs" role="status">
          An accepted risk is recorded with your reason. Write the reason before accepting it.
        </p>
      )}
      {refusal !== "" && (
        <Trouble text={refusal} role="status" variant="small" />
      )}
      <div className="ms-sitting-foot">
        <Button primary disabled={busy || reason.trim() === ""} onClick={onAccept}>
          Accept it
        </Button>
      </div>
    </>
  );
}

/**
 * The New goal sheet, opened with the finding as its intent. The answer is
 * written only from its onDone, which is the ledger saying the goal exists.
 */
function FollowUp({ intent, onClose, onOpened }: { intent: string; onClose: () => void; onOpened: (goal: string) => void }) {
  const [backlog, setBacklog] = useState<Backlog | null>(null);
  const [refusal, setRefusal] = useState("");
  useEffect(() => {
    const aborter = new AbortController();
    loadBacklog(aborter.signal)
      .then(setBacklog)
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setRefusal(error instanceof Error ? error.message : String(error));
        }
      });
    return () => {
      aborter.abort();
    };
  }, []);
  if (refusal !== "") {
    return (
      <Trouble text={`The board could not be read, so no goal can be opened from here: ${refusal}`} role="status" variant="small" />
    );
  }
  if (backlog === null) {
    return <p className="ms-desk-loading">Opening the New goal sheet…</p>;
  }
  return (
    <OpenSheet
      backlog={backlog}
      intent={intent}
      onClose={onClose}
      onDone={(_opened, id) => {
        onOpened(id);
      }}
    />
  );
}

/** A finding's anchor as a press that puts it on the desk, or its words where it names nothing the desk shows. */
export function AnchorPress({ anchor }: { anchor: string }) {
  const { putOnDesk } = usePartner();
  const item = parseAnchor(anchor);
  if (item === null) {
    return <span className="ms-mono">{anchor}</span>;
  }
  return (
    <button
      type="button"
      className="ms-anchor-chip"
      title={`Put ${anchor} on the desk`}
      onClick={() => {
        putOnDesk(item);
      }}
    >
      {anchor}
    </button>
  );
}
