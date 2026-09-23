import { useMemo, useState } from "react";

import "./backlog.css";
import { ActSheet, type Request } from "./ActSheet";
import type { Backlog, Ledger, Row } from "./api";
import { Board, Unplaceable, type Asked } from "./Board";
import { arcsOn, seatsOn, type Filters, type Window } from "./filters";
import { clockTime, shortTip } from "./format";
import { DraftGroup, LaneGroup } from "./LaneGroup";
import { anchorFor, closedLanes, shownLanes, UNPLACEABLE, type LaneId } from "./lanes";
import { OpenSheet } from "./OpenSheet";
import { Statement } from "./Statement";
import { useBacklog, useSlicePlans } from "./state";
import { syncOf } from "./sync";
import { BoardToolbar } from "./Toolbar";
import { Pane } from "../panes/Pane";
import type { Pane as ProjectPayload } from "../project/api";
import {
  readBacklogFilters,
  readBacklogView,
  readDoneWindow,
  writeBacklogFilters,
  writeBacklogView,
  writeDoneWindow,
  type BacklogView,
} from "../storage";
import { Button } from "../shell/controls";

/**
 * The backlog, as a board or as a list.
 *
 * Every goal at the accepted tip appears once, in the lane the server placed
 * it in, with what the record says and what it does not. The board is the
 * day-to-day surface and carries the two acts; the list is for scanning and
 * comparing, and is unchanged. Which one a human last chose is remembered the
 * way the shell remembers its drawer.
 *
 * Concluded work sits behind one toggle either way, because 430 records would
 * otherwise bury the 155 that are live. Freshness belongs to the server: this
 * page reads when it mounts and when a human asks, and says what the last
 * fetch found either way.
 *
 * One toolbar stands between the top of the work area and the work, and
 * nothing else does. What the last read found is a chip in it; the report is
 * that chip's tooltip; and a line appears under the toolbar only when
 * something is actually wrong. The goals this build cannot place moved below
 * the lanes, beside the closed items, where a disclosure that is empty on
 * almost every board costs the work no room.
 */
export function BacklogPane() {
  const { backlog, refresh, moved, attempt } = useBacklog();
  // What the checkout's design records say about each goal's slices, read
  // beside the ledger and refreshed with it. A project that cannot be read
  // leaves the cards without their slice line and changes nothing else.
  const plans = useSlicePlans(attempt);

  if (backlog.state === "loading") {
    return (
      <Pane title="Backlog">
        <div className="ms-backlog">
          <BoardToolbar reading={null} narrowing={null} sync={null} observedAt="" onRefresh={refresh} />
        </div>
      </Pane>
    );
  }

  if (backlog.state === "failed") {
    return (
      <Pane title="Backlog">
        <div className="ms-backlog">
          <div className="ms-error">
            <p className="ms-error-heading">The backlog could not be read</p>
            <p className="ms-error-detail">{backlog.message}</p>
            <Button onClick={refresh}>Retry</Button>
          </div>
        </div>
      </Pane>
    );
  }

  return (
    <Pane title="Backlog">
      <Read backlog={backlog.backlog} plans={plans} onRefresh={refresh} onMoved={moved} />
    </Pane>
  );
}

