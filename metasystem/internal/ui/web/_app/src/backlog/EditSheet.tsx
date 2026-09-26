import { useRef, useState } from "react";

import { actingAs } from "./acting";
import { AskThePartner, FieldProposals } from "../partner/FieldProposals";
import { useOpening } from "../partner/Suggestion";
import { useFieldInHand } from "../partner/writing";
import { BacklogError, editGoal, type Backlog, type Row } from "./api";
import { blockedForEdit, changedIn, draftOf, editNote, labelRefusal, type EditDraft } from "./editing";
import { INTENT_RULE, NEXT_STEP_RULE } from "./opening";
import { Panel } from "./Panel";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { TokenField } from "../shell/TokenField";
import { failureMessage } from "../shell/workspace";

/**
 * A queued goal's three fields, before the edit is published.
 *
 * It is the open sheet's common case over again — what done looks like, what
 * to take on first, and the words the board narrows by — with the fields
 * filled in from the row rather than empty, because this goal already says
 * all three and a human is here to change one of them. The rules under the
 * two lines are the intake rules, word for word: there is one rule for what
 * an intent is, and a goal written at a terminal and a goal written here are
 * held to it equally.
 *
 * What it sends is the part that is not obvious. The sheet remembers what it
 * opened with and sends only the fields that differ from it, so a next step
 * somebody changed at a terminal while this sheet was open is left alone by a
 * save that only touched the intent. A field nobody touched is absent from
 * the body rather than present with a stale copy of it, which is the whole
 * difference between an edit and an overwrite.
 *
 * All three are fields the Project Partner may be offered words for, and the
 * sheet is the one place that can say so: it owns the draft, so it owns both
 * the list and the setter that writes into it. The goal this edit is of is
 * handed over beside them and is not writable — it is what is being edited,
 * not a field anybody rewrites here.
 *
 * Every refusal but the two blanks and the label grammar is the engine's own
 * and is shown in its own words: an approval or a claim that landed since the
 * page read the row is refused at the tip, and the sheet says so with the
 * fields still filled in and nothing moved.
 */
/**
 * The three fields of this sheet the Partner may offer words for, in the order
 * the sheet asks them, each beside the draft key it writes into.
 *
 * It is written once, here, because three things read it: what the sheet tells
 * the store is writable, the setter that writes into one of them, and the link
 * each label carries. A field named in one and missing from another would be a
 * suggestion offered for something nothing can write.
 */
const WRITABLE_FIELDS: Record<string, keyof EditDraft> = {
  Intent: "intent",
  "Next step": "nextStep",
  Labels: "labels",
};

