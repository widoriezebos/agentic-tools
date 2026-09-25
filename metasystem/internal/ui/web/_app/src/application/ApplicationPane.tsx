import { useCallback, useEffect, useMemo, useState } from "react";
import { NavLink } from "react-router";

import { failureMessage, loadApplication, type Landed, type Page as ApplicationPayload, type Problem } from "./api";
import "./application.css";
import {
  CHIPS_SHOWN,
  concludedLine,
  countLine,
  dayOf,
  engineLine,
  firstSentence,
  isNarrowed,
  labelsIn,
  modeWords,
  noNarrowing,
  shown,
  shownCount,
  statusWord,
  subjectLine,
  unreadLine,
  weeksOf,
  WEEKS_SHOWN,
  freshLine,
  type Narrowing,
  type Week,
} from "./application";
import { minuteTime } from "../backlog/format";
import { Help } from "../help/Help";
import { Pane } from "../panes/Pane";
import { documentPath, goalPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useOffersRefresh } from "../shell/refresh";

/**
 * Application: what this workspace has concluded, what is known to be wrong
 * with it, and what it says it is.
 *
 * Three blocks in the order of the jobs a human arrives with. What concluded
 * is the work history — the ledger's own conclusions, by week, newest first —
 * and it is named that rather than "what it does", because a conclusion dates
 * an end and some of those ends are administrative. Known problems is the
 * register, open rows first, with the rows the reader could not read counted
 * rather than dropped. What it is, is one line of links into the reader.
 *
 * Nothing here acts. The page reads, and every act belongs to the goal's own
 * page or to the document reader. The read is made when the pane mounts and
 * again when a human presses the refresh in the header's section cluster.
 * There is no timer, nothing polls, and nothing refetches on a window event.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; page: ApplicationPayload };

export function ApplicationPane() {
  const [read, setRead] = useState<PaneState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  // A sitting's state, and nothing is remembered between visits: a human who
  // came back tomorrow to a page still narrowed to one label would be reading
  // a history quietly missing most of itself.
  const [narrowing, setNarrowing] = useState<Narrowing>(noNarrowing);
  const [openRow, setOpenRow] = useState("");
  const [openProblem, setOpenProblem] = useState("");
  const [earlier, setEarlier] = useState(false);
  const [concluded, setConcluded] = useState(false);

  useEffect(() => {
    const aborter = new AbortController();
    loadApplication(aborter.signal)
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

  const hint = read.state === "read" ? `Read at ${minuteTime(read.page.readAt)} · Refresh` : "Refresh";
  useOffersRefresh(reload, hint);

  // What the Partner is given: the rows on screen — their id and the first
  // sentence of what concluded — and what the human has this page narrowed
  // to, which only this page knows.
  const records = useMemo(
    () =>
      read.state === "read"
        ? shown(read.page.landed, narrowing).map((row) => `${row.id} · ${firstSentence(row.concluded)}`)
        : [],
    [read, narrowing],
  );
  const filters = useMemo(
    () => captureLines(narrowing, openRow, openProblem, earlier, concluded),
    [narrowing, openRow, openProblem, earlier, concluded],
  );
  useAbout(aboutLine("Application", "what concluded"), {
    records,
    filters,
    returnTo: "/application",
  });

  return (
    <Pane title="Application">
      {read.state === "loading" && <Loading />}
      {read.state === "failed" && <Failure message={read.message} onRetry={reload} />}
      {read.state === "read" && (
        <Blocks
          page={read.page}
          narrowing={narrowing}
          onNarrow={setNarrowing}
          openRow={openRow}
          onOpenRow={setOpenRow}
          openProblem={openProblem}
          onOpenProblem={setOpenProblem}
          earlier={earlier}
          onEarlier={setEarlier}
          concluded={concluded}
          onConcluded={setConcluded}
        />
      )}
    </Pane>
  );
}

/**
 * The header and the three blocks, over one payload.
 *
 * They are exported apart from the pane so that a test can render what the
 * page says about a payload without a server behind it: the reading and the
 * network are the pane's, and what reaches the screen is this.
 */
