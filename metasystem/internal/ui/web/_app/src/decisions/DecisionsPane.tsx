import { CircleCheck } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadDecisions,
  type Approved,
  type Item,
  type Need,
  type NotNow,
  type Page as DecisionsPayload,
  type Ruling,
  type Where as Reference,
} from "./api";
import { BulkSheet, type Bulk } from "./BulkSheet";
import "./decisions.css";
import {
  ANY_CLASS,
  ANY_ORIGIN,
  actLabel,
  approvalLine,
  askedLine,
  askedOf,
  classesIn,
  defectLine,
  destinationFor,
  headerLine,
  isNarrowed,
  kindLabel,
  noNarrowing,
  NO_CLASS,
  reviewChip,
  shownRulings,
  tabs,
  waitingOf,
  whenLine,
  wayThrough,
  withdrawable,
  YOURS,
  type Narrowing,
  type TabId,
} from "./decisions";
import { QueueBlock, useNarrowing } from "./QueueBlock";
import { ActSheet, type Request } from "../backlog/ActSheet";
import { loadBacklog, unparkGoal, type Backlog } from "../backlog/api";
import { minuteTime } from "../backlog/format";
import { Help } from "../help/Help";
import { useNotifications } from "../notifications/store";
import { Pane } from "../panes/Pane";
import { Tabs } from "../panes/Tabs";
import type { HelpId } from "../help/terms";
import { goalPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useSession } from "../shell/identity";
import { useOffersRefresh } from "../shell/refresh";

/**
 * Decisions: what needs your choice, and what you decided.
 *
 * Two blocks and no more. The first is one complete list of everything waiting
 * on a human — not a capped group, because a human deciding what to do next
 * needs all of it — with every row saying what is asked, who asks, since when,
 * what happens if they do nothing, and the asker's recommendation where the
 * record carries one. The second is what this human has already said, four
 * ways, with their own rulings rendered whole in their own words.
 *
 * Two acts, and they are the board's. Approve and Withdraw open the board's
 * own sheet, over the whole backlog payload it prefills from, which this page
 * loads when the sheet opens rather than on every read. Everything else names
 * where the decision is made — the record's page, the goal's page, the
 * register, or the terminal command — in the words the engine uses.
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

export function DecisionsPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [acting, setActing] = useState<Acting | null>(null);
  const [bulk, setBulk] = useState<Bulking | null>(null);
  const [tab, setTab] = useState<TabId>("rulings");
  // The queue's own state, all of it this visit's: what is narrowed, what is
  // ticked and what is open. None of it is stored — see QueueBlock.
  const [narrowing, setNarrowing] = useNarrowing();
  const [selected, setSelected] = useState<string[]>([]);
  const [opened, setOpened] = useState<string[]>([]);

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
    setAttempt((previous) => previous + 1);
  }, []);

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

  // Return to queue is one goal and one publication, so it needs nothing but
  // the id: it sends unpark and the page reads its payload again, which is
  // what says the goal came back.
  const [returning, setReturning] = useState("");
  const returnToQueue = useCallback(
    (id: string) => {
      setReturning("");
      unparkGoal(id)
        .then(() => {
          reload();
        })
        .catch((error: unknown) => {
          setReturning(failureMessage(error));
        });
    },
    [reload],
  );

  // A row's act: the single-goal Approve keeps the board's own sheet, and the
  // single-goal Not now is the not-now sheet with one goal in it.
  const actOnRow = useCallback(
    (need: Need, act: "approve" | "park") => {
      if (act === "park") {
        openBulk({ act: "park", goals: [need] });
        return;
      }
      if (need.row !== null) {
        open({ move: "approve", goal: need.row });
      }
    },
    [open, openBulk],
  );

  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  useOffersRefresh(reload, hint);

  // What the Partner is given: the inbox rows on screen — their kind, their
  // id and what they ask — and which tab of what was decided is open. A
  // ruling's own words never travel: the register is a document of the
  // checkout, and the Partner reads it there.
  const records = useMemo(
    () => (read.state === "read" ? read.page.needsYou.map((need) => `${need.kind} · ${need.id} · ${need.asked}`) : []),
    [read],
  );
  // What the capture adds now that the page has two blocks and a queue a
  // human works: which block they are in, what is narrowing it, and how many
  // rows they have ticked. A Partner asked "what am I looking at" answers
  // from these three and not from a guess about the whole payload.
  const filters = useMemo(() => captureLines(narrowing, selected.length, opened.length), [narrowing, selected, opened]);
  useAbout(aboutLine("Decisions", tab), { tab, records, filters, returnTo: "/decisions" });

  return (
    <Pane title="Decisions">
      {read.state === "loading" && <Loading />}
      {read.state === "failed" && <Failure message={read.message} onRetry={reload} />}
      {read.state === "read" && (
        <Blocks
          page={read.page}
          tab={tab}
          onTab={setTab}
          onAct={open}
          onRowAct={actOnRow}
          onBulk={(act, goals) => {
            openBulk({ act, goals });
          }}
          onReturn={returnToQueue}
          narrowing={narrowing}
          onNarrow={setNarrowing}
          selected={selected}
          onSelect={setSelected}
          opened={opened}
          onOpen={setOpened}
        />
      )}
      {returning !== "" && (
        <p className="ms-decisions-refusal" role="alert">
          {returning}
        </p>
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
    </Pane>
  );
}

/**
 * The two blocks, over one payload.
 *
 * They are exported apart from the pane so that a test can render what the
 * page says about a payload without a server behind it: the reading, the
 * network and the sheet are the pane's, and what the two blocks put on the
 * screen is this.
 */
