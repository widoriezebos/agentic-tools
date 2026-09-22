import { useId, useMemo, useState, type DragEvent, type ReactNode } from "react";
import { NavLink } from "react-router";

import { rankGoal, type Backlog, type Row } from "./api";
import {
  anySet,
  arcsOn,
  concludedWithin,
  matches,
  named,
  nameOf,
  noFilters,
  seatsOn,
  windowTitle,
  WINDOWS,
  ANY,
  NONE,
  RANKS,
  type Filters,
  type Rank,
  type Window,
} from "./filters";
import { dateAndTime } from "./format";
import {
  ABANDONED,
  DONE,
  laneFor,
  laneTitle,
  openLanes,
  SPLIT_MEANING,
  SPLIT_TITLE,
  UNPLACEABLE,
  type LaneId,
} from "./lanes";
import { moveFor, refusalFor, targetsFrom, transitions } from "./moves";
import { RankSheet } from "./RankSheet";
import {
  inRankOrder,
  landedNote,
  needsConfirming,
  placementFor,
  stepFor,
  type Placement,
  type Side,
} from "./reorder";
import { arcOn, isParent, membersOf, parentOf } from "./split";
import type { Pane as ProjectPayload } from "../project/api";
import { sliceCount, sliceLine, slicePlans, slicesOf, type SlicePlan } from "../project/pane";
import { dateOf } from "../project/ProjectPane";
import { goalPath } from "../routes";
import { Button, Chip } from "../shell/controls";
import { failureMessage } from "../shell/workspace";

/**
 * The backlog as a board.
 *
 * One column per lane, in the order work moves through them, and every goal
 * in the column the server placed it in. Dragging a card lights the columns
 * it may be dropped on — the two moves that are real acts, and no others —
 * and dims the rest, each of which says in its own head what the move there
 * would have meant and that this build does not publish it.
 *
 * Three things are not columns. Unknown is not: a record this build cannot
 * place is named in one line above the lanes, with the reason it carries,
 * rather than in a box that is empty on every board but one. Concluded work
 * is not a wall: Done reaches back as far as the human asks it to, a day by
 * default, and the rest are one select away. And a goal retired by a split is
 * not a delivered outcome, so it stands with the closed items as its members
 * rather than in Done as a result.
 *
 * The filter bar narrows every lane at once, and each lane's count is the
 * count of what that lane is showing. A count that did not follow its own
 * column would be the one number here a human could not trust.
 *
 * A lane holding nothing is collapsed rather than dropped: it keeps its head,
 * its count and the one line that says why it is empty, and gives the rest of
 * its width to the lanes that have work in them. It is still a drop target,
 * and still answers a drag the way a wide lane does.
 *
 * The drag is the browser's own: draggable cards, dragstart, dragover, drop.
 * No dependency is involved, nothing is animated, and a drop changes nothing
 * on its own: it opens the sheet, and only the ledger's answer moves a card.
 *
 * Everything the mouse can do, the keyboard can do: each card carries a Move
 * button offering the same transitions and opening the same sheet.
 */

/** What the board asks the pane to open. */
export type Asked = { move: "approve" | "withdraw"; goal: Row };

/** The card being dragged: its id and the lane it is in. */
type Dragging = { id: string; lane: LaneId };

/**
 * One column: what it is called, what being in it means, the lane a drop on
 * it asks for, and what it is showing.
 *
 * It is not the lane table, because two of the board's columns are readings
 * of a lane rather than the lane itself: Done shows a window of the done
 * lane, and the split column shows the part of it the master refuses to call
 * delivered. Both carry `lane: "done"`, because that is what a drop on either
 * would be asking about, and the drop rules answer the lane.
 */
type BoardColumn = { key: string; title: string; meaning: string; lane: LaneId; rows: Row[]; head?: ReactNode };

