import { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { NavLink, useLocation } from "react-router";

import {
  failureMessage,
  isNotFound,
  loadDocument,
  loadPane,
  type DocumentPayload,
  type Link as RecordLink,
  type Pane as PanePayload,
} from "./api";
import { Markdown } from "./Markdown";
import { currentRow, outlineOf, type OutlineRow } from "./outline";
import { crumbsFor, kindTitle, nameOf, railFor, shortID, type SiblingRail } from "./pane";
import { dateOf, ownership, timeOf } from "./ProjectPane";
import "./reading.css";
import { Pane } from "../panes/Pane";
import { documentIdFromPath, documentPath } from "../routes";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useWorkspaceState, type WorkspaceState } from "../shell/identity";
import { titleFor, type Identity } from "../title";

/**
 * One document, read as a chapter of a book rather than as a file.
 *
 * A record's head is a strip of facts at the top — what it is, where it
 * stands, what it rests on and what rests on it — and not four bullet lines in
 * the text, because a human reading a design is reading prose and the head is
 * not prose. The left rail is what this document is read among: the book's
 * chapters in reading order, or the records of its kind in its areas, with
 * previous and next at the foot. The right rail is the outline, and it marks
 * where the reader is as they scroll.
 *
 * A document that declares no head is answered exactly as it was before: the
 * same facts line, the same blocks, and no rails it has no siblings for.
 */

type DocumentState =
  | { state: "loading" }
  | { state: "absent" }
  | { state: "failed"; message: string }
  | { state: "read"; document: DocumentPayload };

