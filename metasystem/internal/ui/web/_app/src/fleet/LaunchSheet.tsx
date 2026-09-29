import { useMemo, useRef, useState } from "react";

import { launchMachine, ResourceError, type Launch, type Launching, type Machine } from "./api";
import {
  blockedForLaunch,
  destinationRefusal,
  IDLE_WARNING,
  nicknameRefusal,
  nicknamesTaken,
  proposedDestination,
  proposedNickname,
  WHAT_IT_GETS,
  WHAT_IT_WILL_NOT_DO,
  type LaunchDraft,
} from "./launching";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Sheet } from "../shell/Sheet";
import { failureCode, failureMessage } from "../shell/workspace";
import { Trouble } from "../shell/Trouble";

/**
 * Launch a machine: the form, before the act.
 *
 * It is a work-area sheet, so the Project Partner stays live beside it and a
 * half-filled form is never lost to a stray click. Everything it refuses, it
 * refuses by the engine's own rules — the nickname the presence publisher
 * takes, an absolute path — and everything it proposes, the human may change.
 *
 * Nothing here asks who is launching or why. The signed-in session is the
 * human's own enrollment (g1-s72): the server stamps the new machine's record
 * from the session's proof, and a browser nobody is signed into is answered
 * with the sign-in sheet when Launch is pressed.
 */
export function LaunchSheet({
  machines,
  launches,
  hidden,
  thisSeat,
  where,
  onClose,
  onStarted,
}: {
  machines: readonly Machine[];
  launches: readonly Launch[];
  /** Launches this page has discarded before the server's reading says so. */
  hidden?: ReadonlySet<string>;
  thisSeat: string;
  where: Launching;
  onClose: () => void;
  /** The record the server wrote before anything ran, which becomes the card. */
  onStarted: (started: Launch) => void;
}) {
  const taken = useMemo(() => nicknamesTaken(machines, launches, hidden), [machines, launches, hidden]);
  const proposed = useMemo(() => proposedNickname(thisSeat, taken), [thisSeat, taken]);
  const [draft, setDraft] = useState<LaunchDraft>(() => ({
    machine: proposed,
    destination: proposedDestination(where, proposed),
  }));
  // Whether the human has taken the path over. Until they do, the proposal
  // follows the nickname, because a path named for another nickname is a
  // machine landing in the wrong directory.
  const [ownPath, setOwnPath] = useState(false);
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState("");
  // The code the launch was refused under, where it was refused under one (Sol SOL-S68-03).
  const [refusalCode, setRefusalCode] = useState("");
  const { askToSignIn } = useSession();
  const retried = useRef(false);

  const nameIt = (machine: string) => {
    setDraft((held) => ({
      ...held,
      machine,
      destination: ownPath ? held.destination : proposedDestination(where, machine),
    }));
  };

  const blocked = blockedForLaunch(draft, taken, thisSeat);

  const send = () => {
    setSending(true);
    setRefusal("");
    launchMachine({
      machine: draft.machine.trim(),
      destination: draft.destination.trim(),
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
        setRefusalCode(failureCode(error));
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

      {refusal !== "" && <Trouble text={refusal} code={refusalCode} variant="small" />}
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
  return <Trouble text={said} variant="small" />;
}
