import { useState } from "react";

import "./backlog.css";
import type { Backlog, Ledger, Row } from "./api";
import { clockTime, shortTip } from "./format";
import { DraftGroup, LaneGroup } from "./LaneGroup";
import { anchorFor, closedLanes, openLanes, type LaneId } from "./lanes";
import { LedgerLine } from "./LedgerLine";
import { Statement } from "./Statement";
import { useBacklog } from "./state";
import { Pane } from "../panes/Pane";
import { Button } from "../shell/controls";

/**
 * The backlog as a list by lane.
 *
 * Every goal at the accepted tip appears once, in the lane the server placed
 * it in, with what the record says and what it does not. Concluded work sits
 * behind one toggle, because 430 records would otherwise bury the 155 that are
 * live. Freshness belongs to the server: this page reads when it mounts and
 * when a human asks, and says what the last fetch found either way.
 */
export function BacklogPane() {
  const { backlog, refresh } = useBacklog();

  if (backlog.state === "loading") {
    return (
      <Pane title="Backlog">
        <div className="ms-backlog">
          <LedgerLine ledger={null} observedAt="" onRefresh={refresh} />
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
      <Read backlog={backlog.backlog} onRefresh={refresh} />
    </Pane>
  );
}

function Read({ backlog, onRefresh }: { backlog: Backlog; onRefresh: () => void }) {
  const [closedShown, setClosedShown] = useState(false);
  const ledger = backlog.ledger;
  const closedCount = backlog.closed.length;

  return (
    <div className="ms-backlog">
      <LedgerLine ledger={ledger} observedAt={backlog.observedAt} onRefresh={onRefresh} />
      {ledger.state === "read" ? (
        <>
          <p className="ms-lane-index">
            {openLanes.map((lane, position) => (
              <span key={lane.id}>
                {position > 0 && <span className="ms-lane-index-rule"> · </span>}
                <a className="ms-lane-anchor" href={`#${anchorFor(lane.id)}`}>
                  {lane.title}
                </a>
                {lane.id === "draft" ? " not read" : ` ${String(countOf(backlog, lane.id))}`}
              </span>
            ))}
          </p>
          {openLanes.map((lane) =>
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
          <Button aria-pressed={closedShown} onClick={() => { setClosedShown((shown) => !shown); }}>
            {closedShown ? "Hide" : "Show"} closed items ({closedCount})
          </Button>
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
            Board, outline, dependencies, filters, and goal detail arrive with g1-s10 and g1-s11.
          </p>
        </>
      ) : (
        <LedgerStatement backlog={backlog} />
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
