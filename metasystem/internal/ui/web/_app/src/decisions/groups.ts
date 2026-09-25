import type { GoalState, Need } from "./api";
import { ageLine } from "./decisions";

/**
 * The inbox as groups, which is what turns a wall into an inbox.
 *
 * Everything on this page used to weigh the same: a three-week-old review that
 * stays in force as written and a question asked yesterday were the same card
 * in the same place. A human arriving here asks one question first — is
 * anything waiting on me, and how much — and a page that answers it in two
 * seconds is a page of one line per kind, with how many, how old the newest
 * one is, and how much of it arrived since they were last here.
 *
 * So the whole inbox at rest is ten lines and one open group. Everything below
 * is a statement about those lines that a test can point at. Nothing here
 * reaches the network, reads a clock on its own, or renders anything.
 */

/** The groups, by the name this module knows each one under. */
export type GroupId =
  | "questions"
  | "drafts"
  | "landed"
  | "asks"
  | "alerts"
  | "renewals"
  | "stopped"
  | "parked"
  | "reviews"
  | "queue";

/**
 * The order, which is the design's own and is not a sort.
 *
 * It runs from the cheapest and freshest to the largest and quietest: the four
 * or five things a human can read and answer in a sitting first, then what the
 * steward addressed to them, then the goals and rulings that wait without
 * costing anything, and last the approval queue, which is a hundred rows and
 * is worked rather than read.
 */
const GROUPS: readonly { id: GroupId; kind: string; title: string; standing: string }[] = [
  { id: "questions", kind: "question", title: "Questions", standing: "" },
  { id: "drafts", kind: "draft", title: "Drafts to accept", standing: "" },
  { id: "landed", kind: "landed", title: "Designs landed", standing: "" },
  { id: "asks", kind: "ask", title: "Asks from a seat", standing: "" },
  { id: "alerts", kind: "alert", title: "Alerts", standing: "" },
  { id: "renewals", kind: "renewal", title: "Approvals to renew", standing: "" },
  { id: "stopped", kind: "stopped", title: "Stopped goals", standing: "" },
  { id: "parked", kind: "parked", title: "Parked by a seat", standing: "" },
  // The one quiet group: a ruling past its review date stays in force exactly
  // as written until a human says otherwise, so its line says that instead of
  // shouting an age. It is the group's own silence line, said once.
  { id: "reviews", kind: "ruling-review", title: "Rulings past review", standing: "they stay in force" },
  { id: "queue", kind: "approval", title: "Goals waiting for approval", standing: "" },
];

/** One group as the page shows it: a line, and the rows behind it. */
export type Group = {
  id: GroupId;
  title: string;
  needs: Need[];
  /** How many rows, which is what the line counts. */
  count: number;
  /** How old the newest row is, in the age line's own words, or "". */
  newest: string;
  /** How many of them were recorded since the last visit here. */
  fresh: number;
  /** What this group does if nobody answers it, where it is a quiet one. */
  standing: string;
};

/**
 * The groups with something in them, in the order above.
 *
 * An empty group is not shown at all. A line reading "Alerts 0" is a line a
 * human has to read to learn nothing, and ten of them are why this page was a
 * wall in the first place.
 */
export function groupsOf(needs: readonly Need[], now: Date): Group[] {
  const shown: Group[] = [];
  for (const group of GROUPS) {
    const mine = needs.filter((need) => need.kind === group.kind);
    if (mine.length === 0) {
      continue;
    }
    shown.push({
      id: group.id,
      title: group.title,
      needs: mine,
      count: mine.length,
      newest: newestAge(mine, now),
      fresh: mine.filter((need) => need.new).length,
      standing: group.standing,
    });
  }
  return shown;
}

/**
 * How old the newest row in a group is.
 *
 * The newest rather than the oldest, because the line answers "is there
 * anything here I have not seen", and a group whose newest row arrived this
 * morning is a different group from one nothing has touched in a month. A
 * group whose rows are all undated says nothing rather than "today".
 */
function newestAge(needs: readonly Need[], now: Date): string {
  let newest = "";
  for (const need of needs) {
    if (need.since !== "" && need.since > newest) {
      newest = need.since;
    }
  }
  return ageLine(newest, now);
}

/** What the "n new" part of a group's line says, or "" where none is. */
export function freshLine(group: Group): string {
  return group.fresh === 0 ? "" : `${String(group.fresh)} new`;
}

