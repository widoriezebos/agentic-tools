import { useState, type DragEvent } from "react";
import { NavLink } from "react-router";

import type { Backlog, Row } from "./api";
import { dateAndTime } from "./format";
import { closedLanes, openLanes, type Lane, type LaneId } from "./lanes";
import { moveFor, refusalFor, targetsFrom, transitions } from "./moves";
import { goalPath } from "../routes";
import { Button, Chip } from "../shell/controls";

/**
 * The backlog as a board.
 *
 * One column per lane, in the order work moves through them, and every goal
 * in the column the server placed it in. Dragging a card lights the columns
 * it may be dropped on — the two moves that are real acts, and no others —
 * and dims the rest, each of which says in its own head what the move there
 * would have meant and that this build does not publish it.
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

export function Board({
  backlog,
  closedShown,
  onToggleClosed,
  onAct,
}: {
  backlog: Backlog;
  closedShown: boolean;
  onToggleClosed: () => void;
  onAct: (asked: Asked) => void;
}) {
  const [dragging, setDragging] = useState<Dragging | null>(null);
  const [refused, setRefused] = useState<{ lane: LaneId; reason: string } | null>(null);
  const columns = closedShown ? [...openLanes, ...closedLanes] : openLanes;

  const drop = (lane: Lane) => {
    if (dragging === null) {
      return;
    }
    const transition = moveFor(dragging.lane, lane.id);
    const goal = goalIn(backlog, dragging.id);
    setDragging(null);
    if (transition === null || goal === null) {
      setRefused({ lane: lane.id, reason: refusalFor(dragging.lane, lane.id) });
      return;
    }
    setRefused(null);
    onAct({ move: transition.move, goal });
  };

  return (
    <div className="ms-board-frame">
      {!backlog.authority.proven && (
        <p className="ms-board-unproven" role="status">
          {backlog.authority.reason}
        </p>
      )}
      <div className="ms-board" role="list">
        {columns.map((lane) => (
          <Column
            key={lane.id}
            lane={lane}
            rows={rowsIn(backlog, lane.id)}
            count={backlog.counts[lane.id] ?? 0}
            statement={lane.id === "draft" ? backlog.draft.statement : ""}
            standing={standingFor(dragging, lane.id)}
            refusal={refused !== null && refused.lane === lane.id ? refused.reason : ""}
            onDragStart={(row) => {
              setRefused(null);
              setDragging({ id: row.ref.id, lane: row.lane });
            }}
            onDragEnd={() => {
              setDragging(null);
            }}
            onDrop={() => {
              drop(lane);
            }}
            onAct={onAct}
          />
        ))}
      </div>
      <Button aria-pressed={closedShown} onClick={onToggleClosed}>
        {closedShown ? "Hide" : "Show"} closed items ({backlog.closed.length})
      </Button>
    </div>
  );
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

function Column({
  lane,
  rows,
  count,
  statement,
  standing,
  refusal,
  onDragStart,
  onDragEnd,
  onDrop,
  onAct,
}: {
  lane: Lane;
  rows: Row[];
  count: number;
  statement: string;
  standing: Standing;
  refusal: string;
  onDragStart: (row: Row) => void;
  onDragEnd: () => void;
  onDrop: () => void;
  onAct: (asked: Asked) => void;
}) {
  return (
    <section
      className={`ms-column ms-column--${standing}${rows.length === 0 ? " ms-column--collapsed" : ""}`}
      role="listitem"
      aria-label={`${lane.title}, ${String(count)}`}
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
        <h2 className="ms-column-title" title={lane.meaning}>
          {lane.title}
          <span className="ms-column-count">{lane.id === "draft" ? "—" : count}</span>
        </h2>
        {standing === "closed" && <p className="ms-column-closed">not a target</p>}
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
          <Card key={row.ref.id} row={row} onDragStart={onDragStart} onDragEnd={onDragEnd} onAct={onAct} />
        ))}
      </div>
    </section>
  );
}

function Card({
  row,
  onDragStart,
  onDragEnd,
  onAct,
}: {
  row: Row;
  onDragStart: (row: Row) => void;
  onDragEnd: () => void;
  onAct: (asked: Asked) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const moves = transitions.filter((transition) => transition.from === row.lane);
  const standing = blockerOf(row);
  return (
    <article
      className="ms-card-goal"
      draggable
      onDragStart={(event: DragEvent<HTMLElement>) => {
        // The id travels as plain text so a drop outside this board carries
        // something meaningful rather than an internal handle.
        event.dataTransfer.setData("text/plain", row.ref.id);
        event.dataTransfer.effectAllowed = "move";
        onDragStart(row);
      }}
      onDragEnd={onDragEnd}
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
      {standing !== "" && <p className="ms-card-standing">{standing}</p>}
      {/* The keyboard's way to the same moves, last because the record comes
          first: a card is read before it is moved. A lane with no move from
          it carries no button, which is itself the answer. */}
      {moves.length > 0 && (
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
                  To {titleOf(transition.to)} ({transition.verb})
                </Button>
              ))}
            </div>
          )}
        </div>
      )}
    </article>
  );
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

function titleOf(lane: LaneId): string {
  return [...openLanes, ...closedLanes].find((candidate) => candidate.id === lane)?.title ?? lane;
}

function rowsIn(backlog: Backlog, lane: LaneId): Row[] {
  const rows = lane === "done" || lane === "abandoned" ? backlog.closed : backlog.rows;
  return rows.filter((row) => row.lane === lane);
}

function goalIn(backlog: Backlog, id: string): Row | null {
  return [...backlog.rows, ...backlog.closed].find((row) => row.ref.id === id) ?? null;
}
