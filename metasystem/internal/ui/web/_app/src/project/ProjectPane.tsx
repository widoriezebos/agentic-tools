import { RefreshCw } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink, useNavigate, useParams } from "react-router";

import { failureMessage, loadPane, type DocumentFile, type Pane as PanePayload, type Problem } from "./api";
import {
  briefingFor,
  documentGroups,
  DOCUMENTS_TAB,
  DOCUMENTS_TITLE,
  kindTitle,
  listedIn,
  newActionFor,
  NO_SLICE_PLAN,
  nothingLine,
  pageSections,
  QUESTIONS_TAB,
  QUESTIONS_TITLE,
  sliceCount,
  SLICES_TAB,
  SLICES_TITLE,
  stripActions,
  tabForKind,
  type BookBriefing,
  type Briefing,
  type DesignRuns,
  type DocumentGroup,
  type NewAction,
  type PageSection,
  type Row,
  type SlicePlan,
} from "./pane";
import "./reading.css";
import { Sheet, type Done, type Request } from "./Sheet";
import { type Scope } from "./writing";
import {
  BacklogError,
  blockGoal,
  loadBacklog,
  unblockGoal,
  type Backlog,
  type Row as GoalRow,
} from "../backlog/api";
import { laneTitle } from "../backlog/lanes";
import { showLabel } from "../backlog/showing";
import { Help } from "../help/Help";
import { Pane } from "../panes/Pane";
import { Tabs, tabShown, type Tab } from "../panes/Tabs";
import { backlogPath, documentPath, goalPath, projectPath } from "../routes";
import { CardMenu } from "../backlog/CardMenu";
import { opensMenu, type At } from "../backlog/menu";
import { usePartner } from "../partner/store";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, IconButton, Skeleton } from "../shell/controls";
import { GoalPicker, type PickableGoal } from "../shell/GoalPicker";
import { useSession } from "../shell/identity";
import { failureMessage as actFailureMessage } from "../shell/workspace";
import { readBacklogView, readGoalTab, readProjectTab, writeGoalTab, writeProjectTab } from "../storage";

/**
 * Project: a briefing on what this project is, not a list of its files.
 *
 * The strip under the header is what is on this page — the books, the
 * decisions, the designs, the open questions, the documents — as tabs, one
 * section open at a time. It was an index into a page that carried all six at
 * once, and an index is what a document gets: a briefing is six answers to
 * six different questions, and a human asking what was decided should not
 * have to scroll past the whole intent to find out. The ledger's goals are
 * not in the strip: a goal belongs to the Backlog, and a rail of six hundred
 * of them said that the project's records were a subdivision of the ledger
 * rather than the other way round. Each tab opens its kind in its own words —
 * the intent index's first paragraph, the doctrine's, a design's — and only
 * then offers the links: the books as tables of contents, the decisions and
 * designs as rows carrying their summaries, the designs grouped so that what
 * governs is open and what is finished is one collapsed run. The right column
 * is what is waiting and what was read, and it stands beside every tab.
 *
 * Which tab is open is in the address, so a section of the project is a place
 * that can be sent to somebody and reloaded, and the browser remembers the
 * last one so that coming back to Project comes back to where the human was.
 *
 * One goal of the ledger is the same page scoped to it, and it lives under
 * Backlog: the same tabs, narrowed to the records whose Goals name it, under
 * the ledger's own reason for the goal.
 *
 * Nothing is inferred from a filename, and nothing that is refused is hidden:
 * what the check verb would print is above the strip, on every tab, where a
 * human can act on it.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; pane: PanePayload };

/** The whole project: every record the checkout carries, by kind. */
export function ProjectPane() {
  return <Briefed goal={null} />;
}

/**
 * One goal of the ledger, at /backlog/goal/:id. The payload is the project's
 * own — the same read of /api/project — because what a goal page shows is the
 * project's records, narrowed to the ones that say they are about this goal.
 */
export function GoalPane() {
  const parameters = useParams<{ id?: string }>();
  return <Briefed goal={parameters.id ?? ""} />;
}

