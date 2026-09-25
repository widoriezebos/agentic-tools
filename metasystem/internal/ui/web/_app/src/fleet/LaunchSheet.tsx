import { useMemo, useRef, useState } from "react";

import { launchMachine, ResourceError, type Launch, type Launching, type Machine } from "./api";
import {
  blockedForLaunch,
  clientToday,
  destinationRefusal,
  earliestReviewBy,
  IDLE_WARNING,
  nicknameRefusal,
  nicknamesTaken,
  proposedDestination,
  proposedNickname,
  proposedReviewBy,
  reviewByRefusal,
  TEMPORARY_RULE,
  WHAT_IT_GETS,
  WHAT_IT_WILL_NOT_DO,
  type LaunchDraft,
} from "./launching";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Sheet } from "../shell/Sheet";
import { failureMessage } from "../shell/workspace";

/**
 * Launch a machine: the form, before the act.
 *
 * It is a work-area sheet, so the Project Partner stays live beside it and a
 * half-filled form is never lost to a stray click. Everything it refuses, it
 * refuses by the engine's own rules — the nickname the presence publisher
 * takes, an absolute path, a review date that is not already behind us — and
 * everything it proposes, the human may change.
 *
 * One field is different from every other field in this interface: the
 * authorization. It is the human's own words, it enrolls the machine, and it
 * travels to the verb's argument list and nowhere else — not to the record,
 * not to this sheet's draft, and not into the Partner's capture.
 */
export function LaunchSheet({
  machines,
  launches,
  thisSeat,
  where,
  onClose,
  onStarted,
}: {
  machines: readonly Machine[];
  launches: readonly Launch[];
  thisSeat: string;
  where: Launching;
  onClose: () => void;
  /** The record the server wrote before anything ran, which becomes the card. */
  onStarted: (started: Launch) => void;
}) {
  const now = useMemo(() => new Date(), []);
  const taken = useMemo(() => nicknamesTaken(machines, launches), [machines, launches]);
  const proposed = useMemo(() => proposedNickname(thisSeat, taken), [thisSeat, taken]);
  const [draft, setDraft] = useState<LaunchDraft>(() => ({
    machine: proposed,
    destination: proposedDestination(where, proposed),
    word: "",
    reviewBy: proposedReviewBy(now),
  }));
  // Whether the human has taken the path over. Until they do, the proposal
  // follows the nickname, because a path named for another nickname is a
  // machine landing in the wrong directory.
  const [ownPath, setOwnPath] = useState(false);
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState("");
  const { askToSignIn } = useSession();
  const retried = useRef(false);

  const nameIt = (machine: string) => {
    setDraft((held) => ({
      ...held,
      machine,
      destination: ownPath ? held.destination : proposedDestination(where, machine),
    }));
  };

  const blocked = blockedForLaunch(draft, taken, thisSeat, now);

  const send = () => {
    setSending(true);
    setRefusal("");
    launchMachine({
      machine: draft.machine.trim(),
      destination: draft.destination.trim(),
      word: draft.word,
      reviewBy: draft.reviewBy,
      // The day this browser is on. The server judges the review date
      // against it rather than against its own, so one rule decides at both
      // ends whatever zone either is in.
      today: clientToday(new Date()),
    })
      .then((started) => {
        setSending(false);
        onStarted(started);
      })
      .catch((error: unknown) => {
        setSending(false);
        // A server that found no signed-in human says the remedy is here: the
        // sheet opens, and the act is made once more under the session.
        if (error instanceof ResourceError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  return (
    <Sheet
      open
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      side="right"
      label="Launch a machine on this host"
      title="Launch a machine"
      closeLabel="Close"
      bodyClassName="ms-sheet-body--launch"
      sheetName="Launch a machine"
    >
      <div className="ms-launch-field">
        <label htmlFor="ms-launch-machine">
          Nickname
          <Help id="launch-machine" />
        </label>
        <input
          id="ms-launch-machine"
          type="text"
          autoComplete="off"
          value={draft.machine}
          onChange={(event) => {
            nameIt(event.target.value);
          }}
        />
        <Refused said={nicknameRefusal(draft.machine, taken, thisSeat)} />
      </div>

      <div className="ms-launch-field">
        <label htmlFor="ms-launch-destination">Where</label>
        <input
          id="ms-launch-destination"
          type="text"
          autoComplete="off"
          spellCheck={false}
          value={draft.destination}
          onChange={(event) => {
            setOwnPath(true);
            setDraft((held) => ({ ...held, destination: event.target.value }));
          }}
        />
        <p className="ms-launch-hint">An absolute path on this host, beside this checkout. It must not exist yet.</p>
        <Refused said={destinationRefusal(draft.destination)} />
      </div>

      <section className="ms-launch-block">
        <h3 className="ms-launch-heading">What it gets</h3>
        <p className="ms-launch-hint">{WHAT_IT_GETS}</p>
        <p className="ms-launch-hint">{WHAT_IT_WILL_NOT_DO}</p>
        <p className="ms-launch-hint">{IDLE_WARNING}</p>
      </section>

      <section className="ms-launch-block">
        <h3 className="ms-launch-heading">
          Your authorization
          <Help id="temporary-word" />
        </h3>
        <div className="ms-launch-field">
          <label htmlFor="ms-launch-word">In your own words</label>
          <textarea
            id="ms-launch-word"
            rows={3}
            value={draft.word}
            onChange={(event) => {
              setDraft((held) => ({ ...held, word: event.target.value }));
            }}
          />
          <p className="ms-launch-hint">{TEMPORARY_RULE}</p>
        </div>
        <div className="ms-launch-field">
          <label htmlFor="ms-launch-review">
            Review by
            <Help id="review-by" />
          </label>
          <input
            id="ms-launch-review"
            type="date"
            min={earliestReviewBy(now)}
            value={draft.reviewBy}
            onChange={(event) => {
              setDraft((held) => ({ ...held, reviewBy: event.target.value }));
            }}
          />
          <Refused said={reviewByRefusal(draft.reviewBy, now)} />
        </div>
      </section>

      {refusal !== "" && <p className="ms-launch-refusal">{refusal}</p>}
      {blocked !== "" && <p className="ms-launch-hint">{blocked}</p>}
      <div className="ms-launch-actions">
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          Launch
        </Button>
      </div>
    </Sheet>
  );
}

function Refused({ said }: { said: string }) {
  if (said === "") {
    return null;
  }
  return <p className="ms-launch-refusal">{said}</p>;
}