export function Blocks({
  page,
  narrowing = noNarrowing,
  onNarrow = () => undefined,
  openRow = "",
  onOpenRow = () => undefined,
  openProblem = "",
  onOpenProblem = () => undefined,
  earlier = false,
  onEarlier = () => undefined,
  concluded = false,
  onConcluded = () => undefined,
}: {
  page: ApplicationPayload;
  narrowing?: Narrowing;
  onNarrow?: (next: Narrowing) => void;
  /** The one concluded row open, across every week, or "". */
  openRow?: string;
  onOpenRow?: (id: string) => void;
  /** The one problem open, or "". */
  openProblem?: string;
  onOpenProblem?: (id: string) => void;
  earlier?: boolean;
  onEarlier?: (next: boolean) => void;
  concluded?: boolean;
  onConcluded?: (next: boolean) => void;
}) {
  return (
    <div className="ms-application">
      <Header page={page} />
      <Concluded
        page={page}
        narrowing={narrowing}
        onNarrow={onNarrow}
        openRow={openRow}
        onOpenRow={onOpenRow}
        earlier={earlier}
        onEarlier={onEarlier}
      />
      <Problems
        page={page}
        open={openProblem}
        onOpen={onOpenProblem}
        concluded={concluded}
        onConcluded={onConcluded}
      />
      <WhatItIs page={page} />
    </div>
  );
}

/**
 * The header: which application this is, what this build of the engine is,
 * and how much has concluded.
 *
 * The subject and the mode are first because the whole page is about one of
 * them and not the other: in the self-hosted case the MetaSystem is the
 * product under development, and the instance doing the work is on Fleet.
 */
function Header({ page }: { page: ApplicationPayload }) {
  const words = modeWords(page);
  return (
    <header className="ms-application-header">
      <h2 className="ms-application-subject">{subjectLine(page)}</h2>
      {words !== "" && <p className="ms-application-mode">{words}</p>}
      <p className="ms-application-engine">
        <span>{engineLine(page)}</span>
        <Help id="last-engine" />
      </p>
      <p className="ms-application-counts">
        <span>{countLine(page)}</span>
        <Help id="concluded-new" />
      </p>
    </header>
  );
}

/* --------------------------------------------------------- what concluded -- */

function Concluded({
  page,
  narrowing,
  onNarrow,
  openRow,
  onOpenRow,
  earlier,
  onEarlier,
}: {
  page: ApplicationPayload;
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  openRow: string;
  onOpenRow: (id: string) => void;
  earlier: boolean;
  onEarlier: (next: boolean) => void;
}) {
  const rows = useMemo(() => shown(page.landed, narrowing), [page.landed, narrowing]);
  const labels = useMemo(() => labelsIn(rows), [rows]);
  const weeks = useMemo(() => weeksOf(rows), [rows]);
  const latest = earlier ? weeks : weeks.slice(0, WEEKS_SHOWN);
  const hidden = weeks.length - latest.length;
  return (
    <section className="ms-application-block">
      <h3 className="ms-application-block-title">
        What concluded
        <Help id="what-concluded" />
      </h3>
      <Tools
        narrowing={narrowing}
        onNarrow={onNarrow}
        labels={labels}
        count={shownCount(page.landed.length, rows.length)}
      />
      {weeks.length === 0 ? (
        <p className="ms-application-none">No concluded goal matches.</p>
      ) : (
        <ul className="ms-application-weeks">
          {latest.map((week) => (
            <WeekBlock key={week.id} week={week} openRow={openRow} onOpenRow={onOpenRow} />
          ))}
        </ul>
      )}
      {hidden > 0 && (
        <Button
          onClick={() => {
            onEarlier(true);
          }}
        >
          Show earlier
        </Button>
      )}
    </section>
  );
}

/** One week: the line that says which week and how many, and its rows. */
function WeekBlock({
  week,
  openRow,
  onOpenRow,
}: {
  week: Week;
  openRow: string;
  onOpenRow: (id: string) => void;
}) {
  const fresh = freshLine(week);
  return (
    <li className="ms-application-week">
      <p className="ms-application-week-line">
        <span className="ms-application-week-name">{week.title}</span>
        <span className="ms-application-week-count">{week.count}</span>
        {fresh !== "" && <span className="ms-application-week-new">{fresh}</span>}
      </p>
      <ul className="ms-application-rows">
        {week.rows.map((row) => (
          <LandedRow
            key={row.id}
            row={row}
            open={openRow === row.id}
            onOpen={() => {
              onOpenRow(openRow === row.id ? "" : row.id);
            }}
          />
        ))}
      </ul>
    </li>
  );
}

/**
 * One concluded goal: a line, and what is under it when it is open.
 *
 * The line is the intent, then the conclusion's first sentence muted, the
 * date at the right, and a dot where it concluded since the last visit here.
 * There is no button on a line and no id on one: an id fifty characters long
 * is a third of a line a human cannot read a history by.
 */
