import { useEffect, useLayoutEffect, useMemo, useState, type KeyboardEvent } from "react";
import { NavLink, useLocation } from "react-router";

import {
  editDocument,
  failureMessage,
  isNotFound,
  loadDocument,
  loadPane,
  previewDocument,
  ResourceError,
  type DocumentPayload,
  type Link as RecordLink,
  type Pane as PanePayload,
  type Problem,
} from "./api";
import {
  dirty,
  next,
  opening,
  savedAt,
  statusOf,
  type Editor as EditorState,
  type Outcome,
} from "./editing";
import { Editor } from "./Editor";
import { Markdown } from "./Markdown";
import { outlineOf, useReadingRow, type OutlineRow } from "./outline";
import { aboutOf, crumbsFor, kindTitle, railFor, shortID, type About, type SiblingRail } from "./pane";
import { dateOf, ownership, timeOf } from "./ProjectPane";
import "./reading.css";
import { Sheet, type Done, type Request } from "./Sheet";
import { STATUSES } from "./writing";
import { Pane } from "../panes/Pane";
import { documentIdFromPath, documentPath } from "../routes";
import { aboutLine, useAbout } from "../shell/about";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useWorkspaceState, type WorkspaceState } from "../shell/identity";
import { titleFor, type Identity } from "../title";

