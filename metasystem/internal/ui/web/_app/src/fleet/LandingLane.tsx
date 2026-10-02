import { type ReactNode, useRef, useState } from "react";
import { NavLink } from "react-router";

import { goalPath } from "../routes";
import { Button, Skeleton } from "../shell/controls";
import { useSession } from "../shell/identity";
import { Trouble } from "../shell/Trouble";
import { failureMessage, landNow, ResourceError } from "./api";
import type { Lane, LaneEntry, LandNowAnswer } from "./api";
import { laneLists, laneNotRead, laneState, when, type LaneItem, type ProvingItem } from "./panel";

/**
 * This computer's landing lane on the Fleet page: one state word — Running,
 * Paused or Needs attention — with the one button that makes sense in that
 * state, and then the lane's four lists: what waits, what is being proved,
 * what landed today and what came back. Every item says what it is in plain
 * words; its commit, branch and seat are behind its own Details disclosure,
 * and the lane's root, its agent's pid and its last proof and push behind
 * the block's.
 *
 * It is drawn from the same /api/board response the Doing column and the
 * questions are, so it adds no request. Its one act is Land now (goal
 * fleet-card-can-land-now): it runs `metasystem landing run` through the
 * server and shows the verb's two lines. It is offered only when a hand-in
 * waits, the lane is not paused, can run, no landing agent is alive and no
 * proof runs, and says in one line why not where work waits all the same.
 * What else the person can do about the lane is said in Needs you, as the
 * plain sentence of what to do.
 *
 * A server built before the lane existed sends no `lane` field at all; the
 * block says it does not report one rather than claiming there is none.
 */

/** The block's own frame, its heading and what stands beside it. */
function Panel({ children, state, action }: { children: ReactNode; state?: ReactNode; action?: ReactNode }) {
  return (
    <section className="ms-fleet-block ms-fleet-lane" aria-label="Landing lane">
      <h2 className="ms-fleet-heading">
        Landing lane
        {state}
        {action !== undefined && <span className="ms-fleet-actions">{action}</span>}
      </h2>
      {children}
    </section>
  );
}

export function LaneBlock({
  lane,
  titles = {},
  problem = "",
  unread,
  loading = false,
  now = new Date(),
  onLanded,
}: {
  lane: Lane | null | undefined;
  titles?: Readonly<Record<string, string>>;
  /** Why the response the lane is read from could not be read, or "". */
  problem?: string;
  /** What the lane could not read, by the panel's one rule (unreadOf); any line withholds Land now. */
  unread: readonly string[];
  /** The first read has not answered yet. */
  loading?: boolean;
  now?: Date;
  /** Called after Land now answered, so the block is read again. */
  onLanded?: () => void;
}) {
  if (loading) {
    return (
      <Panel>
        <div className="ms-fleet-skeleton" aria-busy="true" aria-label="Reading the landing lane">
          <Skeleton />
          <Skeleton />
        </div>
      </Panel>
    );
  }
  if (problem !== "" && lane === undefined) {
    return (
      <Panel>
        <Trouble text={`The landing lane could not be read: ${problem}`} role="status" />
      </Panel>
    );
  }
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
        {problem !== "" && (
          <Trouble text={`The landing lane could not be read again, so what is on screen is the last reading: ${problem}`} role="status" />
        )}
        <p className="ms-fleet-quiet">No landing lane is registered on this computer.</p>
      </Panel>
    );
  }
  if (laneNotRead(lane)) {
    return (
      <Panel>
        <Trouble text={`The landing lane could not be read: ${(lane.problems ?? []).join("; ")}`} role="status" />
      </Panel>
    );
  }
  const state = laneState(lane);
  return (
    <LandNow lane={lane} unread={unread} onLanded={onLanded}>
      {(landing) => (
        <Panel
          state={<span className={`ms-fleet-pill ms-fleet-lane-state ms-fleet-lane-state--${state.tone}`}>{state.word}</span>}
          action={landing.button}
        >
          {problem !== "" && (
            <Trouble text={`The landing lane could not be read again, so what is on screen is the last reading: ${problem}`} role="status" />
          )}
          {(lane.problems ?? []).map((one) => (
            <Trouble key={one} text={`Part of the landing lane could not be read: ${one}`} variant="small" />
          ))}
          {landing.lines}
          <Lists lane={lane} titles={titles} now={now} />
          <LaneDetails lane={lane} now={now} />
        </Panel>
      )}
    </LandNow>
  );
}