function Read({
  backlog,
  plans,
  onRefresh,
  onMoved,
}: {
  backlog: Backlog;
  /** The project's records, or null where they could not be read. */
  plans: ProjectPayload | null;
  onRefresh: () => void;
  onMoved: (moved: Backlog) => void;
}) {
  const [closedShown, setClosedShown] = useState(false);
  const [view, setView] = useState<BacklogView>(() => readBacklogView());
  const [asked, setAsked] = useState<Request | null>(null);
  // What the board is narrowed to, and how far back Done reaches. Both are
  // read once, as what this browser was last left on, and written wherever a
  // human changes one.
  const [filters, setFilters] = useState<Filters>(() => readBacklogFilters());
  const [reach, setReach] = useState<Window>(() => readDoneWindow());
  const [opening, setOpening] = useState(false);
  const ledger = backlog.ledger;
  const closedCount = backlog.closed.length;
  // Whether the page is reading a current ledger, which the chip says in four
  // words and the banner says in a sentence when it is not.
  const sync = syncOf(ledger, backlog.observedAt);
  // What the selects offer is what this board carries, so both are read from
  // every row the payload holds rather than from what the fleet could hold.
  const all = useMemo(() => [...backlog.rows, ...backlog.closed], [backlog]);
  const seats = useMemo(() => seatsOn(all), [all]);
  const arcs = useMemo(() => arcsOn(all), [all]);
  // The board and the list want opposite things from the pane: the list wants
  // a measure to read down, the board wants the whole work area to read
  // across and the height that lets its lanes scroll on their own.
  const boarding = ledger.state === "read" && view === "board";

  const choose = (chosen: BacklogView) => {
    setView(chosen);
    writeBacklogView(chosen);
  };

  const narrow = (chosen: Filters) => {
    setFilters(chosen);
    writeBacklogFilters(chosen);
  };

  return (
    <div className={boarding ? "ms-backlog ms-backlog--board" : "ms-backlog"}>
      <BoardToolbar
        reading={
          ledger.state === "read"
            ? { view, onView: choose, onNew: () => { setOpening(true); } }
            : null
        }
        narrowing={boarding ? { filters, onFilters: narrow, seats, arcs } : null}
        sync={sync}
        observedAt={backlog.observedAt}
        onRefresh={onRefresh}
      />
      {/* A ledger that did not project says so in its own statement below, so
          the banner is for the one wrong this page can otherwise only whisper:
          a read that worked and a fetch that did not. */}
      {sync.state === "wrong" && ledger.state === "read" && (
        <p className="ms-board-problem" role="status">
          {sync.wrong}
        </p>
      )}
      {opening && (
        <OpenSheet
          backlog={backlog}
          onClose={() => { setOpening(false); }}
          onDone={(after) => {
            setOpening(false);
            onMoved(after);
          }}
        />
      )}
      {boarding && (
        <Board
          backlog={backlog}
          closedShown={closedShown}
          onToggleClosed={() => { setClosedShown((shown) => !shown); }}
          onAct={(act: Asked) => { setAsked({ move: act.move, goal: act.goal }); }}
          onMoved={onMoved}
          plans={plans}
          filters={filters}
          window={reach}
          onWindow={(days) => { setReach(days); writeDoneWindow(days); }}
        />
      )}
      {asked !== null && (
        <ActSheet
          request={asked}
          backlog={backlog}
          onClose={() => { setAsked(null); }}
          onDone={(after) => {
            setAsked(null);
            onMoved(after);
          }}
        />
      )}
      {ledger.state !== "read" && <LedgerStatement backlog={backlog} />}
      {ledger.state === "read" && view === "list" && (
        <>
          <p className="ms-lane-index">
            {shownLanes.map((lane, position) => (
              <span key={lane.id}>
                {position > 0 && <span className="ms-lane-index-rule"> · </span>}
                <a className="ms-lane-anchor" href={`#${anchorFor(lane.id)}`}>
                  {lane.title}
                </a>
                {lane.id === "draft" ? " not read" : ` ${String(countOf(backlog, lane.id))}`}
              </span>
            ))}
          </p>
          {shownLanes.map((lane) =>
            lane.id === "draft" ? (
              <DraftGroup key={lane.id} lane={lane} statement={backlog.draft.statement} />
            ) : (
              <LaneGroup
                key={lane.id}
                lane={lane}
                count={countOf(backlog, lane.id)}
                rows={rowsIn(backlog.rows, lane.id)}
                tip={ledger.tip}
                unanswered={backlog.admission.answered ? "" : backlog.admission.message}
              />
            ),
          )}
          {/* Unknown is no lane and no group, here as on the board: the goals
              this build cannot place stand under the work with the closed
              items, named with the reason each carries, so that what the
              build could not place costs the work it could no room. */}
          <div className="ms-backlog-below">
            <Button aria-pressed={closedShown} onClick={() => { setClosedShown((shown) => !shown); }}>
              {closedShown ? "Hide" : "Show"} closed items ({closedCount})
            </Button>
            <Unplaceable rows={rowsIn(backlog.rows, UNPLACEABLE)} />
          </div>
          {closedShown &&
            closedLanes.map((lane) => (
              <LaneGroup
                key={lane.id}
                lane={lane}
                count={countOf(backlog, lane.id)}
                rows={rowsIn(backlog.closed, lane.id)}
                tip={ledger.tip}
                unanswered=""
              />
            ))}
          <p className="ms-backlog-footer">
            The outline, dependencies, filters, and goal detail arrive with g1-s10 and g1-s11.
          </p>
        </>
      )}
    </div>
  );
}