export function EditSheet({
  goal,
  backlog,
  onClose,
  onDone,
}: {
  goal: Row;
  backlog: Backlog;
  onClose: () => void;
  /** The ledger as it stands after the act, which is what the page re-reads. */
  onDone: (edited: Backlog, id: string) => void;
}) {
  // What the sheet opened with, kept as it was: it is what the fields were
  // filled from and it is what "changed" is measured against, so the two
  // cannot drift apart while a human types.
  const [opened] = useState(() => draftOf(goal));
  const [draft, setDraft] = useState(opened);
  // This opening of this sheet, minted once. It is what a suggestion for one of
  // these fields belongs to.
  const opening = useOpening();
  // Which of the three the caret is in, reported by the field's own row: the
  // Partner is told it, so "make this shorter" means the field they are in.
  const inHand = useFieldInHand(opening);
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const { session, askToSignIn } = useSession();
  const retried = useRef(false);
  const authority = actingAs(backlog.authority, session);
  const edit = changedIn(opened, draft);
  const blocked = blockedForEdit(draft, edit);
  const labels = labelRefusal(draft.labels);
  // What this board already carries, which is what a half-typed label is
  // suggested from and what marks one nobody has used before.
  const known = [...new Set([...backlog.rows, ...backlog.closed].flatMap((row) => row.labels))].sort();

  /**
   * Put the Partner's words in one of the three, and answer what was there.
   *
   * It replaces the field's whole value, which is what a suggestion is: the
   * field as the Partner would write it. What comes back is what the human had,
   * and it is read here at the moment of the press rather than remembered
   * earlier, so Undo puts back what was actually replaced.
   */
  const putWords = (field: string, text: string): string => {
    const at = WRITABLE_FIELDS[field];
    if (at === undefined) {
      return "";
    }
    const was = draft[at];
    setDraft({ ...draft, [at]: text });
    return was;
  };

  const send = () => {
    if (blocked !== "") {
      return;
    }
    setSending(true);
    setRefusal("");
    editGoal(goal.ref.id, edit)
      .then((edited) => {
        setSending(false);
        onDone(edited, goal.ref.id);
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof BacklogError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  return (
    <Panel
      form
      // The panel's goal card is for the acts that need to name what they act
      // on; here the intent is the first field, so the card would only say it
      // twice (Wido, 2026-09-26). The id goes in the eyebrow instead.
      eyebrow={`${goal.ref.id} · queued, not yet approved`}
      title="Edit goal"
      // What the head's Ask hands over: the three fields as they stand.
      fields={[
        { name: "Goal", value: goal.ref.id },
        { name: "Intent", value: draft.intent },
        { name: "Next step", value: draft.nextStep },
        { name: "Labels", value: draft.labels },
      ]}
      opening={opening}
      writable={Object.keys(WRITABLE_FIELDS)}
      set={putWords}
      unproven={authority.proven ? "" : authority.reason}
      refusal={refusal}
      note={blocked === "" ? editNote(goal.ref.id, edit) : blocked}
      // A save in flight goes on whether or not this sheet is on screen, so
      // while it is in flight the sheet cannot be dismissed: Cancel is
      // disabled and Escape does nothing. Without this a human who pressed
      // Cancel would watch the sheet close and the edit land anyway.
      busy={sending}
      onClose={onClose}
      act={
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          {sending ? "Saving…" : "Save"}
        </Button>
      }
    >
      {/* Each field's own row reports the caret, so what the Partner is told is
          where the human actually was. Focus bubbles, so the row answers for the
          control inside it and for its own "Ask the Partner". */}
      <div
        className="ms-act-field"
        onFocus={() => {
          inHand("Intent");
        }}
      >
        <div className="ms-act-label">
          <label htmlFor="ms-edit-intent">Intent</label>
          <AskThePartner opening={opening} field="Intent" value={draft.intent} />
        </div>
        <textarea
          id="ms-edit-intent"
          rows={2}
          value={draft.intent}
          aria-describedby="ms-edit-intent-hint"
          onChange={(event) => {
            setDraft({ ...draft, intent: event.target.value });
          }}
        />
        <FieldProposals opening={opening} field="Intent" value={draft.intent} />
        <p className="ms-act-hint" id="ms-edit-intent-hint">
          {INTENT_RULE}
        </p>
      </div>

      <div
        className="ms-act-field"
        onFocus={() => {
          inHand("Next step");
        }}
      >
        <div className="ms-act-label">
          <label htmlFor="ms-edit-nextStep">Next step</label>
          <AskThePartner opening={opening} field="Next step" value={draft.nextStep} />
        </div>
        <textarea
          id="ms-edit-nextStep"
          rows={2}
          value={draft.nextStep}
          aria-describedby="ms-edit-nextStep-hint"
          onChange={(event) => {
            setDraft({ ...draft, nextStep: event.target.value });
          }}
        />
        <FieldProposals opening={opening} field="Next step" value={draft.nextStep} />
        <p className="ms-act-hint" id="ms-edit-nextStep-hint">
          {NEXT_STEP_RULE}
        </p>
      </div>

      <div
        className="ms-act-field"
        onFocus={() => {
          inHand("Labels");
        }}
      >
        <div className="ms-act-label">
          <label htmlFor="ms-edit-labels">Labels</label>
          <AskThePartner opening={opening} field="Labels" value={draft.labels} />
        </div>
        <TokenField
          id="ms-edit-labels"
          value={draft.labels}
          known={known}
          placeholder="e.g. ui board"
          onChange={(value) => {
            setDraft({ ...draft, labels: value });
          }}
        />
        <FieldProposals opening={opening} field="Labels" value={draft.labels} />
        <p className="ms-act-hint" id="ms-edit-labels-hint">
          Lowercase words the board narrows by, separated by spaces or commas. Emptying the field clears them.
        </p>
        {labels !== "" && (
          <p className="ms-act-refuse" role="alert">
            {labels}
          </p>
        )}
      </div>
    </Panel>
  );
}
