import { type ReactNode, useRef, useState } from "react";

import { dateAndTime, minuteTime } from "../backlog/format";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Trouble } from "../shell/Trouble";
import { failureMessage, landNow, ResourceError } from "./api";
import type { Lane, LaneEntry, LandNowAnswer, LaneOwner, LaneOwnerState, LaneProof, LanePush, LaneRunningProof } from "./api";

/**
 * The host's landing lane (U12; Wido 2026-09-29: "the status of the landing
 * component on the fleet page"): the plain lane, as landing status --json
 * carries it — paused or not, whether its landing agent is alive, the queue
 * of hand-ins, the proof running, and the last proof and push.
 *
 * It is drawn from the same /api/board response the host board is, so it
 * adds no request. The panel shows; the `metasystem landing` verbs act, so
 * where a person has something to do the panel prints the command as text to
 * copy. Its one act is Land now (goal fleet-card-can-land-now; Wido
 * 2026-10-01: "I want a verb that does that (from any seat) and from the UI
 * especially"): it runs `metasystem landing run` through the server and shows
 * the verb's two lines. It is offered only when a hand-in waits, the lane is
 * not paused and no landing agent is alive, and says in one line why not
 * where work waits all the same.
 *
 * A server built before the lane existed sends no `lane` field at all; the
 * panel says it does not report one rather than claiming the host has none.
 */

/** What a person runs to resume a lane a person stopped. */
export const START_COMMAND = "metasystem landing start";

/** The status colour each owner state takes, in the tokens the page uses. */
const TONE: Record<LaneOwnerState, "ok" | "warn" | "bad" | "neutral"> = {
  running: "ok",
  idle: "neutral",
  stopped: "bad",
  unready: "warn",
};

function toneOf(state: string): string {
  return (TONE as Record<string, string | undefined>)[state] ?? "neutral";
}

/**
 * An instant a person reads, in local time: the clock alone when it falls on
 * today, the day as well when it does not.
 */
function when(stamp: string, now: Date): string {
  const at = new Date(stamp);
  if (Number.isNaN(at.getTime())) {
    return minuteTime(stamp);
  }
  return at.toDateString() === now.toDateString() ? minuteTime(stamp) : dateAndTime(stamp);
}

export function LaneBlock({
  lane,
  now = new Date(),
  onLanded,
}: {
  lane: Lane | null | undefined;
  now?: Date;
  /** Called after Land now answered, so the card is read again. */
  onLanded?: () => void;
}) {
  if (lane === undefined) {
    return (
      <Panel>
        <p className="ms-fleet-quiet">This server does not report the landing lane.</p>
      </Panel>
    );
  }
  if (lane === null) {
    return (
      <Panel>
        <p className="ms-fleet-quiet">No landing lane is registered on this host.</p>
      </Panel>
    );
  }
  return (
    <Panel>
      <p className="ms-fleet-lane-summary">{lane.summary}</p>
      <Owner owner={lane.owner} now={now} />
      <State lane={lane} />
      <Queue queue={lane.queue} />
      {lane.running_proof !== undefined && lane.running_proof !== null && (
        <Proving proof={lane.running_proof} now={now} />
      )}
      {lane.last_proof !== undefined && lane.last_proof !== null && <LastProof proof={lane.last_proof} now={now} />}
      {lane.last_push !== undefined && lane.last_push !== null && <LastPush push={lane.last_push} now={now} />}
      <LandNow lane={lane} onLanded={onLanded} />
      <Root lane={lane} now={now} />
    </Panel>
  );
}

function Panel({ children }: { children: ReactNode }) {
  return (
    <section className="ms-fleet-block ms-fleet-lane" aria-label="Landing lane">
      <h2 className="ms-fleet-heading">Landing lane</h2>
      {children}
    </section>
  );
}

