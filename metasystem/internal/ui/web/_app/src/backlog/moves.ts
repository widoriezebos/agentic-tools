/**
 * What a drop on the board means.
 *
 * Two moves are real acts a human performs, and they are the two the engine
 * has a human verb for: To Do to Ready for Work is `goal approve`, and Ready
 * for Work back to To Do is `goal unapprove`. Every other column is a lane
 * the ledger reads out of records somebody else writes — a seat's claim, a
 * park with its reason, a landing — and dragging a card there would be the
 * browser inventing the fact the lane is a reading of. So every other drop is
 * refused in place, with one line saying what the move would have meant and
 * that this build does not wire it.
 *
 * The table is here, apart from the board, because it is the rule and not the
 * rendering: the same two moves are offered to a mouse and to the keyboard's
 * Move menu, and one table answers both.
 */

import type { Budget, Row } from "./api";
import { lanes, type LaneId } from "./lanes";

/** The act a drop asks for. */
export type Move = "approve" | "withdraw";

/** One allowed move: where from, where to, and what it publishes. */
export type Transition = { from: LaneId; to: LaneId; move: Move; verb: string };

/**
 * The whole table. It is a list rather than a map because that is what it is:
 * two transitions, named, each one a verb the engine already has.
 */
export const transitions: readonly Transition[] = [
  { from: "to-do", to: "ready", move: "approve", verb: "goal approve" },
  { from: "ready", to: "to-do", move: "withdraw", verb: "goal unapprove" },
];

/** The move a drop from one lane onto another asks for, or null. */
export function moveFor(from: LaneId, to: LaneId): Transition | null {
  return transitions.find((transition) => transition.from === from && transition.to === to) ?? null;
}

/** Every lane a card in this lane may be dropped on. */
export function targetsFrom(from: LaneId): LaneId[] {
  return transitions.filter((transition) => transition.from === from).map((transition) => transition.to);
}

/**
 * Why a drop on this column is refused, in one line, or the empty string when
 * it is not refused at all.
 *
 * A lane says what it would take to be in it, because that is the fact the
 * human is missing. Dropping a card on the lane it is already in is not a
 * move and says so plainly.
 */
export function refusalFor(from: LaneId, to: LaneId): string {
  if (moveFor(from, to) !== null) {
    return "";
  }
  if (from === to) {
    return `This goal is already in ${titleOf(to)}.`;
  }
  return MEANINGS[to] ?? `Moving work to ${titleOf(to)} is not something this build can publish.`;
}

/**
 * What each column would need, said as what the record behind it is. These
 * are not apologies: each one names the act or the party that writes the lane
 * a card would have to be read into.
 */
const MEANINGS: Partial<Record<LaneId, string>> = {
  draft: "Draft is a proposal that has not passed intake; this build has no reader for the drafts directory.",
  "to-do": "To Do is work that is not authorized; only withdrawing an approval puts a goal back there.",
  ready: "Ready for Work is approved work the claim gate admits; only an approval puts a goal there.",
  "in-progress": "In Progress is a seat's claim, which a seat makes when the engine runs. The browser cannot claim work.",
  review: "Review and Verification is read from a claim that has built and is waiting to land; nothing here can record that.",
  waiting: "Waiting is a park with a reason, or an authoritative blocker. Park it from the terminal with goal park.",
  done: "Done is a landing with its evidence. Completing implementation is not completing a goal, and no drop can stand in for either.",
  abandoned: "Abandoned is work dropped with a recorded reason. Drop it from the terminal with goal abandon.",
  unknown: "Unknown is not a lane work can be put into; it is what this build says when it cannot place a record.",
};

function titleOf(lane: LaneId): string {
  return lanes.find((candidate) => candidate.id === lane)?.title ?? lane;
}

/* ------------------------------------------------------------ the budget -- */

/** Where a prefilled budget came from, which the sheet says out loud. */
export type BudgetSource = "project" | "goal" | "last-approved" | "none";

export type Prefill = { budget: Budget | null; source: BudgetSource };

/**
 * The budget the approval sheet opens with, and where it came from.
 *
 * A human attaches the budget, and the machinery never invents values. So the
 * sheet prefers, in this order: the tuple this goal already carries, because
 * a goal that has a budget has one a human chose for it and an approval
 * writes whatever it is given; then the project's own declared law for the
 * goal's tier; then the tuple of the goal approved most recently, which is
 * the nearest thing to a house style the ledger has; and otherwise nothing at
 * all, leaving five required fields. Whatever it shows, the sheet says where
 * it came from and the human reads and confirms all five every time.
 */
