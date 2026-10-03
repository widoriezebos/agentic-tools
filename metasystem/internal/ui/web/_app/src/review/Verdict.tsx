import { createContext, useContext, useState } from "react";

import { verdictLine } from "./room";
import { BacklogError, reviewGoal, type Backlog, type Row } from "../backlog/api";
import { useSession } from "../shell/identity";
import { Button } from "../shell/controls";
import { Trouble } from "../shell/Trouble";

/** What the board does with the ledger a card's act answered: the board it now is. */
export const VerdictActed = createContext<(after: Backlog) => void>(() => {});

/**
 * The goal's verdict on its card (g1-s69 §3, D2): "reviewed by Wido · clear to
 * land", "sent back by Wido · the seat that holds G revises on its next turn",
 * "attempt 3 started from your brief", or the holder asking which work — with a press per name that
 * performs the send-back again naming it. Everything it says is the goal's own
 * history, read as every seat reads it.
 */
export function CardVerdict({ row }: { row: Row }) {
  const acted = useContext(VerdictActed);
  const { askToSignIn } = useSession();
  const [pressing, setPressing] = useState("");
  const [refusal, setRefusal] = useState("");
  const verdict = row.verdict;
  const said = verdictLine(verdict, row.ref.id);
  if (verdict === undefined || said === "") {
    return null;
  }
  const name = (work: string) => {
    setPressing(work);
    setRefusal("");
    reviewGoal(row.ref.id, { record: verdict.record, verdict: "send-back", brief: "", work })
      .then(
        (answered) => {
          acted(answered.backlog);
        },
        (error: unknown) => {
          if (error instanceof BacklogError && error.signIn) {
            askToSignIn(() => {
              name(work);
            });
            return;
          }
          setRefusal(error instanceof Error ? error.message : String(error));
        },
      )
      .finally(() => {
        setPressing("");
      });
  };
  const asking = verdict.verdict === "send-back" && (verdict.candidates ?? []).length > 1;
  return (
    <div className="ms-card-verdict">
      <p className="ms-card-verdict-line">{said}</p>
      {asking && (
        <p className="ms-card-verdict-work" role="group" aria-label="Which work the holder revises">
          {(verdict.candidates ?? []).map((work) => (
            <Button key={work} disabled={pressing !== ""} onClick={() => { name(work); }}>
              {work}
            </Button>
          ))}
        </p>
      )}
      {refusal !== "" && (
        <Trouble text={refusal} role="status" variant="small" />
      )}
    </div>
  );
}

/**
 * A verdict the record's Outcome carries and the goal does not (Sol
 * SOL-S69-04), said after a reload stranded the act that began it: that it is
 * written in the review and not on the goal, and where it is given again —
 * under Your verdict, decided on the review as it stands now. It has no press
 * of its own: a verdict written earlier is never sent again from here, because
 * the review may have changed since (RULING-R-143-m1e of read 0096f159).
 */
export function PendingVerdict({ verdict }: { verdict: "clear-to-land" | "send-back" }) {
  const words = verdict === "send-back" ? "send it back" : "looks good, land it";
  return (
    <div className="ms-room-banner ms-send-back" role="status">
      <p className="ms-sitting-said">
        Your verdict is written in the review, and is not yet on the goal: {words}. It is not sent again by itself:
        give it again under Your verdict, where it is decided on the review as it stands now.
      </p>
    </div>
  );
}