function Owner({ owner, now }: { owner: LaneOwner; now: Date }) {
  const facts: string[] = [];
  if (owner.since !== null && owner.since !== "") {
    facts.push(`since ${when(owner.since, now)}`);
  }
  if (owner.pid !== null) {
    facts.push(`pid ${String(owner.pid)}`);
  }
  // Idle is the normal state of a ready lane with nothing to land: it gets no
  // prompt. Unready says why and what fixes it in retry_hint. Only a lane a
  // person stopped is resumed with landing start.
  const needsStart = owner.state === "stopped";
  return (
    <div className="ms-fleet-lane-owner">
      <p className="ms-fleet-lane-line">
        <span className="ms-fleet-lane-label">Owner</span>
        <span className={`ms-fleet-pill ms-fleet-lane-state ms-fleet-lane-state--${toneOf(owner.state)}`}>
          {owner.state.replace("-", " ")}
        </span>
        {facts.length > 0 && <span className="ms-fleet-lane-facts">{facts.join(" · ")}</span>}
      </p>
      {owner.stopped_by !== null && owner.stopped_by !== "" && (
        <p className="ms-fleet-lane-note">
          Stopped by {owner.stopped_by}
          {owner.stopped_because !== undefined && owner.stopped_because !== "" ? `: ${owner.stopped_because}` : ""}.
        </p>
      )}
      {owner.last_exit !== null && owner.last_exit !== "" && (
        <p className="ms-fleet-lane-note">
          Last exit: <span className="ms-mono">{owner.last_exit}</span>
        </p>
      )}
      {owner.retry_hint !== null && owner.retry_hint !== "" && (
        <p className="ms-fleet-lane-note">{owner.retry_hint}</p>
      )}
      {needsStart && (
        <p className="ms-fleet-lane-note">
          To resume it, run <code className="ms-mono ms-fleet-lane-command">{START_COMMAND}</code>
        </p>
      )}
    </div>
  );
}

/** A commit or tree id as a person reads it: its first seven characters. */
function short(id: string): string {
  return id.slice(0, 7);
}

/** Whether the lane is paused, and whether its landing agent is alive. */
function State({ lane }: { lane: Lane }) {
  if (lane.paused === undefined && lane.agent_alive === undefined) {
    return null;
  }
  return (
    <p className="ms-fleet-lane-line">
      <span className="ms-fleet-lane-label">Lane</span>
      <span className="ms-fleet-pill">{lane.paused === true ? "paused" : "not paused"}</span>
      <span className="ms-fleet-lane-facts">{lane.agent_alive === true ? "agent alive" : "no agent running"}</span>
    </p>
  );
}

/**
 * The queue as the terminal shows it: the hand-ins that wait or were
 * returned, with their seat, state and reason; the landed and superseded ones
 * are counted and not listed.
 */
function Queue({ queue }: { queue: LaneEntry[] | undefined }) {
  if (queue === undefined) {
    return null;
  }
  const shown = queue.filter((entry) => entry.state !== "landed" && entry.state !== "superseded");
  const hidden = queue.length - shown.length;
  return (
    <div className="ms-fleet-lane-queue">
      <p className="ms-fleet-lane-sub">Queue</p>
      {shown.length === 0 ? (
        <p className="ms-fleet-quiet">Nothing waits in the queue.</p>
      ) : (
        <ul className="ms-fleet-lane-members">
          {shown.map((entry) => (
            <li key={`${entry.goal}@${entry.seat}@${entry.sha}`} className="ms-fleet-lane-member">
              <span className="ms-mono">{entry.goal}</span> <span className="ms-fleet-lane-seat">@ {entry.seat}</span>{" "}
              <span className="ms-fleet-pill ms-fleet-lane-entry-state">{entry.state}</span>
              {entry.reason !== undefined && entry.reason !== "" && (
                <span className="ms-fleet-lane-facts">{entry.reason}</span>
              )}
            </li>
          ))}
        </ul>
      )}
      {hidden > 0 && <p className="ms-fleet-quiet">{`${String(hidden)} landed or superseded not shown`}</p>}
    </div>
  );
}

