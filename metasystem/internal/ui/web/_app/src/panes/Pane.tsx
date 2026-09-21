import { NavLink, useNavigate } from "react-router";
import type { ReactNode } from "react";

import { Button } from "../shell/controls";
import type { SectionIcon } from "../routes";
import { emptyFor } from "./empties";

/**
 * The work area. It is the skip link's target and names the section it shows,
 * but it is not a tab stop of its own: tabIndex -1 lets the skip link move
 * focus here without adding a stop to the sequential order.
 *
 * The heading is visually hidden because the header already shows the section
 * title in the one place a human looks for it, and two copies of the same word
 * on screen is noise.
 */
export function Pane({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main id="content" className="ms-work" tabIndex={-1}>
      <h1 className="ms-visually-hidden">{title}</h1>
      {children}
    </main>
  );
}

/**
 * One empty state, rendered from the table. Either the link or the action, and
 * for most rows neither: what a row offers is the table's decision, not this
 * component's.
 */
export function EmptyState({ id, icon: Icon, onNavigate }: { id: string; icon: SectionIcon; onNavigate?: () => void }) {
  const empty = emptyFor(id);
  return (
    <div className="ms-empty">
      <Icon className="ms-empty-icon" size={24} strokeWidth={1.75} aria-hidden="true" />
      <h2 className="ms-empty-heading">{empty.heading}</h2>
      <p className="ms-empty-body">{empty.body}</p>
      <p className="ms-empty-slice">{empty.note}</p>
      {empty.link !== null && (
        <NavLink className="ms-empty-link" to={empty.link.to} onClick={onNavigate}>
          {empty.link.label}
        </NavLink>
      )}
      {empty.action !== null && <ActionButton label={empty.action.label} to={empty.action.to} onNavigate={onNavigate} />}
    </div>
  );
}

/**
 * The one action in this build: the way back from an address that matches
 * nothing. It is a button rather than a link because it is an action, and a
 * button inside an anchor would be two controls where a human sees one.
 */
function ActionButton({ label, to, onNavigate }: { label: string; to: string; onNavigate?: () => void }) {
  const navigate = useNavigate();
  return (
    <Button
      onClick={() => {
        onNavigate?.();
        void navigate(to);
      }}
    >
      {label}
    </Button>
  );
}
