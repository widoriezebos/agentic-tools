import { useEffect, useLayoutEffect, useState } from "react";
import { NavLink, useLocation } from "react-router";

import { failureMessage, isNotFound, loadDocument, type DocumentPayload } from "./api";
import { Markdown } from "./Markdown";
import { outlineOf } from "./outline";
import { dateOf, ownership, timeOf } from "./ProjectPane";
import "./reading.css";
import { Pane } from "../panes/Pane";
import { documentIdFromPath } from "../routes";
import { Button, Chip, Skeleton } from "../shell/controls";
import { useWorkspaceState, type WorkspaceState } from "../shell/identity";
import { titleFor, type Identity } from "../title";

/**
 * One document, in a calm reading column.
 *
 * The page shows where the document is, who owns it, when it last changed,
 * when this view was read, and which revision it is: everything a human needs
 * to know whether what they are reading is current. Nothing is summarised, and
 * no revision is claimed for bytes that were not all read.
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

  return (
    <Pane title={name === "" ? "Project" : name}>
      <article className="ms-reading">
        <NavLink className="ms-reading-back" to="/project">
          Project
        </NavLink>
        {document.state === "loading" && <LoadingRows />}
        {document.state === "absent" && <AbsentCard id={id} />}
        {document.state === "failed" && (
          <FailureCard message={document.message} onRetry={() => { setAttempt((previous) => previous + 1); }} />
        )}
        {document.state === "read" && (
          <Read
            document={document.document}
            onReload={() => {
              setAttempt((previous) => previous + 1);
            }}
          />
        )}
      </article>
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

function Read({ document, onReload }: { document: DocumentPayload; onReload: () => void }) {
  const outline = outlineOf(document.headings);
  const lead = leadingTitleId(document);
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
      {document.state !== "readable" && <p className="ms-project-reason">{document.reason}</p>}
      {outline.length > 0 && (
        <nav className="ms-reading-outline" aria-label="Contents">
          <ul className="ms-reading-outline-list">
            {outline.map((row) => (
              <li key={row.id} className={`ms-reading-outline-row ms-reading-indent-${String(row.indent)}`}>
                <a href={`#${row.id}`}>{row.text}</a>
              </li>
            ))}
          </ul>
        </nav>
      )}
      <Markdown blocks={lead === null ? document.blocks : document.blocks.slice(1)} from={document.id} />
    </>
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
