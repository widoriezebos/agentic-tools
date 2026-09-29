import type { ReactNode } from "react";

import { dateAndTime, minuteTime } from "../backlog/format";
import type { Lane, LaneBatch, LaneMember, LaneNext, LaneOwner, LaneOwnerState, LaneWaiting } from "./api";

/**
 * The host's landing lane (U12; Wido 2026-09-29: "the status of the landing
 * component on the fleet page"): one batch-landing lane per host, one batch
 * proving at a time, and an owner process the steward keeps alive until it
 * gives up.
 *
 * It is drawn from the same /api/board response the host board is, so it
 * adds no request. The panel shows and never acts: the `metasystem landing`
 * verbs act, so where a person has something to do the panel prints the
 * command as text to copy, and there is no button on it.
 *
 * A server built before the lane existed sends no `lane` field at all; the
 * panel says it does not report one rather than claiming the host has none.
 */

/** What a person runs to start a stopped or given-up owner. */
export const START_COMMAND = "metasystem landing start";

/** The status colour each owner state takes, in the tokens the page uses. */
const TONE: Record<LaneOwnerState, "ok" | "warn" | "bad" | "neutral"> = {
  running: "ok",
  restarting: "warn",
  stopped: "bad",
  "given-up": "bad",
  "not-started": "neutral",
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

function plural(count: number, word: string): string {
  return `${String(count)} ${word}${count === 1 ? "" : "s"}`;
}

export function LaneBlock({ lane, now = new Date() }: { lane: Lane | null | undefined; now?: Date }) {
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
      <Batch batch={lane.batch} now={now} />
      {lane.next !== null && <Next next={lane.next} />}
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
  if (owner.restarts > 0) {
    facts.push(plural(owner.restarts, "restart"));
  }
  const needsStart = owner.state === "stopped" || owner.state === "given-up" || owner.state === "not-started";
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
        <p className="ms-fleet-lane-note">Stopped by {owner.stopped_by}.</p>
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
          To start it, run <code className="ms-mono ms-fleet-lane-command">{START_COMMAND}</code>
        </p>
      )}
    </div>
  );
}

function Batch({ batch, now }: { batch: LaneBatch | null; now: Date }) {
  if (batch === null) {
    return <p className="ms-fleet-quiet">No batch in hand.</p>;
  }
  return (
    <div className="ms-fleet-lane-batch">
      <p className="ms-fleet-lane-line">
        <span className="ms-fleet-lane-label">Batch</span>
        <span className="ms-mono">{batch.id}</span>
        <span className="ms-fleet-pill ms-fleet-lane-batch-state">{batch.state}</span>
        <span className="ms-fleet-lane-facts">since {when(batch.since, now)}</span>
      </p>
      {batch.reason !== "" && <p className="ms-fleet-lane-note">{batch.reason}</p>}
      <Members members={batch.members} />
      {batch.waiting_for.length > 0 && (
        <div className="ms-fleet-lane-waiting">
          <p className="ms-fleet-lane-sub">Waiting for</p>
          <ul className="ms-fleet-lane-members">
            {batch.waiting_for.map((waiting) => (
              <Waiting key={`${waiting.goal}@${waiting.seat}`} waiting={waiting} now={now} />
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function Waiting({ waiting, now }: { waiting: LaneWaiting; now: Date }) {
  return (
    <li className="ms-fleet-lane-member">
      <span className="ms-mono">{waiting.goal}</span> <span className="ms-fleet-lane-seat">@ {waiting.seat}</span>
      <span className="ms-fleet-lane-facts">
        {waiting.expected === null || waiting.expected === ""
          ? "no expected time"
          : `expected ${when(waiting.expected, now)}`}
      </span>
    </li>
  );
}

function Members({ members }: { members: LaneMember[] }) {
  if (members.length === 0) {
    return <p className="ms-fleet-quiet">no members yet</p>;
  }
  return (
    <ul className="ms-fleet-lane-members">
      {members.map((member) => (
        <li key={`${member.goal}@${member.seat}`} className="ms-fleet-lane-member">
          <span className="ms-mono">{member.goal}</span> <span className="ms-fleet-lane-seat">@ {member.seat}</span>
        </li>
      ))}
    </ul>
  );
}

function Next({ next }: { next: LaneNext }) {
  return (
    <div className="ms-fleet-lane-batch">
      <p className="ms-fleet-lane-line">
        <span className="ms-fleet-lane-label">Next</span>
        <span className="ms-mono">{next.id}</span>
        <span className="ms-fleet-lane-facts">collecting</span>
      </p>
      <Members members={next.members} />
    </div>
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