function LandedRow({ row, open, onOpen }: { row: Landed; open: boolean; onOpen: () => void }) {
  const day = dayOf(row.doneAt);
  return (
    <li className={open ? "ms-application-row-item ms-application-row-item--open" : "ms-application-row-item"}>
      <button type="button" className="ms-application-row-open" aria-expanded={open} onClick={onOpen}>
        <span className="ms-application-row-said">{row.intent === "" ? row.id : row.intent}</span>
        <span className="ms-application-row-note">{firstSentence(row.concluded)}</span>
        <span className="ms-application-row-facts">
          <span className="ms-application-row-when">{day === "" ? "no date" : day}</span>
          {row.new && (
            <>
              <span className="ms-application-dot" aria-hidden="true" />
              <span className="ms-visually-hidden">new</span>
            </>
          )}
        </span>
      </button>
      {open && (
        <div className="ms-application-open">
          <p className="ms-mono ms-application-open-id">{row.id}</p>
          <p className="ms-application-open-words">{row.concluded}</p>
          {(row.labels.length > 0 || row.arc !== "") && (
            <p className="ms-application-open-labels">
              {row.labels.map((label) => (
                <Chip key={label}>{label}</Chip>
              ))}
              {row.arc !== "" && <Chip marker>arc {row.arc}</Chip>}
            </p>
          )}
          <NavLink className="ms-application-way" to={goalPath(row.id)}>
            Open the goal
          </NavLink>
        </div>
      )}
    </li>
  );
}

/**
 * The two tools, on one line in the block's head: find, and narrow to one
 * family. One label at a time — a human reading a history is asking "what is
 * there of this kind", and two labels at once answers a question nobody asked.
 */
function Tools({
  narrowing,
  onNarrow,
  labels,
  count,
}: {
  narrowing: Narrowing;
  onNarrow: (next: Narrowing) => void;
  labels: { label: string; count: number }[];
  count: string;
}) {
  const [all, setAll] = useState(false);
  const chips = all ? labels : labels.slice(0, CHIPS_SHOWN);
  const more = labels.length - chips.length;
  return (
    <div className="ms-application-tools">
      <label className="ms-visually-hidden" htmlFor="ms-application-find">
        Find in what concluded
      </label>
      <input
        id="ms-application-find"
        type="search"
        className="ms-application-find-field"
        placeholder="Find in the id, the intent and the conclusion"
        value={narrowing.find}
        onChange={(event) => {
          onNarrow({ ...narrowing, find: event.target.value });
        }}
      />
      {chips.map((one) => (
        <ChipButton
          key={one.label}
          on={narrowing.label === one.label}
          onPress={() => {
            onNarrow({ ...narrowing, label: narrowing.label === one.label ? "" : one.label });
          }}
        >
          {one.label} {one.count}
        </ChipButton>
      ))}
      {more > 0 && (
        <ChipButton
          on={false}
          onPress={() => {
            setAll(true);
          }}
        >
          +{more}
        </ChipButton>
      )}
      {isNarrowed(narrowing) && (
        <Button
          onClick={() => {
            onNarrow(noNarrowing);
          }}
        >
          Clear
        </Button>
      )}
      <span className="ms-application-tools-count">{count}</span>
    </div>
  );
}

function ChipButton({
  on,
  onPress,
  children,
}: {
  on: boolean;
  onPress: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className={on ? "ms-application-chip ms-application-chip--on" : "ms-application-chip"}
      aria-pressed={on}
      onClick={onPress}
    >
      {children}
    </button>
  );
}

/* ---------------------------------------------------------- known problems -- */

/**
 * The register: open rows first, one line each, and the concluded ones behind
 * a disclosure.
 *
 * The line for the rows the reader could not read is not an apology. Five of
 * this register's open rows carry three or four cells today, and a block that
 * dropped them would say this project knows about fewer problems than it does.
 */
