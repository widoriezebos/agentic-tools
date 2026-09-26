import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadDecisions,
  type Approved,
  type Item,
  type Mention,
  type Need,
  type NotNow,
  type Page as DecisionsPayload,
  type Ruling,
} from "./api";
import { BulkSheet, type Bulk } from "./BulkSheet";
import "./decisions.css";
import {
  ANY_CLASS,
  ANY_ORIGIN,
  approvalLine,
  classesIn,
  defectLine,
  destinationFor,
  isNarrowed,
  noNarrowing,
  NO_CLASS,
  reviewChip,
  shownRulings,
  tabs,
  whenLine,
  withdrawable,
  YOURS,
  type Narrowing,
  type TabId,
} from "./decisions";
import { decidedCount, groupOnScreen, groupsOf, viewTitle, type ViewId } from "./groups";
import { Inbox } from "./Inbox";
import type { Acts } from "./InboxRow";
import { useNarrowing } from "./QueueBlock";
import { ActSheet, type Request } from "../backlog/ActSheet";
import { loadBacklog, unparkGoal, type Backlog, type Row } from "../backlog/api";
import { EditSheet } from "../backlog/EditSheet";
import { minuteTime } from "../backlog/format";
import { Help } from "../help/Help";
import type { HelpId } from "../help/terms";
import { Pane } from "../panes/Pane";
import { Tabs } from "../panes/Tabs";
import { setRecordStatus } from "../project/api";
import { goalPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useSession } from "../shell/identity";
import type { SessionStatus } from "../shell/session";
import { useOffersRefresh } from "../shell/refresh";
import { readDecisionsOpen, writeDecisionsOpen } from "../storage";

/**
 * Decisions: an inbox you can empty, and what you decided.
 *
 * Two views and no more, as tabs with their counts. The Inbox is the page at
 * rest: one line per group, collapsed, with how many, how old the newest one
 * is and how much is new since the last visit — and one group open. Decided is
 * the record of what this human has already said, unchanged.
 *
 * Every act on the page runs through a route that already exists. The board's
 * sheets approve, withdraw, park and unpark a goal; g1-s47's sheet edits a
 * queued one; the record status route accepts a draft and marks a landed
 * design done, each behind a one-line confirmation that names the file and the
 * word it will write. Where a decision is made somewhere else — a question
 * that needs an answering record's reference, a ruling that is revised in the
 * register, a seat's ask that is answered on the channel — the row says where
 * instead of pretending to an act.
 *
 * The page is one read, made when it mounts, again when a human presses the
 * refresh in the header's section cluster, and again after an act completes.
 * There is no timer, nothing polls, and nothing refetches on a window event.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; page: DecisionsPayload };

/** What the sheet needs beside the row, and how far this page has got to it. */
type Acting =
  | { state: "loading"; request: Request }
  | { state: "failed"; request: Request; message: string }
  | { state: "ready"; request: Request; backlog: Backlog };

/** The same, for a sheet opened over several goals at once. */
type Bulking =
  | { state: "loading"; bulk: Bulk }
  | { state: "failed"; bulk: Bulk; message: string }
  | { state: "ready"; bulk: Bulk; backlog: Backlog };

/** And the same, for the editor over one queued goal. */
type Editing =
  | { state: "loading"; goal: Row }
  | { state: "failed"; goal: Row; message: string }
  | { state: "ready"; goal: Row; backlog: Backlog };

/** What a write to a record left behind: which row it was, and what it said. */
type Refused = { at: string; message: string };

const nothingRefused: Refused = { at: "", message: "" };