/** The four lists, each drawn only when it holds something. */
function Lists({ lane, titles, now }: { lane: Lane; titles: Readonly<Record<string, string>>; now: Date }) {
  const lists = laneLists(lane, titles, now);
  if (lists.waiting.length === 0 && lists.proving === null && lists.landed.length === 0 && lists.cameBack.length === 0) {
    return <p className="ms-fleet-quiet">Nothing is waiting to land.</p>;
  }
  return (
    <div className="ms-fleet-lane-lists">
      {lists.waiting.length > 0 && (
        <List name="Waiting">
          {lists.waiting.map((item) => (
            <Item key={key(item.entry)} item={item}>
              <GoalLink goal={item.entry.goal}>{item.title}</GoalLink>
              {item.detail !== "" && <span className="ms-fleet-lane-detail">{item.detail}</span>}
            </Item>
          ))}
        </List>
      )}
      {lists.proving !== null && <Proving proving={lists.proving} lane={lane} now={now} />}
      {lists.landed.length > 0 && (
        <List name="Landed today">
          {lists.landed.map((item) => (
            <Item key={key(item.entry)} item={item}>
              <span className="ms-fleet-lane-time">{when(item.at, now)}</span>
              <span>{item.words}</span>
            </Item>
          ))}
        </List>
      )}
      {lists.cameBack.length > 0 && (
        <List name="Came back">
          {lists.cameBack.map((item) => (
            <Item key={key(item.entry)} item={item}>
              <span>{`${item.title} · ${item.words}`}</span>
              {item.again && <span className="ms-fleet-quiet">handed in again</span>}
              <NavLink className="ms-fleet-lane-open" to={goalPath(item.entry.goal)}>
                Open goal
              </NavLink>
            </Item>
          ))}
        </List>
      )}
    </div>
  );
}

function key(entry: LaneEntry): string {
  return `${entry.goal}@${entry.sha}@${entry.at}`;
}

function List({ name, children }: { name: string; children: ReactNode }) {
  return (
    <div className="ms-fleet-lane-list">
      <p className="ms-fleet-lane-sub">{name}</p>
      <ul className="ms-fleet-lane-members">{children}</ul>
    </div>
  );
}

/** One hand-in on a list: its line, and its Details. */
function Item({ item, children }: { item: LaneItem; children: ReactNode }) {
  const entry = item.entry;
  return (
    <li className="ms-fleet-lane-member">
      <span className="ms-fleet-lane-item">{children}</span>
      <Details
        facts={[
          ["Goal", entry.goal],
          ["Branch", entry.branch],
          ["Commit", entry.sha],
          ["Seat", entry.seat],
          ["Handed in", entry.at],
          ["Returned", entry.returned_at ?? ""],
          ["Landed", entry.landed_at ?? ""],
        ]}
      />
    </li>
  );
}

function GoalLink({ goal, children }: { goal: string; children: ReactNode }) {
  return (
    <NavLink className="ms-fleet-lane-goal" to={goalPath(goal)}>
      {children}
    </NavLink>
  );
}

function Proving({ proving, lane, now }: { proving: ProvingItem; lane: Lane; now: Date }) {
  const proof = lane.running_proof;
  return (
    <List name="Proving">
      <li className="ms-fleet-lane-member">
        {proving.goals.length === 0 ? (
          <span className="ms-fleet-lane-item">{proving.words}</span>
        ) : (
          <span className="ms-fleet-lane-item">
            {proving.goals.map((goal) => (
              <GoalLink key={goal.id} goal={goal.id}>
                {goal.title}
              </GoalLink>
            ))}
            <span className="ms-fleet-lane-detail">{proving.detail}</span>
          </span>
        )}
        {proof !== undefined && proof !== null && (
          <Details
            facts={[
              ["Tree", proof.tree],
              ["Commit", proof.commit ?? ""],
              ["Attempt", proof.attempt],
              ["Since", proof.since === "" ? "" : when(proof.since, now)],
              ["Log", proof.log ?? ""],
            ]}
          />
        )}
      </li>
    </List>
  );
}

/** A commit or tree id as a person compares it by eye. */
function short(id: string): string {
  return id.slice(0, 7);
}