export function Board({
  backlog,
  closedShown,
  onToggleClosed,
  onAct,
  onMoved,
  plans,
  filters,
  onFilters,
  window: reach,
  onWindow,
}: {
  backlog: Backlog;
  closedShown: boolean;
  onToggleClosed: () => void;
  onAct: (asked: Asked) => void;
  /** The ledger as it stands after a re-rank this board published itself. */
  onMoved: (after: Backlog) => void;
  /** The project's records, or null where they could not be read. */
  plans: ProjectPayload | null;
  filters: Filters;
  onFilters: (filters: Filters) => void;
  /** How far back Done reaches, in days, or null for every recorded one. */
  window: Window;
  onWindow: (days: Window) => void;
}) {
  const [dragging, setDragging] = useState<Dragging | null>(null);
  const [refused, setRefused] = useState<{ lane: LaneId; reason: string } | null>(null);
  // What the ledger made of the last re-rank, under the lane it was made in,
  // read from the board the act answered with rather than from what was asked
  // for: the two differ whenever the band shifted under the request.
  const [noted, setNoted] = useState<{ lane: LaneId; line: string } | null>(null);
  const [asking, setAsking] = useState<{ goal: Row; placement: Placement } | null>(null);

  // Every row the payload carries, which is what a relationship is read
  // against: a split member says what it is part of whether or not its parent
  // passes the filter, and a parent counts all of its members.
  const all = useMemo(() => [...backlog.rows, ...backlog.closed], [backlog]);
  const shown = useMemo(() => all.filter((row) => matches(row, filters)), [all, filters]);
  const now = useMemo(() => observedAt(backlog), [backlog]);
  // One plan per goal on the board, built once from the project's records:
  // the same derivation the goal page's Slices tab reads, so the card and the
  // tab cannot say different things about the same goal.
  const sliced = useMemo(
    () => (plans === null ? new Map() : slicePlans(plans, all.map((row) => row.ref.id))),
    [plans, all],
  );

  // A lane reads in the order the engine ranks work in — priority, then
  // sequence — because that is the order the chip on each card claims and the
  // order a drag inside the lane rearranges. Goals with no rank sort after
  // them by id, which is where the engine's own frontier puts them.
  const inLane = (lane: LaneId) => inRankOrder(shown.filter((row) => row.lane === lane));
  const parents = shown.filter(isParent);
  const delivered = inLane(DONE).filter((row) => !isParent(row) && concludedWithin(row, reach, now));
  const closed = [...inLane(ABANDONED), ...parents];

  const columns: BoardColumn[] = [
    ...openLanes.map((lane) => ({
      key: lane.id,
      title: lane.title,
      meaning: lane.meaning,
      lane: lane.id,
      rows: lane.id === "draft" ? [] : inLane(lane.id),
    })),
    {
      key: DONE,
      title: laneTitle(DONE),
      meaning: laneFor(DONE)?.meaning ?? "",
      lane: DONE,
      rows: delivered,
      head: <Reach days={reach} onChange={onWindow} />,
    },
  ];
  if (closedShown) {
    columns.push(
      {
        key: ABANDONED,
        title: laneTitle(ABANDONED),
        meaning: laneFor(ABANDONED)?.meaning ?? "",
        lane: ABANDONED,
        rows: inLane(ABANDONED),
      },
      { key: "split", title: SPLIT_TITLE, meaning: SPLIT_MEANING, lane: DONE, rows: parents },
    );
  }

  const drop = (lane: LaneId) => {
    if (dragging === null) {
      return;
    }
    const transition = moveFor(dragging.lane, lane);
    const goal = all.find((row) => row.ref.id === dragging.id) ?? null;
    setDragging(null);
    if (transition === null || goal === null) {
      setRefused({ lane, reason: refusalFor(dragging.lane, lane) });
      return;
    }
    setRefused(null);
    onAct({ move: transition.move, goal });
  };

  /**
   * Publishing a re-rank, or refusing to.
   *
   * A server that cannot act as the human refuses here, in the lane, before
   * anything is sent: the reason is the proof's own and it is the only thing
   * a human can act on. Everything else is the ledger's answer — the note
   * says where the goal actually landed, which is not always where the drop
   * asked for it, because the band may have shifted since this page was read.
   */
  const publish = (moved: Row, placement: Placement) => {
    if (!backlog.authority.proven) {
      setNoted(null);
      setRefused({ lane: moved.lane, reason: backlog.authority.reason });
      return;
    }
    setRefused(null);
    setNoted(null);
    rankGoal(moved.ref.id, placement.priority, placement.sequence)
      .then((after) => {
        setNoted({ lane: moved.lane, line: landedNote(moved.ref.id, after.rows) });
        onMoved(after);
      })
      .catch((error: unknown) => {
        setRefused({ lane: moved.lane, reason: failureMessage(error) });
      });
  };

  /** A re-rank a seat's work needs confirming first; the rest apply. */
  const rank = (moved: Row, placement: Placement) => {
    if (needsConfirming(moved)) {
      setAsking({ goal: moved, placement });
      return;
    }
    publish(moved, placement);
  };

  /**
   * A drop on a card. Inside the lane it is a re-rank; across lanes it is the
   * move the column would have answered, which the card has to pass on
   * itself because it stopped the drop reaching the column.
   */
  const dropOnCard = (target: Row, side: Side) => {
    if (dragging === null) {
      return;
    }
    if (dragging.lane !== target.lane) {
      drop(target.lane);
      return;
    }
    const moved = all.find((row) => row.ref.id === dragging.id) ?? null;
    setDragging(null);
    if (moved === null) {
      return;
    }
    const placement = placementFor(moved, target, side, all);
    if (placement === null) {
      // The drop asks for the rank the goal already has. Nothing is
      // published, because a ledger transaction that changes nothing is
      // still a ledger transaction.
      return;
    }
    rank(moved, placement);
  };

  const step = (moved: Row, direction: "up" | "down") => {
    const placement = stepFor(moved, direction, all);
    if (placement !== null) {
      rank(moved, placement);
    }
  };

  return (
    <div className="ms-board-frame">
      {!backlog.authority.proven && (
        <p className="ms-board-unproven" role="status">
          {backlog.authority.reason}
        </p>
      )}
      <FilterBar filters={filters} onChange={onFilters} seats={seatsOn(all)} arcs={arcsOn(all)} />
      <Unplaceable rows={inLane(UNPLACEABLE)} />
      <div className="ms-board" role="list">
        {columns.map((column) => (
          <Column
            key={column.key}
            column={column}
            all={all}
            sliced={sliced}
            statement={column.lane === "draft" ? backlog.draft.statement : ""}
            standing={standingFor(dragging, column.lane)}
            refusal={refused !== null && refused.lane === column.lane ? refused.reason : ""}
            note={noted !== null && noted.lane === column.lane ? noted.line : ""}
            onDragStart={(row) => {
              setRefused(null);
              setNoted(null);
              setDragging({ id: row.ref.id, lane: row.lane });
            }}
            onDragEnd={() => {
              setDragging(null);
            }}
            onDrop={() => {
              drop(column.lane);
            }}
            onDropOn={dropOnCard}
            onAct={onAct}
            onStep={step}
            onChooseRank={(row) => {
              setAsking({ goal: row, placement: { priority: row.priority, sequence: row.sequence } });
            }}
          />
        ))}
      </div>
      <Button aria-pressed={closedShown} onClick={onToggleClosed}>
        {closedShown ? "Hide" : "Show"} closed items ({closed.length})
      </Button>
      {asking !== null && (
        <RankSheet
          goal={asking.goal}
          placement={asking.placement}
          backlog={backlog}
          onClose={() => {
            setAsking(null);
          }}
          onDone={(after, id) => {
            setAsking(null);
            setNoted({ lane: asking.goal.lane, line: landedNote(id, after.rows) });
            onMoved(after);
          }}
        />
      )}
    </div>
  );
}

