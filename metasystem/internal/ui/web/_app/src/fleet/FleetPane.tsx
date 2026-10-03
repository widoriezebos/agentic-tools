import { ChevronRight } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadBoard,
  loadFleet,
  type BoardPayload,
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
  claimsTitle,
  copyLine,
  copyProblem,
  emptyFleetWords,
  flagWords,
  healthWords,
  jobWords,
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
  workingByGoal,
  workingSource,
  type WorkingGroup,
} from "./fleet";
import { minuteTime } from "../backlog/format";
import { laneTitle } from "../backlog/lanes";
import { Help } from "../help/Help";
import { onFleetEvent, onStreamReopen } from "../notifications/stream";
import { Pane } from "../panes/Pane";
import { goalPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Hint, Skeleton } from "../shell/controls";
import { LiveDot } from "../shell/LiveLine";
import { useOffersRefresh } from "../shell/refresh";
import { readFleetOpen, writeFleetOpen } from "../storage";
import { captureOfFleet } from "./capture";
import { LaneBlock } from "./LandingLane";
import { LaunchCard } from "./LaunchCard";
import { LaunchSheet } from "./LaunchSheet";
import { visibleCard } from "./launching";
import {
  doingOf,
  needsOf,
  queueOf,
  SCOPE,
  seatOf,
  unreadOf,
  verdictOf,
  type BoardReading,
  type Doing,
  type FleetReading,
  type Need,
  type Verdict,
} from "./panel";
import { Trouble } from "../shell/Trouble";

/**
 * Fleet: what the machine is doing, and whether it needs the person.
 *
 * Read top to bottom it answers the questions a person brings to it, in the
 * order they bring them: one verdict on top, in words, with what it checked;
 * Needs you, collected from every source in one list and absent when empty;
 * this seat; the fleet's table, one row per machine, with what each is doing;
 * and this computer's landing lane with what waits, proves, landed and came
 * back.
 *
 * It is two reads, made together: the fleet (presence and the ledger's
 * claims, composed on the server) and this computer's board, which carries
 * the seats' progress cards, the landing lane and this checkout's questions.
 * Nothing here judges a standing or a stage; the server composed both, and
 * src/fleet/panel.ts turns them into the verdict, the list and the words.
 * A read that fails says so in its own section and names that section in the
 * verdict; the other section still draws.
 *
 * The one act is Land now. Every other fix is said as the plain sentence of
 * what to do. A goal held by a machine that has gone quiet is flagged, and
 * the words name the acts a person makes at a terminal: `goal steal`
 * reassigns a claim, `goal resume` lifts a breach fence.
 *
 * Both are read when the pane mounts, when a person presses the section's
 * refresh, and when the server says a presence attempt or a board change
 * happened — which arrives on the one stream this build holds open. There is
 * no timer here: without that event a mounted page keeps the reading it
 * loaded with.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  // `problem` is what a later read of this page was refused with, standing
  // beside the reading it could not replace. A read that answers replaces this
  // state whole, so it cannot outlive the reading it was recorded against.
  | { state: "read"; page: FleetPayload; problem?: string };

type BoardState = { state: "loading" } | { state: "failed"; message: string } | { state: "read"; board: BoardPayload; problem?: string };

export function FleetPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [board, setBoard] = useState<BoardState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadFleet(aborter.signal)
      .then((answered) => {
        setRead({ state: "read", page: answered });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          // A refused read keeps whatever this page already read, and says so
          // beside it. Failing the whole pane instead would draw the error view
          // over a page that is still good, and take the launch card's inline
          // Retry form and the words typed into it with it (Astra C-05). Only a
          // first read's failure has nothing on screen to keep.
          setRead((held) =>
            held.state === "read"
              ? { ...held, problem: failureMessage(error) }
              : { state: "failed", message: failureMessage(error) },
          );
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  // The board is read beside the fleet, on the same attempt, and kept the
  // same way: a refused re-read leaves the last lane on screen and says so.
  useEffect(() => {
    const aborter = new AbortController();
    loadBoard(aborter.signal)
      .then((answered) => {
        setBoard({ state: "read", board: answered });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setBoard((held) =>
            held.state === "read"
              ? { ...held, problem: failureMessage(error) }
              : { state: "failed", message: failureMessage(error) },
          );
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
    setBoard({ state: "loading" });
    again();
  }, [again]);

  // The server's own signal, and the reconnect that may have missed one. A
  // re-read keeps whatever is on screen until the answer arrives, because a
  // page that blanked itself every minute would be a page nobody could read.
  //
  // A cold load reads once, on mount. The stream's first open is not a
  // reconnect and says nothing here; the stream itself tells an open after a
  // failure from the page's own first one, so a real reconnect still reads.
  useEffect(() => {
    const stopFleet = onFleetEvent(again);
    const stopOpen = onStreamReopen(again);
    return () => {
      stopFleet();
      stopOpen();
    };
  }, [again]);

  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  // The in-place read, and not the blanking one. An offered re-read is made
  // after somebody else's confirmed act — a proposal applied in the Partner's
  // drawer, say — and a failed launch's Retry form is inline in this page, in no
  // sheet: `reload` would unmount it and take the authorization word and the
  // review date a human had typed with it (Astra S58-10). `reload` stays what
  // Refresh and Retry press.
  useOffersRefresh(again, hint);

  return (
    <Pane title="Fleet">
      <Panel fleet={read} board={board} onRetry={reload} onLanded={again} />
    </Pane>
  );
}

/** The board this page has not read yet. */
const UNREAD_BOARD: BoardReading = { state: "loading" };

