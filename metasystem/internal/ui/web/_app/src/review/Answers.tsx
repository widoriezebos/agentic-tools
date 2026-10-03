import { createContext, useEffect, useId, useState } from "react";

import {
  choiceSaid,
  decisionNamed,
  decisionOfAnswer,
  decisionOfRecommend,
  decisionsFor,
  parseAnchor,
  type Decision,
  type RoomFinding,
} from "./room";
import { loadBacklog, type Backlog } from "../backlog/api";
import { OpenSheet } from "../backlog/OpenSheet";
import { usePartner } from "../partner/store";
import { ACCEPTED, FIX, followUp, NOT_A_PROBLEM } from "../partner/sitting";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { Trouble } from "../shell/Trouble";

/**
 * The files that changed on the branch since the version the review names,
 * while the room says it moved (D9): the shaping room's board reads it.
 */
export const MovedFiles = createContext<readonly string[]>([]);

/**
 * A finding's decision (review-findings-read-as-decisions §3): the choices its
 * severity offers, each with the consequence it has, said before the press;
 * the reviewer's recommendation marked while the person has decided nothing,
 * and the person's decision marked once they have. One press records the
 * finding with the decision, and pressing the decision that stands writes
 * nothing (R-129-ui).
 *
 * Two of the four ask for something first. Fix after landing opens the New goal
 * sheet with the title as its intent and why it matters as its next step, and
 * the decision is written only once the goal is open, naming it (R-4). Not a
 * problem and I accept this risk ask for the person's reason in a small sheet;
 * an acceptance's words are kept with the room's drafts while its sheet is open.
 */
export function FindingDecisions({ finding }: { finding: RoomFinding }) {
  const { decideFinding, accepting, noteAccepting, keepRoomNow } = usePartner();
  const [refusal, setRefusal] = useState("");
  const [busy, setBusy] = useState(false);
  const [opening, setOpening] = useState(false);
  const [why, setWhy] = useState<string | null>(null);
  const reason = accepting[finding.id];
  const chosen = decisionOfAnswer(finding.answer);
  const recommended = decisionOfRecommend(finding.recommend);

  const press = async (answer: string): Promise<boolean> => {
    setBusy(true);
    setRefusal("");
    try {
      const refused = await decideFinding(finding.id, answer);
      setRefusal(refused);
      return refused === "";
    } finally {
      setBusy(false);
    }
  };

  const choose = (decision: Decision) => {
    if (decision === chosen) {
      return;
    }
    switch (decision) {
      case "fix":
        void press(FIX);
        return;
      case "follow-up":
        setOpening(true);
        return;
      case "not a problem":
        setWhy("");
        return;
      case "accepted":
        noteAccepting(finding.id, reason ?? "");
    }
  };

  return (
    <div className="ms-decide">
      <span className="ms-decide-label">Your decision</span>
      <div className="ms-decide-choices">
        {decisionsFor(finding.severity).map((decision) => {
          const mine = chosen === decision;
          const advised = chosen === "" && recommended === decision;
          return (
            <button
              key={decision}
              type="button"
              className={`ms-decide-choice${mine ? " ms-decide-choice--chosen" : ""}${advised ? " ms-decide-choice--recommended" : ""}`}
              aria-pressed={mine}
              disabled={busy || finding.id === ""}
              onClick={() => {
                choose(decision);
              }}
            >
              <b>
                {decisionNamed(decision).label}
                {mine && <span className="ms-decide-mark">your decision</span>}
                {advised && <span className="ms-decide-mark">recommended</span>}
              </b>
              <small>{choiceSaid(decision, finding)}</small>
            </button>
          );
        })}
      </div>
      {refusal !== "" && <Trouble text={refusal} role="status" variant="small" />}
      {opening && (
        <FollowUp
          intent={finding.title}
          nextStep={finding.why}
          onClose={() => {
            setOpening(false);
          }}
          onOpened={(goal) => {
            setOpening(false);
            void press(followUp(goal));
          }}
        />
      )}
      <Sheet
        open={why !== null}
        onOpenChange={(open) => {
          if (!open) {
            setWhy(null);
          }
        }}
        side="right"
        label="Not a problem"
        title="Not a problem"
        closeLabel="Close without deciding"
        bodyClassName="ms-sitting-sheet ms-review-sheet"
        sheetName="Not a problem"
      >
        <ReasonSheet
          said="Say why this is not a problem. Your reason is recorded with the finding."
          title={finding.title}
          reason={why ?? ""}
          busy={busy}
          refusal={refusal}
          press="Record: not a problem"
          onReason={setWhy}
          onLeave={() => undefined}
          onPress={() => {
            void press(NOT_A_PROBLEM(why ?? "")).then((done) => {
              if (done) {
                setWhy(null);
              }
            });
          }}
        />
      </Sheet>
      <Sheet
        open={reason !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            noteAccepting(finding.id, null);
          }
        }}
        side="right"
        label="I accept this risk"
        title="I accept this risk"
        closeLabel="Close without accepting"
        bodyClassName="ms-sitting-sheet ms-review-sheet"
        sheetName="I accept this risk"
      >
        <ReasonSheet
          said={choiceSaid("accepted", { ...finding, answer: "" })}
          title={finding.title}
          reason={reason ?? ""}
          busy={busy}
          refusal={refusal}
          press="I accept this risk"
          onReason={(said) => {
            noteAccepting(finding.id, said);
          }}
          onLeave={() => {
            void keepRoomNow();
          }}
          onPress={() => {
            void press(ACCEPTED(reason ?? ""));
          }}
        />
      </Sheet>
    </div>
  );
}

/** A choice's small sheet: what it records, the finding, the person's reason, and the press. */
export function ReasonSheet({
  said,
  title,
  reason,
  busy,
  refusal,
  press,
  onReason,
  onLeave,
  onPress,
}: {
  said: string;
  title: string;
  reason: string;
  busy: boolean;
  refusal: string;
  press: string;
  onReason: (said: string) => void;
  onLeave: () => void;
  onPress: () => void;
}) {
  const field = useId();
  return (
    <>
      <p className="ms-sitting-said">{said}</p>
      <p className="ms-deposit-text">{title}</p>
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
          Write your reason first; it is recorded with your decision.
        </p>
      )}
      {refusal !== "" && <Trouble text={refusal} role="status" variant="small" />}
      <div className="ms-sitting-foot">
        <Button primary disabled={busy || reason.trim() === ""} onClick={onPress}>
          {press}
        </Button>
      </div>
    </>
  );
}

/**
 * The New goal sheet, opened with the finding as its intent. The answer is
 * written only from its onDone, which is the ledger saying the goal exists.
 */
export function FollowUp({
  intent,
  nextStep = "",
  onClose,
  onOpened,
}: {
  intent: string;
  nextStep?: string;
  onClose: () => void;
  onOpened: (goal: string) => void;
}) {
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
      nextStep={nextStep}
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
