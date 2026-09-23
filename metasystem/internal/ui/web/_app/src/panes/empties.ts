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
 * No row offers a link any more. Overview had the only one — a human who had
 * just arrived landed on a page that could say nothing, so it sent them to
 * About this workspace — and Overview now reads the records and answers for
 * itself. The field stays, because a section that is still to come may want
 * one, and a row that offers nothing offers null.
 *
 * Not found is neither kind. An address that matches nothing has one honest
 * next step, so it keeps its action.
 *
 * The Project Partner's two rows are gone. It is connected now: the drawer and
 * the focused page render a real conversation, so the section says what it
 * read like every other projected one, and the subject panel that was going to
 * hold a pinned artifact is not in this slice at all.
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