/**
 * One document, read as a chapter of a book rather than as a file.
 *
 * A record's head is a strip of facts at the top — what it is, where it
 * stands, which goals it is about, what it rests on and what rests on it — and
 * not four bullet lines in the text, because a human reading a design is
 * reading prose and the head is not prose. The left rail is what this document
 * is read among: the book's chapters in reading order, or the records of its
 * own kind — which may be this record alone — with previous and next at the
 * foot where there is anywhere to step. The right rail is the outline, and it
 * marks where the reader is as they scroll.
 *
 * The three are three columns, each naming the one it is in, so a rail with
 * nothing to show leaves its column empty rather than moving the reading into
 * it.
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
  // which records are its siblings, what the ledger calls its goals. They are
  // read once beside the document, and a project that cannot be read leaves
  // the document readable without them rather than failing the page.
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

  // A status change rewrites one line of the file, so the strip is updated in
  // place from what the server read back rather than by fetching the whole
  // document again.
  const restated = (status: string) => {
    setDocument((state) =>
      state.state === "read" && state.document.record !== null
        ? { state: "read", document: { ...state.document, record: { ...state.document.record, status } } }
        : state,
    );
  };

  // A save answers with the document as it now reads from disk, so the page is
  // replaced with what the server read back rather than with what was typed.
  const rewritten = (payload: DocumentPayload) => {
    setDocument({ state: "read", document: payload });
  };

  return (
    <Pane title={name === "" ? "Project" : name}>
      {document.state === "read" ? (
        <Read
          document={document.document}
          pane={pane}
          onReload={retry}
          onStatus={restated}
          onSaved={rewritten}
        />
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
  onStatus,
  onSaved,
}: {
  document: DocumentPayload;
  pane: PanePayload | null;
  onReload: () => void;
  onStatus: (status: string) => void;
  onSaved: (payload: DocumentPayload) => void;
}) {
  const outline = useMemo(() => outlineOf(document.headings), [document]);
  const crumbs = useMemo(() => crumbsFor(pane, document), [pane, document]);
  const rail = useMemo(() => (pane === null ? null : railFor(pane, document)), [pane, document]);
  const current = useReadingRow(outline, document.id);
  const lead = leadingTitleId(document);
  const [sheet, setSheet] = useState<Request | null>(null);
  // The editor, while one is open, and what the last save is remembered as.
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [saved, setSaved] = useState("");

  // What the drawer says this page is about: the record, and the section of it
  // being read, from the outline the reader already follows. The first heading
  // of a document is its own title, which the line says once.
  const heading = outline.find((row) => row.id === current);
  useAbout(aboutLine(document.title, heading?.text ?? ""));

  // A change typed here exists nowhere else, so the browser is asked to say so
  // before the page goes. The guard is armed only while there is something to
  // lose, and it is the one window event in this build.
  const unsaved = editor !== null && dirty(editor);
  useEffect(() => {
    if (!unsaved) {
      return;
    }
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    globalThis.addEventListener("beforeunload", guard);
    return () => {
      globalThis.removeEventListener("beforeunload", guard);
    };
  }, [unsaved]);

  // Every answer lands on the editor as it stands rather than on the one the
  // request was made from, so nothing typed while waiting is thrown away by a
  // reply, and a reply to an editor that has closed lands nowhere.
  const answered = (outcome: Outcome) => {
    setEditor((state) => (state === null ? state : next(state, outcome)));
  };

  const save = () => {
    if (editor === null || editor.pending) {
      return;
    }
    const sending = next(editor, { kind: "sending", saying: "Saving…" });
    setEditor(sending);
    editDocument(document.id, sending.source, sending.revision)
      .then((payload) => {
        setEditor(null);
        setSaved(savedAt(new Date()));
        onSaved(payload);
      })
      .catch((error: unknown) => {
        answered(refused(error));
      });
  };

  const render = () => {
    if (editor === null || editor.pending || editor.mode === "preview") {
      return;
    }
    const sending = next(editor, { kind: "sending", saying: "Rendering…" });
    setEditor(sending);
    previewDocument(sending.source)
      .then((preview) => {
        answered({ kind: "rendered", blocks: preview.blocks });
      })
      .catch((error: unknown) => {
        answered(refused(error));
      });
  };

  // Leaving with changes asks first; leaving without them is not a question.
  const leave = () => {
    if (editor === null) {
      return;
    }
    if (dirty(editor)) {
      setSheet({ mode: "discard", path: document.id });
      return;
    }
    setEditor(null);
  };

  const open = () => {
    setEditor(opening(document.source, document.revision));
  };

  const done = (result: Done) => {
    setSheet(null);
    if (result.mode === "status") {
      onStatus(result.written.record.status);
    }
    if (result.mode === "discard") {
      setEditor(null);
    }
  };

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
          {editor === null ? (
            <>
              {document.record === null ? (
                <PlainFacts
                  document={document}
                  lead={lead}
                  saved={saved}
                  onReload={onReload}
                  onEdit={editable(document) ? open : null}
                />
              ) : (
                <RecordFacts
                  document={document}
                  pane={pane}
                  lead={lead}
                  saved={saved}
                  onReload={onReload}
                  onEdit={editable(document) ? open : null}
                  onStatusChange={(status) => {
                    setSheet({ mode: "status", id: document.record?.id ?? "", title: document.title, path: document.id, status });
                  }}
                />
              )}
              {document.state !== "readable" && <p className="ms-project-reason">{document.reason}</p>}
              <Markdown blocks={lead === null ? document.blocks : document.blocks.slice(1)} from={document.id} />
            </>
          ) : (
            <Editing
              editor={editor}
              from={document.id}
              onType={(source) => {
                answered({ kind: "typed", source });
              }}
              onSource={() => {
                answered({ kind: "source" });
              }}
              onPreview={render}
              onSave={save}
              onCancel={leave}
            />
          )}
        </article>
        {outline.length > 0 && <Outline rows={outline} current={current} />}
      </div>
      {sheet !== null && (
        <Sheet
          request={sheet}
          goals={pane?.goals ?? []}
          onClose={() => {
            setSheet(null);
          }}
          onDone={done}
        />
      )}
    </>
  );
}

/**
 * A document can be edited here when there is text to edit: a file that was
 * too large to read to its end, or that is not text at all, carries no source,
 * and an editor over nothing would save nothing over the file.
 */
function editable(document: DocumentPayload): boolean {
  return document.state === "readable";
}

/** A refusal, as the outcome the editor answers to. */
function refused(error: unknown): Outcome {
  const resource = error instanceof ResourceError ? error : null;
  return {
    kind: "refused",
    status: resource?.status ?? 0,
    said: failureMessage(error),
    problems: resource?.problems ?? [],
  };
}

/**
 * The editor, in the reading column and in the article's place.
 *
 * The article becomes this while it is open, so there is one thing to do: the
 * facts strip, the status control and the file actions are not there to be
 * clicked past. Above the text is the whole of the bar — what is showing, the
 * two acts, and one line saying where things stand.
 *
 * The preview renders through the same component the reader renders with, in
 * the same styles, because a preview in a second set of styles is a preview of
 * a different page.
 */