/**
 * The instant the window is measured from: the server's own observation, not
 * this browser's clock. The payload says when the ledger was read, and "the
 * last day" has to mean the last day of the thing being read; a browser whose
 * clock is a day out would otherwise empty the lane or fill it.
 */
function observedAt(backlog: Backlog): Date {
  const at = new Date(backlog.observedAt);
  return Number.isNaN(at.getTime()) ? new Date() : at;
}

/** How a column stands while a card is in the air. */
type Standing = "idle" | "target" | "closed" | "home";

function standingFor(dragging: Dragging | null, lane: LaneId): Standing {
  if (dragging === null) {
    return "idle";
  }
  if (dragging.lane === lane) {
    return "home";
  }
  return targetsFrom(dragging.lane).includes(lane) ? "target" : "closed";
}

/* ------------------------------------------------------------ the filters -- */

/**
 * What the board is narrowed to, said in one bar above the lanes.
 *
 * The seat and the arc offer what is on this board rather than what the fleet
 * or the plan could hold: a select offering a seat no card carries offers an
 * empty board. A value the browser remembered that nothing on the board
 * carries any more is offered all the same, as itself, so that a lane showing
 * nothing shows why and "clear" is one press away — a filter that widened
 * itself would be the board deciding what a human meant.
 */
function FilterBar({
  filters,
  onChange,
  seats,
  arcs,
}: {
  filters: Filters;
  onChange: (filters: Filters) => void;
  seats: string[];
  arcs: string[];
}) {
  const text = useId();
  return (
    <div className="ms-board-filters" role="group" aria-label="Narrow the board">
      <div className="ms-board-filter">
        <label htmlFor={text}>Find</label>
        <input
          id={text}
          type="search"
          className="ms-board-find"
          value={filters.text}
          placeholder="id or intent"
          onChange={(event) => {
            onChange({ ...filters, text: event.target.value });
          }}
        />
      </div>
      <Choose
        label="Priority"
        value={filters.priority}
        options={RANKS.map((rank) => ({ value: rank, title: rank }))}
        onChange={(value) => {
          onChange({ ...filters, priority: value as Rank });
        }}
      />
      <Choose
        label="Tier"
        value={filters.tier}
        options={RANKS.map((rank) => ({ value: rank, title: rank }))}
        onChange={(value) => {
          onChange({ ...filters, tier: value as Rank });
        }}
      />
      <Choose
        label="Seat"
        value={filters.seat}
        options={[{ value: NONE, title: "unassigned" }, ...offered(seats, filters.seat)]}
        onChange={(value) => {
          onChange({ ...filters, seat: value });
        }}
      />
      <Choose
        label="Arc"
        value={filters.arc}
        options={[{ value: NONE, title: "none" }, ...offered(arcs, filters.arc)]}
        onChange={(value) => {
          onChange({ ...filters, arc: value });
        }}
      />
      {anySet(filters) && (
        <button
          type="button"
          className="ms-project-act"
          onClick={() => {
            onChange(noFilters);
          }}
        >
          clear
        </button>
      )}
    </div>
  );
}

