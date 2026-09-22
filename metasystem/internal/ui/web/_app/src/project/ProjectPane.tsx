import { useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink, useParams } from "react-router";

import { failureMessage, loadPane, type DocumentFile, type Pane as PanePayload, type Problem } from "./api";
import {
  areaRows,
  briefingFor,
  documentGroups,
  type AreaBook,
  type BookBriefing,
  type Briefing,
  type DesignRuns,
  type DocumentGroup,
  type Row,
} from "./pane";
import "./reading.css";
import { Pane } from "../panes/Pane";
import { documentPath } from "../routes";
import { Button, Chip, Skeleton } from "../shell/controls";

/**
 * Project: a briefing on what this project is, not a list of its files.
 *
 * The left column is the area tree. The main column opens each kind in its
 * own words — the intent index's first paragraph, the doctrine's, a design's —
 * and only then offers the links: the books as tables of contents, the
 * decisions and designs as rows carrying their summaries, the designs grouped
 * so that what governs is open and what is finished is one collapsed run. The
 * right column is what is waiting and what was read.
 *
 * Nothing is inferred from a filename, and nothing that is refused is hidden:
 * what the check verb would print is at the top, where a human can act on it.
 */

type PaneState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; pane: PanePayload };

export function ProjectPane() {
  const parameters = useParams<{ slug?: string }>();
  const slug = parameters.slug ?? null;
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
    <Pane title="Project">
      {read.state === "loading" && <LoadingCards />}
      {read.state === "failed" && <FailureCard message={read.message} onRetry={reload} />}
      {read.state === "read" && <Columns pane={read.pane} slug={slug} onReload={reload} />}
    </Pane>
  );
}

function Columns({ pane, slug, onReload }: { pane: PanePayload; slug: string | null; onReload: () => void }) {
  const areas = useMemo(() => areaRows(pane), [pane]);
  const briefing = useMemo(() => briefingFor(pane, slug), [pane, slug]);
  const groups = useMemo(() => documentGroups(pane.documents), [pane]);

  return (
    <div className="ms-briefing">
      <nav className="ms-project-areas" aria-label="Areas">
        {areas.map((area) => (
          <NavLink key={area.slug ?? "project"} className="ms-project-area" to={area.to} end>
            <span className="ms-project-area-name">{area.name}</span>
            <span className="ms-project-area-count">{area.count}</span>
          </NavLink>
        ))}
      </nav>
      <div className="ms-briefing-main">
        <p className="ms-project-read-at">
          Read at {timeOf(pane.readAt)}
          <Button onClick={onReload}>Reload</Button>
        </p>
        {pane.problems.length > 0 && <Problems problems={pane.problems} />}
        {briefing.area !== null && <AreaBlock briefing={briefing} />}
        {briefing.books.map((book) => (
          <BookBlock key={book.id} book={book} />
        ))}
        {briefing.areaBooks.map((book) => (
          <AreaBookBlock key={book.id} book={book} />
        ))}
        <Block id="decisions" title="Decisions" count={briefing.decisions.length}>
          {briefing.decisions.length === 0 ? (
            <p className="ms-project-none">Nothing recorded yet.</p>
          ) : (
            <Rows rows={briefing.decisions} />
          )}
        </Block>
        <Designs designs={briefing.designs} />
        <Block id="questions" title="Open questions" count={briefing.questions.length}>
          {briefing.questions.length === 0 ? (
            <p className="ms-project-none">Nothing recorded yet.</p>
          ) : (
            <Rows rows={briefing.questions} />
          )}
        </Block>
        <Documents groups={groups} total={pane.documents.length} />
      </div>
      <aside className="ms-briefing-aside" aria-label="Beside the briefing">
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
            {count(briefing.checkout.areas, "area")} · {count(briefing.checkout.problems, "problem")}
          </p>
          <p className="ms-briefing-note-line">Read at {timeOf(pane.readAt)}</p>
        </section>
      </aside>
    </div>
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
        {action !== undefined && <span className="ms-briefing-act">{action}</span>}
      </div>
      {children}
    </section>
  );
}

/** The area page opens by saying which area this is, in the index's own word. */
function AreaBlock({ briefing }: { briefing: Briefing }) {
  const area = briefing.area;
  if (area === null) {
    return null;
  }
  return (
    <Block id="area" title={area.name} count={area.count}>
      {area.declared ? (
        <p className="ms-briefing-lede">
          One of the areas the intent index declares, as <span className="ms-mono">{area.slug}</span>. Everything
          below names it.
        </p>
      ) : (
        <p className="ms-project-reason">No area named {area.slug} is declared by the intent index.</p>
      )}
    </Block>
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

/**
 * A book under one area: only the chapters that name it, or one honest line
 * saying there are none and that the project-wide book applies. A project-wide
 * book is not repeated under every area.
 */
function AreaBookBlock({ book }: { book: AreaBook }) {
  return (
    <Block id={book.id} title={book.title} count={book.rows.length}>
      {book.rows.length === 0 ? <p className="ms-project-none">{book.none}</p> : <Rows rows={book.rows} />}
    </Block>
  );
}

/** The designs: what governs, open; what is finished, in one run each. */
function Designs({ designs }: { designs: DesignRuns }) {
  const total = designs.open.length + designs.runs.reduce((sum, run) => sum + run.rows.length, 0);
  return (
    <Block id="designs" title="Designs" count={total}>
      {total === 0 && <p className="ms-project-none">Nothing recorded yet.</p>}
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

function Rows({ rows }: { rows: Row[] }) {
  return (
    <ul className="ms-project-rows">
      {rows.map((row) => (
        <RecordRow key={row.key} row={row} />
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
function RecordRow({ row }: { row: Row }) {
  return (
    <li className="ms-project-row">
      <span className="ms-project-row-title">
        {row.to === null ? row.title : <NavLink to={row.to}>{row.title}</NavLink>}
      </span>
      <span className="ms-project-row-status">{row.status !== "" && <Chip>{row.status}</Chip>}</span>
      <span className="ms-project-row-areas">
        {row.areas.map((area) => (
          <Chip key={area}>{area}</Chip>
        ))}
      </span>
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
    <Block id="documents" title="Documents" note={`${String(total)} files, no kind claimed`}>
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
      <span className="ms-project-row-areas" />
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
