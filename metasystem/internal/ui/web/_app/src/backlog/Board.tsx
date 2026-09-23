import { useId, useMemo, useRef, useState, type DragEvent, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent, type ReactNode } from "react";
import { NavLink, useNavigate } from "react-router";

import { actingAs } from "./acting";
import { BacklogError, rankGoal, type Backlog, type Row } from "./api";
import { concludedWithin, matches, windowTitle, WINDOWS, type Filters, type Window } from "./filters";
import { dateAndTime } from "./format";
import {
  ABANDONED,
  DONE,
  laneFor,
  laneTitle,
  shownLanes,
  SPLIT_HELP,
  SPLIT_TITLE,
  UNPLACEABLE,
  type LaneId,
} from "./lanes";
import { CardMenu } from "./CardMenu";
import { offersFor, opensMenu, type At, type OfferId } from "./menu";
import { moveFor, refusalFor, targetsFrom } from "./moves";
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
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import type { Pane as ProjectPayload } from "../project/api";
import { sliceCount, sliceLine, slicePlans, slicesOf, type SlicePlan } from "../project/pane";
import { dateOf } from "../project/ProjectPane";
import { goalPath } from "../routes";
import { Button, Chip } from "../shell/controls";
import { useSession } from "../shell/identity";
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
 * Four things are not columns. Unknown is not: a record this build cannot
 * place is a disclosure under the lanes, with the reason each one carries,
 * rather than a box that is empty on every board but one. Draft is not,
 * while nothing reads drafts: a column that could only say so took a seventh
 * of the board to say it. Concluded work is not a wall: Done reaches back as
 * far as the human asks it to, a day by default, and the rest are one select
 * away. And a goal retired by a split is not a delivered outcome, so it
 * stands with the closed items as its members rather than in Done as a
 * result.
 *
 * The board is lanes and nothing else. What it is narrowed to, and whether
 * the page is current, are the toolbar's, one row above; each lane's count is
 * the count of what that lane is showing. A count that did not follow its own
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
 * Dragging is the way. No card carries a control of any kind: a board is a
 * hundred cards, and a button on each of them is a hundred buttons competing
 * with the work they are about. The acts are in a menu that is not there until
 * it is asked for, by the pointer's own gesture or by Shift+F10 and the Menu
 * key, and it lists what that card can actually do in the lane it is in and
 * nothing else. So everything the mouse can do, the keyboard can do, and the
 * board still shows only work.
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
type BoardColumn = { key: string; title: string; help: HelpId | null; lane: LaneId; rows: Row[]; head?: ReactNode };