/**
 * The named values a select offers: what is on the board, plus whatever is
 * chosen, so that a choice is never silently dropped from the list that
 * explains it.
 */
function offered(present: string[], chosen: string): { value: string; title: string }[] {
  const name = nameOf(chosen);
  const names = name !== null && !present.includes(name) ? [...present, name] : present;
  return names.map((value) => ({ value: named(value), title: value }));
}

function Choose({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; title: string }[];
  onChange: (value: string) => void;
}) {
  const named_ = useId();
  return (
    <div className="ms-board-filter">
      <label htmlFor={named_}>{label}</label>
      <select
        id={named_}
        className="ms-board-select"
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        <option value={ANY}>any</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.title}
          </option>
        ))}
      </select>
    </div>
  );
}

/** How far back Done reaches, in the lane's own head. */
function Reach({ days, onChange }: { days: Window; onChange: (days: Window) => void }) {
  const named_ = useId();
  return (
    <div className="ms-board-filter ms-column-reach">
      <label htmlFor={named_}>Last</label>
      <select
        id={named_}
        className="ms-board-select"
        value={windowTitle(days)}
        onChange={(event) => {
          const at = WINDOWS.findIndex((candidate) => windowTitle(candidate) === event.target.value);
          onChange(at < 0 ? days : WINDOWS[at]);
        }}
      >
        {WINDOWS.map((candidate) => (
          <option key={windowTitle(candidate)} value={windowTitle(candidate)}>
            {windowTitle(candidate)}
          </option>
        ))}
      </select>
      <span className="ms-column-reach-unit">{days === null ? "" : days === 1 ? "day" : "days"}</span>
    </div>
  );
}

/**
 * The goals this build cannot place, above the lanes.
 *
 * It is a line and not a column, and it is nothing at all when the bucket is
 * empty. Opened, it names each goal with the reason the projection recorded
 * and opens it, because the reason is the only thing a human can act on.
 */
export function Unplaceable({ rows }: { rows: Row[] }) {
  if (rows.length === 0) {
    return null;
  }
  return (
    <details className="ms-board-unplaceable">
      <summary className="ms-board-unplaceable-line">
        {count(rows.length, "goal")} this build cannot place
      </summary>
      <ul className="ms-board-unplaceable-list">
        {rows.map((row) => (
          <li key={row.ref.id}>
            <NavLink className="ms-mono" to={goalPath(row.ref.id)}>
              {row.ref.id}
            </NavLink>
            <span className="ms-board-unplaceable-why">{row.gaps.join("; ")}</span>
          </li>
        ))}
      </ul>
    </details>
  );
}

