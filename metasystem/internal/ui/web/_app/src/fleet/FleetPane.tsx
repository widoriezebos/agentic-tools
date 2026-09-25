import { useCallback, useEffect, useMemo, useState } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadFleet,
  type Held,
  type Machine,
  type Page as FleetPayload,
  type Role,
  type ThisSeat,
} from "./api";
import "./fleet.css";
import {
  armedWords,
  copyLine,
  copyProblem,
  healthWords,
  NEEDS_YOU_REMEDY,
  NO_PRESENCE,
  publicationWords,
  rolesAlive,
  rolesNeedingAttention,
  runningWords,
  seatName,
  seenTitle,
  seenWords,
  shortEngine,
  sinceWords,
} from "./fleet";
import { minuteTime } from "../backlog/format";
import { laneTitle } from "../backlog/lanes";
import { Help } from "../help/Help";
import { onFleetEvent, onStreamOpen } from "../notifications/stream";
import { Pane } from "../panes/Pane";
import { goalPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Hint } from "../shell/controls";
import { useOffersRefresh } from "../shell/refresh";
import { captureOfFleet } from "./capture";

/**
 * Fleet: who is doing what, and whether execution is healthy.
 *
 * It answers one question from two sources and nothing else — the presence
 * copy this interface fetched for itself, for what each machine's standing
 * is, and one accepted ledger tip, for who holds what — and the server
 * composes both into one payload. Nothing here judges: a standing, a flag and
 * the needs-you selection all arrive decided, and this file turns them into
 * rows.
 *
 * Nothing here acts either. A goal held by a machine that has gone quiet is
 * flagged, and the words name the acts a human makes at a terminal: `goal
 * steal` reassigns a claim, `goal resume` lifts a breach fence. The page has
 * neither.
 *
 * It is read when the pane mounts, when a human presses the section's
 * refresh, and when the server says a presence attempt finished — which
 * arrives on the one stream this build holds open. There is no timer here and
 * no second reader: without that event a mounted page would keep showing the
 * reading it loaded with, however many times the fleet ticked.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; page: FleetPayload };

export function FleetPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadFleet(aborter.signal)
      .then((answered) => {
        setRead({ state: "read", page: answered });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setRead({ state: "failed", message: failureMessage(error) });
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const again = useCallback(() => {
    setAttempt((previous) => previous + 1);
  }, []);

  const reload = useCallback(() => {
    setRead({ state: "loading" });
    again();
  }, [again]);

  // The server's own signal, and the reconnect that may have missed one. A
  // re-read keeps whatever is on screen until the answer arrives, because a
  // page that blanked itself every minute would be a page nobody could read.
  //
  // A cold load reads twice: once on mount, and once when the stream opens a
  // moment later. That is deliberate. The open is not known to be the first
  // one — a page navigated to while the stream was already up sees only
  // reconnects — and a reconnect is exactly when attempts may have been
  // missed, so the second read is kept rather than a first-open flag added
  // that would silently skip a real reconnect.
  useEffect(() => {
    const stopFleet = onFleetEvent(again);
    const stopOpen = onStreamOpen(again);
    return () => {
      stopFleet();
      stopOpen();
    };
  }, [again]);

  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  useOffersRefresh(reload, hint);

  return (
    <Pane title="Fleet">
      {read.state === "loading" && <Loading />}
      {read.state === "failed" && <Failure message={read.message} onRetry={reload} />}
      {read.state === "read" && <Blocks page={read.page} />}
    </Pane>
  );
}

function Loading() {
  return (
    <section className="ms-fleet">
      <p className="ms-fleet-quiet">Reading the fleet…</p>
    </section>
  );
}

function Failure({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <section className="ms-fleet">
      <h2 className="ms-fleet-heading">The fleet could not be read</h2>
      <p className="ms-fleet-problem">{message}</p>
      <Button onClick={onRetry}>Try again</Button>
    </section>
  );
}

export function Blocks({ page }: { page: FleetPayload }) {
  // The instant every age on this page is measured against: one clock, read
  // once as the page renders, so two rows cannot be a second apart.
  const now = useMemo(() => new Date(), [page]);
  // What this page is about, for a question asked from it. The rows travel
  // because only the page knows what was on screen; the standings travel as
  // the page judged nothing and displayed them.
  useAbout(aboutLine("Fleet", ""), { returnTo: "/fleet", fleet: captureOfFleet(page, now) });

  return (
    <section className="ms-fleet">
      {page.needsYou.length > 0 && <NeedsYou held={page.needsYou} />}
      <ThisSeatBlock seat={page.this} now={now} />
      <TheFleet page={page} now={now} />
    </section>
  );
}

/**
 * Needs you: one line per held goal whose holder has gone silent.
 *
 * Empty, the block is absent rather than a line saying nothing needs you: the
 * fleet's good outcome is a page with two blocks on it, and a standing "all
 * clear" row is a row a reader learns to skip.
 */
function NeedsYou({ held }: { held: Held[] }) {
  return (
    <section className="ms-fleet-block ms-fleet-block--needs">
      <h2 className="ms-fleet-heading">
        Needs you
        <Help id="standing" />
      </h2>
      <ul className="ms-fleet-needs">
        {held.map((one) => (
          <li key={`${one.machine}/${one.goal}`} className="ms-fleet-needs-row">
            <NavLink className="ms-fleet-goal" to={goalPath(one.goal)}>
              {one.goal}
            </NavLink>
            <span className="ms-fleet-needs-words">is {one.flag}</span>
          </li>
        ))}
      </ul>
      <p className="ms-fleet-quiet">{NEEDS_YOU_REMEDY}</p>
    </section>
  );
}