function Briefed({ goal }: { goal: string | null }) {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  /**
   * The ledger, read on a goal's page and nowhere else.
   *
   * The project payload knows a goal's id, its state and the reason it is
   * open; the relation between goals is the ledger's and lives in the board's
   * own payload. A goal page shows both directions of it, so it reads that
   * payload too. The project's own page does not, because it is about no one
   * goal and would be reading a board it never shows.
   *
   * A backlog that could not be read is not a failed page: the records this
   * page is about are still there, so the dependency block says what it could
   * not read and the rest of the page stands.
   */
  const [ledger, setLedger] = useState<Backlog | null>(null);

  useEffect(() => {
    const aborter = new AbortController();
    loadPane(aborter.signal)
      .then((answered) => {
        setRead({ state: "read", pane: answered });
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

  useEffect(() => {
    if (goal === null) {
      return;
    }
    const aborter = new AbortController();
    loadBacklog(aborter.signal)
      .then((answered) => {
        setLedger(answered);
      })
      .catch(() => {
        if (!aborter.signal.aborted) {
          setLedger(null);
        }
      });
    return () => {
      aborter.abort();
    };
  }, [goal, attempt]);

  const reload = () => {
    setRead({ state: "loading" });
    setAttempt((previous) => previous + 1);
  };

  return (
    <Pane title={goal === null ? "Project" : "Backlog"}>
      {read.state === "loading" && <LoadingCards />}
      {read.state === "failed" && <FailureCard message={read.message} onRetry={reload} />}
      {read.state === "read" && (
        <Columns
          pane={read.pane}
          goal={goal}
          ledger={ledger}
          onLedger={setLedger}
          onReload={reload}
        />
      )}
    </Pane>
  );
}

function Columns({
  pane,
  goal,
  ledger,
  onLedger,
  onReload,
}: {
  pane: PanePayload;
  goal: string | null;
  /** The board as it stands, on a goal's page; null where it was not read. */
  ledger: Backlog | null;
  /** The board after an edge act, which answers with the ledger it left. */
  onLedger: (backlog: Backlog) => void;
  onReload: () => void;
}) {
  const briefing = useMemo(() => briefingFor(pane, goal), [pane, goal]);
  const sections = useMemo(() => pageSections(briefing), [briefing]);
  const groups = useMemo(() => documentGroups(pane.documents), [pane]);
  const [sheet, setSheet] = useState<Request | null>(null);
  const navigate = useNavigate();
  const parameters = useParams<{ tab?: string }>();

  // The remembered tab is read once, as the tab this page opens on where the
  // address names none. From there the address says which tab is open, and
  // the store is written rather than read.
  const [remembered] = useState(() => (goal === null ? readProjectTab() : readGoalTab()));

  const page = goal === null ? "Project" : `Backlog · ${goal}`;
  const open = tabShown(sections, parameters.tab, remembered);

  // What the drawer says this page is about: the page, and the tab of it that
  // is open, which is the only section on the screen.
  const here = sections.find((section) => section.id === open);
  // A goal's page is about that goal, by its ledger id, so the Partner is
  // given the row the board shows rather than the page's label. The project's
  // own page is about no one record, and says only which tab is open.
  const here2 = here?.title ?? "";
  // And what that tab is listing, by each row's own key, so the Partner is
  // told the records on the screen rather than the name of the tab they are
  // on. What each one is, the server reads for itself.
  const listed = useMemo(() => listedIn(briefing, open), [briefing, open]);
  useAbout(
    aboutLine(page, here2),
    goal === null
      ? { tab: here2, records: listed, returnTo: projectPath(open) }
      : { kind: "goal", subject: goal, title: goal, tab: here2, records: listed, returnTo: goalPath(goal, open) },
  );

  // Opening a tab is going somewhere: the address carries it, so a section of
  // the project can be sent to somebody and survives a reload. What the
  // browser remembers is written here, where a human chose it, and never from
  // an address they were merely sent.
  const select = (id: string) => {
    if (goal === null) {
      writeProjectTab(id);
      void navigate(projectPath(id));
      return;
    }
    writeGoalTab(id);
    void navigate(goalPath(goal, id));
  };

  // What is waiting opens the tab it is waiting on, so that a human who takes
  // it up is looking at it. A tab this page does not carry moves nothing.
  const show = (tab: string) => {
    if (tab !== open && sections.some((section) => section.id === tab)) {
      select(tab);
    }
  };

  // A created record is opened, because writing it is the point of creating
  // it; anything else read the project again, so the section shows what the
  // file now says rather than what it said before the write.
  const done = (result: Done) => {
    setSheet(null);
    if (result.mode === "record") {
      void navigate(documentPath(result.written.path));
      return;
    }
    onReload();
  };

  // What a contribution written from this page is about: this goal, or the
  // project as a whole. It comes from the page and is read to the human in the
  // sheet rather than asked of them — a column of actions that offered to file
  // a project-level record under a goal was asking the one question the page
  // it stood on had already answered.
  const scope: Scope = briefing.goal === null ? null : { id: briefing.goal.id, title: briefing.goal.title };

  // The one act a tab offers, as the button at the trailing end of the strip
  // and again under an empty section's sentence. It is built here, once, from
  // the tab's own name, so the two copies cannot come to say different things.
  const newButton = (act: NewAction | null): ReactNode =>
    act === null ? null : (
      <Button
        onClick={() => {
          setSheet(act.kind === null ? { mode: "question", scope } : { mode: "record", kind: act.kind, scope });
        }}
      >
        {act.label}
      </Button>
    );

  /** What an empty tab says, with the one thing to do about it under it. */
  const nothing = (tab: string) => (
    <Nothing line={nothingLine(tab, goal !== null)}>{newButton(newActionFor(tab))}</Nothing>
  );

  // The strip's trailing cluster: what this tab writes, and the refresh, which
  // every tab carries. The refresh says when the page was read in its tooltip,
  // which is where the sentence it replaced said it.
  const trailing = stripActions(open);

  /** Answering is offered on an open question, and on nothing else. */
  const answer = (row: Row): ReactNode => {
    const question = pane.questions.find((candidate) => candidate.id !== "" && candidate.id === row.key);
    if (question === undefined || question.status !== "open") {
      return null;
    }
    return (
      <Button
        onClick={() => {
          setSheet({ mode: "answer", id: question.id, question: question.question });
        }}
      >
        Answer
      </Button>
    );
  };

  // The section behind each tab, by the name the strip and the address call
  // it. The strip's order is pageSections' own, so a section the payload does
  // not produce is absent from both the strip and this.
  const sectionOf = (id: string): ReactNode => {
    const book = briefing.books.find((candidate) => candidate.id === id);
    if (book !== undefined) {
      return <BookBlock book={book} nothing={nothing(id)} />;
    }
    if (id === tabForKind("decision")) {
      return (
        <Block title={kindTitle("decision")} count={briefing.decisions.length}>
          {briefing.decisions.length === 0 ? nothing(id) : <Rows rows={briefing.decisions} />}
        </Block>
      );
    }
    if (id === tabForKind("design")) {
      return <Designs designs={briefing.designs} nothing={nothing(id)} />;
    }
    if (id === QUESTIONS_TAB) {
      return (
        <Block title={QUESTIONS_TITLE} count={briefing.questions.length}>
          {briefing.questions.length === 0 ? nothing(id) : <Rows rows={briefing.questions} action={answer} />}
        </Block>
      );
    }
    if (id === DOCUMENTS_TAB) {
      return <Documents groups={groups} total={pane.documents.length} />;
    }
    if (id === SLICES_TAB && briefing.slices !== null) {
      return <Slices plan={briefing.slices} />;
    }
    return null;
  };

  const tabs: Tab[] = sections.map((section: PageSection) => ({
    id: section.id,
    title: section.title,
    help: section.help,
    panel: sectionOf(section.id),
  }));

  return (
    <>
      {goal !== null && (
        <nav className="ms-reader-crumbs" aria-label="Breadcrumb">
          <span className="ms-reader-crumb">
            <NavLink to="/backlog">Backlog</NavLink>
          </span>
          <span className="ms-reader-crumb">
            <span className="ms-mono" aria-current="page">
              {goal}
            </span>
          </span>
        </nav>
      )}
      {/* What the records refuse and what a goal page is about stand above the
          strip: neither is a section of the page, and a refusal on one tab of
          six would be a refusal hidden on the other five. When there is
          neither, there is no band here at all. */}
      {(pane.problems.length > 0 || briefing.goal !== null) && (
        <div className="ms-briefing-preamble">
          {pane.problems.length > 0 && <Problems problems={pane.problems} />}
          {briefing.goal !== null && <GoalBlock briefing={briefing} />}
          {goal !== null && <Dependencies goal={goal} ledger={ledger} onLedger={onLedger} />}
        </div>
      )}
      <div className="ms-briefing">
        <Tabs
          label={goal === null ? "The project" : `The goal ${goal}`}
          tabs={tabs}
          selected={open}
          onSelect={select}
          action={
            <>
              {newButton(trailing.newAction)}
              <IconButton
                label={trailing.refresh}
                hint={`Read at ${timeOf(pane.readAt)} · ${trailing.refresh}`}
                onClick={onReload}
              >
                <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
              </IconButton>
            </>
          }
          panelClassName="ms-briefing-main"
        />
        <aside className="ms-briefing-aside" aria-label="Beside the briefing">
          <section className="ms-briefing-note">
            <h2 className="ms-briefing-note-title">Needs you</h2>
            {briefing.needsYou.questions === 0 && briefing.needsYou.designs === 0 ? (
              <p className="ms-briefing-note-line">Nothing is waiting here.</p>
            ) : (
              <ul className="ms-briefing-note-list">
                {briefing.needsYou.questions > 0 && (
                  <li>
                    <Act
                      label={count(briefing.needsYou.questions, "open question")}
                      onAct={() => {
                        show(QUESTIONS_TAB);
                      }}
                    />
                  </li>
                )}
                {briefing.needsYou.designs > 0 && (
                  <li>
                    <Act
                      label={`${count(briefing.needsYou.designs, "design")} accepted, not yet built`}
                      onAct={() => {
                        show(tabForKind("design"));
                      }}
                    />
                  </li>
                )}
              </ul>
            )}
          </section>
          <section className="ms-briefing-note">
            <h2 className="ms-briefing-note-title">This checkout</h2>
            <p className="ms-briefing-note-line">
              {count(briefing.checkout.records, "record")} in {count(briefing.checkout.homes, "home")} ·{" "}
              {count(briefing.checkout.goals, "ledger goal")} · {count(briefing.checkout.problems, "problem")}
            </p>
            <p className="ms-briefing-note-line">Read at {timeOf(pane.readAt)}</p>
          </section>
        </aside>
        {sheet !== null && (
          <Sheet
            request={sheet}
            onClose={() => {
              setSheet(null);
            }}
            onDone={done}
          />
        )}
      </div>
    </>
  );
}

/**
 * One action, where a human is already looking. It is a button because it acts
 * rather than navigates, and it reads as a link because in an empty state it
 * is the only thing on the line.
 */
function Act({ label, onAct }: { label: string; onAct: () => void }) {
  return (
    <button type="button" className="ms-project-act" onClick={onAct}>
      {label}
    </button>
  );
}

/**
 * An empty section says what it has none of, and offers the one thing to do
 * about it underneath.
 *
 * "Nothing recorded yet" was said on every empty tab of every page, which told
 * a human on a goal page that a project of four hundred decisions had recorded
 * none. What is empty here is this tab, in this scope, and the sentence says
 * which.
 */
function Nothing({ line, children }: { line: string; children?: ReactNode }) {
  return (
    <div className="ms-project-nothing">
      <p className="ms-project-none">{line}</p>
      {children}
    </div>
  );
}

/** A count and the word for it, pluralised the one way English needs here. */
function count(of: number, word: string): string {
  return `${String(of)} ${word}${of === 1 ? "" : "s"}`;
}

/**
 * One block of the briefing: a heading, a quiet count, and one action.
 *
 * It carries no anchor any more. A tab shows one section and hides the rest,
 * so there is nothing on this page to jump to: the address names the tab, and
 * an id that no link could use would be an id that says the page still
 * scrolls between its sections.
 */
function Block({
  title,
  count: total,
  note,
  action,
  children,
}: {
  title: string;
  count?: number;
  note?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="ms-briefing-block">
      <div className="ms-briefing-head">
        <h2 className="ms-briefing-title">{title}</h2>
        {total !== undefined && <span className="ms-project-count">{total}</span>}
        {note !== undefined && note !== "" && <span className="ms-project-count">{note}</span>}
        {action !== undefined && action !== null && <span className="ms-briefing-act">{action}</span>}
      </div>
      {children}
    </section>
  );
}

/**
 * The goal page opens with the goal itself: its id above the title, the
 * ledger's own reason for it as the lede, where it stands, and the way through
 * to the Backlog, which is where a goal is worked rather than read about.
 *
 * The way through shows this goal. It used to open the Backlog and nothing
 * else — the board came up wherever it opens, and the goal a human had just
 * been reading about was in a lane off to the right or behind a window that
 * did not reach it. It names what it does, in the view this browser is going
 * to get, and the Backlog does it on arrival.
 */
function GoalBlock({ briefing }: { briefing: Briefing }) {
  // Which view the Backlog will open in, read the way that page reads it:
  // once, as what this browser was last left on.
  const [view] = useState(() => readBacklogView());
  const goal = briefing.goal;
  if (goal === null) {
    return null;
  }
  return (
    <section className="ms-briefing-block">
      <p className="ms-facts-eyebrow ms-mono">{goal.id}</p>
      <div className="ms-briefing-head">
        <h2 className="ms-briefing-title">{goal.title}</h2>
        {goal.state !== "" && <Chip>{goal.state}</Chip>}
        <span className="ms-project-count">{goal.count}</span>
        <span className="ms-briefing-act">
          <NavLink className="ms-briefing-link" to={backlogPath(goal.id)}>
            {showLabel(view)}
          </NavLink>
        </span>
      </div>
      {goal.found ? (
        goal.intent !== "" && <Lede intent={goal.intent} />
      ) : (
        <p className="ms-project-reason">The ledger carries no goal named {goal.id}.</p>
      )}
    </section>
  );
}

/**
 * Both directions of the blocked relation, on the goal's own page.
 *
 * "Waits for" is what the record says: the goals in this one's BlockedBy.
 * "Holds" is the same relation read the other way — the goals whose own lists
 * name this one — which no record stores and the server computes. They are
 * shown together because a human asking "why is this parked" and a human
 * asking "what am I holding up" are the same human on the same page, and a
 * page that answered only the first would send them to look for the second by
 * reading four hundred other goals.
 *
 * Each row says where that goal stands and links to it, because an id alone
 * answers neither question. A goal the board does not carry is said to be
 * unknown rather than left looking live.
 *
 * Both acts go through the one route pair, and the path id is always the goal
 * that waits: removing a row under "Holds" on this page is an unblock on that
 * other goal, with this one as its blocker. They go through the ordinary act
 * path, so a 403 opens the sign-in sheet and the act is sent again.
 */
function Dependencies({
  goal,
  ledger,
  onLedger,
}: {
  goal: string;
  ledger: Backlog | null;
  onLedger: (backlog: Backlog) => void;
}) {
  const [adding, setAdding] = useState<"waits" | "holds" | null>(null);
  const [chosen, setChosen] = useState<string[]>([]);
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const { askToSignIn } = useSession();
  const retried = useRef(false);

  if (ledger === null) {
    return null;
  }
  const rows = [...ledger.rows, ...ledger.closed];
  const mine = rows.find((row) => row.ref.id === goal);
  if (mine === undefined) {
    return null;
  }
  const pickable: PickableGoal[] = rows.map((row) => ({
    id: row.ref.id,
    intent: row.intent,
    lane: laneTitle(row.lane),
    concluded: row.lane === "done" || row.lane === "abandoned" ? row.lane : "",
  }));

  /** One act, with the sign-in retry every act on this board carries. */
  const run = (act: () => Promise<Backlog>) => {
    setSending(true);
    setRefusal("");
    act()
      .then((answered) => {
        setSending(false);
        setAdding(null);
        setChosen([]);
        onLedger(answered);
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof BacklogError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(() => {
            run(act);
          });
          return;
        }
        setRefusal(actFailureMessage(error));
      });
  };

  const add = (side: "waits" | "holds") => {
    const picked = chosen.at(0);
    if (picked === undefined) {
      return;
    }
    run(() => (side === "waits" ? blockGoal(goal, picked) : blockGoal(picked, goal)));
  };

  const list = (side: "waits" | "holds", named: readonly string[]) => (
    <div className="ms-edges-side">
      <h3 className="ms-edges-title">
        {side === "waits" ? "Waits for" : "Holds"}
        <Help id={side === "waits" ? "waits-for" : "holds"} />
      </h3>
      {named.length === 0 ? (
        <p className="ms-project-note">
          {side === "waits" ? "This goal waits for nothing." : "No goal waits for this one."}
        </p>
      ) : (
        <ul className="ms-edges-list">
          {named.map((id) => (
            <li className="ms-edges-row" key={id}>
              <NavLink className="ms-edges-id ms-mono" to={goalPath(id)}>
                {id}
              </NavLink>
              <Chip>{standingOf(rows, id)}</Chip>
              <button
                type="button"
                className="ms-act-link"
                disabled={sending}
                onClick={() => {
                  run(() => (side === "waits" ? unblockGoal(goal, id) : unblockGoal(id, goal)));
                }}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
      {adding === side ? (
        <div className="ms-edges-add">
          <GoalPicker
            id={`ms-edges-${side}`}
            goals={pickable}
            chosen={chosen}
            exclude={goal}
            placeholder="e.g. refund-queue"
            onChoose={(picked) => {
              // One edge per act: the field keeps the goal named last, so a
              // second choice replaces the first rather than queueing a
              // second request behind it.
              setChosen(picked.slice(-1));
            }}
          />
          <Button
            disabled={chosen.length === 0 || sending}
            onClick={() => {
              add(side);
            }}
          >
            Add
          </Button>
          <button
            type="button"
            className="ms-act-link"
            onClick={() => {
              setAdding(null);
              setChosen([]);
            }}
          >
            Cancel
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="ms-act-link"
          onClick={() => {
            setAdding(side);
            setChosen([]);
            setRefusal("");
          }}
        >
          {side === "waits" ? "Wait for a goal…" : "Hold a goal up…"}
        </button>
      )}
    </div>
  );

  return (
    <section className="ms-briefing-block ms-edges">
      <div className="ms-edges-sides">
        {list("waits", mine.blockedBy)}
        {list("holds", mine.holds)}
      </div>
      {refusal !== "" && (
        <p className="ms-act-refuse" role="alert">
          {refusal}
        </p>
      )}
    </section>
  );
}

/** Where one named goal stands, or that the board does not carry it. */
function standingOf(rows: readonly GoalRow[], id: string): string {
  const row = rows.find((candidate) => candidate.ref.id === id);
  return row === undefined ? "unknown to the ledger" : row.state;
}

/**
 * The ledger's own reason for a goal, as it wrote it — which in this ledger is
 * a paragraph and not a line.
 *
 * It opens at five lines and the whole of it is one button away. The clamp is
 * CSS over the whole text rather than a shortened string, so find-in-page,
 * selection and a screen reader reach every word either way, and expanding
 * releases it where it stands rather than opening anything.
 *
 * Whether five lines are fewer than the whole is a question about the rendered
 * text, so it is measured rather than guessed from the length of the string:
 * the observer answers again when the column changes width or the font
 * arrives, and nothing here is on a timer.
 */
function Lede({ intent }: { intent: string }) {
  const [shown, setShown] = useState(false);
  const [clipped, setClipped] = useState(false);
  const paragraph = useRef<HTMLParagraphElement | null>(null);

  useEffect(() => {
    const element = paragraph.current;
    if (shown || element === null || typeof ResizeObserver !== "function") {
      return;
    }
    const measure = () => {
      setClipped(element.scrollHeight > element.clientHeight + 1);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => {
      observer.disconnect();
    };
  }, [intent, shown]);

  return (
    <>
      <p ref={paragraph} className={shown ? "ms-briefing-lede" : "ms-briefing-lede ms-briefing-lede--clamped"}>
        {intent}
      </p>
      {(clipped || shown) && (
        <button
          type="button"
          className="ms-project-act ms-briefing-lede-more"
          aria-expanded={shown}
          onClick={() => {
            setShown((open) => !open);
          }}
        >
          {shown ? "Show less" : "Show all"}
        </button>
      )}
    </>
  );
}

/**
 * A book, opened: its own first paragraph as the lede, then its reading order
 * as a table of contents. The titles are the index's own, which is why they
 * carry their numbers here and nothing numbers them again.
 */
function BookBlock({ book, nothing }: { book: BookBriefing; nothing: ReactNode }) {
  return (
    <Block
      title={book.title}
      note={`${book.status === "" ? "no index" : book.status} · ${String(book.chapters.length)} chapters`}
      action={
        book.to === null ? undefined : (
          <NavLink className="ms-briefing-link" to={book.to}>
            Read the {book.title.toLowerCase()} →
          </NavLink>
        )
      }
    >
      {book.lede !== "" && <p className="ms-briefing-lede">{book.lede}</p>}
      {book.chapters.length === 0 ? (
        nothing
      ) : (
        <ul className="ms-briefing-toc">
          {book.chapters.map((chapter) => (
            <li key={chapter.key} className="ms-briefing-toc-row">
              {chapter.to === null ? (
                <span>{chapter.title}</span>
              ) : (
                <NavLink to={chapter.to} title={chapter.summary === "" ? undefined : chapter.summary}>
                  {chapter.title}
                </NavLink>
              )}
            </li>
          ))}
        </ul>
      )}
    </Block>
  );
}

/** The designs: what governs, open; what is finished, in one run each. */
function Designs({ designs, nothing }: { designs: DesignRuns; nothing: ReactNode }) {
  const total = designs.open.length + designs.runs.reduce((sum, run) => sum + run.rows.length, 0);
  return (
    <Block title={kindTitle("design")} count={total}>
      {total === 0 && nothing}
      {designs.open.length > 0 && <Rows rows={designs.open} />}
      {designs.runs.map((run) => (
        <details key={run.status} className="ms-project-run">
          <summary className="ms-project-run-summary">
            {String(run.rows.length)} {run.status}
          </summary>
          <Rows rows={run.rows} />
        </details>
      ))}
    </Block>
  );
}

/**
 * A goal's slice plan, read and not edited.
 *
 * Two things are recorded and both are shown: the goal's own slicing boundary,
 * which says when the pre-reservation happened and which seat made it, and the
 * list each governing design wrote under its Slices heading, as it wrote it,
 * with a link to the design it came from. Nothing here is editable, and that
 * is the point rather than an omission: the repository has slice admission and
 * a first-slicing marker and no editable slice-plan owner, and the master is
 * explicit that a browser-only checklist cannot stand in for the missing one.
 * So the tab says what is recorded, names the design that recorded it, and
 * says plainly when nothing is.
 */
function Slices({ plan }: { plan: SlicePlan }) {
  return (
    <Block title={SLICES_TITLE} count={sliceCount(plan)}>
      {plan.started === null ? (
        <p className="ms-project-note">Slicing has not started on this goal.</p>
      ) : (
        <p className="ms-project-note">
          Slicing started {dateOf(plan.started.at)}, by <span className="ms-mono">{plan.started.machine}</span>
          {plan.started.lineage === "" ? "" : ` (${plan.started.lineage})`}. Once it has, the goal can only advance
          through a split.
        </p>
      )}
      {plan.designs.length === 0 ? (
        <p className="ms-project-none">{NO_SLICE_PLAN}</p>
      ) : (
        plan.designs.map((design) => (
          <section key={design.key} className="ms-slice-run">
            <div className="ms-briefing-head">
              <h3 className="ms-slice-design">
                <NavLink to={design.to}>{design.title}</NavLink>
              </h3>
              {design.status !== "" && <Chip>{design.status}</Chip>}
              <span className="ms-project-count">{design.slices.length}</span>
            </div>
            <ol className="ms-slice-list">
              {design.slices.map((slice, at) => (
                <li key={`${design.key}-${String(at)}`}>{slice}</li>
              ))}
            </ol>
          </section>
        ))
      )}
    </Block>
  );
}

function LoadingCards() {
  return (
    <div className="ms-pane-stack">
      {[0, 1, 2].map((row) => (
        <section key={row} className="ms-card">
          <Skeleton />
        </section>
      ))}
    </div>
  );
}

function FailureCard({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="ms-pane-stack">
      <section className="ms-card">
        <h2 className="ms-card-title">Project could not be read</h2>
        <p className="ms-project-reason">{message}</p>
        <Button onClick={onRetry}>Retry</Button>
      </section>
    </div>
  );
}

/** What the check verb would refuse, at the top, where it can be acted on. */
function Problems({ problems }: { problems: Problem[] }) {
  return (
    <section className="ms-briefing-block">
      <div className="ms-briefing-head">
        <h2 className="ms-briefing-title">Refused by the records themselves</h2>
        <span className="ms-project-count">{problems.length}</span>
      </div>
      <ul className="ms-project-rows">
        {problems.map((problem) => (
          <li key={`${problem.path}:${String(problem.line)}:${problem.message}`} className="ms-project-problem">
            <span className="ms-mono ms-project-problem-at">
              {problem.path}:{problem.line}
            </span>
            <span>{problem.message}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Rows({ rows, action }: { rows: Row[]; action?: (row: Row) => ReactNode }) {
  return (
    <ul className="ms-project-rows">
      {rows.map((row) => (
        <RecordRow key={row.key} row={row} action={action?.(row)} />
      ))}
    </ul>
  );
}

/**
 * One row: the title, what it says it is about, what it declares, and where it
 * lives. The path is one line, clipped at its end, and carries the whole of
 * itself in a tooltip, so a long path never breaks the row across the screen a
 * character at a time.
 */
function RecordRow({ row, action }: { row: Row; action?: ReactNode }) {
  // The board's own menu, on a record's row: right-click, Shift+F10 and the
  // Menu key, with the one act a listing offers. A row is a thing to ask
  // about, and a listing that offered it as a button on every line would be a
  // control per row competing with the records they are about.
  const [menuAt, setMenuAt] = useState<At | null>(null);
  const { ask } = usePartner();
  return (
    <li
      className="ms-project-row"
      tabIndex={0}
      onContextMenu={(event) => {
        event.preventDefault();
        setMenuAt({ x: event.clientX, y: event.clientY });
      }}
      onKeyDown={(event) => {
        if (!opensMenu(event.key, event.shiftKey)) {
          return;
        }
        event.preventDefault();
        const box = event.currentTarget.getBoundingClientRect();
        setMenuAt({ x: box.left + 8, y: box.top + 8 });
      }}
    >
      {menuAt !== null && (
        <CardMenu
          at={menuAt}
          label={`Acts on ${row.title}`}
          offers={[{ id: "ask", label: "Ask about this" }]}
          onClose={() => {
            setMenuAt(null);
          }}
          onChoose={() => {
            setMenuAt(null);
            ask({
              kind: "record",
              id: row.path,
              title: row.title,
              source: "the checkout's records as they stand",
              summary: [row.status, row.note, row.summary].filter((part) => part !== "").join(" · "),
              to: row.to ?? undefined,
            });
          }}
        />
      )}
      <span className="ms-project-row-title">
        {row.to === null ? row.title : <NavLink to={row.to}>{row.title}</NavLink>}
      </span>
      <span className="ms-project-row-status">
        {row.status !== "" && <Chip>{row.status}</Chip>}
        {row.note !== "" && <span className="ms-project-row-note">{row.note}</span>}
      </span>
      <span className="ms-project-row-act">{action}</span>
      <span className="ms-mono ms-project-row-path" title={row.path}>
        {row.path}
      </span>
      {row.summary !== "" && <span className="ms-project-row-summary">{row.summary}</span>}
    </li>
  );
}

/**
 * The rest of the checkout's Markdown. It is large — thousands of files in
 * this checkout — so every run is collapsed, and a run's rows are built only
 * once it is opened.
 */
function Documents({ groups, total }: { groups: DocumentGroup[]; total: number }) {
  return (
    <Block title={DOCUMENTS_TITLE} note={`${String(total)} files, no kind claimed`}>
      {groups.length === 0 ? (
        <p className="ms-project-none">Nothing recorded yet.</p>
      ) : (
        <>
          <p className="ms-project-note">Every other Markdown file in this checkout, by path. No kind is claimed.</p>
          {groups.map((group) => (
            <DocumentRun key={group.id} group={group} />
          ))}
        </>
      )}
    </Block>
  );
}

function DocumentRun({ group }: { group: DocumentGroup }) {
  const [open, setOpen] = useState(false);
  return (
    <details
      className="ms-project-run"
      onToggle={(event) => {
        setOpen(event.currentTarget.open);
      }}
    >
      <summary className="ms-project-run-summary">
        <span className="ms-mono">{group.title}</span>
        <span className="ms-project-count">{group.files.length}</span>
      </summary>
      {open && (
        <ul className="ms-project-rows">
          {group.files.map((file) => (
            <DocumentRow key={file.path} file={file} />
          ))}
        </ul>
      )}
    </details>
  );
}

function DocumentRow({ file }: { file: DocumentFile }) {
  return (
    <li className="ms-project-row">
      <span className="ms-project-row-title">
        <NavLink to={documentPath(file.path)}>{file.title}</NavLink>
      </span>
      <span className="ms-project-row-status" />
      <span className="ms-project-row-act" />
      <span className="ms-mono ms-project-row-path" title={file.path}>
        {file.path}
      </span>
    </li>
  );
}

/** The engine's ownership answer, in the words the master uses for it. */
export function ownership(owner: string): string {
  switch (owner) {
    case "metasystem-generic":
      return "the MetaSystem's";
    case "app-owned":
      return "the application's";
    case "runtime":
      return "runtime state";
    default:
      return "ownership unknown";
  }
}

/** A recorded instant, shown in the reader's own locale, never reinterpreted. */
export function dateOf(stamp: string): string {
  const at = new Date(stamp);
  return Number.isNaN(at.getTime()) ? stamp : at.toLocaleDateString();
}

export function timeOf(stamp: string): string {
  const at = new Date(stamp);
  return Number.isNaN(at.getTime()) ? stamp : at.toLocaleTimeString();
}