export function Blocks({
  page,
  tab = "rulings",
  onTab = () => undefined,
  onAct = () => undefined,
  onRowAct = () => undefined,
  onBulk = () => undefined,
  onReturn = () => undefined,
  narrowing = noNarrowing,
  onNarrow = () => undefined,
  selected = [],
  onSelect = () => undefined,
  opened = [],
  onOpen = () => undefined,
}: {
  page: DecisionsPayload;
  tab?: TabId;
  onTab?: (id: TabId) => void;
  onAct?: (request: Request) => void;
  onRowAct?: (need: Need, act: "approve" | "park") => void;
  onBulk?: (act: "approve" | "park", goals: Need[]) => void;
  onReturn?: (id: string) => void;
  narrowing?: Narrowing;
  onNarrow?: (next: Narrowing) => void;
  selected?: readonly string[];
  onSelect?: (ids: string[]) => void;
  opened?: readonly string[];
  onOpen?: (ids: string[]) => void;
}) {
  const now = new Date();
  const asked = askedOf(page.needsYou);
  const waiting = waitingOf(page.needsYou);
  return (
    <div className="ms-decisions">
      <p className="ms-decisions-header">
        <a className="ms-decisions-header-link" href="#decisions-asked">
          {String(page.counts.asked)} asked of you
        </a>
        <span className="ms-decisions-header-dot">·</span>
        <a className="ms-decisions-header-link" href="#decisions-waiting">
          {String(page.counts.waiting)} waiting for your approval
        </a>
        <span className="ms-visually-hidden">{headerLine(page.counts)}</span>
      </p>
      <AskedOf page={page} asked={asked} now={now} onAct={onAct} onReturn={onReturn} />
      <QueueBlock
        waiting={waiting}
        signedIn={!page.signIn}
        narrowing={narrowing}
        onNarrow={onNarrow}
        selected={selected}
        onSelect={onSelect}
        opened={opened}
        onOpen={onOpen}
        onAct={onRowAct}
        onBulk={onBulk}
        now={now}
      />
      <Decided page={page} tab={tab} onTab={onTab} onAct={onAct} onReturn={onReturn} />
    </div>
  );
}

/**
 * The capture's own lines: which block is being worked, what is narrowing it
 * and how much is ticked. They are the page's state rather than its payload,
 * which is why nothing else can answer them.
 */
function captureLines(narrowing: Narrowing, selected: number, opened: number): string[] {
  const lines = [`queue order: ${narrowing.order === "newest" ? "newest first" : "backlog order"}`];
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
  lines.push(`${String(selected)} selected`, `${String(opened)} rows open`);
  return lines;
}

/* ---------------------------------------------------- needs your choice -- */

