import { useEffect, useState } from "react";

import { CandidateError, loadCandidate, startCandidate, stopCandidate, type Candidate } from "./candidate";
import { candidateHref, pillOf, pressSignedIn } from "./room";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Trouble } from "../shell/Trouble";

/**
 * Try it (review-findings-read-as-decisions §3, step 3): the version under
 * review, started on this computer so the person can click through it, in
 * words — what Start does and where, Open it, and Stop. It runs the exact
 * version the page shows, not the branch or the review as they moved since
 * (RF-04, fix round 3 F-1): every read and press names that commit, and the
 * server runs exactly it, or says why it cannot. It reads once when the room opens and once after each press,
 * and at no other time. Start and Stop are the person's own acts under their
 * signed-in session; and where what runs is not the version on screen, it says so.
 */
export function TryIt({ goal, reviewed, lead }: { goal: string; reviewed: string; lead?: string }) {
  const [read, setRead] = useState<Candidate | { refusal: string; code: string } | null>(null);
  const [acting, setActing] = useState(false);
  // What Start or Stop was refused with, and the code it was refused under, so the
  // trouble carries the code the Partner reads the register by (Sol SOL-S68-03).
  const [refusal, setRefusal] = useState({ refusal: "", code: "" });
  const { askToSignIn } = useSession();

  useEffect(() => {
    if (goal === "") {
      return;
    }
    const aborter = new AbortController();
    loadCandidate(goal, reviewed, aborter.signal).then(setRead, (error: unknown) => {
      if (!aborter.signal.aborted) {
        setRead(refusalOf(error));
      }
    });
    return () => {
      aborter.abort();
    };
  }, [goal, reviewed]);

  // A press refused for want of a sign-in opens the sign-in sheet, and signing
  // in presses it again, once (Sol SOL-S69-03).
  const press = (act: (goal: string, commit: string) => Promise<Candidate>) => {
    void pressSignedIn({
      act: () => {
        setActing(true);
        setRefusal({ refusal: "", code: "" });
        return act(goal, reviewed);
      },
      done: () => {
        void loadCandidate(goal, reviewed)
          .then(setRead, (error: unknown) => {
            setRead(refusalOf(error));
          })
          .finally(() => {
            setActing(false);
          });
      },
      refused: (error) => {
        setActing(false);
        setRefusal(refusalOf(error));
      },
      signIn: askToSignIn,
    });
  };

  const pill = pillOf(read, reviewed);
  const href = candidateHref(pill.address);
  return (
    <div className="ms-try" data-state={pill.state} aria-disabled={pill.state === "no-contract"}>
      <p className="ms-try-words" role="status">
        {acting && pill.run ? "Starting this version…" : pill.state === "stopped" && lead !== undefined ? lead : pill.words}
        <Help id="the-candidate" />
      </p>
      <p className="ms-try-presses">
        {pill.run && (
          <Button primary disabled={acting} onClick={() => { press(startCandidate); }}>
            Start
          </Button>
        )}
        {href !== "" && (pill.state === "reviewed" || pill.state === "moved") && (
          <a className="ms-try-open" href={href} target="_blank" rel="noopener noreferrer">
            Open it
          </a>
        )}
        {pill.stop && (
          <Button disabled={acting} onClick={() => { press(stopCandidate); }}>
            Stop
          </Button>
        )}
      </p>
      {refusal.refusal !== "" && (
        <Trouble text={refusal.refusal} code={refusal.code} role="status" as="span" variant="small" />
      )}
    </div>
  );
}

function refusalOf(error: unknown): { refusal: string; code: string } {
  if (error instanceof CandidateError) {
    return { refusal: error.message, code: error.code };
  }
  return { refusal: error instanceof Error ? error.message : String(error), code: "" };
}