export function DecisionsPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [acting, setActing] = useState<Acting | null>(null);
  const [bulk, setBulk] = useState<Bulking | null>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [view, setView] = useState<ViewId>("inbox");
  const [tab, setTab] = useState<TabId>("rulings");
  // Which group this viewer left open. null is "nothing stored"; "" is a
  // group they closed, which is a choice and not an absence.
  const [chosen, setChosen] = useState<string | null>(() => readDecisionsOpen());
  // The one row open, across every group, and the queue's own sitting state.
  const [openRow, setOpenRow] = useState("");
  const [narrowing, setNarrowing] = useNarrowing();
  const [selected, setSelected] = useState<string[]>([]);
  // The one act this page publishes without a sheet: a record's status, and a
  // seat's park returning to the queue. Both say what they are doing and both
  // say, in the row they came from, why they were refused.
  const [busy, setBusy] = useState("");
  const [refused, setRefused] = useState<Refused>(nothingRefused);
  // Who is signed in NOW, rather than who was signed in when this payload was
  // composed. A human who opens the page signed out, signs in through the
  // sheet and presses Not now would otherwise meet a disabled button until
  // they thought to press Refresh.
  const { session, askToSignIn } = useSession();
  const signedIn = signedInNow(session);

  useEffect(() => {
    const aborter = new AbortController();
    loadDecisions(aborter.signal)
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
    setBusy("");
    setRefused(nothingRefused);
    setAttempt((previous) => previous + 1);
  }, []);

  // And the payload itself is read again when the session settles into
  // signed-in, because signIn is one of the things it carries: the sign-in
  // bar at the top comes from the payload, and a page that left it standing
  // after a human signed in would be telling them to do something they have
  // already done. Only on the transition — a page that re-read on every
  // render of a signed-in session would never stop reading.
  const wasSignedIn = useRef(signedIn);
  useEffect(() => {
    const justSignedIn = signedIn && !wasSignedIn.current;
    wasSignedIn.current = signedIn;
    if (justSignedIn) {
      reload();
    }
  }, [signedIn, reload]);

  // The sheet is opened over the whole backlog payload, exactly as the board
  // opens it: the authority, the project's budget law and the rows it
  // prefills from are all in that one resource, and a sheet given only a row
  // would have nothing to prefill from and nothing to say about who is
  // acting. It is read when the sheet opens rather than with the page,
  // because most visits to this page open no sheet at all.
  const open = useCallback((request: Request) => {
    setActing({ state: "loading", request });
    loadBacklog()
      .then((backlog) => {
        setActing({ state: "ready", request, backlog });
      })
      .catch((error: unknown) => {
        setActing({ state: "failed", request, message: failureMessage(error) });
      });
  }, []);

  // A bulk sheet needs the same payload for the same reason: it prefills one
  // budget per goal from the project's law and the rows it is opened over.
  const openBulk = useCallback((asked: Bulk) => {
    setBulk({ state: "loading", bulk: asked });
    loadBacklog()
      .then((backlog) => {
        setBulk({ state: "ready", bulk: asked, backlog });
      })
      .catch((error: unknown) => {
        setBulk({ state: "failed", bulk: asked, message: failureMessage(error) });
      });
  }, []);

  // And so does the editor: it suggests labels from what the board already
  // carries, and it says who is acting.
  const openEdit = useCallback((goal: Row) => {
    setEditing({ state: "loading", goal });
    loadBacklog()
      .then((backlog) => {
        setEditing({ state: "ready", goal, backlog });
      })
      .catch((error: unknown) => {
        setEditing({ state: "failed", goal, message: failureMessage(error) });
      });
  }, []);

  // Return to queue is one goal and one publication, so it needs nothing but
  // the id: it sends unpark and the page reads its payload again, which is
  // what says the goal came back.
  const returnToQueue = useCallback(
    (id: string) => {
      setBusy(id);
      setRefused(nothingRefused);
      unparkGoal(id)
        .then(() => {
          reload();
        })
        .catch((error: unknown) => {
          setBusy("");
          setRefused({ at: id, message: failureMessage(error) });
        });
    },
    [reload],
  );

  // Accepting a draft and marking a landed design done are one write each to
  // the record's Status line, through the route the reader's own act writes
  // through. The confirmation the row showed is the whole of their safety;
  // this is what happens after it.
  const writeStatus = useCallback(
    (need: Need, status: string) => {
      setBusy(need.id);
      setRefused(nothingRefused);
      setRecordStatus(need.id, status)
        .then(() => {
          reload();
        })
        .catch((error: unknown) => {
          setBusy("");
          setRefused({ at: need.id, message: failureMessage(error) });
        });
    },
    [reload],
  );

  const acts: Acts = useMemo(
    () => ({
      signedIn,
      onSignIn: () => {
        askToSignIn();
      },
      onApprove: (need: Need) => {
        if (need.row !== null) {
          open({ move: "approve", goal: need.row });
        }
      },
      onPark: (need: Need) => {
        openBulk({ act: "park", goals: [need] });
      },
      onEdit: (need: Need) => {
        if (need.row !== null) {
          openEdit(need.row);
        }
      },
      onReturn: returnToQueue,
      onWrite: writeStatus,
      writing: busy,
      refusedAt: refused.at,
      refusal: refused.message,
    }),
    [signedIn, askToSignIn, open, openBulk, openEdit, returnToQueue, writeStatus, busy, refused],
  );

  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  useOffersRefresh(reload, hint);

  // What the Partner is given: the inbox rows on screen — their kind, their
  // id and what they ask — and which view of the page is open. A ruling's own
  // words never travel: the register is a document of the checkout, and the
  // Partner reads it there.
  const records = useMemo(
    () => (read.state === "read" ? read.page.needsYou.map((need) => `${need.kind} · ${need.id} · ${need.asked}`) : []),
    [read],
  );
  // And what the capture adds: which view a human is in, which group they
  // have open and which row, what is narrowing the queue, and how many rows
  // they have ticked. A Partner asked "what am I looking at" answers from
  // these and not from a guess about the whole payload.
  const filters = useMemo(
    () =>
      captureLines(
        view,
        read.state === "read" ? groupOnScreen(read.page.needsYou, chosen) : null,
        openRow,
        narrowing,
        selected.length,
      ),
    [view, read, chosen, openRow, narrowing, selected],
  );
  useAbout(aboutLine("Decisions", view === "decided" ? tab : "inbox"), {
    tab: view,
    records,
    filters,
    returnTo: "/decisions",
  });

  return (
    <Pane title="Decisions">
      {read.state === "loading" && <Loading />}
      {read.state === "failed" && <Failure message={read.message} onRetry={reload} />}
      {read.state === "read" && (
        <Views
          page={read.page}
          signedIn={signedIn}
          view={view}
          onView={setView}
          tab={tab}
          onTab={setTab}
          chosen={chosen}
          onChoose={(next) => {
            setChosen(next);
            writeDecisionsOpen(next);
            setOpenRow("");
          }}
          openRow={openRow}
          onOpenRow={setOpenRow}
          acts={acts}
          narrowing={narrowing}
          onNarrow={setNarrowing}
          selected={selected}
          onSelect={setSelected}
          onBulk={(act, goals) => {
            openBulk({ act, goals });
          }}
          onAct={open}
        />
      )}
      {bulk !== null && bulk.state === "failed" && (
        <p className="ms-decisions-refusal" role="alert">
          The backlog could not be read, so the sheet could not open: {bulk.message}
        </p>
      )}
      {bulk !== null && bulk.state === "ready" && (
        <BulkSheet
          bulk={bulk.bulk}
          backlog={bulk.backlog}
          onClose={() => {
            setBulk(null);
          }}
          onDone={() => {
            setBulk(null);
            setSelected([]);
            reload();
          }}
          onStopped={() => {
            setSelected([]);
            reload();
          }}
        />
      )}
      {acting !== null && acting.state === "failed" && (
        <p className="ms-decisions-refusal" role="alert">
          The backlog could not be read, so the sheet could not open: {acting.message}
        </p>
      )}
      {acting !== null && acting.state === "ready" && (
        <ActSheet
          request={acting.request}
          backlog={acting.backlog}
          onClose={() => {
            setActing(null);
          }}
          onDone={() => {
            // The act answered with the ledger as it then stood, and this
            // page is composed from more than the ledger, so it reads its own
            // payload again rather than pretending it can derive one.
            setActing(null);
            reload();
          }}
        />
      )}
      {editing !== null && editing.state === "failed" && (
        <p className="ms-decisions-refusal" role="alert">
          The backlog could not be read, so the sheet could not open: {editing.message}
        </p>
      )}
      {editing !== null && editing.state === "ready" && (
        <EditSheet
          goal={editing.goal}
          backlog={editing.backlog}
          onClose={() => {
            setEditing(null);
          }}
          onDone={() => {
            setEditing(null);
            reload();
          }}
          // A save nobody could confirm may have landed, so this page reads its
          // own payload again and leaves the sheet, and its words, where they are.
          onReread={() => {
            reload();
          }}
        />
      )}
    </Pane>
  );
}

