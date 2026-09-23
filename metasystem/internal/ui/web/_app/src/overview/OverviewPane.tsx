import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadOverview,
  type Claimed,
  type Group,
  type Item,
  type Page as OverviewPayload,
} from "./api";
import "./overview.css";
import {
  columns,
  destinationFor,
  healthLine,
  laneStrip,
  moreLine,
  plural,
  progressLine,
  whenLine,
  windowLine,
  type BlockId,
} from "./overview";
import { minuteTime } from "../backlog/format";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import { useNotifications } from "../notifications/store";
import { Pane } from "../panes/Pane";
import { backlogPath, projectPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useSession } from "../shell/identity";
import { useOffersRefresh } from "../shell/refresh";

/**
 * Overview: the page a human lands on when they come back to the project.
 *
 * Top to bottom in importance it answers five questions — what needs me, what
 * changed while I was away, what is being worked on now, what the project's
 * memory holds, and whether anything is wrong — and every number on it is a
 * link to the place where the thing is done. Nothing here is a dashboard tile:
 * a count a human cannot follow is a number they have to go and look up
 * somewhere else, which is the reading this page exists to replace.
 *
 * When nothing needs the human it says so plainly. A calm page is the good
 * outcome rather than an empty one, which is why the primary block's empty
 * state is a sentence and not an invitation to go and find something.
 *
 * The page is one read, made when it mounts and again when a human presses the
 * refresh in the header's section cluster. There is no timer, nothing polls,
 * and nothing refetches on a window event: the server's own loop keeps the
 * accepted ledger current, and the marker that decides the window is recorded
 * by the read itself.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; page: OverviewPayload };

export function OverviewPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadOverview(aborter.signal)
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

  const reload = useCallback(() => {
    setRead({ state: "loading" });
    setAttempt((previous) => previous + 1);
  }, []);

  // The refresh the header shows for this section, with when the page was read
  // in its tooltip, which is where every other page's refresh says it.
  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  useOffersRefresh(reload, hint);
  useAbout(aboutLine("Overview", ""));

  return (
    <Pane title="Overview">
      {read.state === "loading" && <Loading />}
      {read.state === "failed" && <Failure message={read.message} onRetry={reload} />}
      {read.state === "read" && <Blocks page={read.page} />}
    </Pane>
  );
}

function Blocks({ page }: { page: OverviewPayload }) {
  const { left, right } = useMemo(() => columns(), []);
  const block = (id: BlockId): ReactNode => {
    switch (id) {
      case "needs-you":
        return <NeedsYou key={id} page={page} />;
      case "changed":
        return <Changed key={id} page={page} />;
      case "work":
        return <WorkNow key={id} page={page} />;
      case "memory":
        return <Memory key={id} page={page} />;
      case "health":
        return <Health key={id} page={page} />;
    }
  };
  return (
    <div className="ms-overview">
      <div className="ms-overview-column">{left.map(block)}</div>
      <div className="ms-overview-column">{right.map(block)}</div>
    </div>
  );
}

/* --------------------------------------------------------------- blocks -- */

/** One block: a title, the term that explains it, and its rows. */
function Block({ title, help, note, children }: { title: string; help: HelpId; note?: string; children: ReactNode }) {
  return (
    <section className="ms-overview-block">
      <div className="ms-overview-head">
        <h2 className="ms-overview-title">{title}</h2>
        <Help id={help} />
        {note !== undefined && note !== "" && <span className="ms-overview-note">{note}</span>}
      </div>
      {children}
    </section>
  );
}