/**
 * The fleet's blocks from one fleet reading, beside whatever board reading
 * there is; with none given, the board is still being read.
 */
export function Blocks({ page, board = UNREAD_BOARD }: { page: FleetPayload; board?: BoardReading }) {
  const fleet = useMemo<FleetReading>(() => ({ state: "read", page }), [page]);
  return <Panel fleet={fleet} board={board} />;
}

/** The whole panel, top to bottom, from both readings in whatever state each is. */
export function Panel({
  fleet,
  board,
  onRetry,
  onLanded,
}: {
  fleet: FleetReading;
  board: BoardReading;
  onRetry?: () => void;
  onLanded?: () => void;
}) {
  // The instant every age on this page is measured against: one clock, read
  // once as the page renders, so two rows cannot be a second apart.
  const now = useMemo(() => new Date(), [fleet, board]);
  // The one rule: what each section could not read, which the verdict, Needs
  // you and Land now read from here.
  const unread = unreadOf(fleet, board);
  const needs = needsOf(fleet, board, now);
  const verdict = verdictOf(fleet, board, needs, now);
  const read = board.state === "read" ? board.board : null;
  return (
    <section className="ms-fleet">
      <VerdictStrip verdict={verdict} />
      {(needs.length > 0 || unread.questions.length > 0) && <NeedsYou needs={needs} questionsUnread={unread.questions} />}
      {fleet.state === "loading" && <Loading />}
      {fleet.state === "failed" && <Failure message={fleet.message} onRetry={onRetry} />}
      {fleet.state === "read" && (
        <FleetBlocks page={fleet.page} problem={fleet.problem} board={read} needs={needs} now={now} />
      )}
      <LaneBlock
        lane={read === null ? undefined : read.lane}
        titles={read?.titles}
        problem={board.state === "failed" ? board.message : board.state === "read" ? (board.problem ?? "") : ""}
        unread={unread.lane}
        loading={board.state === "loading"}
        now={now}
        onLanded={onLanded}
      />
    </section>
  );
}

/** The one sentence on top, the counts beside it, and what it checked. */
function VerdictStrip({ verdict }: { verdict: Verdict }) {
  return (
    <section className={`ms-fleet-verdict ms-fleet-verdict--${verdict.tone}`} aria-label="Verdict" role="status">
      <p className="ms-fleet-verdict-line">
        <span className="ms-fleet-verdict-words">{verdict.words}</span>
        {verdict.facts.length > 0 && <span className="ms-fleet-verdict-facts">{verdict.facts.join(" · ")}</span>}
      </p>
      <p className="ms-fleet-quiet">{SCOPE}</p>
    </section>
  );
}