/**
 * The two views, over one payload.
 *
 * They are exported apart from the pane so that a test can render what the
 * page says about a payload without a server behind it: the reading, the
 * network and the sheets are the pane's, and what the two views put on the
 * screen is this.
 */
export function Views({
  page,
  signedIn,
  view = "inbox",
  onView = () => undefined,
  tab = "rulings",
  onTab = () => undefined,
  chosen = null,
  onChoose = () => undefined,
  openRow = "",
  onOpenRow = () => undefined,
  acts,
  narrowing = noNarrowing,
  onNarrow = () => undefined,
  selected = [],
  onSelect = () => undefined,
  onBulk = () => undefined,
  onAct = () => undefined,
  now = new Date(),
}: {
  page: DecisionsPayload;
  /**
   * Whether a human is signed in NOW. The payload's own signIn is what this
   * page was composed against; a sign-in that happened since is not in it.
   */
  signedIn?: boolean;
  view?: ViewId;
  onView?: (id: ViewId) => void;
  tab?: TabId;
  onTab?: (id: TabId) => void;
  chosen?: string | null;
  /** The group to open, or "" to close the one that is open. */
  onChoose?: (id: string) => void;
  openRow?: string;
  onOpenRow?: (id: string) => void;
  acts: Acts;
  narrowing?: Narrowing;
  onNarrow?: (next: Narrowing) => void;
  selected?: readonly string[];
  onSelect?: (ids: string[]) => void;
  onBulk?: (act: "approve" | "park", goals: Need[]) => void;
  onAct?: (request: Request) => void;
  /**
   * The clock every age on this page is said against. It is the real one in
   * the app and a fixed instant in a test, so that "yesterday" and "5 days"
   * are asserted against the payload's own day rather than the day the test
   * happens to run.
   */
  now?: Date;
}) {
  const acting = signedIn ?? !page.signIn;
  const groups = groupsOf(page.needsYou, now);
  const open = groupOnScreen(page.needsYou, chosen);
  return (
    <div className="ms-decisions">
      {page.signIn && <SignIn onSignIn={acts.onSignIn} />}
      <Tabs
        label="Decisions"
        tabs={[
          {
            id: "inbox",
            title: viewTitle("inbox", page.counts.needsYou),
            help: "inbox",
            panel: (
              <Inbox
                groups={groups}
                open={open}
                // The toggle is decided here rather than where the choice is
                // stored, because only here is it known WHICH group is open:
                // with nothing remembered, one is open by the fallback, and a
                // pane comparing the press against its own empty memory would
                // answer the first press on that group by opening it again.
                onOpen={(id) => {
                  onChoose(open === id ? "" : id);
                }}
                openRow={openRow}
                onOpenRow={onOpenRow}
                acts={{ ...acts, signedIn: acting }}
                narrowing={narrowing}
                onNarrow={onNarrow}
                selected={selected}
                onSelect={onSelect}
                onBulk={onBulk}
                now={now}
              />
            ),
          },
          {
            id: "decided",
            title: viewTitle("decided", decidedCount(page.decided)),
            panel: (
              <Decided page={page} acting={acting} tab={tab} onTab={onTab} onAct={onAct} acts={acts} now={now} />
            ),
          },
        ]}
        selected={view}
        onSelect={(id) => {
          onView(id === "decided" ? "decided" : "inbox");
        }}
        action={<Help id="new-here" />}
      />
    </div>
  );
}

