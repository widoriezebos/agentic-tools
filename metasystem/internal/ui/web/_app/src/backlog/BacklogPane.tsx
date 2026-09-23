import { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router";

import "./backlog.css";
import { ActSheet, type Request } from "./ActSheet";
import type { Backlog, Ledger, Row } from "./api";
import { Board, observedAt, Unplaceable, type Asked } from "./Board";
import { arcsOn, noFilters, seatsOn, type Filters, type Window } from "./filters";
import { clockTime, shortTip } from "./format";
import { DraftGroup, LaneGroup } from "./LaneGroup";
import { anchorFor, closedLanes, shownLanes, UNPLACEABLE, type LaneId } from "./lanes";
import { OpenSheet } from "./OpenSheet";
import { landingFor, placementOf, SHOWN, type Landing } from "./showing";
import { Statement } from "./Statement";
import { useBacklog, useSlicePlans } from "./state";
import { syncOf } from "./sync";
import { BoardToolbar } from "./Toolbar";
import { Pane } from "../panes/Pane";
import type { Pane as ProjectPayload } from "../project/api";
import { SHOWN_GOAL } from "../routes";
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
  // Which goal a link asked this page to show. It is read here and answered
  // once the ledger is, below; and it is dropped from the address as soon as
  // it has been answered, so that a reload is this page rather than that
  // arrival a second time.
  const [address, setAddress] = useSearchParams();
  const asked = address.get(SHOWN_GOAL) ?? "";

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
      <Read
        backlog={backlog.backlog}
        plans={plans}
        onRefresh={refresh}
        onMoved={moved}
        asked={asked}
        onLanded={() => {
          setAddress({}, { replace: true });
        }}
      />
    </Pane>
  );
}