function Loading() {
  return (
    <section className="ms-fleet-block" aria-label="The fleet">
      <h2 className="ms-fleet-heading">The fleet</h2>
      <div className="ms-fleet-skeleton" aria-busy="true" aria-label="Reading the fleet">
        <Skeleton />
        <Skeleton />
        <Skeleton />
      </div>
    </section>
  );
}

function Failure({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <section className="ms-fleet-block" aria-label="The fleet">
      <h2 className="ms-fleet-heading">The fleet</h2>
      <Trouble text={`The fleet could not be read: ${message}`} />
      {onRetry !== undefined && <Button onClick={onRetry}>Try again</Button>}
    </section>
  );
}

/** This seat and the table, from one fleet reading and the board beside it. */
function FleetBlocks({
  page,
  problem,
  board,
  needs,
  now,
}: {
  page: FleetPayload;
  problem: string | undefined;
  board: BoardPayload | null;
  needs: Need[];
  now: Date;
}) {
  // Which rows this viewer left open. It is read once, here, because the
  // capture below has to say which rows were open as well: what a human was
  // looking at is the Doing words alone or the words with the goal, the job,
  // the box and the chain under it, and those are two different screens.
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
  const doing = useMemo(() => {
    const words = new Map<string, Doing>();
    for (const machine of page.machines) {
      words.set(machine.machine, doingOf(machine, seatOf(board, machine.machine), queueOf(board), now));
    }
    return words;
  }, [page, board, now]);
  // What this page is about, for a question asked from it. The rows travel
  // because only the page knows what was on screen; the standings travel as
  // the page judged nothing and displayed them.
  useAbout(aboutLine("Fleet", ""), {
    returnTo: "/fleet",
    fleet: captureOfFleet(page, now, open, {
      needsYou: needs.map((one) => one.words),
      doing: Object.fromEntries([...doing].map(([machine, one]) => [machine, one.words])),
    }),
  });

  return (
    <>
      {problem !== undefined && (
        <Trouble text={`The fleet could not be read again, so what is on screen is the last reading: ${problem}`} role="status" />
      )}
      <ThisSeatBlock seat={page.this} doing={page.this.machine === "" ? undefined : doing.get(page.this.machine)} now={now} />
      <TheFleet page={page} board={board} doing={doing} now={now} open={open} onToggle={toggle} />
    </>
  );
}

/**
 * Needs you: everything that needs the person, from every source, in one
 * list, newest first — each with what to do in plain words, and the goal to
 * open or the question to answer where it has one.
 *
 * Empty, the block is absent rather than a line saying nothing needs you: the
 * verdict on top already says All good, and a standing "all clear" row is a
 * row a reader learns to skip.
 */