/* ------------------------------------------------------------- the lanes -- */

function Column({
  column,
  all,
  sliced,
  statement,
  standing,
  refusal,
  note,
  onDragStart,
  onDragEnd,
  onDrop,
  onDropOn,
  onAct,
  onStep,
  onChooseRank,
}: {
  column: BoardColumn;
  all: readonly Row[];
  sliced: Map<string, SlicePlan>;
  statement: string;
  standing: Standing;
  refusal: string;
  /** What the ledger made of the last re-rank published from this lane. */
  note: string;
  onDragStart: (row: Row) => void;
  onDragEnd: () => void;
  onDrop: () => void;
  onDropOn: (target: Row, side: Side) => void;
  onAct: (asked: Asked) => void;
  onStep: (row: Row, direction: "up" | "down") => void;
  onChooseRank: (row: Row) => void;
}) {
  const rows = column.rows;
  const shown = statement === "" ? String(rows.length) : "—";
  return (
    <section
      className={`ms-column ms-column--${standing}${rows.length === 0 ? " ms-column--collapsed" : ""}`}
      role="listitem"
      aria-label={`${column.title}, ${shown}`}
      onDragOver={(event: DragEvent<HTMLElement>) => {
        // Every column takes the drag, because every column has an answer:
        // the two that act, and the rest that say why they do not.
        event.preventDefault();
      }}
      onDrop={(event: DragEvent<HTMLElement>) => {
        event.preventDefault();
        onDrop();
      }}
    >
      <header className="ms-column-head">
        <h2 className="ms-column-title" title={column.meaning}>
          {column.title}
          <span className="ms-column-count">{shown}</span>
        </h2>
        {column.head}
        {standing === "closed" && <p className="ms-column-closed">not a target</p>}
        {note !== "" && (
          <p className="ms-column-note" role="status">
            {note}
          </p>
        )}
        {refusal !== "" && (
          <p className="ms-column-refusal" role="alert">
            {refusal}
          </p>
        )}
      </header>
      <div className="ms-column-cards">
        {statement !== "" && <p className="ms-column-empty">{statement}</p>}
        {statement === "" && rows.length === 0 && <p className="ms-column-empty">Nothing here.</p>}
        {rows.map((row) => (
          <Card
            key={row.ref.id}
            row={row}
            all={all}
            plan={sliced.get(row.ref.id) ?? null}
            onDragStart={onDragStart}
            onDragEnd={onDragEnd}
            onDropOn={onDropOn}
            onAct={onAct}
            onStep={onStep}
            onChooseRank={onChooseRank}
          />
        ))}
      </div>
    </section>
  );
}

