import {
  BookOpen,
  ChevronRight,
  CircleCheck,
  Compass,
  Gavel,
  MessageCircle,
  PenTool,
  type LucideIcon,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadOverview,
  type Claimed,
  type Item,
  type Page as OverviewPayload,
  type Waiting,
  type Where as Reference,
} from "./api";
import "./overview.css";
import {
  blockAnchor,
  changedCounts,
  columns,
  destinationFor,
  glance,
  healthPills,
  laneStrip,
  moreLine,
  needsKinds,
  plural,
  seeAll,
  timeline,
  whenLine,
  windowLine,
  type BlockId,
  type Entry,
  type Kind,
  type Pill,
  type Tile,
} from "./overview";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import { useNotifications } from "../notifications/store";
import { Pane } from "../panes/Pane";
import { backlogPath, projectPath } from "../routes";
import { usePartner } from "../partner/store";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Hint, Skeleton } from "../shell/controls";
import { useSession } from "../shell/identity";
import { useOffersRefresh } from "../shell/refresh";
import { minuteTime } from "../backlog/format";

/**
 * Overview: the page a human lands on when they come back to the project.
 *
 * It answers five questions — what needs me, what changed while I was away,
 * what is being worked on now, what the project's memory holds, and whether
 * anything is wrong — and it answers them in numbers before words. Six tiles
 * across the top carry the whole page as six figures, each one a way into the
 * block or the board behind it; under them every list is one line per thing,
 * ellipsised, with the row itself as the link and the time or the count
 * right-aligned in the muted mono face. Detail is on demand: what needs a
 * human is a list of kinds that open, not a wall of items.
 *
 * When nothing needs the human it says so plainly. A calm page is the good
 * outcome rather than an empty one, which is why the primary block's empty
 * state is a sentence with a check beside it and not an invitation to go and
 * find something.
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
  useAbout(aboutLine("Overview", ""), { returnTo: "/overview" });

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
      <Glance page={page} />
      <div className="ms-overview-columns">
        <div className="ms-overview-column">{left.map(block)}</div>
        <div className="ms-overview-column">{right.map(block)}</div>
      </div>
    </div>
  );
}

/* ----------------------------------------------------------- the glance -- */

/**
 * The strip across the top: the whole page as six figures.
 *
 * It is a nav rather than a row of cards because every tile is a way through —
 * three of them to a block of this page and three to the board — and a human
 * reading the page by number is navigating it, not looking at a dashboard.
 */
function Glance({ page }: { page: OverviewPayload }) {
  return (
    <nav className="ms-overview-glance" aria-label="At a glance">
      {glance(page).map((tile) => (
        <GlanceTile key={tile.id} tile={tile} />
      ))}
    </nav>
  );
}

function GlanceTile({ tile }: { tile: Tile }) {
  const body = (
    <>
      <span
        className={`ms-overview-figure ms-overview-tone-${tile.tone}${tile.word ? " ms-overview-figure--word" : ""}`}
      >
        {tile.value}
      </span>
      <span className="ms-overview-figure-label">{tile.label}</span>
    </>
  );
  // A place on this page is a fragment of this address, which the browser
  // scrolls to; the router never sees it and nothing navigates.
  if (tile.anchor) {
    return (
      <a className="ms-overview-tile" href={tile.to}>
        {body}
      </a>
    );
  }
  return (
    <NavLink className="ms-overview-tile" to={tile.to}>
      {body}
    </NavLink>
  );
}

/* --------------------------------------------------------------- blocks -- */

/**
 * One block: an eyebrow, the term that explains it, and its rows. The eyebrow
 * is the reading style the Project section already uses for the name of a
 * group of facts, and the blocks are told apart by the space between them
 * rather than by a heavier border around each.
 */