function Problems({
  page,
  open,
  onOpen,
  concluded,
  onConcluded,
}: {
  page: ApplicationPayload;
  open: string;
  onOpen: (id: string) => void;
  concluded: boolean;
  onConcluded: (next: boolean) => void;
}) {
  const register = page.problems;
  const unread = unreadLine(register.unread);
  return (
    <section className="ms-application-block">
      <h3 className="ms-application-block-title">
        Known problems
        <Help id="known-problems" />
      </h3>
      {register.open.length === 0 ? (
        <p className="ms-application-none">No open row in the register.</p>
      ) : (
        <ul className="ms-application-rows">
          {register.open.map((problem) => (
            <ProblemRow
              key={problem.id}
              problem={problem}
              open={open === problem.id}
              onOpen={() => {
                onOpen(open === problem.id ? "" : problem.id);
              }}
            />
          ))}
        </ul>
      )}
      {unread !== "" && (
        <p className="ms-application-unread">
          <span>{unread}</span>
          <NavLink className="ms-application-way" to={documentPath(register.register)}>
            Open the register
          </NavLink>
        </p>
      )}
      {register.concluded.length > 0 && !concluded && (
        <Button
          onClick={() => {
            onConcluded(true);
          }}
        >
          {concludedLine(register.concluded)}
        </Button>
      )}
      {register.concluded.length > 0 && concluded && (
        <ul className="ms-application-rows">
          {register.concluded.map((problem) => (
            <ProblemRow
              key={problem.id}
              problem={problem}
              open={open === problem.id}
              onOpen={() => {
                onOpen(open === problem.id ? "" : problem.id);
              }}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * One problem: the id, the symptom's first sentence and the date on a line,
 * and the cost, the lever and the whole status when it is open.
 *
 * A concluded row keeps its status word on the line, because an accepted
 * limitation still exists and "ACCEPTED" is a different thing to know about a
 * problem than "FIXED".
 */
function ProblemRow({ problem, open, onOpen }: { problem: Problem; open: boolean; onOpen: () => void }) {
  return (
    <li className={open ? "ms-application-row-item ms-application-row-item--open" : "ms-application-row-item"}>
      <button type="button" className="ms-application-row-open" aria-expanded={open} onClick={onOpen}>
        <span className="ms-mono ms-application-row-id">{problem.id}</span>
        <span className="ms-application-row-said">{firstSentence(problem.what)}</span>
        <span className="ms-application-row-facts">
          {!problem.open && <Chip>{statusWord(problem)}</Chip>}
          <span className="ms-application-row-when">{problem.date === "" ? "no date" : problem.date}</span>
        </span>
      </button>
      {open && (
        <div className="ms-application-open">
          <p className="ms-application-open-words">{problem.what}</p>
          {problem.consequence !== "" && (
            <p className="ms-application-open-fact">
              <span className="ms-application-open-name">Cost when it bites</span>
              {problem.consequence}
            </p>
          )}
          {problem.lever !== "" && (
            <p className="ms-application-open-fact">
              <span className="ms-application-open-name">Fix direction or lever</span>
              {problem.lever}
            </p>
          )}
          <p className="ms-application-open-fact">
            <span className="ms-application-open-name">Status</span>
            {problem.status}
          </p>
        </div>
      )}
    </li>
  );
}

/* ------------------------------------------------------------ what it is -- */

/**
 * One line of links, opened in the Project pane's document reader. Nothing is
 * rendered here: the reader renders documents, and there is one of it.
 */
function WhatItIs({ page }: { page: ApplicationPayload }) {
  return (
    <section className="ms-application-block">
      <h3 className="ms-application-block-title">What it is</h3>
      {page.docs.length === 0 ? (
        <p className="ms-application-none">This checkout carries none of the documents this block links.</p>
      ) : (
        <p className="ms-application-docs">
          {page.docs.map((doc) => (
            <NavLink key={doc.path} className="ms-application-way" to={documentPath(doc.path)}>
              {doc.title}
            </NavLink>
          ))}
        </p>
      )}
    </section>
  );
}

/* --------------------------------------------------------- the two states -- */

function Loading() {
  return (
    <div className="ms-application">
      <Skeleton />
    </div>
  );
}

function Failure({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="ms-pane-stack">
      <section className="ms-card">
        <h2 className="ms-card-title">Application could not be read</h2>
        <p className="ms-application-reason">{message}</p>
        <Button onClick={onRetry}>Retry</Button>
      </section>
    </div>
  );
}

/**
 * The capture's own lines: what a human has this page narrowed to and what
 * they have open. They are the page's state rather than its payload, which is
 * why nothing else can answer them.
 */
function captureLines(
  narrowing: Narrowing,
  openRow: string,
  openProblem: string,
  earlier: boolean,
  concluded: boolean,
): string[] {
  const lines = [`block open: what concluded${earlier ? " with the earlier weeks" : ""}`];
  if (narrowing.find.trim() !== "") {
    lines.push(`find: ${narrowing.find.trim()}`);
  }
  if (narrowing.label !== "") {
    lines.push(`label: ${narrowing.label}`);
  }
  if (!isNarrowed(narrowing)) {
    lines.push("nothing is narrowed");
  }
  if (openRow !== "") {
    lines.push(`row open: ${openRow}`);
  }
  if (openProblem !== "") {
    lines.push(`problem open: ${openProblem}`);
  }
  if (concluded) {
    lines.push("the concluded problems are shown");
  }
  return lines;
}
