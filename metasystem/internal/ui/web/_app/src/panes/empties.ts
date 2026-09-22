/**
 * What every pane in this build says, in one table.
 *
 * The master distinguishes two kinds of emptiness, because only one of them
 * has a first action. A section whose records do not exist yet offers the
 * action that would create them. A section this build does not project yet is
 * not empty at all — the records may well exist, and for most of these they do
 * — so it says that the view is absent and which delivery gate brings it, and
 * offers no action of its own, because every candidate would either mislead or
 * send the human to a terminal. So `action: null` here is the rule, not an
 * omission, and no pane claims that anything is absent.
 *
 * A section this build does project has no row here at all: its pane says what
 * it read, from the response, rather than what it has not built yet.
 *
 * A link, where a row has one, is a text link to a surface this build actually
 * serves. Overview has the only one: a human who has just arrived lands there,
 * and About this workspace is this build's one live answer to "what is this".
 *
 * Not found is neither kind. An address that matches nothing has one honest
 * next step, so it keeps its action.
 */

export type EmptyKind = "not projected" | "not a section";

export type EmptyLink = { label: string; to: string };

export type Empty = {
  id: string;
  kind: EmptyKind;
  heading: string;
  body: string;
  /** The 12px line beneath the body: which slice or gate brings the view. */
  note: string;
  link: EmptyLink | null;
  action: EmptyLink | null;
};

export const empties: readonly Empty[] = [
  {
    id: "overview",
    kind: "not projected",
    heading: "Overview is not projected yet",
    body: "This build does not read the records. Overview will show what needs you and what changed since your last visit, linked to its records.",
    note: "Arrives with g1-s14",
    link: { label: "About this workspace", to: "/settings" },
    action: null,
  },
  {
    id: "fleet",
    kind: "not projected",
    heading: "Fleet is not projected yet",
    body: "This machine's sessions, jobs, census, and health are recorded and will be shown with observation times and gaps.",
    note: "Arrives with g1-s13",
    link: null,
    action: null,
  },
  {
    id: "decisions",
    kind: "not projected",
    heading: "Decisions are not projected yet",
    body: "Questions from every seat, approvals, delegations, and rulings are in the ledger and will be read here.",
    note: "Arrives with g1-s12",
    link: null,
    action: null,
  },
  {
    id: "application",
    kind: "not projected",
    heading: "Application is not projected yet",
    body: "Implemented and released behaviour and its evidence are recorded and will be shown here.",
    note: "Arrives with gate 6",
    link: null,
    action: null,
  },
  {
    id: "settings",
    kind: "not projected",
    heading: "Settings pages are not built yet",
    body: "Runtimes and models, connections and channels, identity and authority, execution defaults, storage and retention.",
    note: "Arrives with gate 7",
    link: null,
    action: null,
  },
  {
    id: "brain",
    kind: "not projected",
    heading: "The Brain is not connected in this build",
    body: "Conversation, shared context, activity, and actions.",
    note: "Arrives with gate 3",
    link: null,
    action: null,
  },
  {
    id: "subject",
    kind: "not projected",
    heading: "No subject selected",
    body: "The subject panel will show the artifact under discussion.",
    note: "Arrives with gate 3",
    link: null,
    action: null,
  },
  {
    id: "not-found",
    kind: "not a section",
    heading: "No page at this address",
    body: "The address matches no section of this workspace.",
    note: "Check the address in the address bar",
    link: null,
    action: { label: "Go to Overview", to: "/overview" },
  },
];

export function emptyFor(id: string): Empty {
  const found = empties.find((empty) => empty.id === id);
  if (found === undefined) {
    throw new Error(`no empty state is written for ${id}`);
  }
  return found;
}
