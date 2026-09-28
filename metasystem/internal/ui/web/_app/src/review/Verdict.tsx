import { createContext, useContext, useState } from "react";

import { verdictLine } from "./room";
import { BacklogError, reviewGoal, type Backlog, type Row } from "../backlog/api";
import { useSession } from "../shell/identity";
import { Button } from "../shell/controls";

/** What the board does with the ledger a card's act answered: the board it now is. */
export const VerdictActed = createContext<(after: Backlog) => void>(() => {});

/**
 * The goal's verdict on its card (g1-s69 §3, D2): "reviewed by Wido · clear to
 * land", "sent back by Wido · awaiting the holder", "attempt 3 started from
 * your brief", or the holder asking which work — with a press per name that
 * performs the send-back again naming it. Everything it says is the goal's own
 * history, read as every seat reads it.
 */
export function CardVerdict({ row }: { row: Row }) {
  const acted = useContext(VerdictActed);
  const { askToSignIn } = useSession();
  const [pressing, setPressing] = useState("");
  const [refusal, setRefusal] = useState("");
  const verdict = row.verdict;
  const said = verdictLine(verdict);
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
        <p className="ms-deposit-refusal" role="status">
          {refusal}
        </p>
      )}
    </div>
  );
}
