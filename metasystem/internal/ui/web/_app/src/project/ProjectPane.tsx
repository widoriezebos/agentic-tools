import { useEffect, useMemo, useState, type ReactNode } from "react";
import { NavLink, useParams } from "react-router";

import { failureMessage, loadPane, type DocumentFile, type Pane as PanePayload, type Problem } from "./api";
import {
  areaRows,
  documentGroups,
  sectionsFor,
  type DocumentGroup,
  type Row,
  type Section as PaneSection,
} from "./pane";
import "./reading.css";
import { Pane } from "../panes/Pane";
import { documentPath } from "../routes";
import { Button, Chip, Skeleton } from "../shell/controls";

/**
 * Project: what the project's records declare, and the rest of its Markdown
 * by path.
 *
 * The left column is the area tree: the whole project, then each area the
 * intent index declares, with the number of records that name it. The main
 * column is the selection, in reading order: intent, doctrine, decisions,
 * designs, open questions, and then the documents that declare nothing, which
 * are listed by path with no kind claimed.
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
  const sections = useMemo(() => sectionsFor(pane, slug), [pane, slug]);
  const groups = useMemo(() => documentGroups(pane.documents), [pane]);
  const declared = slug === null || pane.areas.some((area) => area.slug === slug);

  return (
    <div className="ms-project">
      <nav className="ms-project-areas" aria-label="Areas">
        {areas.map((area) => (
          <NavLink key={area.slug ?? "project"} className="ms-project-area" to={area.to} end>
            <span className="ms-project-area-name">{area.name}</span>
            <span className="ms-project-area-count">{area.count}</span>
          </NavLink>
        ))}
      </nav>
      <div className="ms-project-main">
        <p className="ms-project-read-at">
          Read at {timeOf(pane.readAt)}
          <Button onClick={onReload}>Reload</Button>
        </p>
        {pane.problems.length > 0 && <Problems problems={pane.problems} />}
        {!declared && (
          <p className="ms-project-reason">No area named {slug} is declared by the intent index.</p>
        )}
        {sections.map((section) => (
          <SectionCard key={section.id} section={section} />
        ))}
        <Documents groups={groups} total={pane.documents.length} />
      </div>
    </div>
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
    <section className="ms-card">
      <h2 className="ms-card-title">
        Refused by the records themselves
        <Count of={problems.length} />
      </h2>
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

function SectionCard({ section }: { section: PaneSection }) {
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">
        {section.title}
        {section.rows.length > 0 && <Count of={section.rows.length} />}
      </h2>
      {section.rows.length === 0 ? (
        <p className="ms-project-none">Nothing recorded</p>
      ) : (
        <ul className="ms-project-rows">
          {section.rows.map((row) => (
            <RecordRow key={row.key} row={row} />
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * One row: the title, what it declares, and where it lives. The path is one
 * line, clipped at its end, and carries the whole of itself in a tooltip, so a
 * long path never breaks the row across the screen a character at a time.
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
    <section className="ms-card">
      <h2 className="ms-card-title">
        Documents
        {total > 0 && <Count of={total} />}
      </h2>
      {groups.length === 0 ? (
        <p className="ms-project-none">Nothing recorded</p>
      ) : (
        <>
          <p className="ms-project-note">Every other Markdown file in this checkout, by path. No kind is claimed.</p>
          {groups.map((group) => (
            <DocumentRun key={group.id} group={group} />
          ))}
        </>
      )}
    </section>
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
        <Count of={group.files.length} />
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

function Count({ of }: { of: number }): ReactNode {
  return <span className="ms-project-count">{of}</span>;
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
