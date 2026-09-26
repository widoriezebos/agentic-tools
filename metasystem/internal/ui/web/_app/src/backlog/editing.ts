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
 *
 * The third part, at the foot, is what an answer means. A save that reports
 * itself has to be conservative about the one thing it cannot see: whether an
 * act it did not get a confirmation for landed anyway.
 */

import { BacklogError, type Row } from "./api";
import { labelsOf, oneLine } from "./opening";
import { refusedSave, unresolvedSave, type NotSaved } from "../partner/suggesting";
import { failureMessage } from "../shell/workspace";

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

/* ------------------------------------------- what came back, and what it means -- */

/**
 * The code the route answers with when the act LANDED and the proof of who made
 * it did not (internal/ui/act/act.go settle).
 *
 * It is the one answer that says, in its own words, that the ledger moved and
 * that the act must not be run again. Treating it as a refusal would tell a human
 * the opposite of what happened and invite the one press that would do harm, so
 * it is named here rather than read off a status.
 */
export const LANDED_NOT_RECORDED = "proof-not-recorded";

/**
 * The ledger outcomes that are not a rejection: the journal did not confirm the
 * act, and it did not say the act was refused either.
 *
 * `confirmed-late` means it landed after the wait was over; `lost` means nobody
 * knows; `abandoned` and `expired` are operations the engine stopped waiting for.
 * The route carries each as the code of a 409, beside the rejections, so the code
 * is what tells them apart (internal/goal/journal.go Outcome).
 */
const UNSETTLED = ["confirmed-late", "lost", "abandoned", "expired"];

/**
 * What a failed save means, conservatively: refused where nothing landed, and
 * unresolved wherever this page cannot know.
 *
 * The act layer has no unresolved of its own — every publication error is
 * collapsed into a refusal — so the mapping is made here, from what the route
 * says rather than from what it means to call it (Astra F1 on g1-s56):
 *
 *   - a definite rejection is a refusal in the engine's own sentence: the
 *     request was wrong, the seat is not a human's, or the ledger refused the
 *     act in the state it is in, which is the 4xx family;
 *   - an engine that could not answer at all is unresolved, which is the 5xx
 *     family, and the answer that says the act landed but its proof did not is
 *     kept in its own words inside it;
 *   - a ledger outcome that neither confirms nor rejects is unresolved,
 *     whatever status carried it;
 *   - and anything that is not a refusal the server explained — a transport
 *     that failed, a body nobody could parse — is unresolved, because a request
 *     that never came back may still have been received.
 *
 * A confirmed answer never reaches here: it is a backlog, and the caller has it,
 * which is why this answers with the two that carry words and never with saved.
 */
export function outcomeOf(error: unknown): NotSaved {
  if (!(error instanceof BacklogError)) {
    return unresolvedSave(failureMessage(error));
  }
  if (error.code === LANDED_NOT_RECORDED || error.status >= 500 || UNSETTLED.includes(error.code)) {
    return unresolvedSave(error.message);
  }
  return refusedSave(error.message);
}