/**
 * Which group a page opens on: the one this viewer left open, else the first
 * with something new in it, else the first.
 *
 * The remembered choice wins because it is a choice: a human working through
 * the queue comes back to the queue. It loses only when it names a group this
 * inbox no longer has, which is what happens when the last draft is accepted —
 * and then the first group with something new is the honest answer, because
 * something new is the reason to be here at all.
 */
export function openGroup(groups: readonly Group[], remembered: string | null): GroupId | null {
  if (groups.length === 0) {
    return null;
  }
  const kept = groups.find((group) => group.id === remembered);
  if (kept !== undefined) {
    return kept.id;
  }
  return (groups.find((group) => group.fresh > 0) ?? groups[0]).id;
}

/* -------------------------------------------------------------- a row -- */

/**
 * The one line of substance a row is.
 *
 * Not a label for the thing: the thing. The question itself, the ruling's own
 * words, the record's title, the goal's intent. A row whose record carries
 * none of them falls back to its id, which is the master's rule for an
 * unresolved reference rather than a blank line.
 */
export function rowLine(need: Need): string {
  const said = substanceOf(need);
  return said === "" ? need.id : said;
}

function substanceOf(need: Need): string {
  switch (need.kind) {
    case "question":
      return need.asked;
    case "ruling-review":
      // The first sentence, because a ruling is a paragraph and a row is a
      // line; the whole of it is in the open row.
      return firstSentence(need.words);
    default:
      // Every other kind's title is already the substance the server read: a
      // record's own title, or a goal's intent cut at its first sentence.
      return need.title;
  }
}

/** A record's first statement, without the rest of it. */
export function firstSentence(text: string): string {
  const trimmed = text.trim();
  const cut = trimmed.search(/[.\n]/);
  return cut > 0 ? trimmed.slice(0, cut).trim() : trimmed;
}

/** How many labels a row's muted end carries before it stops. */
export const ROW_LABELS = 2;

/**
 * What stands at the muted end of a row: whose goal it is, its tier, a label
 * or two, and how old it is.
 *
 * Only what tells one row from its neighbours at a glance. The id is not here:
 * it is fifty characters of nothing to decide by, and it is in the open row
 * where a human who needs it is looking for it.
 */
export type RowFacts = { yours: boolean; tier: number; labels: string[]; age: string };

export function rowFacts(need: Need, now: Date): RowFacts {
  const row = need.row;
  return {
    yours: row !== null && row.origin === "human",
    tier: row === null ? 0 : row.tier,
    labels: (row === null ? [] : row.labels).slice(0, ROW_LABELS),
    age: ageLine(need.since, now),
  };
}

/* ------------------------------------------------- what an open row says -- */

/** The goals a landed design named, as one line each: the goal and its state. */
export function goalLine(goal: GoalState): string {
  return goal.state === "" ? `${goal.id} · not in this ledger` : `${goal.id} · ${goal.state}`;
}

/**
 * The two record statuses this page writes, which are the resolver's own
 * words: accepting a draft and marking a landed design done.
 */
export const ACCEPTED = "accepted";
export const DONE = "done";

/**
 * What a one-click write to a record says before it is sent.
 *
 * The confirmation is the whole of its safety, so it names the record and the
 * word it will write rather than asking "are you sure": a human who reads it
 * knows what will be in the file.
 */
export function confirmLine(need: Need, status: string): string {
  return `Write “Status: ${status}” on ${need.path === "" ? need.id : need.path}?`;
}

/** What the button that opens that confirmation says, per kind. */
export function writeLabel(kind: string): string {
  return kind === "landed" ? "Mark done" : "Accept";
}

/** Which status that write records, per kind. */
export function writeStatus(kind: string): string {
  return kind === "landed" ? DONE : ACCEPTED;
}

/* ----------------------------------------------------------- the header -- */

/**
 * The two views, with what is behind each one.
 *
 * The counts are in the names because they are what a human chooses by: a tab
 * that says "Inbox" tells them what is behind it, and one that says "Inbox 38"
 * tells them whether to open it first.
 */
export type ViewId = "inbox" | "decided";

export function viewTitle(view: ViewId, count: number): string {
  return `${view === "inbox" ? "Inbox" : "Decided"} ${String(count)}`;
}

/** How much this human has already said, which is what Decided holds. */
export function decidedCount(decided: {
  rulings: readonly unknown[];
  decisions: readonly unknown[];
  answered: readonly unknown[];
  approved: readonly unknown[];
  notNow: readonly unknown[];
}): number {
  return (
    decided.rulings.length +
    decided.decisions.length +
    decided.answered.length +
    decided.approved.length +
    decided.notNow.length
  );
}
