import { useState } from "react";

import type { Need } from "./api";
import {
  approvePlan,
  parkPlan,
  progressLine,
  sendable,
  stoppedLine,
  type Planned,
} from "./decisions";
import { approveGoal, BacklogError, parkGoal, type Backlog } from "../backlog/api";
import { Panel } from "../backlog/Panel";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { failureMessage } from "../shell/workspace";

/**
 * Approving, or parking, several goals at once.
 *
 * There is no bulk verb in this engine and this sheet does not invent one: it
 * sends one publication per goal, in the order the list shows, and says how
 * far it has got. That is the whole of its safety, and so is the stop rule —
 * the first answer that is not an answer ends the run, and the goals after it
 * were never sent.
 *
 * What it will not do is claim to know what landed. An answer can fail after
 * the publication it carried succeeded, so the goal it stopped at is named as
 * UNRESOLVED rather than as refused, and the page's own re-read — which
 * reports the accepted ref as observed — is what says what the ledger did.
 *
 * The approve list carries the budget each goal would be approved with and
 * where that budget came from, in `prefillFor`'s own words, so a human reads
 * here exactly what the single-goal sheet would have shown them. A goal whose
 * prefill is null has no tuple to confirm in a list, so it is listed, named
 * and left unsent.
 */

export type Bulk = { act: "approve" | "park"; goals: Need[] };

type Run =
  | { state: "ready" }
  | { state: "running"; done: number; total: number }
  | { state: "stopped"; line: string };

export function BulkSheet({
  bulk,
  backlog,
  onClose,
  onDone,
  onStopped,
}: {
  bulk: Bulk;
  backlog: Backlog;
  onClose: () => void;
  /** Every goal was sent: the sheet is done and the page reads again. */
  onDone: () => void;
  /**
   * The run stopped at a refusal. The page reads its payload again — the
   * re-read is what says what landed — but the sheet STAYS OPEN, because the
   * sentence saying which goal it stopped at and in what words is the whole
   * of what a human has to act on.
   */
  onStopped: () => void;
}) {
  const approving = bulk.act === "approve";
  const plan = approving
    ? approvePlan(bulk.goals, backlog.budgetDefaults, backlog.rows)
    : parkPlan(bulk.goals);
  const sending = sendable(plan);
  const [reason, setReason] = useState("");
  const [run, setRun] = useState<Run>({ state: "ready" });
  const { askToSignIn } = useSession();
  const because = reason.trim();
  const blocked = blockedFor(bulk.act, sending.length, because);

  const send = () => {
    setRun({ state: "running", done: 0, total: sending.length });
    void (async () => {
      let done = 0;
      for (const one of sending) {
        try {
          await (approving
            ? approveGoal(one.id, one.budget as NonNullable<typeof one.budget>)
            : parkGoal(one.id, because));
        } catch (error: unknown) {
          // A server that found no human says the remedy is in this page, so
          // the sign-in sheet opens on it. The run still stops: the goals
          // after this one were not sent, and a human presses the button
          // again once they are signed in.
          if (error instanceof BacklogError && error.signIn) {
            askToSignIn();
          }
          setRun({ state: "stopped", line: stoppedLine(one, failureMessage(error), done, sending.length) });
          onStopped();
          return;
        }
        done += 1;
        setRun({ state: "running", done, total: sending.length });
      }
      onDone();
    })();
  };

  return (
    <Panel
      eyebrow={approving ? "To Do → Ready for Work" : "To Do → Not now"}
      title={approving ? `Approve ${goalsWord(sending.length)}` : `Not now for ${goalsWord(sending.length)}`}
      sheetName={approving ? "Approve selected" : "Not now for selected"}
      fields={[
        { name: "Goals", value: sending.map((one) => one.id).join(", ") },
        { name: "Reason", value: because },
      ]}
      unproven=""
      refusal={run.state === "stopped" ? run.line : ""}
      note={blocked === "" ? noteFor(bulk.act, sending.length) : blocked}
      form
      onClose={onClose}
      act={
        <Button primary disabled={blocked !== "" || run.state === "running"} onClick={send}>
          {approving ? "Approve" : "Not now"}
        </Button>
      }
      aside={
        run.state === "running" ? (
          <span className="ms-decisions-progress">{progressLine(run.done, run.total)}</span>
        ) : undefined
      }
    >
      {!approving && (
        <div className="ms-act-field">
          <label htmlFor="ms-decisions-because">Reason, on every goal</label>
          <input
            id="ms-decisions-because"
            type="text"
            value={reason}
            onChange={(event) => {
              setReason(event.target.value);
            }}
          />
        </div>
      )}
      <ul className="ms-decisions-plan">
        {plan.map((one) => (
          <li key={one.id} className={one.excluded === "" ? "ms-decisions-planned" : "ms-decisions-planned ms-decisions-planned--out"}>
            <span className="ms-mono ms-decisions-planned-id">{one.id}</span>
            <span className="ms-decisions-planned-title">{one.title}</span>
            {one.excluded === "" ? (
              <span className="ms-decisions-planned-budget">{budgetOf(one, approving, because)}</span>
            ) : (
              <span className="ms-decisions-planned-out">{one.excluded}</span>
            )}
          </li>
        ))}
      </ul>
    </Panel>
  );
}

/** What one line of the list says about what the act would carry. */
function budgetOf(one: Planned, approving: boolean, because: string): string {
  if (!approving) {
    return because === "" ? "waits for the reason above" : because;
  }
  const budget = one.budget;
  if (budget === null) {
    return "";
  }
  return `${budget.elapsedLimit} · ${String(budget.attemptLimit)} attempts · ${String(budget.reservedJobMinutesLimit)} reserved min · ${String(budget.activeJobLimit)} active · ${String(budget.reviewRoundLimit)} rounds — from ${one.source}`;
}

/** "1 goal", "12 goals": the sheet's own head counts what it would send. */
function goalsWord(sending: number): string {
  return `${String(sending)} ${sending === 1 ? "goal" : "goals"}`;
}

function blockedFor(act: "approve" | "park", sending: number, because: string): string {
  if (sending === 0) {
    return "Nothing here can be sent: every goal selected needs its budget first.";
  }
  if (act === "park" && because === "") {
    return "A park needs its reason: a pause without a why is a stall in disguise.";
  }
  return "";
}

function noteFor(act: "approve" | "park", sending: number): string {
  const goals = goalsWord(sending);
  return act === "approve"
    ? `One publication per goal, in this order, over ${goals}. The first refusal stops the run.`
    : `One publication per goal, in this order, over ${goals}, each with the reason above. The first refusal stops the run.`;
}