function Card({
  row,
  all,
  plan,
  onDragStart,
  onDragEnd,
  onDropOn,
  onAct,
  onStep,
  onChooseRank,
}: {
  row: Row;
  all: readonly Row[];
  /** This goal's slice plan, or null where the project could not be read. */
  plan: SlicePlan | null;
  onDragStart: (row: Row) => void;
  onDragEnd: () => void;
  onDropOn: (target: Row, side: Side) => void;
  onAct: (asked: Asked) => void;
  onStep: (row: Row, direction: "up" | "down") => void;
  onChooseRank: (row: Row) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  // Which half of this card the pointer is over, so a drop lands where the
  // line says it will rather than where the card happens to be.
  const [edge, setEdge] = useState<Side | null>(null);
  // Whether the card is a drag handle right now. It stops being one while the
  // pointer is down on something inside it that is not a handle.
  const [grabbable, setGrabbable] = useState(true);
  if (isParent(row)) {
    return <SplitCard row={row} members={membersOf(row, all)} />;
  }
  const moves = transitions.filter((transition) => transition.from === row.lane);
  const standing = blockerOf(row);
  const parent = parentOf(row, all);
  const arc = arcOn(row, all);
  return (
    <article
      className={`ms-card-goal${edge === null ? "" : ` ms-card-goal--${edge}`}`}
      draggable={grabbable}
      onMouseUp={() => {
        setGrabbable(true);
      }}
      onMouseLeave={() => {
        setGrabbable(true);
      }}
      onDragStart={(event: DragEvent<HTMLElement>) => {
        // The id travels as plain text so a drop outside this board carries
        // something meaningful rather than an internal handle.
        event.dataTransfer.setData("text/plain", row.ref.id);
        event.dataTransfer.effectAllowed = "move";
        onDragStart(row);
      }}
      onDragEnd={() => {
        setEdge(null);
        onDragEnd();
      }}
      // A card is its own drop target, because a drop inside a lane is a
      // re-rank rather than a move and the position depends on which card it
      // landed beside. The column's own drop still answers everywhere else,
      // so the drag between lanes is untouched.
      onDragOver={(event: DragEvent<HTMLElement>) => {
        event.preventDefault();
        event.stopPropagation();
        setEdge(sideOf(event));
      }}
      onDragLeave={() => {
        setEdge(null);
      }}
      onDrop={(event: DragEvent<HTMLElement>) => {
        event.preventDefault();
        event.stopPropagation();
        setEdge(null);
        onDropOn(row, sideOf(event));
      }}
    >
      <NavLink className="ms-card-id ms-mono" to={goalPath(row.ref.id)}>
        {row.ref.id}
      </NavLink>
      <p className="ms-card-intent">{row.intent}</p>
      <div className="ms-card-chips">
        <Chip>{row.state}</Chip>
        {row.priority > 0 && row.sequence > 0 && (
          <Chip>
            {row.priority}:{row.sequence}
          </Chip>
        )}
        {row.tier > 0 && <Chip>tier {row.tier}</Chip>}
        {row.claim !== undefined && <Chip marker>{row.claim.machine}</Chip>}
      </div>
      {parent !== null && (
        <p className="ms-card-lineage">
          part of{" "}
          <NavLink className="ms-mono" to={goalPath(parent.ref.id)}>
            {parent.ref.id}
          </NavLink>
        </p>
      )}
      {arc !== "" && <p className="ms-card-lineage">arc {arc}</p>}
      <Slices plan={plan} onGrabbable={setGrabbable} />
      {standing !== "" && <p className="ms-card-standing">{standing}</p>}
      {/* The keyboard's way to everything the mouse can do to this card, last
          because the record comes first: a card is read before it is moved.
          The two lane moves are what a drag between columns does; the three
          beneath them are what a drag inside one does, which no keyboard can
          perform by dragging. */}
      {(moves.length > 0 || ranked(row)) && (
        <div className="ms-card-moves">
          <Button
            className="ms-card-move"
            aria-expanded={menuOpen}
            aria-haspopup="menu"
            onClick={() => {
              setMenuOpen((open) => !open);
            }}
          >
            Move…
          </Button>
          {menuOpen && (
            <div className="ms-card-menu" role="menu" aria-label={`Move ${row.ref.id}`}>
              {moves.map((transition) => (
                <Button
                  key={transition.to}
                  role="menuitem"
                  className="ms-card-move"
                  onClick={() => {
                    setMenuOpen(false);
                    onAct({ move: transition.move, goal: row });
                  }}
                >
                  To {laneTitle(transition.to)} ({transition.verb})
                </Button>
              ))}
              {ranked(row) && (
                <>
                  <Button
                    role="menuitem"
                    className="ms-card-move"
                    disabled={stepFor(row, "up", all) === null}
                    onClick={() => {
                      setMenuOpen(false);
                      onStep(row, "up");
                    }}
                  >
                    Move up
                  </Button>
                  <Button
                    role="menuitem"
                    className="ms-card-move"
                    disabled={stepFor(row, "down", all) === null}
                    onClick={() => {
                      setMenuOpen(false);
                      onStep(row, "down");
                    }}
                  >
                    Move down
                  </Button>
                  <Button
                    role="menuitem"
                    className="ms-card-move"
                    onClick={() => {
                      setMenuOpen(false);
                      onChooseRank(row);
                    }}
                  >
                    Set priority…
                  </Button>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </article>
  );
}

/**
 * What a card says about its slices: one line, opening in place to the list.
 *
 * The line carries what is recorded and nothing more — how many slices the
 * governing designs plan, and when slicing started — with whichever of the two
 * the records answer. There is no state beside a slice and there is not going
 * to be one until something owns a slice plan: the ledger records that slicing
 * began and nothing finer, and reading "done" or "underway" out of prose
 * somebody wrote in a design or a next step is the reconstruction the master
 * refuses. Each slice links to the design that lists it, which is where its
 * state is discussed by the only thing qualified to discuss it.
 *
 * It is closed until it is opened, because a card is read before it is opened,
 * and the summary stops a drag rather than starting one: a human reaching for
 * the disclosure is not reaching for the card.
 */
function Slices({ plan, onGrabbable }: { plan: SlicePlan | null; onGrabbable: (may: boolean) => void }) {
  if (plan === null) {
    return null;
  }
  const line = sliceLine(sliceCount(plan), plan.started === null ? "" : dateOf(plan.started.at));
  if (line === "") {
    return null;
  }
  const slices = slicesOf(plan);
  return (
    <details
      className="ms-card-slices"
      // A drag that begins on the disclosure is a drag nobody asked for, and
      // it cannot be refused where it is felt: the browser fires dragstart on
      // the draggable element, which is the card, not on the child the pointer
      // was over, so preventing it here would prevent an event that never
      // arrives. What works is taking the handle away before the browser looks
      // for one — the card stops being draggable while the pointer is down on
      // this summary, and becomes draggable again when it is released.
      onMouseDown={() => {
        onGrabbable(false);
      }}
    >
      <summary className="ms-card-slices-line">{line}</summary>
      {slices.length > 0 && (
        <ul className="ms-card-slices-list">
          {slices.map((slice) => (
            <li key={slice.key}>
              <span>{slice.text}</span>{" "}
              <NavLink className="ms-card-slices-design" to={slice.to} title={slice.title}>
                {slice.title}
              </NavLink>
            </li>
          ))}
        </ul>
      )}
    </details>
  );
}

/**
 * A goal a split retired, as what it became.
 *
 * It carries no state chip and no move: the master is explicit that a parent
 * retired by decomposition must not read as a delivered outcome merely
 * because its underlying state is done. What it is, is its members, and each
 * of them opens.
 */
function SplitCard({ row, members }: { row: Row; members: Row[] }) {
  return (
    <article className="ms-card-goal ms-card-goal--split">
      <NavLink className="ms-card-id ms-mono" to={goalPath(row.ref.id)}>
        {row.ref.id}
      </NavLink>
      <p className="ms-card-intent">{row.intent}</p>
      <p className="ms-card-split">Split into {count(members.length, "goal")}</p>
      {members.length > 0 && (
        <ul className="ms-card-members">
          {members.map((member) => (
            <li key={member.ref.id}>
              <NavLink className="ms-mono" to={goalPath(member.ref.id)}>
                {member.ref.id}
              </NavLink>
            </li>
          ))}
        </ul>
      )}
    </article>
  );
}

/**
 * Which half of a card the pointer is over. A drop above the middle puts the
 * dragged card before this one, and below it after, which is what the line
 * drawn on that edge is promising.
 */
function sideOf(event: DragEvent<HTMLElement>): Side {
  const box = event.currentTarget.getBoundingClientRect();
  return event.clientY < box.top + box.height / 2 ? "above" : "below";
}

/** True when this goal is in a priority band, and so has a rank to change. */
function ranked(row: Row): boolean {
  return row.where === "live" && row.priority > 0 && row.sequence > 0;
}

/** A count and the word for it, pluralised the one way English needs here. */
function count(of: number, word: string): string {
  return `${String(of)} ${word}${of === 1 ? "" : "s"}`;
}

/**
 * The one line a card carries beyond its own facts: what holds it, or what it
 * is waiting on a human for. A card is not the record; the id opens that.
 */
function blockerOf(row: Row): string {
  if (row.fence !== undefined) {
    return `stopped ${dateAndTime(row.fence.closedAt)}: ${row.fence.reason}`;
  }
  if (row.waiting !== undefined && row.waiting.reason !== "") {
    return `parked: ${row.waiting.reason}`;
  }
  if (row.openBlockers.length > 0) {
    return `blocked by ${row.openBlockers.join(", ")}`;
  }
  if (row.approved !== undefined && row.approved.expired) {
    return `approval expired: ${row.approved.expiredWhy}`;
  }
  return row.gaps[0] ?? "";
}
