import { useContext, useId, useState } from "react";

import { gateLine, LAND_WITHOUT_SITTING, LAND_WITHOUT_SITTING_SAID, offersLandWithoutSitting, REASON_NEEDED } from "./gating";
import { VerdictActed } from "./Verdict";
import { BacklogError, landWithoutSitting, type Backlog, type Row } from "../backlog/api";
import { Help } from "../help/Help";
import { useSession } from "../shell/identity";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import { Trouble } from "../shell/Trouble";

/** Who this human is, as a hold's `by` names them, or "". */
function useYou(): string {
  const { session } = useSession();
  return session.state === "known" ? session.session.human : "";
}

/**
 * The landing gate on a Review lane card (g1-s70 D5, §3): "eligible to land in
 * 3h 12m", "eligible to land, waiting for the holder", "held by your sitting",
 * or "waits for your review" with Land without a sitting beside Review it. The
 * words are the server's reading and the render's own instant; nothing ticks.
 */
export function CardGate({ row, now = new Date() }: { row: Row; now?: Date }) {
  const acted = useContext(VerdictActed);
  const you = useYou();
  const said = gateLine(row.gate, you, now);
  if (said === "") {
    return null;
  }
  return (
    <div className="ms-card-gate">
      <p className="ms-card-gate-line">{said}</p>
      {offersLandWithoutSitting(row.gate) && <LandWithoutSittingPress goal={row.ref.id} onDone={acted} />}
    </div>
  );
}

/** The press and the Decide sheet it opens, wherever the goal waits for this human. */
export function LandWithoutSittingPress({ goal, onDone }: { goal: string; onDone: (after: Backlog) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        onClick={() => {
          setOpen(true);
        }}
      >
        {LAND_WITHOUT_SITTING}
      </Button>
      <Help id="landing-gate" />
      {open && (
        <LandWithoutSittingSheet
          goal={goal}
          open={open}
          onOpenChange={setOpen}
          onDone={(after) => {
            setOpen(false);
            onDone(after);
          }}
        />
      )}
    </>
  );
}

/**
 * The Decide sheet (g1-s70 D4): the reason is required, and the press waits
 * until there is one. A refusal for want of a sign-in opens the sign-in sheet
 * and presses again once; any other refusal is said in the engine's words.
 */
export function LandWithoutSittingSheet({
  goal,
  open,
  onOpenChange,
  onDone,
}: {
  goal: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: (after: Backlog) => void;
}) {
  const { askToSignIn } = useSession();
  const [reason, setReason] = useState("");
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState("");
  const title = `${LAND_WITHOUT_SITTING}: ${goal}`;
  const press = (again: boolean) => {
    setSending(true);
    setRefusal("");
    landWithoutSitting(goal, reason.trim())
      .then(onDone, (error: unknown) => {
        if (again && error instanceof BacklogError && error.signIn) {
          askToSignIn(() => {
            press(false);
          });
          return;
        }
        setRefusal(error instanceof Error ? error.message : String(error));
      })
      .finally(() => {
        setSending(false);
      });
  };
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      side="right"
      label={title}
      title={title}
      closeLabel="Close without deciding"
      bodyClassName="ms-sitting-sheet"
      sheetName={LAND_WITHOUT_SITTING}
    >
      <DecideBody
        reason={reason}
        onReason={setReason}
        sending={sending}
        refusal={refusal}
        onPress={() => {
          press(true);
        }}
      />
    </Sheet>
  );
}

/**
 * What the Decide sheet holds: the one field and its press. It is its own
 * component because the sheet renders it into a portal, and what it says is
 * what a test reads.
 */
export function DecideBody({
  reason,
  onReason,
  sending,
  refusal,
  onPress,
}: {
  reason: string;
  onReason: (reason: string) => void;
  sending: boolean;
  refusal: string;
  onPress: () => void;
}) {
  const field = useId();
  return (
    <>
      <p className="ms-sitting-said">{LAND_WITHOUT_SITTING_SAID}</p>
      <label className="ms-sitting-label" htmlFor={field}>
        Why this goal needs no sitting
      </label>
      <textarea
        id={field}
        className="ms-deposit-field"
        rows={3}
        value={reason}
        onChange={(event) => {
          onReason(event.target.value);
        }}
      />
      {reason.trim() === "" && (
        <p className="ms-deposit-needs" role="status">
          {REASON_NEEDED}
        </p>
      )}
      {refusal !== "" && <Trouble text={refusal} role="status" variant="small" />}
      <div className="ms-sitting-foot">
        <Button primary disabled={reason.trim() === "" || sending} onClick={onPress}>
          {sending ? "Deciding…" : LAND_WITHOUT_SITTING}
        </Button>
      </div>
    </>
  );
}