/** Whether the live session is one a human has signed into. */
function signedInNow(status: SessionStatus): boolean {
  return status.state === "known" && status.session.signedIn;
}

/**
 * The one thing on this page that is not something to decide: nothing proves
 * a human on this seat, and until something does, none of the acts can
 * publish.
 */
function SignIn({ onSignIn }: { onSignIn: () => void }) {
  return (
    <div className="ms-decisions-signin">
      <button type="button" className="ms-decisions-signin-button" onClick={onSignIn}>
        Sign in to act
      </button>
      <span className="ms-decisions-signin-words">Nothing proves a human on this seat yet.</span>
    </div>
  );
}

/**
 * The capture's own lines: where a human is in this page and what they have
 * narrowed it to. They are the page's state rather than its payload, which is
 * why nothing else can answer them.
 */
function captureLines(
  view: ViewId,
  open: string | null,
  openRow: string,
  narrowing: Narrowing,
  selected: number,
): string[] {
  // The group the page resolved, not the one this viewer's browser remembers:
  // a closed inbox and an inbox whose remembered group has gone both have an
  // answer, and it is the one on the screen.
  const lines = [`view: ${view}`, `group open: ${open ?? "none"}`];
  if (openRow !== "") {
    lines.push(`row open: ${openRow}`);
  }
  lines.push(`queue order: ${narrowing.order === "newest" ? "newest first" : "backlog order"}`);
  if (narrowing.find.trim() !== "") {
    lines.push(`find: ${narrowing.find.trim()}`);
  }
  if (narrowing.label !== "") {
    lines.push(`label: ${narrowing.label}`);
  }
  if (narrowing.origin !== ANY_ORIGIN) {
    lines.push(`origin: ${narrowing.origin === YOURS ? "yours" : "seats'"}`);
  }
  if (!isNarrowed(narrowing)) {
    lines.push("nothing is narrowed");
  }
  lines.push(`${String(selected)} selected`);
  return lines;
}

