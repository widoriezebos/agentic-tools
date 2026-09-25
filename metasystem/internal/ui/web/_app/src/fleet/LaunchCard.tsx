import { useRef, useState } from "react";

import { launchMachine, ResourceError, type Launch, type LaunchStep } from "./api";
import {
  earliestReviewBy,
  failedRemedy,
  failedStep,
  foldedLine,
  healthCommand,
  IDLE_WARNING,
  launchHeadline,
  proposedReviewBy,
  retryAsksForTheWord,
  reviewByRefusal,
  reviewDay,
  stepTitle,
  TEMPORARY_RULE,
} from "./launching";
import { Button, Hint } from "../shell/controls";
import { useSession } from "../shell/identity";
import { failureMessage } from "../shell/workspace";

/**
 * The launch card: what is being made, how far it got, and what to do next.
 *
 * It draws the record and judges nothing. The steps are the verb's, in the
 * verb's order, each with the outcome the verb recorded and, under a failed
 * one, that owner's own sentence — a gate fence refusing the new clone's
 * build is a sentence a human acts on, and translating it here would lose it.
 *
 * A machine that joined folds to one line, because its row is in the table
 * below and a card repeating it is a card a reader learns to skip. A launch
 * that armed is its own state and not a failure: the machine is enrolled and
 * running, and only the confirmation is missing.
 */
export function LaunchCard({
  record,
  joined,
  now,
  onStarted,
  onDismiss,
}: {
  record: Launch;
  /** Whether the machine's own row is in the table below. */
  joined: boolean;
  now: Date;
  /** The record a retry answered with, which replaces this one. */
  onStarted: (started: Launch) => void;
  onDismiss: () => void;
}) {
  if (joined && record.outcome !== "failed") {
    return <Folded record={record} now={now} onDismiss={onDismiss} />;
  }
  return <Full record={record} onStarted={onStarted} />;
}

/** The one line a joined machine folds to, with the two commands beside it. */
function Folded({ record, now, onDismiss }: { record: Launch; now: Date; onDismiss: () => void }) {
  return (
    <section className="ms-launch-card ms-launch-card--folded">
      <p className="ms-launch-folded">{foldedLine(record, now)}</p>
      <Commands record={record} />
      <div className="ms-launch-actions">
        <Button onClick={onDismiss}>Dismiss</Button>
      </div>
    </section>
  );
}

function Full({ record, onStarted }: { record: Launch; onStarted: (started: Launch) => void }) {
  const stopped = failedStep(record);
  return (
    <section className={`ms-launch-card ms-launch-card--${record.outcome}`}>
      <h3 className="ms-launch-heading">{launchHeadline(record)}</h3>
      <p className="ms-launch-where">
        <span className="ms-mono">{record.destination}</span>
      </p>
      <ol className="ms-launch-steps">
        {record.steps.map((step) => (
          <StepRow key={step.step} step={step} />
        ))}
      </ol>
      {record.orientation !== "" && <p className="ms-launch-hint">Next work there: {record.orientation}</p>}
      {record.outcome === "armed" && (
        <p className="ms-launch-hint">
          Read its health: <span className="ms-mono">{healthCommand(record)}</span>
        </p>
      )}
      {record.reviewBy !== "" && (
        <p className="ms-launch-hint">Temporary enrollment, review due {reviewDay(record.reviewBy)}.</p>
      )}
      {record.outcome !== "failed" && <Commands record={record} />}
      {stopped !== null && <Retry record={record} onStarted={onStarted} />}
    </section>
  );
}

function StepRow({ step }: { step: LaunchStep }) {
  const title = stepTitle(step);
  const name = <span className="ms-launch-step-name">{step.step}</span>;
  return (
    <li className={`ms-launch-step ms-launch-step--${step.outcome}`}>
      <span className={`ms-launch-dot ms-launch-dot--${step.outcome}`} aria-hidden="true" />
      {title === "" ? name : <Hint label={title}>{name}</Hint>}
      <span className="ms-launch-step-outcome">{step.outcome}</span>
      {step.words !== "" && <span className="ms-launch-step-words">{step.words}</span>}
    </li>
  );
}

/** The two commands a human has for a machine that is up. */
function Commands({ record }: { record: Launch }) {
  return (
    <div className="ms-launch-commands">
      <p className="ms-launch-hint">{IDLE_WARNING}</p>
      <p className="ms-launch-command">
        Start a session: <span className="ms-mono">{record.next.session}</span>
      </p>
      <p className="ms-launch-command">
        Stop the machine: <span className="ms-mono">{record.next.stop}</span>
      </p>
    </div>
  );
}

/**
 * Retry: this launch again, by its id.
 *
 * It asks for the word and the date again whenever the launch still has to
 * enroll, because the record never held them — that is what keeps an
 * authorization out of a file this page can read. Everything else the resume
 * verifies for itself: each done step's postcondition, and what this launch
 * created.
 */
function Retry({ record, onStarted }: { record: Launch; onStarted: (started: Launch) => void }) {
  const now = useRef(new Date()).current;
  const asks = retryAsksForTheWord(record);
  const [word, setWord] = useState("");
  const [reviewBy, setReviewBy] = useState(() => (record.reviewBy === "" ? proposedReviewBy(now) : record.reviewBy));
  const [sending, setSending] = useState(false);
  const [refusal, setRefusal] = useState("");
  const { askToSignIn } = useSession();
  const retried = useRef(false);
  const dateRefused = reviewByRefusal(reviewBy, now);
  const blocked = asks && (word.trim() === "" || dateRefused !== "");

  const send = () => {
    setSending(true);
    setRefusal("");
    launchMachine({ resume: record.launch, word, reviewBy })
      .then((started) => {
        setSending(false);
        onStarted(started);
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof ResourceError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  return (
    <div className="ms-launch-retry">
      <p className="ms-launch-hint">{failedRemedy(record)}</p>
      {asks && (
        <>
          <div className="ms-launch-field">
            <label htmlFor="ms-retry-word">Your authorization, again</label>
            <textarea
              id="ms-retry-word"
              rows={2}
              value={word}
              onChange={(event) => {
                setWord(event.target.value);
              }}
            />
            <p className="ms-launch-hint">
              The record never held it, so this launch asks again. {TEMPORARY_RULE}
            </p>
          </div>
          <div className="ms-launch-field">
            <label htmlFor="ms-retry-review">Review by</label>
            <input
              id="ms-retry-review"
              type="date"
              min={earliestReviewBy(now)}
              value={reviewBy}
              onChange={(event) => {
                setReviewBy(event.target.value);
              }}
            />
            {dateRefused !== "" && <p className="ms-launch-refusal">{dateRefused}</p>}
          </div>
        </>
      )}
      {refusal !== "" && <p className="ms-launch-refusal">{refusal}</p>}
      <div className="ms-launch-actions">
        <Button primary disabled={blocked || sending} onClick={send}>
          Retry
        </Button>
      </div>
    </div>
  );
}