function AskedOf({
  page,
  asked,
  now,
  onAct,
  onReturn,
}: {
  page: DecisionsPayload;
  asked: Need[];
  now: Date;
  onAct: (request: Request) => void;
  onReturn: (id: string) => void;
}) {
  const { askToSignIn } = useSession();
  return (
    <section className="ms-decisions-block" id="decisions-asked">
      <div className="ms-decisions-head">
        <h2 className="ms-decisions-title">Asked of you</h2>
        <Help id="asked-of-you" />
        {/* Two terms, because the block has two ideas in it: what belongs
            here at all, and the sentence every row ends with. */}
        <Help id="silence" />
        <span className="ms-decisions-count">{page.counts.asked}</span>
      </div>
      {page.signIn && (
        <div className="ms-decisions-signin">
          <button
            type="button"
            className="ms-decisions-signin-button"
            onClick={() => {
              askToSignIn();
            }}
          >
            Sign in to act
          </button>
          <span className="ms-decisions-signin-words">Nothing proves a human on this seat yet.</span>
        </div>
      )}
      {asked.length === 0 ? (
        <p className="ms-decisions-calm">
          <CircleCheck className="ms-decisions-calm-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
          Nothing is asked of you.
        </p>
      ) : (
        <ul className="ms-decisions-inbox">
          {asked.map((need) => (
            <NeedCard
              key={`${need.kind}-${need.id}`}
              need={need}
              now={now}
              signedIn={!page.signIn}
              onAct={onAct}
              onReturn={onReturn}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * One thing waiting on a human: the kind, the title, what is asked, the muted
 * line that says who asked and what silence does, the recommendation where
 * there is one, and on the right the act, the link, or the command.
 */
function NeedCard({
  need,
  now,
  signedIn,
  onAct,
  onReturn,
}: {
  need: Need;
  now: Date;
  signedIn: boolean;
  onAct: (request: Request) => void;
  onReturn: (id: string) => void;
}) {
  const row = need.row;
  return (
    <li className="ms-decisions-need">
      <div className="ms-decisions-need-body">
        <p className="ms-decisions-need-head">
          <Chip>{kindLabel(need.kind)}</Chip>
          <span className="ms-decisions-need-title">{need.title === "" ? need.id : need.title}</span>
        </p>
        <p className="ms-decisions-need-asked">{need.asked}</p>
        <p className="ms-decisions-need-muted">{askedLine(need, now)}</p>
        {need.recommend !== "" && (
          <p className="ms-decisions-need-recommend">Recommended: {need.recommend}</p>
        )}
      </div>
      <div className="ms-decisions-need-way">
        {need.act === "unpark" ? (
          // A seat's park. A human has not seen it, so it is still asked of
          // them — and the way out of it is the same button the Not now tab
          // carries, rather than a command to copy into a terminal.
          <Button
            disabled={!signedIn}
            onClick={() => {
              onReturn(need.id);
            }}
          >
            Return to queue
          </Button>
        ) : need.act !== "" && row !== null ? (
          <Button
            primary
            onClick={() => {
              onAct({ move: need.act === "approve" ? "approve" : "withdraw", goal: row });
            }}
          >
            {actLabel(need.act)}
          </Button>
        ) : (
          <Way where={need.where} />
        )}
        {need.command !== "" && <code className="ms-mono ms-decisions-command">{need.command}</code>}
      </div>
    </li>
  );
}

/**
 * Where a row's decision is made, where it is not an act of this interface: a
 * link where the destination is an address, a button where it is a panel, and
 * plain words where this build has no surface for it — which is the master's
 * rule for an unresolved reference, rather than a link that would refuse.
 */
function Way({ where }: { where: Reference }) {
  const { openPanel } = useNotifications();
  const destination = destinationFor(where);
  const words = wayThrough(where);
  if (words === "") {
    return null;
  }
  if (destination.kind === "link") {
    return (
      <NavLink className="ms-decisions-way" to={destination.to}>
        {words}
      </NavLink>
    );
  }
  if (destination.kind === "notifications") {
    return (
      <button
        type="button"
        className="ms-decisions-way"
        onClick={() => {
          openPanel(destination.at === "" ? undefined : destination.at);
        }}
      >
        {words}
      </button>
    );
  }
  return <span className="ms-decisions-way ms-decisions-way--words">{words}</span>;
}

/* ------------------------------------------------------------- decided -- */

/**
 * The term beside each tab's name. Two of the four are words this project
 * uses in its own way; the other two are ordinary English and carry none.
 */
const TAB_TERMS: Readonly<Partial<Record<TabId, HelpId>>> = {
  rulings: "ruling",
  decisions: "decisions",
  "not-now": "not-now",
};

function Decided({
  page,
  tab,
  onTab,
  onAct,
  onReturn,
}: {
  page: DecisionsPayload;
  tab: TabId;
  onTab: (id: TabId) => void;
  onAct: (request: Request) => void;
  onReturn: (id: string) => void;
}) {
  const decided = page.decided;
  const strip = tabs(page);
  const panels: Record<TabId, ReactNode> = {
    rulings: <Rulings rulings={decided.rulings} defects={decided.defects} />,
    decisions: <Items items={decided.decisions} empty="This project has recorded no decisions yet." />,
    answered: <Items items={decided.answered} empty="No question of the register has been answered yet." />,
    approved: <Approvals approved={decided.approved} onAct={onAct} />,
    "not-now": <NotNowList parks={decided.notNow} signedIn={!page.signIn} onReturn={onReturn} />,
  };
  return (
    <section className="ms-decisions-block" id="decisions-decided">
      <div className="ms-decisions-head">
        {/* The section's own term is in the header, beside the section name,
            so it is not repeated here. The two terms this block needs — what
            a ruling is, and what a review condition is — are on the tab and
            beside the control that narrows by one. */}
        <h2 className="ms-decisions-title">Decided</h2>
      </div>
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
    </section>
  );
}

function Rulings({ rulings, defects }: { rulings: Ruling[]; defects: string[] }) {
  const [find, setFind] = useState("");
  const [narrowed, setNarrowed] = useState<string>(ANY_CLASS);
  const classes = useMemo(() => classesIn(rulings), [rulings]);
  const shown = useMemo(() => shownRulings(rulings, find, narrowed), [rulings, find, narrowed]);
  const broken = defectLine(defects);
  const now = new Date();
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
 * goals the words name as chips that open them.
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
          <NavLink key={mention} className="ms-decisions-mention" to={goalPath(mention)}>
            {mention}
          </NavLink>
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
}: {
  parks: NotNow[];
  signedIn: boolean;
  onReturn: (id: string) => void;
}) {
  const now = new Date();
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

function Items({ items, empty }: { items: Item[]; empty: string }) {
  const now = new Date();
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

function Approvals({ approved, onAct }: { approved: Approved[]; onAct: (request: Request) => void }) {
  const now = new Date();
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
      {[0, 1].map((block) => (
        <section key={block} className="ms-decisions-block">
          <Skeleton />
        </section>
      ))}
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