/* ------------------------------------------------------------- decided -- */

/**
 * The term beside each tab's name. Two of the five are words this project
 * uses in its own way; the others are ordinary English and carry none.
 */
const TAB_TERMS: Readonly<Partial<Record<TabId, HelpId>>> = {
  rulings: "ruling",
  decisions: "decisions",
  "not-now": "not-now",
};

function Decided({
  page,
  acting,
  tab,
  onTab,
  onAct,
  acts,
  now,
}: {
  page: DecisionsPayload;
  acting: boolean;
  tab: TabId;
  onTab: (id: TabId) => void;
  onAct: (request: Request) => void;
  acts: Acts;
  now: Date;
}) {
  const decided = page.decided;
  const strip = tabs(page);
  const panels: Record<TabId, ReactNode> = {
    rulings: <Rulings rulings={decided.rulings} defects={decided.defects} now={now} />,
    decisions: <Items items={decided.decisions} empty="This project has recorded no decisions yet." now={now} />,
    answered: <Items items={decided.answered} empty="No question of the register has been answered yet." now={now} />,
    approved: <Approvals approved={decided.approved} onAct={onAct} now={now} />,
    "not-now": <NotNowList parks={decided.notNow} signedIn={acting} onReturn={acts.onReturn} now={now} />,
  };
  return (
    <Tabs
      label="What you decided"
      tabs={strip.map((one) => ({
        id: one.id,
        title: one.title,
        panel: panels[one.id],
        help: TAB_TERMS[one.id],
      }))}
      selected={tab}
      onSelect={(id) => {
        onTab(id as TabId);
      }}
    />
  );
}

