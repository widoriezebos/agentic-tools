import { ChevronRight } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadFleet,
  type Box,
  type Held,
  type Launch,
  type Machine,
  type Page as FleetPayload,
  type Role,
  type ThisSeat,
  type Working,
} from "./api";
import "./fleet.css";
import {
  armedWords,
  attemptsLeftWords,
  attemptWords,
  barShare,
  capWords,
  chainWhen,
  chainWords,
  copyLine,
  copyProblem,
  emptyFleetWords,
  flagWords,
  healthWords,
  jobWords,
  NEEDS_YOU_REMEDY,
  NO_BOX,
  publicationWords,
  reservedWords,
  RESERVED_MEANING,
  rolesAlive,
  rolesNeedingAttention,
  runningWords,
  seatName,
  seenTitle,
  seenWords,
  shortEngine,
  sinceWords,
  workingSource,
  workingWords,
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
import { readFleetOpen, writeFleetOpen } from "../storage";
import { captureOfFleet } from "./capture";
import { LaunchCard } from "./LaunchCard";
import { LaunchSheet } from "./LaunchSheet";
import { cardFor } from "./launching";

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
  // Which rows this viewer left open. It is read once, here, because the
  // capture below has to say which rows were open as well: what a human was
  // looking at is the phase sentence alone or the phase with the goal, the
  // job, the box and the chain under it, and those are two different screens.
  const [open, setOpen] = useState(() => readFleetOpen());
  const toggle = useCallback((machine: string) => {
    setOpen((was) => {
      const next = new Set(was);
      if (!next.delete(machine)) {
        next.add(machine);
      }
      writeFleetOpen(next);
      return next;
    });
  }, []);
  // What this page is about, for a question asked from it. The rows travel
  // because only the page knows what was on screen; the standings travel as
  // the page judged nothing and displayed them.
  useAbout(aboutLine("Fleet", ""), { returnTo: "/fleet", fleet: captureOfFleet(page, now, open) });

  return (
    <section className="ms-fleet">
      {page.needsYou.length > 0 && <NeedsYou held={page.needsYou} />}
      <ThisSeatBlock seat={page.this} now={now} />
      <TheFleet page={page} now={now} open={open} onToggle={toggle} />
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
        <Help id="fleet-needs-you" />
      </h2>
      <ul className="ms-fleet-needs">
        {held.map((one) => (
          <li key={`${one.machine}/${one.goal}`} className="ms-fleet-needs-row">
            <NavLink className="ms-fleet-goal" to={goalPath(one.goal)}>
              {one.goal}
            </NavLink>
            <span className="ms-fleet-needs-words">is {flagWords(one)}</span>
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
        <Help id="this-seat" />
      </h2>
      <p className="ms-fleet-seat-name">{seatName(seat)}</p>
      <ul className="ms-fleet-facts">
        <li className="ms-fleet-fact">{armedWords(seat)}</li>
        <li className="ms-fleet-fact">
          {publicationWords(seat, now)}
          {/* The rung is a word only this line shows, so its explanation
              belongs on this line and nowhere else. */}
          {seat.publication !== null && seat.publication.rung > 0 && <Help id="rung" />}
        </li>
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
function TheFleet({
  page,
  now,
  open,
  onToggle,
}: {
  page: FleetPayload;
  now: Date;
  open: Set<string>;
  onToggle: (machine: string) => void;
}) {
  const problem = copyProblem(page);
  // The launch this block shows, if any: the newest one still worth a card.
  // It is state rather than a derived value because two things change it —
  // the act's own answer, which arrives before the next read does, and a
  // human dismissing a machine that has joined.
  const [started, setStarted] = useState<Launch | null>(null);
  const [opening, setOpening] = useState(false);
  const [dismissed, setDismissed] = useState("");
  // The act's own answer is what the card is drawn from until the server's
  // reading catches up with it — and not one moment longer. A payload that
  // carries this launch replaces it, so the card follows the record the verb
  // is rewriting rather than the one the act answered with minutes ago.
  const fromServer = page.launches.find((one) => one.launch === started?.launch) ?? null;
  const shown = fromServer ?? started ?? cardFor(page.launches);
  const card = shown === null || shown.launch === dismissed ? null : shown;
  const joined = card !== null && page.machines.some((machine) => machine.machine === card.machine);

  return (
    <section className="ms-fleet-block">
      <h2 className="ms-fleet-heading">
        The fleet
        <Help id="fleet" />
        <span className="ms-fleet-actions">
          <Button
            onClick={() => {
              setOpening(true);
            }}
          >
            Launch a machine
          </Button>
        </span>
      </h2>
      {opening && (
        <LaunchSheet
          machines={page.machines}
          launches={page.launches}
          thisSeat={page.this.machine}
          where={page.launching}
          onClose={() => {
            setOpening(false);
          }}
          onStarted={(record) => {
            setOpening(false);
            setDismissed("");
            setStarted(record);
          }}
        />
      )}
      {card !== null && (
        <LaunchCard
          record={card}
          joined={joined}
          now={now}
          onStarted={setStarted}
          onDismiss={() => {
            setDismissed(card.launch);
            setStarted(null);
          }}
        />
      )}
      {page.machines.length === 0 ? (
        <p className="ms-fleet-quiet">{emptyFleetWords(page)}</p>
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
              <th scope="col">
                Running
                <Help id="phase" />
              </th>
              <th scope="col">
                Holds
                <Help id="machine-holds" />
              </th>
              <th scope="col">
                Engine
                <Help id="engine" />
              </th>
            </tr>
          </thead>
          <tbody>
            {page.machines.map((machine) => (
              <MachineRow
                key={machine.machine}
                machine={machine}
                now={now}
                open={open.has(machine.machine)}
                onToggle={onToggle}
              />
            ))}
          </tbody>
        </table>
      )}
      <p className="ms-fleet-provenance">{copyLine(page, now)}</p>
      {problem !== "" && <p className="ms-fleet-problem">{problem}</p>}
      {/* The Partner's own file, where this server could not write it. The
          page is fine and the tool's reading of it is not, which is a
          different fact and says so quietly rather than as a failure. */}
      {page.copy.metadataProblem !== "" && <p className="ms-fleet-quiet">{page.copy.metadataProblem}</p>}
    </section>
  );
}

/**
 * One machine, and under it what that machine is doing.
 *
 * The disclosure is a second row rather than a `<details>`, which is what the
 * rest of this build uses. A table row cannot contain another row, and the
 * block has to open in the row's own width — under the machine and across all
 * six columns — rather than inside one cell. So the control is a button that
 * says whether it is expanded and what it controls, and the block is a row
 * the table draws beneath this one. Its open state is remembered per viewer
 * either way, which a native disclosure would have had to be told anyway.
 */
function MachineRow({
  machine,
  now,
  open,
  onToggle,
}: {
  machine: Machine;
  now: Date;
  open: boolean;
  onToggle: (machine: string) => void;
}) {
  const seen = seenWords(machine, now);
  const title = seenTitle(machine);
  const since = sinceWords(machine);
  const disclosed = `ms-fleet-work-${machine.machine}`;
  return (
    <>
      <tr className={open ? "ms-fleet-row ms-fleet-row--open" : "ms-fleet-row"}>
        <td className="ms-fleet-cell ms-fleet-cell--machine">
          <span className="ms-fleet-label">Machine</span>
          <button
            type="button"
            className="ms-fleet-disclose"
            aria-expanded={open}
            aria-controls={disclosed}
            aria-label={`What ${machine.machine} is doing`}
            onClick={() => {
              onToggle(machine.machine);
            }}
          >
            <ChevronRight className="ms-fleet-chevron" size={14} strokeWidth={2} aria-hidden="true" />
          </button>
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
          <span>{workingWords(machine, now)}</span>
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
      <tr className="ms-fleet-opened" id={disclosed} hidden={!open}>
        <td className="ms-fleet-open-cell" colSpan={6}>
          <Work machine={machine} now={now} />
        </td>
      </tr>
    </>
  );
}

/**
 * What one machine is doing, opened: the goal, the job in hand, the goal's
 * box and the chain.
 *
 * This seat's row can carry several things in flight, because only this host
 * can read its own job records; every other row carries the one chain its
 * presence published, and says when that was.
 */
function Work({ machine, now }: { machine: Machine; now: Date }) {
  if (machine.workingProblem !== "") {
    return <p className="ms-fleet-problem">{machine.workingProblem}</p>;
  }
  if (machine.working.length === 0) {
    return <p className="ms-fleet-quiet">This machine is running nothing.</p>;
  }
  return (
    <div className="ms-fleet-work">
      {machine.working.map((working) => (
        <WorkingBlock key={working.job.id} working={working} now={now} />
      ))}
      <p className="ms-fleet-quiet">{workingSource(machine, now)}</p>
    </div>
  );
}

function WorkingBlock({ working, now }: { working: Working; now: Date }) {
  const cap = capWords(working.job, now);
  return (
    <div className="ms-fleet-working">
      <p className="ms-fleet-work-line">
        <span className="ms-fleet-work-name">Goal</span>
        <NavLink className="ms-fleet-goal" to={goalPath(working.goal)}>
          {working.goal}
        </NavLink>
      </p>
      <p className="ms-fleet-work-line">
        <span className="ms-fleet-work-name">This job</span>
        <span>
          {working.phase.role}
          {working.phase.round > 0 && ` round ${String(working.phase.round)}`}
          {working.phase.round > 0 && working.phase.roundLimit !== null && ` of ${String(working.phase.roundLimit)}`}
        </span>
        <Chip>{working.job.status}</Chip>
        <span>{jobWords(working.job, now)}</span>
        {cap !== "" && (
          <span className="ms-fleet-bound">
            {cap}
            <Help id="bound" />
          </span>
        )}
      </p>
      <div className="ms-fleet-work-line ms-fleet-work-line--box">
        <span className="ms-fleet-work-name">
          Box
          <Help id="box" />
        </span>
        <BoxBlock box={working.box} />
      </div>
      <div className="ms-fleet-work-line ms-fleet-work-line--chain">
        <span className="ms-fleet-work-name">
          Chain
          <Help id="chain" />
        </span>
        <ul className="ms-fleet-chain">
          {working.chain.map((member) => (
            <li key={member.job} className="ms-fleet-chain-row">
              <span className="ms-mono ms-fleet-chain-job">{member.job}</span>
              <span>{chainWords(member)}</span>
              <Chip>{member.status}</Chip>
              <span className="ms-fleet-quiet">{chainWhen(member)}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

/**
 * The goal's box: two numbers, two bars, and what a reserved minute counts.
 *
 * A goal with none says so; a projection that could not be made says its own
 * reason and draws no bars, because a bar at nought would say the goal has
 * spent nothing.
 */
function BoxBlock({ box }: { box: Box | null }) {
  if (box === null) {
    return <span className="ms-fleet-quiet">{NO_BOX}</span>;
  }
  if (box.problem !== "") {
    return <span className="ms-fleet-quiet">{box.problem}</span>;
  }
  const attempts = attemptWords(box);
  const reserved = reservedWords(box);
  return (
    <span className="ms-fleet-box">
      {attempts !== "" && (
        <span className="ms-fleet-measure">
          <span className="ms-fleet-measure-words">
            {attempts} · {attemptsLeftWords(box)}
          </span>
          <Bar share={barShare(box.attempts ?? 0, box.attemptLimit ?? 0)} />
        </span>
      )}
      {reserved !== "" && (
        <span className="ms-fleet-measure">
          <span className="ms-fleet-measure-words">{reserved}</span>
          <Bar share={barShare(box.reservedMinutes ?? 0, box.reservedMinutesLimit ?? 0)} />
        </span>
      )}
      <span className="ms-fleet-quiet">{RESERVED_MEANING}</span>
    </span>
  );
}

/**
 * One bar, in the tokens this interface already has.
 *
 * The share is drawn in twentieths, as one of twenty-one classes the
 * stylesheet defines, because nothing on this page carries a style attribute
 * and a bar is a width. A twentieth is a pixel or two at the width these bars
 * are, which is finer than an eye reads and finer than the number beside it.
 */
function Bar({ share }: { share: number }) {
  const twentieths = Math.round(share * 20);
  return (
    <span className="ms-fleet-bar" aria-hidden="true">
      <span className={`ms-fleet-bar-fill ms-fleet-bar-fill--${String(twentieths)}`} />
    </span>
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
      {held.flag !== "" && <Chip marker>{flagWords(held)}</Chip>}
    </span>
  );
}