function Proving({ proof, now }: { proof: LaneRunningProof; now: Date }) {
  return (
    <p className="ms-fleet-lane-line">
      <span className="ms-fleet-lane-label">Proving</span>
      {proof.state === "died" ? (
        <span>
          tree <span className="ms-mono">{short(proof.tree)}</span> (attempt {proof.attempt}) died without a result; the
          next proof runs it again
        </span>
      ) : (
        <span>
          tree <span className="ms-mono">{short(proof.tree)}</span> as attempt {proof.attempt}
          <span className="ms-fleet-lane-facts">since {when(proof.since, now)}</span>
        </span>
      )}
    </p>
  );
}

function LastProof({ proof, now }: { proof: LaneProof; now: Date }) {
  return (
    <p className="ms-fleet-lane-line">
      <span className="ms-fleet-lane-label">Last proof</span>
      <span className="ms-fleet-pill">{proof.result}</span>
      <span className="ms-mono">{short(proof.commit)}</span>
      {proof.reason !== undefined && proof.reason !== "" && <span className="ms-fleet-lane-facts">{proof.reason}</span>}
      <span className="ms-fleet-lane-facts">at {when(proof.at, now)}</span>
    </p>
  );
}

function LastPush({ push, now }: { push: LanePush; now: Date }) {
  return (
    <p className="ms-fleet-lane-line">
      <span className="ms-fleet-lane-label">Last push</span>
      <span className="ms-mono">{`${short(push.old)} → ${short(push.commit)}`}</span>
      <span className="ms-fleet-lane-facts">at {when(push.at, now)}</span>
    </p>
  );
}

function Root({ lane, now }: { lane: Lane; now: Date }) {
  if (lane.root === null || lane.root === "") {
    return <p className="ms-fleet-provenance">No lane root is registered.</p>;
  }
  const by = lane.registered_by !== null && lane.registered_by !== "" ? ` by ${lane.registered_by}` : "";
  const at = lane.registered_at !== null && lane.registered_at !== "" ? ` at ${when(lane.registered_at, now)}` : "";
  return (
    <p className="ms-fleet-provenance ms-fleet-lane-root">
      root <span className="ms-mono">{lane.root}</span>
      {by !== "" || at !== "" ? ` · registered${by}${at}` : ""}
    </p>
  );
}

/** Whether Land now is offered, and the one line that says why not. */
export type LandNowOffer = { offered: boolean; reason: string };

/**
 * Whether a hand-in waits in the queue. A server that sends no queue is read
 * by the keeper's wake reasons instead.
 */
function hasWaitingWork(lane: Lane): boolean {
  if (lane.queue !== undefined) {
    return lane.queue.some((entry) => entry.state === "waiting");
  }
  return lane.wake !== undefined && lane.wake !== null && lane.wake.reasons.length > 0;
}

/**
 * Land now is offered when a hand-in waits, the lane is not paused and no
 * landing agent is alive. With nothing waiting it is not drawn at all; with
 * a hand-in waiting and an agent alive, the lane paused, or the lane unable
 * to run (unready), it says in one line why it is not offered. Everything
 * else — the helm, a race with the keeper — is the verb's to judge, and its
 * answer says so.
 */
export function landNowOffer(lane: Lane): LandNowOffer {
  if (!hasWaitingWork(lane)) {
    return { offered: false, reason: "" };
  }
  if (lane.agent_alive === true || lane.owner.state === "running") {
    return { offered: false, reason: "The landing agent is already running; it lands the queued work." };
  }
  if (lane.paused === true || lane.owner.state === "stopped") {
    return { offered: false, reason: "Land now waits until the lane is started again." };
  }
  if (lane.owner.state === "unready") {
    // The lane cannot run, or cannot tell whether an agent runs: a press
    // could start nothing (Sol LN-02). Its own fix is the one line.
    const fix = lane.owner.retry_hint ?? "";
    return {
      offered: false,
      reason:
        fix === ""
          ? "Land now is unavailable until the lane can run."
          : `Land now is unavailable until the lane can run: ${fix}`,
    };
  }
  return { offered: true, reason: "" };
}

/** A word as a shell reads it: quoted only where it must be. */
function shellWord(word: string): string {
  return /^[\w@%+=:,./-]+$/u.test(word) ? word : `'${word.replaceAll("'", "'\\''")}'`;
}