function NeedsYou({ needs, questionsUnread }: { needs: Need[]; questionsUnread: readonly string[] }) {
  return (
    <section className="ms-fleet-block ms-fleet-block--needs" aria-label="Needs you">
      <h2 className="ms-fleet-heading">
        Needs you
        <Help id="fleet-needs-you" />
      </h2>
      {questionsUnread.length > 0 && (
        <Trouble text={`This checkout's questions could not be read: ${questionsUnread.join("; ")}`} variant="small" />
      )}
      {needs.length > 0 && (
        <ul className="ms-fleet-needs">
          {needs.map((one) => (
            <li key={one.key} className="ms-fleet-needs-row">
              <span className="ms-fleet-needs-words">{one.words}</span>
              {one.goal !== "" && (
                <NavLink className="ms-fleet-needs-act" to={goalPath(one.goal)}>
                  Open goal
                </NavLink>
              )}
              {one.answer && (
                <NavLink className="ms-fleet-needs-act" to="/decisions">
                  Answer
                </NavLink>
              )}
              {one.goals.length > 0 && (
                <span className="ms-fleet-needs-goals">
                  {one.goals.map((goal) => (
                    <NavLink key={goal.id} className="ms-fleet-hold-title" to={goalPath(goal.id)}>
                      {goal.title}
                    </NavLink>
                  ))}
                </span>
              )}
              {one.todo !== "" && <span className="ms-fleet-needs-todo">{one.todo}</span>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * This seat: what it is called, whether it is armed, and what it is doing —
 * in its own row's Doing words where the table has a row for it, so the two
 * never disagree.
 */
function ThisSeatBlock({ seat, doing, now }: { seat: ThisSeat; doing: Doing | undefined; now: Date }) {
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
        <li className="ms-fleet-fact">{doing === undefined ? runningWords(seat.running, seat.runningProblem) : doing.words}</li>
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
  board,
  doing,
  now,
  open,
  onToggle,
}: {
  page: FleetPayload;
  board: BoardPayload | null;
  doing: ReadonlyMap<string, Doing>;
  now: Date;
  open: Set<string>;
  onToggle: (machine: string) => void;
}) {
  const problem = copyProblem(page);
  // The launch this block shows, if any: the newest one still worth a card.
  // It is state rather than a derived value because two things change it —
  // the act's own answer, which arrives before the next read does, and a
  // human putting a card away.
  const [started, setStarted] = useState<Launch | null>(null);
  const [opening, setOpening] = useState(false);
  // The launches whose discard the server has answered. The record on disk
  // carries the mark, so the next reading says the same; this only keeps the
  // card from standing in the moment before that reading arrives.
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set());
  // The act's own answer is what the card is drawn from until the server's
  // reading catches up with it — and not one moment longer. A payload that
  // carries this launch replaces it, so the card follows the record the verb
  // is rewriting rather than the one the act answered with minutes ago.
  const card = visibleCard(page.launches, started, hidden);
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
          hidden={hidden}
          thisSeat={page.this.machine}
          where={page.launching}
          onClose={() => {
            setOpening(false);
          }}
          onStarted={(record) => {
            setOpening(false);
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
          onDiscarded={(discarded) => {
            setHidden((held) => new Set(held).add(discarded.launch));
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
                Doing
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
                doing={doing.get(machine.machine) ?? IDLE}
                now={now}
                open={open.has(machine.machine)}
                onToggle={onToggle}
              />
            ))}
          </tbody>
        </table>
      )}
      {board !== null && !board.readable && (
        <Trouble
          text={`This computer's board could not be read, so Doing says what each seat's records say: ${board.reason ?? ""}`}
          variant="small"
        />
      )}
      {board !== null && board.readable && (board.unreadable ?? []).length > 0 && (
        <Trouble
          text={`Part of this computer's board could not be read, so Doing says what those seats' records say: ${(board.unreadable ?? []).join("; ")}`}
          variant="small"
        />
      )}
      <p className="ms-fleet-provenance" title={claimsTitle(page)}>
        {copyLine(page, now)}
      </p>
      {problem !== "" && <Trouble text={problem} />}
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
/** What a row says before anything was composed for it. */
const IDLE: Doing = { words: "idle", active: false, source: "none" };

function MachineRow({
  machine,
  doing,
  now,
  open,
  onToggle,
}: {
  machine: Machine;
  doing: Doing;
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
        <td className="ms-fleet-cell ms-fleet-cell--doing">
          <span className="ms-fleet-label">Doing</span>
          {/* The rail's dot, before the words of a machine that is working;
              the words themselves are the table's own. */}
          {doing.active && <LiveDot />}
          <span>{doing.words}</span>
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
            <Hint label={machine.engine}>
              <span className="ms-mono">{shortEngine(machine.engine)}</span>
            </Hint>
          )}
        </td>
      </tr>
      <tr className="ms-fleet-opened" id={disclosed} hidden={!open}>
        <td className="ms-fleet-open-cell" colSpan={6}>
          <p className="ms-fleet-work-line">
            <span className="ms-fleet-work-name">Engine</span>
            <span className="ms-mono">
              {machine.engine === "" ? "unknown" : machine.engine} · generation {machine.generation}
            </span>
          </p>
          <Work machine={machine} doing={doing} now={now} />
        </td>
      </tr>
    </>
  );
}

/**
 * What one machine is doing, opened: its goals, and under each the jobs in
 * hand with their boxes and chains.
 *
 * This seat's row can carry several things in flight, because only this host
 * can read its own job records; every other row carries the one chain its
 * presence published, and says when that was. Several of them are often one
 * goal's — a round and the critique of it — so the goal is named once, above
 * the jobs that are on it.
 */
function Work({ machine, doing, now }: { machine: Machine; doing: Doing; now: Date }) {
  if (machine.workingProblem !== "") {
    return <Trouble text={machine.workingProblem} />;
  }
  if (machine.working.length === 0) {
    return <p className="ms-fleet-quiet">{NOTHING_IN_HAND[doing.source]}</p>;
  }
  return (
    <div className="ms-fleet-work">
      {workingByGoal(machine.working).map((group) => (
        <GoalWork
          key={group.goal}
          group={group}
          // The goal's title, where this page already has one. It is carried
          // on the holds the row was composed with, so a goal this machine
          // does not hold has a chip and no title rather than an invented one.
          title={machine.holds.find((held) => held.goal === group.goal)?.title ?? ""}
          now={now}
        />
      ))}
      <p className="ms-fleet-quiet">{workingSource(machine, now)}</p>
    </div>
  );
}

/**
 * What the opened row says where no job is in hand: where the Doing words
 * came from instead, so the row and its opening never disagree.
 */
const NOTHING_IN_HAND: Record<Doing["source"], string> = {
  board: "No job of this machine's is in hand; what it is doing is read from this computer's board.",
  lane: "No job of this machine's is in hand; its work waits in this computer's landing lane.",
  jobs: "This machine publishes only its newest chain, so its jobs cannot be opened here.",
  none: "This machine is running nothing.",
  problem: "This machine is running nothing.",
};

/** One goal of the opened block: the goal once, then every job on it. */
function GoalWork({ group, title, now }: { group: WorkingGroup; title: string; now: Date }) {
  return (
    <div className="ms-fleet-work-goal">
      <p className="ms-fleet-work-line">
        <span className="ms-fleet-work-name">Goal</span>
        {/* A goal-free critique is lawful work, and a chip naming an empty
            goal would open a page nobody could find. */}
        {group.goal === "" ? (
          <span className="ms-fleet-quiet">this work names no goal</span>
        ) : (
          <NavLink className="ms-fleet-goal" to={goalPath(group.goal)}>
            {group.goal}
          </NavLink>
        )}
        {title !== "" && <span className="ms-fleet-quiet">{title}</span>}
        {group.working.length > 1 && (
          <span className="ms-fleet-quiet">{group.working.length} jobs in flight</span>
        )}
      </p>
      {group.working.map((working) => (
        <WorkingBlock key={working.job.id} working={working} now={now} />
      ))}
    </div>
  );
}

function WorkingBlock({ working, now }: { working: Working; now: Date }) {
  const cap = capWords(working.job, now);
  return (
    <div className="ms-fleet-working">
      <p className="ms-fleet-work-line">
        <span className="ms-fleet-work-name">This job</span>
        <span>
          {working.phase.role}
          {working.phase.round > 0 && ` round ${String(working.phase.round)}`}
          {working.phase.round > 0 && working.phase.roundLimit !== null && ` of ${String(working.phase.roundLimit)}`}
        </span>
        <Chip>{working.job.status}</Chip>
        {working.job.startedAt !== null && (
          <span className="ms-fleet-quiet">started {minuteTime(working.job.startedAt)}</span>
        )}
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

/**
 * One goal a machine holds, by its title, as the link that opens it: the id
 * is in the link and in its hint, with the goal's lane.
 */
function HoldChip({ held }: { held: Held }) {
  const chip = (
    <NavLink className="ms-fleet-hold-title" to={goalPath(held.goal)}>
      {held.title === "" ? held.goal : held.title}
    </NavLink>
  );
  return (
    <span className="ms-fleet-hold">
      <Hint label={`${laneTitle(held.lane)} · ${held.goal}`}>{chip}</Hint>
      {held.flag !== "" && <Chip marker>{flagWords(held)}</Chip>}
    </span>
  );
}