function Editing({
  editor,
  from,
  onType,
  onSource,
  onPreview,
  onSave,
  onCancel,
}: {
  editor: EditorState;
  from: string;
  onType: (source: string) => void;
  onSource: () => void;
  onPreview: () => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  // The two shortcuts an editor is expected to have, taken on the way up from
  // whatever holds focus, so they work on the bar and over the preview as well
  // as in the text. The text itself binds them inside CodeMirror, which is the
  // only place that can keep the browser's own Mod-s from opening a save
  // dialog; when that binding has answered it has already said so by
  // preventing the default, and this one stands down rather than acting twice.
  const keys = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.defaultPrevented) {
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      onCancel();
      return;
    }
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
      event.preventDefault();
      onSave();
    }
  };

  return (
    <div className="ms-editing" onKeyDown={keys}>
      <div className="ms-editor-bar">
        <div className="ms-editor-modes" role="group" aria-label="What the editor shows">
          <button
            type="button"
            className="ms-editor-mode"
            aria-pressed={editor.mode === "source"}
            disabled={editor.pending}
            onClick={onSource}
          >
            Write
          </button>
          <button
            type="button"
            className="ms-editor-mode"
            aria-pressed={editor.mode === "preview"}
            disabled={editor.pending}
            onClick={onPreview}
          >
            Preview
          </button>
        </div>
        <Button primary disabled={editor.pending} onClick={onSave}>
          Save
        </Button>
        <Button disabled={editor.pending} onClick={onCancel}>
          Cancel
        </Button>
        <span className="ms-editor-status" role="status">
          {statusOf(editor)}
        </span>
      </div>
      {editor.problems.length > 0 && <Problems problems={editor.problems} />}
      {editor.mode === "source" ? (
        <Editor value={editor.source} onChange={onType} onSave={onSave} onCancel={onCancel} autoFocus />
      ) : (
        <Markdown blocks={editor.blocks} from={from} />
      )}
    </div>
  );
}

/**
 * What the project refused the save over, above the text, each anchored at the
 * line a human has to go and fix.
 */
function Problems({ problems }: { problems: Problem[] }) {
  return (
    <ul className="ms-editor-problems">
      {problems.map((problem, index) => (
        <li key={`${problem.path}-${String(problem.line)}-${String(index)}`}>
          <span className="ms-mono">line {problem.line}</span> {problem.message}
        </li>
      ))}
    </ul>
  );
}

/** The facts line a document that declares nothing has always carried. */
function PlainFacts({
  document,
  lead,
  saved,
  onReload,
  onEdit,
}: {
  document: DocumentPayload;
  lead: string | null;
  saved: string;
  onReload: () => void;
  onEdit: (() => void) | null;
}) {
  return (
    <>
      <h1 className="ms-md-h1" id={lead ?? undefined}>
        {document.title}
      </h1>
      <p className="ms-reading-facts">
        <span className="ms-mono">{document.path}</span>
        <FileActions path={document.path} onEdit={onEdit} />
        <Chip>{ownership(document.owner)}</Chip>
        <span>changed {dateOf(document.modifiedAt)}</span>
        <span>read at {timeOf(document.readAt)}</span>
        {document.revision !== "" && <span className="ms-mono">{shortRevision(document.revision)}</span>}
        <Button onClick={onReload}>Reload</Button>
        {saved !== "" && <span role="status">{saved}</span>}
      </p>
    </>
  );
}

/**
 * A record's head, as facts: what kind it is above the title, where it stands
 * and where it lives beneath it, which goals it is about, and the records it
 * rests on and that rest on it as links.
 */
function RecordFacts({
  document,
  pane,
  lead,
  saved,
  onReload,
  onEdit,
  onStatusChange,
}: {
  document: DocumentPayload;
  pane: PanePayload | null;
  lead: string | null;
  saved: string;
  onReload: () => void;
  onEdit: (() => void) | null;
  onStatusChange: (status: string) => void;
}) {
  const head = document.record;
  if (head === null) {
    return null;
  }
  return (
    <header className="ms-facts-strip">
      <p className="ms-facts-eyebrow">{kindTitle(head.kind)}</p>
      <h1 className="ms-md-h1" id={lead ?? undefined}>
        {document.title}
      </h1>
      <p className="ms-facts-line">
        <Status status={head.status} onChange={onStatusChange} />
        {head.id !== "" && (
          <span className="ms-mono" title={head.id}>
            {shortID(head.id)}
          </span>
        )}
        <span className="ms-mono ms-facts-path" title={document.path}>
          {document.path}
        </span>
        <FileActions path={document.path} onEdit={onEdit} />
        <span>
          changed {dateOf(document.modifiedAt)} · read at {timeOf(document.readAt)}
        </span>
        <Button onClick={onReload}>Reload</Button>
        {saved !== "" && <span role="status">{saved}</span>}
      </p>
      <Relationships document={document} pane={pane} />
    </header>
  );
}

