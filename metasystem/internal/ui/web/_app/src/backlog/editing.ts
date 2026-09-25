/**
 * What `goal edit` takes from this browser, and what it does not.
 *
 * Three fields: the intent, the next step and the labels. Not the tier and
 * not the risk, because both enter the digest a human approved and changing
 * one silently would change what the approval meant; not the dependencies,
 * which have block and unblock already and change other goals' readiness; not
 * the why and not the evidence, which belong to the acts that need them. That
 * is the one direct edit the master design admits without a proposal, and a
 * sheet offering more would be promising a record this act cannot write.
 *
 * The other half of this file is subtraction. A save sends the fields whose
 * value differs from what the sheet opened with and no others, because the
 * engine's fields are nullable and a browser that republished all three would
 * overwrite a terminal's edit of a field the human never touched with a copy
 * of what the page read minutes ago.
 */

import type { Row } from "./api";
import { labelsOf, oneLine } from "./opening";

/** The three fields as they are typed, before they are anything else. */
export type EditDraft = {
  intent: string;
  nextStep: string;
  /** Space- or comma-separated, as the token field carries them. */
  labels: string;
};

/**
 * The draft a sheet opens on: the goal as the ledger has it. It is what the
 * fields are filled with and it is also what "changed" is measured against,
 * so the two can never disagree about what this human started from.
 */
export function draftOf(row: Row): EditDraft {
  return { intent: row.intent, nextStep: row.nextStep, labels: row.labels.join(" ") };
}

/**
 * What travels to the edit route. Every field is optional, and an absent one
 * is not a field set to nothing: it is a field this save says nothing about,
 * which the engine leaves exactly as it found it.
 */
export type GoalEdit = { intent?: string; nextStep?: string; labels?: string[] };

/**
 * The fields that differ from what the sheet opened with.
 *
 * The two lines are compared folded, because that is the form they travel in:
 * a human who added a line break to an intent changed nothing the ledger can
 * record. The labels are compared canonically — sorted and deduplicated, as
 * the engine writes them — so reordering the words a human already had is not
 * a change either, while emptying the field is, and travels as an empty list
 * that clears them.
 */
export function changedIn(opened: EditDraft, draft: EditDraft): GoalEdit {
  const edit: GoalEdit = {};
  const intent = oneLine(draft.intent);
  if (intent !== oneLine(opened.intent)) {
    edit.intent = intent;
  }
  const nextStep = oneLine(draft.nextStep);
  if (nextStep !== oneLine(opened.nextStep)) {
    edit.nextStep = nextStep;
  }
  const labels = labelsOf(draft.labels);
  if (canonical(labels) !== canonical(labelsOf(opened.labels))) {
    edit.labels = labels;
  }
  return edit;
}

/** The labels as the ledger keeps them, which is what two lists are compared by. */
function canonical(labels: readonly string[]): string {
  return [...new Set(labels)].sort().join(" ");
}

/** True when this save would say nothing, which is not a save. */
export function nothingChanged(edit: GoalEdit): boolean {
  return Object.keys(edit).length === 0;
}

/**
 * The label grammar, as `internal/goal/goal.go` writes it, so the field can
 * say no before the act is sent rather than after it is refused.
 */
const LABEL = /^[a-z][a-z0-9-]{0,31}$/;

/**
 * What the labels themselves refuse, in the engine's own sentence, or "".
 * The words are the engine's because the refusal is: a human who reads one
 * sentence here and another one from a terminal has been told there are two
 * rules.
 */
export function labelRefusal(typed: string): string {
  for (const label of labelsOf(typed)) {
    if (!LABEL.test(label)) {
      return `label "${label}" must match ${LABEL.source}`;
    }
  }
  return "";
}

/**
 * Why the sheet's own button is disabled, or "" when it is not.
 *
 * Nothing about proof is here, as nothing about proof is in the open sheet's:
 * the act asks, and a server with no human behind it answers with the sign-in
 * this page opens and the act it then retries.
 */
export function blockedForEdit(draft: EditDraft, edit: GoalEdit): string {
  if (draft.intent.trim() === "") {
    return "The intent says what done looks like, in one line.";
  }
  if (draft.nextStep.trim() === "") {
    return "The next step states intent, constraints and freedoms — never a script of the how.";
  }
  const label = labelRefusal(draft.labels);
  if (label !== "") {
    return label;
  }
  if (nothingChanged(edit)) {
    return "Nothing has changed yet. A save sends the fields you changed, and this one would send none.";
  }
  return "";
}

/**
 * True for the one goal this browser may edit directly: queued, and carrying
 * no approval.
 *
 * It reads the state rather than the lane. A lane is a reading — `waiting`
 * covers an approved goal held up by a dependency as well as an unapproved
 * one — and the question here is what the ledger will admit, which is a
 * question about the goal's state and its approval and about nothing else.
 */
export function editable(row: Row): boolean {
  return row.state === "queued" && row.approved === undefined;
}

/**
 * Why this goal is not edited here, in words that name the act that would let
 * it be, or "" where there is nothing to say.
 *
 * Three states have an answer, and each answer is a different act: an
 * approval is withdrawn, a claim is another seat's and is edited where that
 * seat is, and a park is lifted. A goal that is over has no answer, and a
 * sentence inventing one would be offering an edit that does not exist.
 */
export function editReason(row: Row): string {
  if (editable(row)) {
    return "";
  }
  // The state is read before the approval, as the mutation reads them: an
  // approval survives a claim and survives a park, so a goal a seat holds
  // would otherwise be said to need its approval withdrawn when what stands
  // in the way is the claim.
  if (row.state === "claimed") {
    return `claimed by ${pairOf(row)}: edit at a terminal`;
  }
  if (row.state === "parked") {
    return "parked: return it to the queue to edit";
  }
  if (row.state === "approved" || row.approved !== undefined) {
    return "approved: withdraw the approval to edit the intent";
  }
  return "";
}

/** The pair holding a claim, as the engine names one, or the word for none. */
function pairOf(row: Row): string {
  return row.claim === undefined ? "another pair" : `${row.claim.machine}+${row.claim.lineage}`;
}

/** What the sheet says the act will do, and what it will leave alone. */
export function editNote(id: string, edit: GoalEdit): string {
  const fields = Object.keys(edit);
  if (fields.length === 0) {
    return `Publishes nothing for ${id} until something changes.`;
  }
  return `Publishes goal edit for ${id}, sending ${named(fields)} and leaving every other field as the ledger has it.`;
}

/** The changed fields as a human reads them, in the order the sheet asks them. */
function named(fields: readonly string[]): string {
  const words = ORDER.filter((field) => fields.includes(field.key)).map((field) => field.name);
  if (words.length === 1) {
    return words[0];
  }
  return `${words.slice(0, -1).join(", ")} and ${words[words.length - 1]}`;
}

const ORDER = [
  { key: "intent", name: "the intent" },
  { key: "nextStep", name: "the next step" },
  { key: "labels", name: "the labels" },
];