function Rulings({ rulings, defects, now }: { rulings: Ruling[]; defects: string[]; now: Date }) {
  const [find, setFind] = useState("");
  const [narrowed, setNarrowed] = useState<string>(ANY_CLASS);
  const classes = useMemo(() => classesIn(rulings), [rulings]);
  const shown = useMemo(() => shownRulings(rulings, find, narrowed), [rulings, find, narrowed]);
  const broken = defectLine(defects);
  return (
    <div className="ms-decisions-rulings">
      <div className="ms-decisions-find">
        <label className="ms-visually-hidden" htmlFor="ms-decisions-find">
          Find in the rulings
        </label>
        <input
          id="ms-decisions-find"
          type="search"
          className="ms-decisions-find-field"
          placeholder="Find in the words and the context"
          value={find}
          onChange={(event) => {
            setFind(event.target.value);
          }}
        />
        <label className="ms-visually-hidden" htmlFor="ms-decisions-class">
          Narrow to a review class
        </label>
        <Help id="review-condition" />
        <select
          id="ms-decisions-class"
          className="ms-decisions-class"
          value={narrowed}
          onChange={(event) => {
            setNarrowed(event.target.value);
          }}
        >
          <option value={ANY_CLASS}>Every class</option>
          {classes.map((one) => (
            <option key={one} value={one}>
              {one === NO_CLASS ? "no review condition" : one}
            </option>
          ))}
        </select>
      </div>
      {broken !== null && (
        <details className="ms-decisions-defects">
          <summary className="ms-decisions-defects-line">{broken}</summary>
          <ul className="ms-decisions-defect-list">
            {defects.map((defect) => (
              <li key={defect} className="ms-decisions-defect">
                {defect}
              </li>
            ))}
          </ul>
        </details>
      )}
      {shown.length === 0 ? (
        <p className="ms-decisions-none">No ruling of the register matches.</p>
      ) : (
        <ul className="ms-decisions-cards">
          {shown.map((ruling) => (
            <RulingCard key={ruling.id} ruling={ruling} now={now} />
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * One ruling, whole: its id and date, the human's own words, the context
 * behind a disclosure, the owner, the review condition as a chip, and the
 * goals and records the words name as chips that open them.
 */
function RulingCard({ ruling, now }: { ruling: Ruling; now: Date }) {
  const chip = reviewChip(ruling, now);
  return (
    <li className="ms-decisions-card">
      <p className="ms-decisions-card-head">
        <span className="ms-mono ms-decisions-card-id">{ruling.id}</span>
        <span className="ms-mono ms-decisions-card-date">{ruling.date}</span>
        {chip !== "" && <Chip marker={ruling.duePassed}>{chip}</Chip>}
      </p>
      <p className="ms-decisions-card-words">{ruling.words}</p>
      <p className="ms-decisions-card-facts">
        {ruling.owner !== "" && <span className="ms-decisions-card-owner">owner {ruling.owner}</span>}
        {ruling.mentions.map((mention) => (
          <Mentioned key={mention.id} mention={mention} />
        ))}
      </p>
      {ruling.context !== "" && (
        <details className="ms-decisions-context">
          <summary className="ms-decisions-context-line">Context</summary>
          <p className="ms-decisions-context-words">{ruling.context}</p>
        </details>
      )}
    </li>
  );
}

/**
 * One id a ruling names, as a chip that opens it.
 *
 * The words name a goal or one of the checkout's records, and the two open
 * different things, so the chip follows the payload's own destination rather
 * than assuming the id is a goal's. An id this build has no surface for reads
 * as the id and nothing else: a chip that refused would be worse than a word.
 */
function Mentioned({ mention }: { mention: Mention }) {
  const destination = destinationFor(mention.where);
  if (destination.kind !== "link") {
    return <span className="ms-decisions-mention">{mention.id}</span>;
  }
  return (
    <NavLink className="ms-decisions-mention" to={destination.to}>
      {mention.id}
    </NavLink>
  );
}

/**
 * Not now: the parks this human made, with the whole of what they said.
 *
 * Each row is the Overview's own item row with the reason as its note, and a
 * button that returns the goal to the queue — one goal, one publication, and
 * then the page reads its payload again. A blocker park names what it is
 * waiting for, because it returns by itself when that lands and the row
 * should say so rather than look like a pause somebody forgot.
 */
function NotNowList({
  parks,
  signedIn,
  onReturn,
  now,
}: {
  parks: NotNow[];
  signedIn: boolean;
  onReturn: (id: string) => void;
  now: Date;
}) {
  if (parks.length === 0) {
    return <p className="ms-decisions-none">You have paused nothing.</p>;
  }
  return (
    <>
      <p className="ms-decisions-quiet">
        Return to queue lifts one pause.
        <Help id="return-to-queue" />
      </p>
      <ul className="ms-decisions-lines">
        {parks.map((park) => (
          <li key={park.id} className="ms-decisions-line ms-decisions-line--act">
            <NavLink className="ms-decisions-row" to={goalPath(park.id)}>
              <span className="ms-mono ms-decisions-row-id">{park.id}</span>
              <span className="ms-decisions-row-title">{park.title === "" ? park.id : park.title}</span>
              <span className="ms-decisions-row-note">{park.because}</span>
              <span className="ms-decisions-row-note">
                {park.by}
                {park.blocker === "" ? "" : ` · waits for ${park.blocker}`}
              </span>
              <span className="ms-mono ms-decisions-row-tail">{whenLine(park.at, now)}</span>
            </NavLink>
            <Button
              disabled={!signedIn}
              onClick={() => {
                onReturn(park.id);
              }}
            >
              Return to queue
            </Button>
          </li>
        ))}
      </ul>
    </>
  );
}

function Items({ items, empty, now }: { items: Item[]; empty: string; now: Date }) {
  if (items.length === 0) {
    return <p className="ms-decisions-none">{empty}</p>;
  }
  return (
    <ul className="ms-decisions-lines">
      {items.map((item, position) => (
        <li key={`${item.id}-${String(position)}`} className="ms-decisions-line">
          <ItemRow item={item} now={now} />
        </li>
      ))}
    </ul>
  );
}

function ItemRow({ item, now }: { item: Item; now: Date }) {
  const when = whenLine(item.at, now);
  const destination = destinationFor(item.where);
  const body = (
    <>
      <span className="ms-decisions-row-title">{item.title === "" ? item.id : item.title}</span>
      {item.note !== "" && <span className="ms-decisions-row-note">{item.note}</span>}
      {when !== "" && <span className="ms-mono ms-decisions-row-tail">{when}</span>}
    </>
  );
  if (destination.kind === "link") {
    return (
      <NavLink className="ms-decisions-row" to={destination.to}>
        {body}
      </NavLink>
    );
  }
  return <span className="ms-decisions-row">{body}</span>;
}

function Approvals({ approved, onAct, now }: { approved: Approved[]; onAct: (request: Request) => void; now: Date }) {
  if (approved.length === 0) {
    return <p className="ms-decisions-none">No goal carries an approval yet.</p>;
  }
  return (
    <ul className="ms-decisions-lines">
      {approved.map((one) => (
        <li key={one.id} className="ms-decisions-line ms-decisions-line--act">
          <NavLink className="ms-decisions-row" to={goalPath(one.id)}>
            <span className="ms-mono ms-decisions-row-id">{one.id}</span>
            <span className="ms-decisions-row-title">{one.title === "" ? one.id : one.title}</span>
            <span className="ms-decisions-row-note">{approvalLine(one, now)}</span>
          </NavLink>
          {withdrawable(one.row) && (
            <Button
              onClick={() => {
                onAct({ move: "withdraw", goal: one.row });
              }}
            >
              Withdraw approval
            </Button>
          )}
        </li>
      ))}
    </ul>
  );
}

/* -------------------------------------------------------- the two states -- */

function Loading() {
  return (
    <div className="ms-decisions">
      <Skeleton />
    </div>
  );
}

function Failure({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="ms-pane-stack">
      <section className="ms-card">
        <h2 className="ms-card-title">Decisions could not be read</h2>
        <p className="ms-decisions-reason">{message}</p>
        <Button onClick={onRetry}>Retry</Button>
      </section>
    </div>
  );
}
