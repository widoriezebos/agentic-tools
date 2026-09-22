import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { NavLink, useNavigate, useParams } from "react-router";

import { failureMessage, loadPane, type DocumentFile, type Pane as PanePayload, type Problem } from "./api";
import { useReadingRow } from "./outline";
import {
  briefingFor,
  documentGroups,
  DOCUMENTS_TITLE,
  kindTitle,
  pageSections,
  QUESTIONS_TITLE,
  type BookBriefing,
  type Briefing,
  type DesignRuns,
  type DocumentGroup,
  type PageSection,
  type Row,
} from "./pane";
import "./reading.css";
import { Sheet, type Done, type Request } from "./Sheet";
import { ACTIONS, type Kind } from "./writing";
import { Pane } from "../panes/Pane";
import { documentPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";

/**
 * Project: a briefing on what this project is, not a list of its files.
 *
 * The strip under the header is what is on this page — the books, the
 * decisions, the designs, the open questions, the documents — as anchors, with
 * the one being read marked as the human scrolls. The ledger's goals are not
 * in it: a goal belongs to the Backlog, and a rail of six hundred of them said
 * that the project's records were a subdivision of the ledger rather than the
 * other way round. The main column opens each kind in its own words — the intent index's
 * first paragraph, the doctrine's, a design's — and only then offers the
 * links: the books as tables of contents, the decisions and designs as rows
 * carrying their summaries, the designs grouped so that what governs is open
 * and what is finished is one collapsed run, in the width the outline used to
 * take. The right column is what is waiting and what was read.
 *
 * One goal of the ledger is the same page scoped to it, and it lives under
 * Backlog: the same briefing, narrowed to the records whose Goals name it,
 * opening with the ledger's own reason for the goal.
 *
 * Nothing is inferred from a filename, and nothing that is refused is hidden:
 * what the check verb would print is at the top, where a human can act on it.
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

  const reload = () => {
    setRead({ state: "loading" });
    setAttempt((previous) => previous + 1);
  };

  return (
    <Pane title={goal === null ? "Project" : "Backlog"}>
      {read.state === "loading" && <LoadingCards />}
      {read.state === "failed" && <FailureCard message={read.message} onRetry={reload} />}
      {read.state === "read" && <Columns pane={read.pane} goal={goal} onReload={reload} />}
    </Pane>
  );
}

function Columns({ pane, goal, onReload }: { pane: PanePayload; goal: string | null; onReload: () => void }) {
  const briefing = useMemo(() => briefingFor(pane, goal), [pane, goal]);
  const sections = useMemo(() => pageSections(briefing), [briefing]);
  const groups = useMemo(() => documentGroups(pane.documents), [pane]);
  const [sheet, setSheet] = useState<Request | null>(null);
  const navigate = useNavigate();

  const where = goal ?? "Everything";
  const page = goal === null ? "Project" : `Backlog · ${goal}`;
  const current = useReadingRow(sections, goal ?? "");

  // What the drawer says this page is about: the page, and the section of it
  // being read, from the same outline the tabs mark.
  const here = sections.find((row) => row.id === current);
  useAbout(aboutLine(page, here?.title ?? ""));

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

  // The same four actions appear in three places — the aside, a block's head,
  // and an empty block's one line — so each is written once and used where it
  // belongs. On a goal page they carry that goal, already chosen.
  const openRecord = (kind: Kind) => () => {
    setSheet({ mode: "record", kind, goal });
  };
  const openQuestion = () => {
    setSheet({ mode: "question", goal });
  };
  const recordAct = (kind: Kind) => <Act label={ACTIONS[kind].offer} onAct={openRecord(kind)} />;
  const questionAct = <Act label="Ask a question" onAct={openQuestion} />;

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
      <PageTabs sections={sections} current={current} />
      <div className="ms-briefing">
        <div className="ms-briefing-main">
          <p className="ms-project-read-at">
            Read at {timeOf(pane.readAt)}
            <Button onClick={onReload}>Reload</Button>
          </p>
          {pane.problems.length > 0 && <Problems problems={pane.problems} />}
          {briefing.goal !== null && <GoalBlock briefing={briefing} />}
          {briefing.books.map((book) => (
            <BookBlock key={book.id} book={book} />
          ))}
          <Block
            id="decisions"
            title={kindTitle("decision")}
            count={briefing.decisions.length}
            action={recordAct("decision")}
          >
            {briefing.decisions.length === 0 ? (
              <Nothing>{recordAct("decision")}</Nothing>
            ) : (
              <Rows rows={briefing.decisions} />
            )}
          </Block>
          <Designs designs={briefing.designs} action={recordAct("design")} />
          <Block id="questions" title={QUESTIONS_TITLE} count={briefing.questions.length} action={questionAct}>
            {briefing.questions.length === 0 ? (
              <Nothing>{questionAct}</Nothing>
            ) : (
              <Rows rows={briefing.questions} action={answer} />
            )}
          </Block>
          {goal === null && <Documents groups={groups} total={pane.documents.length} />}
        </div>
        <aside className="ms-briefing-aside" aria-label="Beside the briefing">
          <section className="ms-briefing-note">
            <h2 className="ms-briefing-note-title">{goal === null ? "Contribute" : `Contribute to ${where}`}</h2>
            <div className="ms-briefing-contribute">
              {/* An intent chapter needs a title and nothing else; it joins the
                  index's reading order wherever it was written from. */}
              <Button onClick={openRecord("intent")}>{ACTIONS.intent.offer}</Button>
              <Button onClick={openRecord("decision")}>{ACTIONS.decision.offer}</Button>
              <Button onClick={openRecord("design")}>{ACTIONS.design.offer}</Button>
              <Button onClick={openQuestion}>Ask a question</Button>
            </div>
            <p className="ms-briefing-note-line">
              Each writes a draft in its own home with a fresh id and{" "}
              <span className="ms-mono">Status: draft</span>, and opens it. Nothing here accepts anything; status is
              yours to change on the record.
            </p>
          </section>
          <section className="ms-briefing-note">
            <h2 className="ms-briefing-note-title">Needs you</h2>
            {briefing.needsYou.questions === 0 && briefing.needsYou.designs === 0 ? (
              <p className="ms-briefing-note-line">Nothing is waiting here.</p>
            ) : (
              <ul className="ms-briefing-note-list">
                {briefing.needsYou.questions > 0 && (
                  <li>
                    <a href="#questions">{count(briefing.needsYou.questions, "open question")}</a>
                  </li>
                )}
                {briefing.needsYou.designs > 0 && (
                  <li>
                    <a href="#designs">{count(briefing.needsYou.designs, "design")} accepted, not yet built</a>
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
            goals={pane.goals}
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
 * What is on this page, and where in it the human is: a strip of tabs under
 * the header, above the reading rather than beside it.
 *
 * It was a column, and a column of six rows cost the reading a fifth of the
 * width to say six words. Across the top it costs one line, and the main
 * column takes the width back. Every tab is an anchor to a section of this
 * same page, so the strip moves the page rather than navigating; it stays
 * under the header as the page scrolls, and the mark follows the reader the
 * way the document reader's outline does, through the same observer and with
 * no timer. Narrower than its tabs, the strip scrolls sideways rather than
 * wrapping into a block that would push the reading down the page.
 */
function PageTabs({ sections, current }: { sections: PageSection[]; current: string | null }) {
  return (
    <nav className="ms-page-tabs" aria-label="On this page">
      <ul className="ms-page-tabs-list">
        {sections.map((section) => (
          <li key={section.id} className="ms-page-tab">
            <a href={`#${section.id}`} aria-current={section.id === current ? "location" : undefined}>
              {section.title}
            </a>
          </li>
        ))}
      </ul>
    </nav>
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

/** An empty section says so, and says what to do about it, on one line. */
function Nothing({ children }: { children: ReactNode }) {
  return (
    <p className="ms-project-none">
      Nothing recorded yet. {children}
    </p>
  );
}

/** A count and the word for it, pluralised the one way English needs here. */
function count(of: number, word: string): string {
  return `${String(of)} ${word}${of === 1 ? "" : "s"}`;
}

/** One block of the briefing: a heading, a quiet count, and one action. */
function Block({
  id,
  title,
  count: total,
  note,
  action,
  children,
}: {
  id: string;
  title: string;
  count?: number;
  note?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="ms-briefing-block" id={id}>
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
 */
function GoalBlock({ briefing }: { briefing: Briefing }) {
  const goal = briefing.goal;
  if (goal === null) {
    return null;
  }
  return (
    <section className="ms-briefing-block" id="goal">
      <p className="ms-facts-eyebrow ms-mono">{goal.id}</p>
      <div className="ms-briefing-head">
        <h2 className="ms-briefing-title">{goal.title}</h2>
        {goal.state !== "" && <Chip>{goal.state}</Chip>}
        <span className="ms-project-count">{goal.count}</span>
        <span className="ms-briefing-act">
          <NavLink className="ms-briefing-link" to="/backlog">
            Open in the Backlog list →
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
function BookBlock({ book }: { book: BookBriefing }) {
  return (
    <Block
      id={book.id}
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
        <p className="ms-project-none">Nothing recorded yet.</p>
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
function Designs({ designs, action }: { designs: DesignRuns; action: ReactNode }) {
  const total = designs.open.length + designs.runs.reduce((sum, run) => sum + run.rows.length, 0);
  return (
    <Block id="designs" title={kindTitle("design")} count={total} action={action}>
      {total === 0 && <Nothing>{action}</Nothing>}
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
  return (
    <li className="ms-project-row">
      <span className="ms-project-row-title">
        {row.to === null ? row.title : <NavLink to={row.to}>{row.title}</NavLink>}
      </span>
      <span className="ms-project-row-status">{row.status !== "" && <Chip>{row.status}</Chip>}</span>
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
    <Block id="documents" title={DOCUMENTS_TITLE} note={`${String(total)} files, no kind claimed`}>
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