/** This seat: what it is called, whether it is armed, and what it is doing. */
function ThisSeatBlock({ seat, now }: { seat: ThisSeat; now: Date }) {
  const attention = rolesNeedingAttention(seat.health);
  const alive = rolesAlive(seat.health);
  return (
    <section className="ms-fleet-block">
      <h2 className="ms-fleet-heading">
        This seat
        <Help id="seat" />
      </h2>
      <p className="ms-fleet-seat-name">{seatName(seat)}</p>
      <ul className="ms-fleet-facts">
        <li className="ms-fleet-fact">{armedWords(seat)}</li>
        <li className="ms-fleet-fact">{publicationWords(seat, now)}</li>
        <li className="ms-fleet-fact">{runningWords(seat.running, seat.runningProblem)}</li>
        <li className="ms-fleet-fact">{healthWords(seat.health, now)}</li>
      </ul>
      {attention.length > 0 && (
        <ul className="ms-fleet-roles">
          {attention.map((role) => (
            <RoleRow key={role.role} role={role} />
          ))}
        </ul>
      )}
      {alive.length > 0 && (
        <details className="ms-fleet-alive">
          <summary className="ms-fleet-alive-summary">{alive.length} roles alive</summary>
          <ul className="ms-fleet-roles">
            {alive.map((role) => (
              <RoleRow key={role.role} role={role} />
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}

function RoleRow({ role }: { role: Role }) {
  return (
    <li className="ms-fleet-role">
      <span className={`ms-fleet-dot ms-fleet-dot--${role.status}`} aria-hidden="true" />
      <span className="ms-mono ms-fleet-role-name">{role.role}</span>
      <span className="ms-fleet-role-reason">{role.reason}</span>
    </li>
  );
}

/**
 * The fleet: one row per machine, this seat first, then the machines holding
 * a flagged goal, then the reachable, then the silent, then the unknown. The
 * server decided that order; the table draws it.
 */
function TheFleet({ page, now }: { page: FleetPayload; now: Date }) {
  const problem = copyProblem(page);
  return (
    <section className="ms-fleet-block">
      <h2 className="ms-fleet-heading">
        The fleet
        <Help id="fleet" />
      </h2>
      {page.machines.length === 0 ? (
        <p className="ms-fleet-quiet">{NO_PRESENCE}</p>
      ) : (
        <table className="ms-fleet-table">
          <thead>
            <tr>
              <th scope="col">Machine</th>
              <th scope="col">
                Standing
                <Help id="standing" />
              </th>
              <th scope="col">
                Seen
                <Help id="presence" />
              </th>
              <th scope="col">Running</th>
              <th scope="col">Holds</th>
              <th scope="col">Engine</th>
            </tr>
          </thead>
          <tbody>
            {page.machines.map((machine) => (
              <MachineRow key={machine.machine} machine={machine} now={now} />
            ))}
          </tbody>
        </table>
      )}
      <p className="ms-fleet-provenance">{copyLine(page, now)}</p>
      {problem !== "" && <p className="ms-fleet-problem">{problem}</p>}
    </section>
  );
}

function MachineRow({ machine, now }: { machine: Machine; now: Date }) {
  const seen = seenWords(machine, now);
  const title = seenTitle(machine);
  const since = sinceWords(machine);
  return (
    <tr className="ms-fleet-row">
      <td className="ms-fleet-cell ms-fleet-cell--machine">
        <span className="ms-fleet-label">Machine</span>
        <span className="ms-mono">{machine.machine}</span>
        {machine.this && <span className="ms-fleet-here">this seat</span>}
      </td>
      <td className="ms-fleet-cell ms-fleet-cell--standing">
        <span className="ms-fleet-label">Standing</span>
        <Hint label={machine.reason}>
          <span className={`ms-fleet-pill ms-fleet-pill--${machine.standing}`}>{machine.standing}</span>
        </Hint>
      </td>
      <td className="ms-fleet-cell">
        <span className="ms-fleet-label">Seen</span>
        {title === "" ? <span>{seen}</span> : <Hint label={title}>{<span>{seen}</span>}</Hint>}
        {since !== "" && <span className="ms-fleet-since">{since}</span>}
      </td>
      <td className="ms-fleet-cell">
        <span className="ms-fleet-label">Running</span>
        <span>{runningWords(machine.running, "")}</span>
      </td>
      <td className="ms-fleet-cell ms-fleet-cell--holds">
        <span className="ms-fleet-label">Holds</span>
        {machine.holds.length === 0 ? (
          <span className="ms-fleet-quiet">nothing</span>
        ) : (
          <span className="ms-fleet-chips">
            {machine.holds.map((held) => (
              <HoldChip key={held.goal} held={held} />
            ))}
          </span>
        )}
      </td>
      <td className="ms-fleet-cell ms-fleet-cell--engine">
        <span className="ms-fleet-label">Engine</span>
        {machine.engine === "" ? (
          <span className="ms-fleet-quiet">unknown</span>
        ) : (
          <span className="ms-mono">
            {shortEngine(machine.engine)} · generation {machine.generation}
          </span>
        )}
      </td>
    </tr>
  );
}

/** One goal a machine holds, as the chip that opens it. */
function HoldChip({ held }: { held: Held }) {
  const chip = (
    <NavLink className="ms-fleet-goal" to={goalPath(held.goal)}>
      {held.goal}
    </NavLink>
  );
  return (
    <span className="ms-fleet-hold">
      <Hint label={`${laneTitle(held.lane)}${held.title === "" ? "" : ` · ${held.title}`}`}>{chip}</Hint>
      {held.flag !== "" && <Chip marker>{held.flag}</Chip>}
    </span>
  );
}