function Read({
  backlog,
  plans,
  onRefresh,
  onMoved,
  asked,
  onLanded,
}: {
  backlog: Backlog;
  /** The project's records, or null where they could not be read. */
  plans: ProjectPayload | null;
  onRefresh: () => void;
  onMoved: (moved: Backlog) => void;
  /** The goal the address asked this page to show, or "" for none. */
  asked: string;
  /** Said once the landing has been made, so the address can drop the goal. */
  onLanded: () => void;
}) {
  const ledger = backlog.ledger;
  // What this page opens on, decided once, when the read it is showing
  // arrived: what this browser was last left on, widened or opened where the
  // address asked for a goal that would otherwise be out of sight. None of it
  // is written back, because a landing changes what this page is showing and
  // not what this browser prefers.
  const [opened] = useState(() => opensOn(ledger.state === "read" ? asked : "", backlog));
  const [closedShown, setClosedShown] = useState(opened.closedShown);
  const [view, setView] = useState<BacklogView>(opened.view);
  const [acting, setActing] = useState<Request | null>(null);
  // What the board is narrowed to, and how far back Done reaches. Both are
  // read once, as what this browser was last left on, and written wherever a
  // human changes one.
  const [filters, setFilters] = useState<Filters>(opened.filters);
  const [reach, setReach] = useState<Window>(opened.reach);
  const [opening, setOpening] = useState(false);
  // The goal the landing is showing, until its ring has faded. The fade is a
  // CSS animation and its own end clears this, so there is no timer here and
  // nothing to cancel.
  const [showing, setShowing] = useState(opened.showing);
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

  // Where the goal is, once it has been rendered. A layout effect is after
  // the render and before the paint, so the first frame a human sees is
  // already the right one, and no timer is involved.
  //
  // The two axes are moved separately, and deliberately. Sideways, the board's
  // own scroller is set by hand so that the lane lands in the middle of the
  // board: asking the browser to centre the card instead centres it in every
  // scrollable ancestor, and at phone width that carried the rail, the header
  // and the toolbar off the left of the screen to show a card that was
  // already going to be visible. Downwards, the browser is asked, because
  // there the ancestors are exactly the ones that should move: the lane's own
  // list of cards, and the work area. By then the card is already in view
  // sideways, so "nearest" moves nothing horizontally at all.
  useLayoutEffect(() => {
    if (opened.showing === "") {
      return;
    }
    const card = globalThis.document.querySelector(`.${SHOWN}`);
    if (card === null) {
      return;
    }
    const board = card.closest(".ms-board");
    if (board !== null) {
      const lane = card.getBoundingClientRect();
      const frame = board.getBoundingClientRect();
      board.scrollLeft += lane.left - frame.left - (frame.width - lane.width) / 2;
    }
    card.scrollIntoView({ block: "center", inline: "nearest" });
  }, []);

  // The address has said what it came to say. Replacing it rather than
  // pushing keeps the way back where it was, and dropping the goal means a
  // reload opens the Backlog rather than landing here a second time. It runs
  // once, on the read this component mounted with, which is where everything
  // it depends on was decided.
  useEffect(() => {
    if (opened.landed) {
      onLanded();
    }
  }, []);

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
      {/* What the landing had to change to show the goal it was sent for, or
          that it could not find one. It stands above the lanes because it is
          about the whole board: the Done selector says 30 days and a human
          who did not choose 30 days is owed the sentence saying who did. */}
      {opened.landing.notes.map((note) => (
        <p className="ms-board-showing" key={note} role="status">
          {note}
        </p>
      ))}
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
          onAct={(act: Asked) => { setActing({ move: act.move, goal: act.goal }); }}
          onMoved={onMoved}
          plans={plans}
          filters={filters}
          window={reach}
          onWindow={(days) => { setReach(days); writeDoneWindow(days); }}
          showing={showing}
          onFaded={() => { setShowing(""); }}
        />
      )}
      {acting !== null && (
        <ActSheet
          request={acting}
          backlog={backlog}
          onClose={() => { setActing(null); }}
          onDone={(after) => {
            setActing(null);
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
                showing={showing}
                onFaded={() => { setShowing(""); }}
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
                showing={showing}
                onFaded={() => { setShowing(""); }}
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

/**
 * What the page opens on: the preferences this browser was left on, and the
 * landing the address asked for on top of them.
 *
 * The two are decided together, once, because they are one answer: a stored
 * Done window of one day and an address naming a goal concluded a fortnight
 * ago are not a conflict to be resolved later but a board that opens on
 * thirty days and says so. Nothing is written to storage here — a human who
 * left the board narrowed to their own seat has not asked for that to change
 * because they followed a link — so leaving this page leaves the preferences
 * exactly as they were.
 */
type Opening = {
  view: BacklogView;
  filters: Filters;
  reach: Window;
  closedShown: boolean;
  /** The goal to ring, which is none where the board does not carry it. */
  showing: string;
  /** True when the address asked for a goal and this page answered. */
  landed: boolean;
  landing: Landing;
};

function opensOn(asked: string, backlog: Backlog): Opening {
  const view = readBacklogView();
  const filters = readBacklogFilters();
  const reach = readDoneWindow();
  const asItWasLeft: Opening = {
    view,
    filters,
    reach,
    closedShown: false,
    showing: "",
    landed: false,
    landing: { notes: [] },
  };
  if (asked === "") {
    return asItWasLeft;
  }
  const placement = placementOf([...backlog.rows, ...backlog.closed], asked, observedAt(backlog), view);
  const landing = landingFor(
    asked,
    placement,
    reach,
    // Both views open with concluded work put away, so that is what a landing
    // is deciding against.
    false,
    // The toolbar's filters narrow the board and nothing else, so there is
    // nothing for a landing on the list to clear.
    view === "board" ? filters : noFilters,
  );
  return {
    view,
    filters: landing.clearFilters === true ? noFilters : filters,
    // A window of null is "every recorded conclusion", which is a window this
    // build offers; absent is the landing leaving the selector alone.
    reach: landing.doneDays === undefined ? reach : landing.doneDays,
    closedShown: landing.openClosed === true,
    showing: placement.where === "missing" ? "" : asked,
    landed: true,
    landing,
  };
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
