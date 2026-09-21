import { BookOpen } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { NavLink } from "react-router";

import {
  failureMessage,
  loadThread,
  type Covenant,
  type DocumentEntry,
  type Group,
  type Purpose,
  type Subsection,
  type Thread,
} from "./api";
import "./reading.css";
import { EmptyState, Pane } from "../panes/Pane";
import { routeFor } from "../routes";
import { Button, Chip, Skeleton } from "../shell/controls";

/**
 * Project: the six subsections of the master's section, over documents that
 * already exist at their canonical locations.
 *
 * Nothing here is written to make a screen appear. A subsection with no source
 * says "Not yet recorded" and names, with absolute paths, where it looked; a
 * document that cannot be read says why and offers no link; and the sitting
 * store, which this build has not got, says which gate brings it.
 */

type ThreadState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "read"; thread: Thread };

export function ProjectPane() {
  const [thread, setThread] = useState<ThreadState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadThread(aborter.signal)
      .then((read) => {
        setThread({ state: "read", thread: read });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setThread({ state: "failed", message: failureMessage(error) });
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const reload = () => {
    setThread({ state: "loading" });
    setAttempt((previous) => previous + 1);
  };

  return (
    <Pane title="Project">
      <div className="ms-pane-stack">
        {thread.state === "loading" && <LoadingCards />}
        {thread.state === "failed" && <FailureCard message={thread.message} onRetry={reload} />}
        {thread.state === "read" && (
          <>
            <p className="ms-project-read-at">
              Read at {timeOf(thread.thread.readAt)}
              <Button onClick={reload}>Reload</Button>
            </p>
            {thread.thread.subsections.map((subsection) => (
              <SubsectionCard key={subsection.id} subsection={subsection} />
            ))}
          </>
        )}
      </div>
    </Pane>
  );
}

function LoadingCards() {
  return (
    <>
      {[0, 1, 2].map((row) => (
        <section key={row} className="ms-card">
          <Skeleton />
        </section>
      ))}
    </>
  );
}

function FailureCard({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">Project could not be read</h2>
      <p className="ms-project-reason">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </section>
  );
}

function SubsectionCard({ subsection }: { subsection: Subsection }) {
  if (subsection.state === "not-projected") {
    return (
      <section className="ms-card">
        <h2 className="ms-card-title">{subsection.title}</h2>
        <EmptyState id={subsection.id} icon={BookOpen} />
      </section>
    );
  }
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">{subsection.title}</h2>
      {subsection.state === "not-recorded" && <NotRecorded subsection={subsection} />}
      {subsection.purpose !== null && <PurposeRows purpose={subsection.purpose} />}
      {subsection.covenant !== null && <CovenantRows subsection={subsection} />}
      {subsection.groups.map((group) => (
        <GroupRows key={group.id} group={group} />
      ))}
    </section>
  );
}

function NotRecorded({ subsection }: { subsection: Subsection }) {
  return (
    <div className="ms-project-absent">
      <p className="ms-project-absent-statement">Not yet recorded</p>
      {subsection.lookedFor.length > 0 && (
        <>
          <p className="ms-project-absent-lead">Looked for:</p>
          <ul className="ms-project-paths">
            {subsection.lookedFor.map((path) => (
              <li key={path} className="ms-mono ms-project-path">
                {path}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

function PurposeRows({ purpose }: { purpose: Purpose }) {
  return (
    <div className="ms-project-purpose">
      <p className="ms-project-prose">{purpose.text}</p>
      <p className="ms-mono ms-project-path">{purpose.path}</p>
    </div>
  );
}

/** Intent shows what the covenant claims; Constraints shows what proves it. */
function CovenantRows({ subsection }: { subsection: Subsection }) {
  const covenant = subsection.covenant as Covenant;
  if (covenant.error !== undefined && covenant.error !== "") {
    return (
      <p className="ms-project-reason">
        Covenant at {covenant.path} could not be read: {covenant.error}
      </p>
    );
  }
  const identity = covenant.identity;
  const battery = covenant.battery;
  return (
    <div className="ms-project-covenant">
      <p className="ms-mono ms-project-path">{covenant.path}</p>
      {subsection.id === "intent" && identity !== undefined && (
        <>
          <dl className="ms-facts">
            <Fact name="Name">{identity.name}</Fact>
            <Fact name="Entry point">
              <span className="ms-mono">{identity.entryPoint}</span>
            </Fact>
            <Fact name="Source paths">
              <span className="ms-mono">{identity.sourcePaths.join(", ")}</span>
            </Fact>
          </dl>
          <RequirementsTable covenant={covenant} />
        </>
      )}
      {subsection.id === "constraints" && (
        <>
          {battery !== undefined && (
            <dl className="ms-facts">
              <Fact name="Battery">
                <span className="ms-mono">{battery.command}</span>
              </Fact>
              <Fact name="Threshold">
                {battery.metric} {battery.direction} {battery.threshold}
              </Fact>
            </dl>
          )}
          <BoundsList title="Budgets" rows={(covenant.budgets ?? []).map((budget) => `${budget.metric} ${budget.direction} ${String(budget.bound)}`)} />
          <BoundsList title="Guards" rows={(covenant.guards ?? []).map((guard) => `${guard.name}: ${guard.command}`)} />
          <BoundsList title="Guardrails" rows={covenant.guardrails ?? []} />
        </>
      )}
    </div>
  );
}

function RequirementsTable({ covenant }: { covenant: Covenant }) {
  const requirements = covenant.requirements ?? [];
  if (requirements.length === 0) {
    return null;
  }
  return (
    <div className="ms-md-table-scroll">
      <table className="ms-md-table">
        <thead>
          <tr>
            <th className="ms-md-left">Requirement</th>
            <th className="ms-md-left">Statement</th>
            <th className="ms-md-left">Proof</th>
          </tr>
        </thead>
        <tbody>
          {requirements.map((requirement) => (
            <tr key={requirement.id}>
              <td className="ms-md-left">{requirement.id}</td>
              <td className="ms-md-left">{requirement.ref}</td>
              <td className="ms-md-left">
                <span className="ms-mono">{requirement.proof}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BoundsList({ title, rows }: { title: string; rows: string[] }) {
  if (rows.length === 0) {
    return null;
  }
  return (
    <div className="ms-project-group">
      <h3 className="ms-project-group-title">{title}</h3>
      <ul className="ms-project-paths">
        {rows.map((row) => (
          <li key={row} className="ms-mono ms-project-path">
            {row}
          </li>
        ))}
      </ul>
    </div>
  );
}

function GroupRows({ group }: { group: Group }) {
  return (
    <div className="ms-project-group">
      {group.title !== "" && <h3 className="ms-project-group-title">{group.title}</h3>}
      <ul className="ms-project-documents">
        {group.documents.map((entry) => (
          <li key={entry.id} className="ms-project-row">
            <DocumentRow entry={entry} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function DocumentRow({ entry }: { entry: DocumentEntry }) {
  const to = entry.state === "readable" ? routeFor({ kind: "document", id: entry.id }) : null;
  return (
    <>
      <span className="ms-project-row-title">
        {to === null ? entry.title : <NavLink to={to}>{entry.title}</NavLink>}
      </span>
      <span className="ms-mono ms-project-row-path">{entry.id}</span>
      <Chip>{ownership(entry.owner)}</Chip>
      <span className="ms-project-row-date">{dateOf(entry.modifiedAt)}</span>
      {entry.reason !== "" && <span className="ms-project-row-reason">{entry.reason}</span>}
    </>
  );
}

function Fact({ name, children }: { name: string; children: ReactNode }) {
  return (
    <>
      <dt className="ms-fact-name">{name}</dt>
      <dd className="ms-fact-value">{children}</dd>
    </>
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