export function DocumentPane() {
  const location = useLocation();
  const id = documentIdFromPath(location.pathname);
  const [document, setDocument] = useState<DocumentState>({ state: "loading" });
  const [pane, setPane] = useState<PanePayload | null>(null);
  const [attempt, setAttempt] = useState(0);
  const { workspace } = useWorkspaceState();

  useEffect(() => {
    const aborter = new AbortController();
    setDocument({ state: "loading" });
    loadDocument(id, aborter.signal)
      .then((read) => {
        setDocument({ state: "read", document: read });
      })
      .catch((error: unknown) => {
        if (aborter.signal.aborted) {
          return;
        }
        setDocument(isNotFound(error) ? { state: "absent" } : { state: "failed", message: failureMessage(error) });
      });
    return () => {
      aborter.abort();
    };
  }, [id, attempt]);

  // The rails are the project's own structure: which book this belongs to,
  // which records are its siblings, what the areas are called. They are read
  // once beside the document, and a project that cannot be read leaves the
  // document readable without them rather than failing the page.
  useEffect(() => {
    const aborter = new AbortController();
    loadPane(aborter.signal)
      .then((answered) => {
        setPane(answered);
      })
      .catch(() => {
        if (!aborter.signal.aborted) {
          setPane(null);
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const name = document.state === "read" ? document.document.title : "";
  useEffect(() => {
    const identity = identityOf(workspace);
    globalThis.document.title = titleFor(name === "" ? "Project" : `${name} · Project`, identity);
    return () => {
      globalThis.document.title = titleFor("Project", identity);
    };
  }, [name, workspace]);

  // A fragment on load scrolls its heading into view once the article is in
  // the tree, which a layout effect after the render guarantees without a timer.
  useLayoutEffect(() => {
    if (document.state !== "read" || location.hash.length < 2) {
      return;
    }
    globalThis.document.getElementById(location.hash.slice(1))?.scrollIntoView();
  }, [document, location.hash]);

  const retry = () => {
    setAttempt((previous) => previous + 1);
  };

  return (
    <Pane title={name === "" ? "Project" : name}>
      {document.state === "read" ? (
        <Read document={document.document} pane={pane} onReload={retry} />
      ) : (
        <div className="ms-reader">
          <article className="ms-reading">
            <NavLink className="ms-reading-back" to="/project">
              Project
            </NavLink>
            {document.state === "loading" && <LoadingRows />}
            {document.state === "absent" && <AbsentCard id={id} />}
            {document.state === "failed" && <FailureCard message={document.message} onRetry={retry} />}
          </article>
        </div>
      )}
    </Pane>
  );
}

function LoadingRows() {
  return (
    <div className="ms-reading-loading">
      {[0, 1, 2, 3].map((row) => (
        <Skeleton key={row} />
      ))}
    </div>
  );
}

function AbsentCard({ id }: { id: string }) {
  return (
    <div className="ms-reading-absent">
      <h1 className="ms-md-h1">Not found in this checkout</h1>
      <p className="ms-mono ms-project-path">{id}</p>
      <NavLink className="ms-reading-back" to="/project">
        Back to Project
      </NavLink>
    </div>
  );
}

function FailureCard({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="ms-reading-absent">
      <h1 className="ms-md-h1">This document could not be read</h1>
      <p className="ms-project-reason">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </div>
  );
}

function Read({
  document,
  pane,
  onReload,
}: {
  document: DocumentPayload;
  pane: PanePayload | null;
  onReload: () => void;
}) {
  const outline = useMemo(() => outlineOf(document.headings), [document]);
  const crumbs = useMemo(() => crumbsFor(pane, document), [pane, document]);
  const rail = useMemo(() => (pane === null ? null : railFor(pane, document)), [pane, document]);
  const current = useReadingRow(outline, document.id);
  const lead = leadingTitleId(document);

  return (
    <>
      <nav className="ms-reader-crumbs" aria-label="Breadcrumb">
        {crumbs.map((crumb, index) => (
          <span key={`${crumb.label}-${String(index)}`} className="ms-reader-crumb">
            {crumb.to === null ? <span aria-current="page">{crumb.label}</span> : <NavLink to={crumb.to}>{crumb.label}</NavLink>}
          </span>
        ))}
      </nav>
      <div className="ms-reader">
        {rail !== null && <SiblingsRail rail={rail} />}
        <article className="ms-reading">
          {document.record === null ? (
            <PlainFacts document={document} lead={lead} onReload={onReload} />
          ) : (
            <RecordFacts document={document} pane={pane} lead={lead} onReload={onReload} />
          )}
          {document.state !== "readable" && <p className="ms-project-reason">{document.reason}</p>}
          <Markdown blocks={lead === null ? document.blocks : document.blocks.slice(1)} from={document.id} />
        </article>
        {outline.length > 0 && <Outline rows={outline} current={current} />}
      </div>
    </>
  );
}

/** The facts line a document that declares nothing has always carried. */
function PlainFacts({
  document,
  lead,
  onReload,
}: {
  document: DocumentPayload;
  lead: string | null;
  onReload: () => void;
}) {
  return (
    <>
      <h1 className="ms-md-h1" id={lead ?? undefined}>
        {document.title}
      </h1>
      <p className="ms-reading-facts">
        <span className="ms-mono">{document.path}</span>
        <Chip>{ownership(document.owner)}</Chip>
        <span>changed {dateOf(document.modifiedAt)}</span>
        <span>read at {timeOf(document.readAt)}</span>
        {document.revision !== "" && <span className="ms-mono">{shortRevision(document.revision)}</span>}
        <Button onClick={onReload}>Reload</Button>
      </p>
    </>
  );
}

/**
 * A record's head, as facts: what kind it is and where it belongs above the
 * title, where it stands and where it lives beneath it, and the records it
 * rests on and that rest on it as links.
 */
function RecordFacts({
  document,
  pane,
  lead,
  onReload,
}: {
  document: DocumentPayload;
  pane: PanePayload | null;
  lead: string | null;
  onReload: () => void;
}) {
  const head = document.record;
  if (head === null) {
    return null;
  }
  const areas = head.areas.map((area) => (pane === null ? area : nameOf(pane, area)));
  return (
    <header className="ms-facts-strip">
      <p className="ms-facts-eyebrow">
        {kindTitle(head.kind)}
        {areas.length > 0 && ` · ${areas.join(", ")}`}
      </p>
      <h1 className="ms-md-h1" id={lead ?? undefined}>
        {document.title}
      </h1>
      <p className="ms-facts-line">
        {head.status !== "" && <Chip>{head.status}</Chip>}
        {head.areas.map((area) => (
          <Chip key={area}>{area}</Chip>
        ))}
        {head.id !== "" && (
          <span className="ms-mono" title={head.id}>
            {shortID(head.id)}
          </span>
        )}
        <span className="ms-mono ms-facts-path" title={document.path}>
          {document.path}
        </span>
        <span>
          changed {dateOf(document.modifiedAt)} · read at {timeOf(document.readAt)}
        </span>
        <Button onClick={onReload}>Reload</Button>
      </p>
      <Relationships document={document} pane={pane} />
    </header>
  );
}

/** What this record names, and what names it. A relationship is a link. */
function Relationships({ document, pane }: { document: DocumentPayload; pane: PanePayload | null }) {
  const head = document.record;
  if (head === null) {
    return null;
  }
  const lists: { label: string; links: RecordLink[] }[] = [
    { label: "Rests on", links: byID(pane, head.cites) },
    { label: "Affects", links: byID(pane, head.affects) },
    { label: "Supersedes", links: byID(pane, head.supersedes) },
    { label: "Superseded by", links: document.supersededBy },
    { label: "Referenced by", links: document.referencedBy },
  ].filter((list) => list.links.length > 0);
  if (lists.length === 0) {
    return null;
  }
  return (
    <div className="ms-facts-links">
      {lists.map((list) => (
        <p key={list.label} className="ms-facts-link-row">
          <span className="ms-facts-link-label">{list.label}</span>
          {list.links.map((link) =>
            link.path === "" ? (
              <span key={link.id} className="ms-mono">
                {link.title}
              </span>
            ) : (
              <NavLink key={link.id} to={documentPath(link.path)}>
                {link.title}
              </NavLink>
            ),
          )}
        </p>
      ))}
    </div>
  );
}

/**
 * The records a head names, as links. An id no record in this checkout
 * declares is shown as the id it is, and opens nothing: the reference may name
 * something that has not been written yet, and inventing a destination for it
 * would be worse than saying so.
 */
function byID(pane: PanePayload | null, ids: string[]): RecordLink[] {
  return ids.map((id) => {
    const record = pane?.records.find((candidate) => candidate.id === id);
    return record === undefined
      ? { id, title: id, path: "", kind: "" }
      : { id, title: record.title, path: record.path, kind: record.kind };
  });
}

/** The left rail: the siblings, the current one marked, and the way on. */
function SiblingsRail({ rail }: { rail: SiblingRail }) {
  return (
    <nav className="ms-reader-rail" aria-label={rail.title}>
      <h2 className="ms-reader-rail-title">{rail.title}</h2>
      <ul className="ms-reader-rail-list">
        {rail.siblings.map((sibling) => (
          <li key={sibling.key}>
            <NavLink
              className="ms-reader-rail-row"
              to={sibling.to}
              aria-current={sibling.current ? "page" : undefined}
            >
              <span className="ms-reader-rail-name">{sibling.title}</span>
              {sibling.note !== "" && <span className="ms-reader-rail-note">{sibling.note}</span>}
            </NavLink>
          </li>
        ))}
      </ul>
      <div className="ms-reader-steps">
        {rail.previous === null ? (
          <span className="ms-reader-step-none">← no previous</span>
        ) : (
          <NavLink className="ms-reader-step" to={rail.previous.to}>
            ← {rail.previous.title}
          </NavLink>
        )}
        {rail.next === null ? (
          <span className="ms-reader-step-none">no next →</span>
        ) : (
          <NavLink className="ms-reader-step" to={rail.next.to}>
            {rail.next.title} →
          </NavLink>
        )}
      </div>
    </nav>
  );
}

function Outline({ rows, current }: { rows: OutlineRow[]; current: string | null }) {
  return (
    <aside className="ms-reader-outline" aria-label="On this page">
      <h2 className="ms-reader-rail-title">On this page</h2>
      <ul className="ms-reading-outline-list">
        {rows.map((row) => (
          <li key={row.id} className={`ms-reading-outline-row ms-reading-indent-${String(row.indent)}`}>
            <a href={`#${row.id}`} aria-current={row.id === current ? "location" : undefined}>
              {row.text}
            </a>
          </li>
        ))}
      </ul>
    </aside>
  );
}

/**
 * Which outline row the reader is in, observed rather than timed.
 *
 * An IntersectionObserver reports each heading as it enters and leaves, and
 * the row is decided from the set of headings on screen, in the document's own
 * order. There is no timer and no scroll handler: the browser tells this hook
 * when something changed, and nothing else wakes it.
 */
function useReadingRow(rows: OutlineRow[], id: string): string | null {
  const [current, setCurrent] = useState<string | null>(null);

  useEffect(() => {
    setCurrent(null);
    if (rows.length === 0 || typeof IntersectionObserver !== "function") {
      return;
    }
    const visible = new Set<string>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            visible.add(entry.target.id);
          } else {
            visible.delete(entry.target.id);
          }
        }
        setCurrent((previous) => currentRow(rows, visible, previous));
      },
      // The bottom margin keeps the mark on the section being read rather than
      // on whichever heading happens to be at the foot of the window.
      { rootMargin: "0px 0px -60% 0px" },
    );
    for (const row of rows) {
      const heading = globalThis.document.getElementById(row.id);
      if (heading !== null) {
        observer.observe(heading);
      }
    }
    return () => {
      observer.disconnect();
    };
  }, [rows, id]);

  return current;
}

/**
 * The id of the document's own title heading, when its first block is one.
 *
 * A document that opens with its title would otherwise show it twice, so the
 * page renders one of them and the heading's id moves to it: the outline, and
 * a link from another document, still land where they said they would.
 */
export function leadingTitleId(document: DocumentPayload): string | null {
  const first = document.blocks[0];
  const heading = document.headings[0];
  if (first === undefined || heading === undefined) {
    return null;
  }
  if (first.type !== "heading" || first.level !== 1 || first.id !== heading.id) {
    return null;
  }
  return heading.text === document.title ? heading.id : null;
}

/** The first seven characters of the blob id, as Git itself abbreviates one. */
export function shortRevision(revision: string): string {
  const digest = revision.startsWith("blob:") ? revision.slice("blob:".length) : revision;
  return digest.slice(0, 7);
}

/** The tab title needs three cases, and the workspace state carries four. */
function identityOf(workspace: WorkspaceState): Identity {
  if (workspace.state === "loading") {
    return { state: "loading" };
  }
  if (workspace.state === "failed") {
    return { state: "unknown" };
  }
  const described = workspace.workspace;
  return { state: "known", subject: described.subject, mode: described.mode, conflict: described.conflict };
}