/**
 * Status, as the one control the reader can change.
 *
 * Choosing a status does not write it: it opens the sheet with the line that
 * would be rewritten, and the write happens when a human confirms it there.
 * So the select is controlled by the record's own status and goes back to it
 * if the sheet is cancelled, and never shows a status the file does not carry.
 *
 * A status the grammar does not carry is shown and not offered: this is a
 * reader, and a select that silently replaced a refused value would hide the
 * one thing the check verb is complaining about.
 */
function Status({ status, onChange }: { status: string; onChange: (status: string) => void }) {
  if (status === "") {
    return null;
  }
  if (!STATUSES.includes(status)) {
    return <Chip>{status}</Chip>;
  }
  return (
    <span className="ms-facts-status">
      <label className="ms-visually-hidden" htmlFor="ms-facts-status">
        Status
      </label>
      <select
        id="ms-facts-status"
        className="ms-status-select"
        data-status={status}
        value={status}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        {STATUSES.map((candidate) => (
          <option key={candidate} value={candidate}>
            {candidate}
          </option>
        ))}
      </select>
    </span>
  );
}

/**
 * What can be done with the file: edited here, or its absolute path put on the
 * clipboard.
 *
 * Edit is first because it is the one that does the work. The link that handed
 * the file to a local editor is gone: the editor is on this page now, and a
 * second way out of it was a way to two unsaved copies of the same document. A
 * document this interface could not read to its end has no Edit — there is
 * nothing to open an editor over — and the copy still works on a file this
 * build will not show.
 *
 * The copy confirmation is a CSS animation on an element that is remounted for
 * each copy, so a second copy says so again; there is no timer anywhere in this
 * build, and a confirmation that needed one would be the first.
 */
function FileActions({ path, onEdit }: { path: string; onEdit: (() => void) | null }) {
  const [copies, setCopies] = useState(0);
  const [refused, setRefused] = useState(false);

  const copy = () => {
    try {
      void navigator.clipboard.writeText(path).then(
        () => {
          setRefused(false);
          setCopies((previous) => previous + 1);
        },
        () => {
          setRefused(true);
        },
      );
    } catch {
      setRefused(true);
    }
  };

  return (
    <span className="ms-facts-file">
      {onEdit !== null && (
        <button type="button" className="ms-project-act" onClick={onEdit}>
          Edit
        </button>
      )}
      <button type="button" className="ms-project-act" onClick={copy}>
        Copy path
      </button>
      {copies > 0 && (
        <span key={copies} className="ms-facts-copied" role="status">
          Copied
        </span>
      )}
      {refused && <span className="ms-project-reason">This browser did not allow the copy.</span>}
    </span>
  );
}

/**
 * What this record is about, what it names, and what names it. A relationship
 * is a link, and so is a goal: the About line leads to the goal's own page,
 * where the designs, decisions and questions about it are gathered.
 */
function Relationships({ document, pane }: { document: DocumentPayload; pane: PanePayload | null }) {
  const head = document.record;
  if (head === null) {
    return null;
  }
  const about = aboutOf(pane, head.goals);
  const lists: { label: string; links: RecordLink[] }[] = [
    { label: "Rests on", links: byID(pane, head.cites) },
    { label: "Affects", links: byID(pane, head.affects) },
    { label: "Supersedes", links: byID(pane, head.supersedes) },
    { label: "Superseded by", links: document.supersededBy },
    { label: "Referenced by", links: document.referencedBy },
  ].filter((list) => list.links.length > 0);
  if (lists.length === 0 && about.length === 0) {
    return null;
  }
  return (
    <div className="ms-facts-links">
      {about.length > 0 && <AboutRow about={about} />}
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

/** The goals a record is about, each a link to its own page. */
function AboutRow({ about }: { about: About[] }) {
  return (
    <p className="ms-facts-link-row">
      <span className="ms-facts-link-label">About</span>
      {about.map((goal) => (
        <NavLink key={goal.id} className="ms-mono" to={goal.to}>
          {goal.name}
        </NavLink>
      ))}
    </p>
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
      {/* Where there is neither a previous nor a next there is nowhere to
          step, and two lines saying so would be the whole foot of the rail. */}
      {(rail.previous !== null || rail.next !== null) && (
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
      )}
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