export function prefillFor(
  goal: Row,
  defaults: Partial<Record<string, Budget>>,
  rows: readonly Row[],
): Prefill {
  if (goal.budget !== undefined) {
    return { budget: goal.budget, source: "goal" };
  }
  const declared = defaults[String(goal.tier === 0 ? 3 : goal.tier)];
  if (declared !== undefined) {
    return { budget: declared, source: "project" };
  }
  const recent = mostRecentlyApproved(rows);
  if (recent !== null) {
    return { budget: recent, source: "last-approved" };
  }
  return { budget: null, source: "none" };
}

/** The budget of the goal whose approval is the latest one recorded. */
function mostRecentlyApproved(rows: readonly Row[]): Budget | null {
  let at = "";
  let budget: Budget | null = null;
  for (const row of rows) {
    if (row.approved === undefined || row.budget === undefined || row.approved.at <= at) {
      continue;
    }
    at = row.approved.at;
    budget = row.budget;
  }
  return budget;
}

/* ------------------------------------------------------------- the sheet -- */

/** The fields of the approval sheet, as typed rather than as parsed. */
export type BudgetDraft = {
  elapsedLimit: string;
  attemptLimit: string;
  reservedJobMinutesLimit: string;
  activeJobLimit: string;
  reviewRoundLimit: string;
};

export const emptyBudgetDraft: BudgetDraft = {
  elapsedLimit: "",
  attemptLimit: "",
  reservedJobMinutesLimit: "",
  activeJobLimit: "",
  reviewRoundLimit: "",
};

export function draftOf(budget: Budget | null): BudgetDraft {
  if (budget === null) {
    return emptyBudgetDraft;
  }
  return {
    elapsedLimit: budget.elapsedLimit,
    attemptLimit: String(budget.attemptLimit),
    reservedJobMinutesLimit: String(budget.reservedJobMinutesLimit),
    activeJobLimit: String(budget.activeJobLimit),
    reviewRoundLimit: String(budget.reviewRoundLimit),
  };
}

/**
 * The tuple a draft makes, or null where it does not make one. The engine
 * validates it again and has the last word; this only keeps the sheet from
 * sending something that is plainly not a budget.
 */
export function budgetOf(draft: BudgetDraft): Budget | null {
  const elapsed = draft.elapsedLimit.trim();
  const attempts = positive(draft.attemptLimit);
  const minutes = positive(draft.reservedJobMinutesLimit);
  const active = positive(draft.activeJobLimit);
  const rounds = whole(draft.reviewRoundLimit);
  if (!DURATION.test(elapsed) || attempts === null || minutes === null || active === null || rounds === null) {
    return null;
  }
  return {
    elapsedLimit: elapsed,
    attemptLimit: attempts,
    reservedJobMinutesLimit: minutes,
    activeJobLimit: active,
    reviewRoundLimit: rounds,
  };
}

/** One or more positive day, hour, or minute segments, and nothing else. */
const DURATION = /^(?:[1-9][0-9]*[dhm])+$/;

function positive(value: string): number | null {
  const parsed = whole(value);
  return parsed === null || parsed < 1 ? null : parsed;
}

function whole(value: string): number | null {
  const trimmed = value.trim();
  return /^[0-9]+$/.test(trimmed) ? Number(trimmed) : null;
}

/**
 * Why the sheet's own button is disabled, or the empty string when it is not.
 *
 * A server that cannot act as the human says so here, in the same words the
 * route would answer with, so the human reads the reason before they press
 * rather than after. An incomplete budget is the other reason, and it is the
 * human's own to fix.
 */
export function blockedFor(
  move: Move,
  proven: boolean,
  reason: string,
  draft: BudgetDraft,
): string {
  if (!proven) {
    return reason === "" ? "This interface cannot act as a human." : reason;
  }
  if (move === "approve" && budgetOf(draft) === null) {
    return "An approval carries the complete budget: an elapsed limit such as 4h, and positive attempt, reserved-minute and active-job limits, with the review rounds.";
  }
  return "";
}

/** What the sheet promises an approval will do, named for the human. */
export function approveNote(human: string): string {
  const who = human === "" ? "the enrolled human" : `human:${human}`;
  return `Publishes goal approve as ${who}; an eligible seat may claim it as soon as the engine runs.`;
}

/** What the sheet says a withdrawal will do, including the race it cannot win. */
export const withdrawNote =
  "Publishes goal unapprove and returns the goal to To Do. If a seat has claimed it since this page was read, the engine parks the work with this reason rather than pretending it never began; stop that seat from the terminal if that is not what you want.";
