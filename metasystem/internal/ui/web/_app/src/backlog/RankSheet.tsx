import { useRef, useState } from "react";

import { actingAs } from "./acting";
import { BacklogError, rankGoal, type Backlog, type Row } from "./api";
import { claimedConsequence, needsConfirming, rankOf, type Placement } from "./reorder";
import { Panel } from "./Panel";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { failureMessage } from "../shell/workspace";

/**
 * A re-rank, before it is made.
 *
 * It opens for two reasons and never for a third. A drop inside To Do, Ready
 * for Work or Waiting publishes where it lands: nobody is working on that
 * goal, the act changes an order and the lane says where the card went. A
 * drop on claimed work opens this, because a human dragging a card in In
 * Progress is reasonably expecting to change what that seat is doing, and the
 * act does not do that — the sentence here says so before the act rather than
 * after it. "Set priority…" opens it too, because a rank typed rather than
 * dragged has nothing to read the band from.
 *
 * Both fields are editable either way. The placement a drag computed is a
 * proposal, and a sheet that showed it without letting a human correct it
 * would be a confirmation dialog rather than an act.
 */
export function RankSheet({
  goal,
  placement,
  backlog,
  onClose,
  onDone,
}: {
  goal: Row;
  /** Where the drag proposed to put it, or the rank it already has. */
  placement: Placement;
  backlog: Backlog;
  onClose: () => void;
  /** The ledger as it stands after the act, which is what moves the card. */
  onDone: (moved: Backlog, id: string) => void;
}) {
  const [priority, setPriority] = useState(String(placement.priority));
  const [sequence, setSequence] = useState(String(placement.sequence));
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const { session, askToSignIn } = useSession();
  const retried = useRef(false);
  const authority = actingAs(backlog.authority, session);
  const blocked = blockedForRank(priority, sequence);

  const send = () => {
    if (blocked !== "") {
      return;
    }
    setSending(true);
    setRefusal("");
    rankGoal(goal.ref.id, Number(priority), sequence.trim() === "" ? null : Number(sequence))
      .then((moved) => {
        setSending(false);
        onDone(moved, goal.ref.id);
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof BacklogError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  return (
    <Panel
      eyebrow={`${rankOf(goal)} → ${rankOf({ priority: Number(priority) || 0, sequence: Number(sequence) || 0 })}`}
      title="Set priority"
      goal={goal}
      unproven={authority.proven ? "" : authority.reason}
      refusal={refusal}
      note={blocked === "" ? note(goal) : blocked}
      onClose={onClose}
      act={
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          Set priority
        </Button>
      }
    >
      {needsConfirming(goal) && (
        <p className="ms-act-consequence" role="status">
          {claimedConsequence}
        </p>
      )}
      <div className="ms-act-budget">
        <div className="ms-act-field">
          <label htmlFor="ms-rank-priority">Priority</label>
          <select
            id="ms-rank-priority"
            value={priority}
            onChange={(event) => {
              setPriority(event.target.value);
            }}
          >
            {["1", "2", "3"].map((band) => (
              <option key={band} value={band}>
                {band}
              </option>
            ))}
          </select>
        </div>
        <div className="ms-act-field">
          <label htmlFor="ms-rank-sequence">Position in that priority</label>
          <input
            id="ms-rank-sequence"
            type="text"
            inputMode="numeric"
            value={sequence}
            placeholder="leave empty to put it last"
            onChange={(event) => {
              setSequence(event.target.value);
            }}
          />
        </div>
      </div>
      <p className="ms-act-source">
        A position is counted among every live goal at that priority, not among the cards in this lane, and the engine
        renumbers the rest of the band behind it. A position past the end of the band is refused rather than rounded
        down, so what comes back is either the rank you asked for or the reason you cannot have it.
      </p>
    </Panel>
  );
}

function note(goal: Row): string {
  return needsConfirming(goal)
    ? `Publishes goal set-priority for ${goal.ref.id}. ${claimedConsequence}`
    : `Publishes goal set-priority for ${goal.ref.id}, and renumbers the band behind it.`;
}

/**
 * Why the button is disabled, or the empty string when it is not. Proof is
 * not a reason: the act asks, and a server with no human behind it answers
 * with the sign-in this page opens and the act it then retries.
 */
export function blockedForRank(priority: string, sequence: string): string {
  if (!["1", "2", "3"].includes(priority)) {
    return "A priority is 1, 2, or 3.";
  }
  const typed = sequence.trim();
  if (typed !== "" && !/^[1-9][0-9]*$/.test(typed)) {
    return "A position is a whole number from 1, or nothing at all, which puts the goal last.";
  }
  return "";
}
