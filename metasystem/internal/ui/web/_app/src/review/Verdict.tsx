import { createContext, useContext, useState } from "react";

import { verdictLine, type ToPerform } from "./room";
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
 * A verdict the record's Outcome carries and the goal does not, offered again
 * after a reload stranded the act Record it began (Sol SOL-S69-04): the
 * verdict, a send-back's brief as it will travel, and the press that records it
 * on the goal. A refusal is said by the room's refusal line, with the engine's
 * words.
 */
export function PendingVerdict({ pending, busy, onPress }: { pending: ToPerform; busy: boolean; onPress: () => void }) {
  const words = pending.asked.verdict === "send-back" ? "send back" : "clear to land";
  return (
    <div className="ms-room-banner ms-send-back" role="status">
      <p className="ms-sitting-said">The Outcome is recorded, and the verdict is not yet on the goal: {words}.</p>
      {pending.asked.brief !== "" && (
        <pre className="ms-send-back-brief" aria-label="The correction brief">
          {pending.asked.brief}
        </pre>
      )}
      <p>
        <Button primary disabled={busy} onClick={onPress}>
          Record the verdict on the goal
        </Button>
      </p>
    </div>
  );
}
