import { useEffect, useState } from "react";

import { CandidateError, loadCandidate, startCandidate, stopCandidate, type Candidate } from "./candidate";
import { candidateHref, pillOf, pressSignedIn } from "./room";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Trouble } from "../shell/Trouble";

/**
 * The candidate's pill in the room's header (g1-s69 D3): what app status says
 * of the goal's candidate run, compared with the tip the room reviews, and Run
 * and Stop as the human's own acts. It reads once when the room opens on a goal
 * and once after each press, and at no other time.
 */
export function CandidatePill({ goal, reviewed }: { goal: string; reviewed: string }) {
  const [read, setRead] = useState<Candidate | { refusal: string; code: string } | null>(null);
  const [acting, setActing] = useState(false);
  const [refusal, setRefusal] = useState("");
  const { askToSignIn } = useSession();

  useEffect(() => {
    if (goal === "") {
      return;
    }
    const aborter = new AbortController();
    loadCandidate(goal, aborter.signal).then(setRead, (error: unknown) => {
      if (!aborter.signal.aborted) {
        setRead(refusalOf(error));
      }
    });
    return () => {
      aborter.abort();
    };
  }, [goal]);

  // Run and Stop are the human's own acts: a press refused for want of a
  // sign-in opens the sign-in sheet, and signing in presses it again, once
  // (Sol SOL-S69-03).
  const press = (act: (goal: string) => Promise<Candidate>) => {
    void pressSignedIn({
      act: () => {
        setActing(true);
        setRefusal("");
        return act(goal);
      },
      done: () => {
        void loadCandidate(goal)
          .then(setRead, (error: unknown) => {
            setRead(refusalOf(error));
          })
          .finally(() => {
            setActing(false);
          });
      },
      refused: (error) => {
        setActing(false);
        setRefusal(refusalOf(error).refusal);
      },
      signIn: askToSignIn,
    });
  };

  const pill = pillOf(read, reviewed);
  const href = candidateHref(pill.address);
  return (
    <span className="ms-candidate" data-state={pill.state} aria-disabled={pill.state === "no-contract"}>
      <span className="ms-candidate-words" role="status" title={pill.state === "no-contract" ? pill.words : undefined}>
        {acting && pill.run ? "the candidate is starting…" : pill.words}
      </span>
      {href !== "" && (pill.state === "reviewed" || pill.state === "moved") && (
        <a className="ms-candidate-open" href={href} target="_blank" rel="noopener noreferrer">
          Open {pill.address}
        </a>
      )}
      {pill.run && (
        <Button disabled={acting} onClick={() => { press(startCandidate); }}>
          Run
        </Button>
      )}
      {pill.stop && (
        <Button disabled={acting} onClick={() => { press(stopCandidate); }}>
          Stop
        </Button>
      )}
      <Help id="the-candidate" />
      {refusal !== "" && (
        <Trouble text={refusal} role="status" as="span" variant="small" />
      )}
    </span>
  );
}

function refusalOf(error: unknown): { refusal: string; code: string } {
  if (error instanceof CandidateError) {
    return { refusal: error.message, code: error.code };
  }
  return { refusal: error instanceof Error ? error.message : String(error), code: "" };
}