function countOf(backlog: Backlog, lane: LaneId): number {
  return backlog.counts[lane] ?? 0;
}

function rowsIn(rows: Row[], lane: LaneId): Row[] {
  return rows.filter((row) => row.lane === lane);
}

/** What the pane says for each way the accepted ledger cannot be projected. */
function LedgerStatement({ backlog }: { backlog: Backlog }) {
  const ledger = backlog.ledger;
  switch (ledger.state) {
    case "absent":
      return (
        <Statement
          heading="No accepted tip in this clone yet"
          body={`This clone's accepted ref, refs/metasystem/goals/accepted, is created by this server's fetch loop once the canonical branch's tip validates. git clone and git fetch never bring it, so a new clone starts without one. This build reads goals from the accepted tip, never from the working tree, ${holding(backlog)}`}
          note={absentNote(ledger)}
        />
      );
    case "no-ledger":
      return (
        <Statement
          heading="The accepted tip carries no ledger"
          body={`The ref points at ${shortTip(ledger.tip)}, whose tree has no plans/goals/backlog.md. The engine never sets the ref to such a commit, so something else did. The fetch loop cannot move it: ${lastTick(ledger)}.`}
          note="No verb heals this. Remove the ref the way it was made, from a terminal: git update-ref -d refs/metasystem/goals/accepted; the loop then creates it from the canonical branch."
        />
      );
    case "broken":
      return (
        <Statement
          heading="The accepted ref cannot be read"
          body={`${ledger.message}. The last fetch: ${lastTick(ledger)}.`}
          note={ledger.fetch.outcome === "failed" ? "Repair refs/metasystem/goals/accepted before continuing." : undefined}
        />
      );
    case "unreadable":
      return (
        <Statement
          heading={`The ledger at ${shortTip(ledger.tip)} does not validate`}
          body="The engine refuses a tree with any problem whole rather than showing part of it, so this is every problem it found."
          note={`The last fetch: ${lastTick(ledger)}.`}
        >
          <ul className="ms-problems">
            {ledger.problems.map((problem) => (
              <li className="ms-mono" key={problem}>
                {problem}
              </li>
            ))}
          </ul>
        </Statement>
      );
    default:
      return (
        <Statement
          heading="The ledger could not be projected"
          body={`${ledger.message}. The last fetch: ${lastTick(ledger)}.`}
          note="Check goal.sync-remote and goal.sync-branch in this checkout's git configuration."
        />
      );
  }
}

/** What the checkout holds beside the tip, where a count could be taken. */
function holding(backlog: Backlog): string {
  const { liveFiles, archivedFiles } = backlog.workingTree;
  if (liveFiles === null || archivedFiles === null) {
    return "whose goal files this build did not count.";
  }
  return `which here holds ${String(liveFiles)} goal files and ${String(archivedFiles)} archived records.`;
}

function absentNote(ledger: Ledger): string {
  const loop = ledger.fetch;
  switch (loop.outcome) {
    case "running":
      return `Fetching since ${clockTime(loop.startedAt)}.`;
    case "failed":
      return `The last fetch, at ${clockTime(loop.finishedAt)}, failed: ${loop.message}. Next attempt ${
        loop.nextAt === "" ? "once the server is running again" : clockTime(loop.nextAt)
      }.`;
    default:
      return loop.nextAt === ""
        ? "The server is stopping, so no fetch is due."
        : `First fetch due ${clockTime(loop.nextAt)}.`;
  }
}

/** The loop's last outcome, in a clause a statement can carry. */
export function lastTick(ledger: Ledger): string {
  const loop = ledger.fetch;
  switch (loop.outcome) {
    case "never":
      return "no fetch has completed yet";
    case "running":
      return `running since ${clockTime(loop.startedAt)}`;
    case "advanced":
      return `accepted ${shortTip(loop.tip)} at ${clockTime(loop.finishedAt)}`;
    case "current":
      return `${loop.detail} at ${clockTime(loop.finishedAt)}`;
    case "failed":
      return `failed at ${clockTime(loop.finishedAt)}: ${loop.message}`;
  }
}