function NeedsYou({ page }: { page: OverviewPayload }) {
  const needs = page.needsYou;
  const { askToSignIn } = useSession();
  const { openPanel } = useNotifications();
  const empty = needs.total === 0 && !needs.signIn;
  return (
    <Block title="Needs you" help="overview-needs-you">
      {empty ? (
        <p className="ms-overview-calm">Nothing needs you.</p>
      ) : (
        <ul className="ms-overview-rows">
          <Row
            label={`${plural(needs.approvals.count, "goal")} waiting for your approval`}
            group={needs.approvals}
            to={backlogPath()}
          />
          <Row
            label={`${plural(needs.questions.count, "question")} nobody has answered`}
            group={needs.questions}
            to={projectPath("questions")}
          />
          <Row
            label={`${plural(needs.drafts.count, "draft")} waiting to be accepted`}
            group={needs.drafts}
            to={projectPath("designs")}
          />
          <Row
            label={`${plural(needs.designs.count, "design")} whose work has all landed`}
            group={needs.designs}
            to={projectPath("designs")}
          />
          <Row
            label={`${plural(needs.alerts.count, "message")} the steward sent you`}
            group={needs.alerts}
            onMore={() => {
              openPanel();
            }}
          />
          {needs.signIn && (
            <li className="ms-overview-row">
              <p className="ms-overview-row-head">
                <button
                  type="button"
                  className="ms-overview-act"
                  onClick={() => {
                    askToSignIn();
                  }}
                >
                  Sign in to act
                </button>
              </p>
            </li>
          )}
        </ul>
      )}
    </Block>
  );
}

function Changed({ page }: { page: OverviewPayload }) {
  const changed = page.changed;
  const { openPanel } = useNotifications();
  // The window is read once, as the page renders, against the browser's own
  // clock: nothing here counts, and the line stays what it was until the next
  // response.
  const line = windowLine(page.since, page.first, new Date());
  return (
    <Block title="Since your last visit" help="overview-changed" note={line}>
      {changed.total === 0 ? (
        <p className="ms-overview-calm">Nothing changed since your last visit.</p>
      ) : (
        <ul className="ms-overview-rows">
          <Row
            label={`${plural(changed.concluded.count, "goal")} concluded`}
            group={changed.concluded}
            to={backlogPath()}
          />
          <Row label={`${plural(changed.moved.count, "goal")} moved`} group={changed.moved} to={backlogPath()} />
          <Row
            label={`${plural(changed.records.count, "record")} changed`}
            group={changed.records}
            to={projectPath("designs")}
          />
          {changed.messages > 0 && (
            <li className="ms-overview-row">
              <p className="ms-overview-row-head">
                <button
                  type="button"
                  className="ms-overview-act"
                  onClick={() => {
                    openPanel();
                  }}
                >
                  {plural(changed.messages, "message")} from the steward
                </button>
              </p>
            </li>
          )}
        </ul>
      )}
    </Block>
  );
}

function WorkNow({ page }: { page: OverviewPayload }) {
  const work = page.work;
  const strip = laneStrip(work.lanes);
  return (
    <Block title="Work now" help="overview-work">
      <h3 className="ms-overview-subtitle">In Progress</h3>
      {work.inProgress.length === 0 ? (
        <p className="ms-overview-none">No seat is working on anything.</p>
      ) : (
        <ul className="ms-overview-rows">
          {work.inProgress.map((one) => (
            <ClaimedRow key={one.id} claimed={one} />
          ))}
        </ul>
      )}

      <h3 className="ms-overview-subtitle">Next up</h3>
      {work.next.length === 0 ? (
        <p className="ms-overview-none">Nothing is approved and ready to claim.</p>
      ) : (
        <ul className="ms-overview-items">
          {work.next.map((item) => (
            <ItemRow key={item.id} item={item} />
          ))}
        </ul>
      )}

      <h3 className="ms-overview-subtitle">Waiting</h3>
      {work.waiting.count === 0 ? (
        <p className="ms-overview-none">Nothing is held.</p>
      ) : (
        <p className="ms-overview-line">
          <NavLink className="ms-overview-link" to={backlogPath()}>
            {plural(work.waiting.count, "goal")} held
          </NavLink>
          {work.waiting.reason !== "" && (
            <>
              {" · oldest: "}
              <NavLink className="ms-overview-link" to={backlogPath()}>
                {work.waiting.title === "" ? work.waiting.id : work.waiting.title}
              </NavLink>
              <span className="ms-overview-reason"> — {work.waiting.reason}</span>
            </>
          )}
        </p>
      )}

      <p className="ms-overview-strip">
        {strip.map((lane, position) => (
          <span key={lane.id}>
            {position > 0 && <span className="ms-overview-strip-rule"> · </span>}
            <NavLink className="ms-overview-link" to={lane.to}>
              {lane.title} {lane.count}
            </NavLink>
          </span>
        ))}
      </p>
    </Block>
  );
}