/**
 * The verb's answer as the card shows it: line 1 is its summary, line 2 its
 * next step — the command, and why — or the reason alone. A refusal or a
 * failure is drawn as one; a repeat that started nothing (unchanged) is a
 * success. The outcome's own word is never shown.
 */
export function landNowLines(answer: LandNowAnswer): { line1: string; command: string; reason: string; refused: boolean } {
  return {
    line1: answer.summary,
    command: answer.next === null ? "" : answer.next.argv.map(shellWord).join(" "),
    reason: answer.next === null ? "" : answer.next.reason,
    refused: answer.outcome !== "confirmed" && answer.outcome !== "unchanged",
  };
}

/** Land now as drawn: the button or the reason, and the last answer. */
export function LandNowView({
  offer,
  sending,
  answer,
  problem,
  onPress,
}: {
  offer: LandNowOffer;
  sending: boolean;
  answer: LandNowAnswer | null;
  problem: string;
  onPress: () => void;
}) {
  if (!offer.offered && offer.reason === "" && answer === null && problem === "") {
    return null;
  }
  const lines = answer === null ? null : landNowLines(answer);
  return (
    <div className="ms-fleet-lane-landnow">
      {offer.offered && (
        <div className="ms-fleet-lane-landnow-act">
          <Button primary disabled={sending} onClick={onPress}>
            Land now
          </Button>
        </div>
      )}
      {!offer.offered && offer.reason !== "" && <p className="ms-fleet-lane-note">{offer.reason}</p>}
      {lines !== null && !lines.refused && (
        <div className="ms-fleet-lane-answer" role="status">
          <AnswerLines lines={lines} />
        </div>
      )}
      {lines !== null && lines.refused && (
        <Trouble text={answerSentence(lines)} as="div" variant="small" role="status">
          <AnswerLines lines={lines} />
        </Trouble>
      )}
      {problem !== "" && <Trouble text={problem} variant="small" />}
    </div>
  );
}

type AnswerLinesOf = ReturnType<typeof landNowLines>;

/** The verb's answer in one sentence, as a refusal travels when asked about. */
function answerSentence(lines: AnswerLinesOf): string {
  const next = lines.command !== "" ? `Next: ${lines.command}${lines.reason !== "" ? ` (${lines.reason})` : ""}` : lines.reason;
  return next === "" ? lines.line1 : `${lines.line1}. ${next}`;
}

/** Line 1, and line 2 as the command to copy and why, or the reason alone. */
function AnswerLines({ lines }: { lines: AnswerLinesOf }) {
  return (
    <>
      <span className="ms-fleet-lane-answer-line">{lines.line1}</span>
      {lines.command !== "" && (
        <span className="ms-fleet-lane-answer-line ms-fleet-lane-note">
          Next: <code className="ms-mono ms-fleet-lane-command">{lines.command}</code>
          {lines.reason !== "" && ` (${lines.reason})`}
        </span>
      )}
      {lines.command === "" && lines.reason !== "" && (
        <span className="ms-fleet-lane-answer-line ms-fleet-lane-note">{lines.reason}</span>
      )}
    </>
  );
}

/**
 * Land now's press: one POST, the sign-in sheet once where the route asks for
 * it, and the verb's answer kept until the next press.
 */
function LandNow({ lane, onLanded }: { lane: Lane; onLanded?: () => void }) {
  const [sending, setSending] = useState(false);
  const [answer, setAnswer] = useState<LandNowAnswer | null>(null);
  const [problem, setProblem] = useState("");
  const { askToSignIn } = useSession();
  const retried = useRef(false);

  const send = () => {
    setSending(true);
    setProblem("");
    landNow()
      .then((answered) => {
        setSending(false);
        retried.current = false;
        setAnswer(answered);
        onLanded?.();
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof ResourceError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        retried.current = false;
        setAnswer(null);
        setProblem(failureMessage(error));
      });
  };

  return <LandNowView offer={landNowOffer(lane)} sending={sending} answer={answer} problem={problem} onPress={send} />;
}