function Block({
  id,
  title,
  help,
  note,
  children,
}: {
  id: BlockId;
  title: string;
  help: HelpId;
  note?: string;
  children: ReactNode;
}) {
  return (
    <section className="ms-overview-block" id={blockAnchor(id)}>
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
  const kinds = needsKinds(needs);
  const { askToSignIn } = useSession();
  return (
    <Block id="needs-you" title="Needs you" help="overview-needs-you">
      {kinds.length === 0 && !needs.signIn ? (
        <p className="ms-overview-calm">
          <CircleCheck className="ms-overview-calm-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
          Nothing needs you.
        </p>
      ) : (
        <ul className="ms-overview-kinds">
          {kinds.map((kind) => (
            <KindRow key={kind.id} kind={kind} />
          ))}
          {needs.signIn && (
            <li className="ms-overview-kind">
              <button
                type="button"
                className="ms-overview-signin"
                onClick={() => {
                  askToSignIn();
                }}
              >
                Sign in to act
              </button>
            </li>
          )}
        </ul>
      )}
    </Block>
  );
}

/**
 * One kind of thing that is waiting: the count, what it is, and the first
 * three of them when a human asks.
 *
 * It is a native disclosure, so the open state is the element's own and the
 * summary already says whether it is open to a screen reader. Nothing here
 * animates and nothing is on a timer: the row opens on the press and that is
 * the whole of it.
 */
function KindRow({ kind }: { kind: Kind }) {
  const { openPanel } = useNotifications();
  const more = moreLine(kind.group);
  return (
    <li className="ms-overview-kind">
      <details className="ms-overview-disclosure">
        <summary className="ms-overview-summary">
          <span className="ms-overview-badge">{kind.group.count}</span>
          <span className="ms-overview-kind-label">{kind.label}</span>
          <ChevronRight className="ms-overview-chevron" size={14} strokeWidth={2} aria-hidden="true" />
        </summary>
        <ul className="ms-overview-lines">
          {kind.group.items.map((item, position) => (
            <ItemLine key={`${item.id}-${String(position)}`} item={item} />
          ))}
          {more !== null && (
            <li className="ms-overview-line">
              {kind.to === null ? (
                <button
                  type="button"
                  className="ms-overview-more"
                  onClick={() => {
                    openPanel();
                  }}
                >
                  {more}
                </button>
              ) : (
                <NavLink className="ms-overview-more" to={kind.to}>
                  {more}
                </NavLink>
              )}
            </li>
          )}
        </ul>
      </details>
    </li>
  );
}

function Changed({ page }: { page: OverviewPayload }) {
  const changed = page.changed;
  // The window is read once, as the page renders, against the browser's own
  // clock: nothing here counts, and the line stays what it was until the next
  // response.
  const line = windowLine(page.since, page.first, new Date());
  const entries = timeline(changed);
  const rest = seeAll(changed, entries);
  return (
    <Block id="changed" title="Since your last visit" help="overview-changed" note={line}>
      {changed.total === 0 ? (
        <p className="ms-overview-calm">Nothing changed since your last visit.</p>
      ) : (
        <>
          <p className="ms-overview-counts">{changedCounts(changed)}</p>
          {entries.length > 0 && (
            <ul className="ms-overview-lines">
              {entries.map((one) => (
                <EntryLine key={one.key} entry={one} />
              ))}
            </ul>
          )}
          {rest !== null && (
            <p className="ms-overview-foot">
              <NavLink className="ms-overview-more" to={rest}>
                See all →
              </NavLink>
            </p>
          )}
        </>
      )}
    </Block>
  );
}

function WorkNow({ page }: { page: OverviewPayload }) {
  const work = page.work;
  const strip = laneStrip(work.lanes);
  return (
    <Block id="work" title="Work now" help="overview-work">
      <h3 className="ms-overview-subtitle">In Progress</h3>
      {work.inProgress.length === 0 ? (
        <p className="ms-overview-none">No seat is working on anything.</p>
      ) : (
        <ul className="ms-overview-cards">
          {work.inProgress.map((one) => (
            <ClaimedCard key={one.id} claimed={one} />
          ))}
        </ul>
      )}

      <h3 className="ms-overview-subtitle">Next up</h3>
      {work.next.length === 0 ? (
        <p className="ms-overview-none">Nothing is approved and ready to claim.</p>
      ) : (
        <ol className="ms-overview-ranked">
          {work.next.map((item) => (
            <li key={item.id} className="ms-overview-line">
              <Where where={item.where} className="ms-overview-row">
                <span className="ms-overview-row-title">{item.title === "" ? item.id : item.title}</span>
              </Where>
            </li>
          ))}
        </ol>
      )}

      <h3 className="ms-overview-subtitle">Waiting</h3>
      {work.waiting.count === 0 ? (
        <p className="ms-overview-none">Nothing is held.</p>
      ) : (
        <WaitingLine waiting={work.waiting} />
      )}

      <ul className="ms-overview-pills">
        {strip.map((lane) => (
          <li key={lane.id} className="ms-overview-pill-item">
            <NavLink className="ms-overview-pill ms-overview-pill--link" to={lane.to}>
              <span className="ms-overview-dot ms-overview-tone-plain" aria-hidden="true" />
              <span className="ms-overview-pill-words">{lane.title}</span>
              <span className="ms-mono ms-overview-pill-count">{lane.count}</span>
            </NavLink>
          </li>
        ))}
      </ul>
    </Block>
  );
}

/**
 * What is held, in one line: the count, and the oldest of them by name.
 *
 * The blocker's own words are a tooltip on that name rather than a sentence on
 * the page. They are a fact about one goal, often a paragraph long, and a page
 * read at a glance cannot carry a paragraph for the one row that has one.
 */
function WaitingLine({ waiting }: { waiting: Waiting }) {
  const name = waiting.title === "" ? waiting.id : waiting.title;
  const oldest = (
    <Where where={{ kind: "goal", id: waiting.id }} className="ms-overview-held-goal">
      {name}
    </Where>
  );
  return (
    <p className="ms-overview-held">
      <NavLink className="ms-overview-more" to={backlogPath()}>
        {plural(waiting.count, "goal")} held
      </NavLink>
      {name !== "" && (
        <>
          <span className="ms-overview-held-rule"> · oldest </span>
          {waiting.reason === "" ? oldest : <Hint label={waiting.reason}>{oldest}</Hint>}
        </>
      )}
    </p>
  );
}

function ClaimedCard({ claimed }: { claimed: Claimed }) {
  // The instant is read against the browser's clock as the card renders, once.
  const when = whenLine(claimed.at, new Date());
  const seat =
    claimed.seat.machine === ""
      ? "seat not recorded"
      : `${claimed.seat.machine}${claimed.seat.lineage === "" ? "" : `+${claimed.seat.lineage}`}`;
  return (
    <li className="ms-overview-card">
      <Where where={{ kind: "goal", id: claimed.id }} className="ms-overview-row">
        <span className="ms-mono ms-overview-card-id">{claimed.id}</span>
        <span className="ms-overview-row-title">{claimed.title === "" ? claimed.id : claimed.title}</span>
      </Where>
      <p className="ms-overview-card-facts">
        <Chip>{seat}</Chip>
        {claimed.phase !== "" && <Chip>{claimed.phase}</Chip>}
        {when !== "" && <span className="ms-mono ms-overview-card-since">since {when}</span>}
      </p>
    </li>
  );
}

/** One line of the timeline: when, a chip for what happened, and what it was. */
function EntryLine({ entry }: { entry: Entry }) {
  const when = whenLine(entry.at, new Date());
  return (
    <li className="ms-overview-line">
      <Where where={entry.where} className="ms-overview-row">
        <span className="ms-mono ms-overview-row-when">{when}</span>
        <Chip>{entry.chip}</Chip>
        <span className="ms-overview-row-title">{entry.title}</span>
      </Where>
    </li>
  );
}

/** One item under an opened kind: what it is, and when it last said so. */
function ItemLine({ item }: { item: Item }) {
  const when = whenLine(item.at, new Date());
  return (
    <li className="ms-overview-line">
      <Where where={item.where} className="ms-overview-row">
        <span className="ms-overview-row-title">{item.title === "" ? item.id : item.title}</span>
        {when !== "" && <span className="ms-mono ms-overview-row-tail">{when}</span>}
      </Where>
      <AskAbout item={item} note={when} />
    </li>
  );
}

/**
 * Ask, at the end of an Overview row.
 *
 * The row itself still navigates — it is the link, and that is what a human
 * presses — so this stands after it, out of the way until the pointer or the
 * caret is on the row. It is a button rather than a menu because an Overview
 * row is not a card: it has one act, and a menu for one act is a menu.
 */
function AskAbout({ item, note }: { item: Item; note: string }) {
  const { ask } = usePartner();
  return (
    <button
      type="button"
      className="ms-overview-ask"
      onClick={() => {
        ask({
          kind: "overview",
          id: item.id,
          title: item.title === "" ? item.id : item.title,
          source: "the landing page as this server composed it",
          summary: [item.note, note].filter((part) => part !== "").join(" · "),
          to: destinationOf(item),
        });
      }}
    >
      Ask
    </button>
  );
}

/** Where an item opens, where this build has an address for it. */
function destinationOf(item: Item): string | undefined {
  const destination = destinationFor(item.where);
  return destination.kind === "link" ? destination.to : undefined;
}

/**
 * What the project has written down, as five tiles: an icon, a number and a
 * word, each one the way into the tab that holds it. The vision sentence is
 * not here — it lives on Project, where there is room to read it.
 *
 * Three of the five count the project's own records — the ones whose head
 * names no goal — with the rest named beneath them, because scope is a filter
 * with a sensible default rather than two lists, and the same rule holds
 * wherever counts appear. A tile that counted four hundred designs said only
 * that the project is large; one that counts the twelve that shape everything
 * says what the project's own memory holds, and the line under it says the
 * rest is there. The two books have no such line: a chapter of either is about
 * the project as a whole by definition.
 */
type MemoryTile = {
  id: string;
  icon: LucideIcon;
  value: number;
  word: string;
  note: string;
  /** How many more of this kind are under goals, or "" where none are. */
  under: string;
  to: string;
};

/** The line beneath a tile's figure: the rest of this kind, or nothing. */
export function underGoalsLine(count: number): string {
  return count === 0 ? "" : `+${String(count)} under goals`;
}

function Memory({ page }: { page: OverviewPayload }) {
  const memory = page.memory;
  const scoped = memory.scoped;
  const tiles: MemoryTile[] = [
    {
      id: "intent",
      icon: Compass,
      value: memory.intent.chapters,
      word: "Intent",
      note: "",
      under: "",
      to: projectPath("intent"),
    },
    {
      id: "doctrine",
      icon: BookOpen,
      value: memory.doctrine.chapters,
      word: "Doctrine",
      note: "",
      under: "",
      to: projectPath("doctrine"),
    },
    {
      id: "decisions",
      icon: Gavel,
      value: scoped.decisions.own,
      word: "Decisions",
      // The draft count is the whole shelf's, because it is a fact about the
      // decisions rather than about this scope of them, and the page that
      // opens from here says which are which.
      note: memory.decisions.drafts > 0 ? `${String(memory.decisions.drafts)} draft in all` : "",
      under: underGoalsLine(scoped.decisions.underGoals),
      to: projectPath("decisions"),
    },
    {
      id: "designs",
      icon: PenTool,
      value: scoped.designs.own,
      word: "Designs",
      note: memory.designs.done > 0 ? `${String(memory.designs.done)} done in all` : "",
      under: underGoalsLine(scoped.designs.underGoals),
      to: projectPath("designs"),
    },
    {
      id: "questions",
      icon: MessageCircle,
      value: scoped.questions.own,
      word: "Open questions",
      note: "",
      under: underGoalsLine(scoped.questions.underGoals),
      to: projectPath("questions"),
    },
  ];
  return (
    <Block id="memory" title="The project's memory" help="overview-memory">
      <ul className="ms-overview-memory">
        {tiles.map(({ id, icon: Icon, value, word, note, under, to }) => (
          <li key={id} className="ms-overview-memory-item">
            <NavLink className="ms-overview-memory-tile" to={to}>
              <Icon className="ms-overview-memory-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
              <span className="ms-overview-memory-figure">{value}</span>
              <span className="ms-overview-memory-word">{word}</span>
              {note !== "" && <span className="ms-overview-memory-note">{note}</span>}
              {under !== "" && <span className="ms-overview-memory-under">{under}</span>}
            </NavLink>
          </li>
        ))}
      </ul>
    </Block>
  );
}

function Health({ page }: { page: OverviewPayload }) {
  const health = page.health;
  // The gap is measured against the browser's clock as the block renders,
  // once. Nothing counts: the words stay what they were until the next read.
  const pills = healthPills(health, new Date());
  const more = moreLine(health.problems);
  return (
    <Block id="health" title="Health" help="overview-health">
      <ul className="ms-overview-pills">
        {pills.map((pill) => (
          <HealthPill key={pill.id} pill={pill} />
        ))}
      </ul>
      {!health.ok && (
        <ul className="ms-overview-lines">
          {health.problems.items.map((problem, position) => (
            <li key={`${problem.where.kind}-${problem.id}-${String(position)}`} className="ms-overview-line">
              <Where where={problem.where} className="ms-overview-row">
                <span className="ms-overview-row-title">{problem.title}</span>
                {problem.note !== "" && <span className="ms-mono ms-overview-row-tail">{problem.note}</span>}
              </Where>
            </li>
          ))}
          {more !== null && (
            <li className="ms-overview-line">
              <NavLink className="ms-overview-more" to={backlogPath()}>
                {more}
              </NavLink>
            </li>
          )}
        </ul>
      )}
    </Block>
  );
}

/**
 * One of the three health pills. A pill that is fine is not a link: there is
 * nowhere to go and a link that leads to nothing is a promise the page cannot
 * keep. A pill that is not fine opens the first problem of its own source.
 */
function HealthPill({ pill }: { pill: Pill }) {
  const body = (
    <>
      <span className={`ms-overview-dot ms-overview-tone-${pill.tone}`} aria-hidden="true" />
      <span className="ms-overview-pill-words">{pill.words}</span>
    </>
  );
  return (
    <li className="ms-overview-pill-item">
      {pill.where === null ? (
        <span className="ms-overview-pill">{body}</span>
      ) : (
        <Where where={pill.where} className="ms-overview-pill ms-overview-pill--link">
          {body}
        </Where>
      )}
    </li>
  );
}

/**
 * One reference, as the surface that opens it: a link where the destination is
 * an address, a button where it is a panel or a sheet, and plain text where
 * this build has no surface for it — which is the master's rule for an
 * unresolved reference, rather than a link that would refuse.
 *
 * Every one of the three carries the same class, because the row is the link:
 * what a human presses is the whole line, and a line that is a button on one
 * row and an anchor on the next must not look like two kinds of thing.
 */
function Where({
  where,
  className,
  children,
}: {
  where: Reference;
  className: string;
  children: ReactNode;
}) {
  const { openPanel } = useNotifications();
  const { askToSignIn } = useSession();
  const destination = destinationFor(where);
  if (destination.kind === "link") {
    return (
      <NavLink className={className} to={destination.to}>
        {children}
      </NavLink>
    );
  }
  if (destination.kind === "notifications") {
    return (
      <button
        type="button"
        className={className}
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
        className={className}
        onClick={() => {
          askToSignIn();
        }}
      >
        {children}
      </button>
    );
  }
  return <span className={className}>{children}</span>;
}

/* -------------------------------------------------------- the two states -- */

function Loading() {
  return (
    <div className="ms-overview">
      <div className="ms-overview-glance">
        {[0, 1, 2, 3, 4, 5].map((tile) => (
          <span key={tile} className="ms-overview-tile">
            <Skeleton />
          </span>
        ))}
      </div>
      <div className="ms-overview-columns">
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