function Memory({ page }: { page: OverviewPayload }) {
  const memory = page.memory;
  return (
    <Block title="The project's memory" help="overview-memory">
      <ul className="ms-overview-rows">
        <li className="ms-overview-row">
          <p className="ms-overview-row-head">
            <NavLink className="ms-overview-link" to={projectPath("intent")}>
              Intent · {plural(memory.intent.chapters, "chapter")}
            </NavLink>
          </p>
          {memory.intent.summary !== "" && <p className="ms-overview-lede">{memory.intent.summary}</p>}
        </li>
        <li className="ms-overview-row">
          <p className="ms-overview-row-head">
            <NavLink className="ms-overview-link" to={projectPath("doctrine")}>
              Doctrine · {plural(memory.doctrine.chapters, "chapter")}
            </NavLink>
          </p>
        </li>
        <li className="ms-overview-row">
          <p className="ms-overview-row-head">
            <NavLink className="ms-overview-link" to={projectPath("decisions")}>
              {plural(memory.decisions.total, "decision")}
              {memory.decisions.drafts > 0 && ` · ${String(memory.decisions.drafts)} draft`}
            </NavLink>
          </p>
        </li>
        <li className="ms-overview-row">
          <p className="ms-overview-row-head">
            <NavLink className="ms-overview-link" to={projectPath("designs")}>
              {plural(memory.designs.total, "design")} · {String(memory.designs.done)} done
            </NavLink>
          </p>
          {memory.designs.progress.length > 0 && (
            <ul className="ms-overview-items">
              {memory.designs.progress.map((design) => (
                <li key={design.id} className="ms-overview-item">
                  <Where where={{ kind: "document", id: design.path }}>{design.title}</Where>
                  <span className="ms-overview-item-note">{progressLine(design.done, design.goals)}</span>
                </li>
              ))}
              {memory.designs.inFlight > memory.designs.progress.length && (
                <li className="ms-overview-item">
                  <NavLink className="ms-overview-link" to={projectPath("designs")}>
                    {`and ${String(memory.designs.inFlight - memory.designs.progress.length)} more →`}
                  </NavLink>
                </li>
              )}
            </ul>
          )}
        </li>
        <li className="ms-overview-row">
          <p className="ms-overview-row-head">
            <NavLink className="ms-overview-link" to={projectPath("questions")}>
              {plural(memory.questions, "open question")}
            </NavLink>
          </p>
        </li>
      </ul>
    </Block>
  );
}

function Health({ page }: { page: OverviewPayload }) {
  const health = page.health;
  return (
    <Block title="Health" help="overview-health">
      {health.ok ? (
        <p className="ms-overview-calm">{healthLine(health.syncedAt)}</p>
      ) : (
        <ul className="ms-overview-items">
          {health.problems.items.map((problem, position) => (
            <li key={`${problem.where.kind}-${problem.id}-${String(position)}`} className="ms-overview-item">
              <Where where={problem.where}>{problem.title}</Where>
              {problem.note !== "" && <span className="ms-mono ms-overview-item-note">{problem.note}</span>}
            </li>
          ))}
          {moreLine(health.problems) !== null && (
            <li className="ms-overview-item">
              <NavLink className="ms-overview-link" to={backlogPath()}>
                {moreLine(health.problems)}
              </NavLink>
            </li>
          )}
        </ul>
      )}
    </Block>
  );
}

/* ----------------------------------------------------------------- rows -- */

/**
 * One row of a block: a count that is a link, the first few items under it,
 * and the way through to the rest. A row with nothing in it is not rendered at
 * all — the block's own empty state says the whole of it in one sentence
 * rather than in five zeroes.
 */