/** The lane's own particulars: where it is, its agent, its last proof and push. */
function LaneDetails({ lane, now }: { lane: Lane; now: Date }) {
  const owner = lane.owner;
  const proof = lane.last_proof ?? null;
  const push = lane.last_push ?? null;
  return (
    <Details
      label="Details"
      facts={[
        ["Root", lane.root ?? ""],
        ["Registered", [lane.registered_by ?? "", lane.registered_at === null || lane.registered_at === "" ? "" : when(lane.registered_at, now)].filter((part) => part !== "").join(" · ")],
        ["Agent", owner.pid === null ? "" : `pid ${String(owner.pid)}${owner.since === null || owner.since === "" ? "" : ` since ${when(owner.since, now)}`}`],
        ["Last exit", owner.last_exit ?? ""],
        ["Last proof", proof === null ? "" : `${proof.result} · ${short(proof.commit)} · ${when(proof.at, now)}${proof.reason === undefined || proof.reason === "" ? "" : ` · ${proof.reason}`}`],
        ["Proof log", proof === null ? "" : proof.log],
        ["Last push", push === null ? "" : `${short(push.old)} → ${short(push.commit)} · ${when(push.at, now)}`],
        ["Summary", lane.summary],
      ]}
    />
  );
}

/**
 * One Details disclosure: the particulars a person wants only now and then,
 * as name and value, the empty ones left out.
 */
function Details({ facts, label = "Details" }: { facts: readonly (readonly [string, string])[]; label?: string }) {
  const shown = facts.filter(([, value]) => value !== "");
  if (shown.length === 0) {
    return null;
  }
  return (
    <details className="ms-fleet-details">
      <summary className="ms-fleet-details-summary">{label}</summary>
      <dl className="ms-fleet-details-list">
        {shown.map(([name, value]) => (
          <div key={name} className="ms-fleet-details-row">
            <dt>{name}</dt>
            <dd className="ms-mono">{value}</dd>
          </div>
        ))}
      </dl>
    </details>
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
 * Land now is offered when a hand-in waits, every part of the lane was read,
 * the lane is not paused, can run, no landing agent is alive and no proof
 * runs. With nothing waiting it is not drawn at all; with a hand-in waiting
 * and any of the rest not so, it says in one line why it is not offered. A
 * lane with a part it could not read — the running proof's record, say — is
 * a lane whose state this page cannot vouch for, so it offers no act on it.
 * Everything else — the helm, a race with the keeper — is the verb's to
 * judge, and its answer says so.
 */
export function landNowOffer(lane: Lane, unread: readonly string[]): LandNowOffer {
  if (!hasWaitingWork(lane)) {
    return { offered: false, reason: "" };
  }
  if (unread.length > 0) {
    return { offered: false, reason: "Land now waits until the landing lane can be read." };
  }
  if (lane.agent_alive === true || lane.owner.state === "running") {
    return { offered: false, reason: "The landing agent is already running; it lands the queued work." };
  }
  if (lane.paused === true || lane.owner.state === "stopped") {
    return { offered: false, reason: "Land now waits until the lane is resumed." };
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
  if (lane.running_proof?.state === "running") {
    return { offered: false, reason: "Land now waits while a proof runs." };
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
  return (
    <div className="ms-fleet-lane-landnow">
      {offer.offered && (
        <div className="ms-fleet-lane-landnow-act">
          <LandNowButton sending={sending} onPress={onPress} />
        </div>
      )}
      <LandNowLines offer={offer} answer={answer} problem={problem} />
    </div>
  );
}

/** The button itself, which stands beside the lane's state word. */
function LandNowButton({ sending, onPress }: { sending: boolean; onPress: () => void }) {
  return (
    <Button primary disabled={sending} onClick={onPress}>
      Land now
    </Button>
  );
}

/** Why Land now is not offered, and the verb's answer to the last press. */
function LandNowLines({ offer, answer, problem }: { offer: LandNowOffer; answer: LandNowAnswer | null; problem: string }) {
  if ((offer.offered || offer.reason === "") && answer === null && problem === "") {
    return null;
  }
  const lines = answer === null ? null : landNowLines(answer);
  return (
    <>
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
    </>
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
 * it, and the verb's answer kept until the next press. It hands the block the
 * button, for beside the state word, and the lines, for under it.
 */
function LandNow({
  lane,
  unread,
  onLanded,
  children,
}: {
  lane: Lane;
  unread: readonly string[];
  onLanded?: () => void;
  children: (landing: { button: ReactNode; lines: ReactNode }) => ReactNode;
}) {
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

  const offer = landNowOffer(lane, unread);
  return children({
    button: offer.offered ? <LandNowButton sending={sending} onPress={send} /> : undefined,
    lines: <LandNowLines offer={offer} answer={answer} problem={problem} />,
  });
}
