import { useRef, useState } from "react";

import { actingAs } from "./acting";
import { BacklogError, editGoal, type Backlog, type Row } from "./api";
import { blockedForEdit, changedIn, draftOf, editNote, labelRefusal } from "./editing";
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
 * Every refusal but the two blanks and the label grammar is the engine's own
 * and is shown in its own words: an approval or a claim that landed since the
 * page read the row is refused at the tip, and the sheet says so with the
 * fields still filled in and nothing moved.
 */
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
      eyebrow="Queued, not yet approved"
      title="Edit goal"
      goal={goal}
      // What the head's Ask hands over: the three fields as they stand.
      fields={[
        { name: "Goal", value: goal.ref.id },
        { name: "Intent", value: draft.intent },
        { name: "Next step", value: draft.nextStep },
        { name: "Labels", value: draft.labels },
      ]}
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
      <div className="ms-act-field">
        <label htmlFor="ms-edit-intent">Intent</label>
        <textarea
          id="ms-edit-intent"
          rows={2}
          value={draft.intent}
          aria-describedby="ms-edit-intent-hint"
          onChange={(event) => {
            setDraft({ ...draft, intent: event.target.value });
          }}
        />
        <p className="ms-act-hint" id="ms-edit-intent-hint">
          {INTENT_RULE}
        </p>
      </div>

      <div className="ms-act-field">
        <label htmlFor="ms-edit-nextStep">Next step</label>
        <textarea
          id="ms-edit-nextStep"
          rows={2}
          value={draft.nextStep}
          aria-describedby="ms-edit-nextStep-hint"
          onChange={(event) => {
            setDraft({ ...draft, nextStep: event.target.value });
          }}
        />
        <p className="ms-act-hint" id="ms-edit-nextStep-hint">
          {NEXT_STEP_RULE}
        </p>
      </div>

      <div className="ms-act-field">
        <label htmlFor="ms-edit-labels">Labels</label>
        <TokenField
          id="ms-edit-labels"
          value={draft.labels}
          known={known}
          placeholder="e.g. ui board"
          onChange={(value) => {
            setDraft({ ...draft, labels: value });
          }}
        />
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