export function Board({
  backlog,
  closedShown,
  onToggleClosed,
  onAct,
  onMoved,
  plans,
  filters,
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
  /** What the toolbar has narrowed every lane to. */
  filters: Filters;
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
  const navigate = useNavigate();
  // Whether the board may act, and as whom: the browser session where one is
  // signed in, and the server's own boot proof otherwise.
  const { session, askToSignIn } = useSession();
  const acting = actingAs(backlog.authority, session);
  const retried = useRef(false);

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
    ...shownLanes.map((lane) => ({
      key: lane.id,
      title: lane.title,
      help: lane.help,
      lane: lane.id,
      rows: lane.id === "draft" ? [] : inLane(lane.id),
    })),
    {
      key: DONE,
      title: laneTitle(DONE),
      help: laneFor(DONE)?.help ?? null,
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
        help: laneFor(ABANDONED)?.help ?? null,
        lane: ABANDONED,
        rows: inLane(ABANDONED),
      },
      { key: "split", title: SPLIT_TITLE, help: SPLIT_HELP, lane: DONE, rows: parents },
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
    if (!acting.proven) {
      setNoted(null);
      setRefused({ lane: moved.lane, reason: acting.reason });
      askToSignIn(() => {
        publish(moved, placement);
      });
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
        if (error instanceof BacklogError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(() => {
            publish(moved, placement);
          });
          return;
        }
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
            onOpen={(row) => {
              void navigate(goalPath(row.ref.id));
            }}
            onChooseRank={(row) => {
              setAsking({ goal: row, placement: { priority: row.priority, sequence: row.sequence } });
            }}
          />
        ))}
      </div>
      {/* What stands under the board: the work that is over, and the records
          this build could not place. Both are disclosures of the same kind,
          because both are things a human opens now and then rather than
          things the board is about. */}
      <div className="ms-board-below">
        <Button aria-pressed={closedShown} onClick={onToggleClosed}>
          {closedShown ? "Hide" : "Show"} closed items ({closed.length})
        </Button>
        <Unplaceable rows={inLane(UNPLACEABLE)} />
      </div>
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
 * The goals this build cannot place, under the lanes.
 *
 * It is a disclosure and not a column, and it is nothing at all when the
 * bucket is empty, which is almost every board. It stood above the lanes and
 * took a band of the work area from the work in order to say nothing; it
 * stands beside the closed items now, and says the same thing to whoever
 * opens it. Opened, it names each goal with the reason the projection
 * recorded and opens it, because the reason is the only thing a human can act
 * on.
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
  onOpen,
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
  onOpen: (row: Row) => void;
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
        {/* What a lane means used to be the browser's own tooltip on this
            heading, which only a mouse could reach and which said nothing to
            a keyboard or a finger. The help icon says the same thing to all
            three, and stopping its press keeps the column's drag intact. */}
        <h2 className="ms-column-title">
          {column.title}
          <span className="ms-column-count">{shown}</span>
          {column.help !== null && <Help id={column.help} />}
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
            onOpen={onOpen}
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
  onOpen,
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
  onOpen: (row: Row) => void;
}) {
  // Where the menu was asked for, or null when it was not asked for at all.
  // Nothing on the card opens it: the pointer's own gesture does, and so do
  // the two keystrokes that mean the same thing without one.
  const [menuAt, setMenuAt] = useState<At | null>(null);
  // Which half of this card the pointer is over, so a drop lands where the
  // line says it will rather than where the card happens to be.
  const [edge, setEdge] = useState<Side | null>(null);
  // Whether the card is a drag handle right now. It stops being one while the
  // pointer is down on something inside it that is not a handle.
  const [grabbable, setGrabbable] = useState(true);
  if (isParent(row)) {
    // A goal a split retired offers no act: it is a historical record, and its
    // links are what there is to do with it. Right-clicking it gets the
    // browser's own menu, which is the honest answer to "what can I do here".
    return <SplitCard row={row} members={membersOf(row, all)} />;
  }
  const standing = blockerOf(row);

  /** What each row of the menu does when it is chosen. */
  const choose = (id: OfferId) => {
    switch (id) {
      case "approve":
      case "withdraw":
        onAct({ move: id, goal: row });
        return;
      case "up":
      case "down":
        onStep(row, id === "up" ? "up" : "down");
        return;
      case "rank":
        onChooseRank(row);
        return;
      case "open":
        onOpen(row);
    }
  };
  const parent = parentOf(row, all);
  const arc = arcOn(row, all);
  return (
    <article
      className={`ms-card-goal${edge === null ? "" : ` ms-card-goal--${edge}`}`}
      draggable={grabbable}
      // The card is a tab stop so that the keyboard can reach the acts the
      // pointer reaches: there is no button to tab to, because there is no
      // button.
      tabIndex={0}
      onMouseUp={() => {
        setGrabbable(true);
      }}
      onMouseLeave={() => {
        setGrabbable(true);
      }}
      onContextMenu={(event: MouseEvent<HTMLElement>) => {
        event.preventDefault();
        setMenuAt({ x: event.clientX, y: event.clientY });
      }}
      onKeyDown={(event: ReactKeyboardEvent<HTMLElement>) => {
        if (!opensMenu(event.key, event.shiftKey)) {
          return;
        }
        event.preventDefault();
        // No pointer said where, so the card says where: its own top-left
        // corner, which is where a menu about this card belongs.
        const box = event.currentTarget.getBoundingClientRect();
        setMenuAt({ x: box.left + 8, y: box.top + 8 });
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
      {menuAt !== null && (
        <CardMenu
          at={menuAt}
          label={`Acts on ${row.ref.id}`}
          offers={offersFor(row, all)}
          onClose={() => {
            setMenuAt(null);
          }}
          onChoose={(id) => {
            setMenuAt(null);
            choose(id);
          }}
        />
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