function Row({
  label,
  group,
  to,
  onMore,
}: {
  label: string;
  group: Group;
  /** Where the count and "and N more" go, for a row that is an address. */
  to?: string;
  /** What they do instead, for a row that opens a panel. */
  onMore?: () => void;
}) {
  if (group.count === 0) {
    return null;
  }
  const more = moreLine(group);
  return (
    <li className="ms-overview-row">
      <p className="ms-overview-row-head">
        {to === undefined ? (
          <button
            type="button"
            className="ms-overview-act"
            onClick={() => {
              onMore?.();
            }}
          >
            {label}
          </button>
        ) : (
          <NavLink className="ms-overview-link" to={to}>
            {label}
          </NavLink>
        )}
      </p>
      <ul className="ms-overview-items">
        {group.items.map((item, position) => (
          <ItemRow key={`${item.id}-${String(position)}`} item={item} />
        ))}
        {more !== null && (
          <li className="ms-overview-item">
            {to === undefined ? (
              <button
                type="button"
                className="ms-overview-act"
                onClick={() => {
                  onMore?.();
                }}
              >
                {more}
              </button>
            ) : (
              <NavLink className="ms-overview-link" to={to}>
                {more}
              </NavLink>
            )}
          </li>
        )}
      </ul>
    </li>
  );
}

function ItemRow({ item }: { item: Item }) {
  // The instant is read against the browser's clock as the row renders, once.
  // Nothing counts and nothing is on a timer: the line stays what it was until
  // the next response.
  const when = whenLine(item.at, new Date());
  return (
    <li className="ms-overview-item">
      <Where where={item.where}>{item.title === "" ? item.id : item.title}</Where>
      {item.note !== "" && <Chip>{item.note}</Chip>}
      {when !== "" && <span className="ms-overview-item-note">{when}</span>}
    </li>
  );
}

function ClaimedRow({ claimed }: { claimed: Claimed }) {
  return (
    <li className="ms-overview-item">
      <Where where={{ kind: "goal", id: claimed.id }}>{claimed.title === "" ? claimed.id : claimed.title}</Where>
      <span className="ms-mono ms-overview-item-note">
        {claimed.seat.machine === "" ? "seat not recorded" : claimed.seat.machine}
        {claimed.seat.lineage === "" ? "" : `+${claimed.seat.lineage}`}
      </span>
      {claimed.phase !== "" && <Chip>{claimed.phase}</Chip>}
      {whenLine(claimed.at, new Date()) !== "" && (
        <span className="ms-overview-item-note">since {whenLine(claimed.at, new Date())}</span>
      )}
    </li>
  );
}

/**
 * One reference, as the surface that opens it: a link where the destination is
 * an address, a button where it is a panel or a sheet, and plain text where
 * this build has no surface for it — which is the master's rule for an
 * unresolved reference, rather than a link that would refuse.
 */
function Where({ where, children }: { where: { kind: string; id: string }; children: ReactNode }) {
  const { openPanel } = useNotifications();
  const { askToSignIn } = useSession();
  const destination = destinationFor(where);
  if (destination.kind === "link") {
    return (
      <NavLink className="ms-overview-link" to={destination.to}>
        {children}
      </NavLink>
    );
  }
  if (destination.kind === "notifications") {
    return (
      <button
        type="button"
        className="ms-overview-act"
        onClick={() => {
          openPanel(destination.at === "" ? undefined : destination.at);
        }}
      >
        {children}
      </button>
    );
  }
  if (destination.kind === "sign-in") {
    return (
      <button
        type="button"
        className="ms-overview-act"
        onClick={() => {
          askToSignIn();
        }}
      >
        {children}
      </button>
    );
  }
  return <span>{children}</span>;
}

/* -------------------------------------------------------- the two states -- */

function Loading() {
  return (
    <div className="ms-overview">
      <div className="ms-overview-column">
        {[0, 1].map((row) => (
          <section key={row} className="ms-overview-block">
            <Skeleton />
          </section>
        ))}
      </div>
      <div className="ms-overview-column">
        {[0, 1, 2].map((row) => (
          <section key={row} className="ms-overview-block">
            <Skeleton />
          </section>
        ))}
      </div>
    </div>
  );
}

function Failure({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="ms-pane-stack">
      <section className="ms-card">
        <h2 className="ms-card-title">Overview could not be read</h2>
        <p className="ms-overview-reason">{message}</p>
        <Button onClick={onRetry}>Retry</Button>
      </section>
    </div>
  );
}
