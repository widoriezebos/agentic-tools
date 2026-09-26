import { useRef, useState } from "react";

import { actingAs } from "./acting";
import { AskThePartner, FieldProposals } from "../partner/FieldProposals";
import { useOpening } from "../partner/Suggestion";
import { useFieldInHand } from "../partner/writing";
import { BacklogError, editGoal, loadBacklog, type Backlog, type Row } from "./api";
import {
  blockedForEdit,
  changedIn,
  draftOf,
  editNote,
  labelRefusal,
  outcomeOf,
  type EditDraft,
} from "./editing";
import { IN_FLIGHT, refusedSave, SAVED, type Outcome } from "../partner/suggesting";
import { INTENT_RULE, NEXT_STEP_RULE } from "./opening";
import { Panel } from "./Panel";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { TokenField } from "../shell/TokenField";

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
 *
 * There is one way out of this sheet to the ledger, and it takes the draft it is
 * to send as an argument: `submit`. Save calls it with what the fields hold, and
 * a proposal's Use and save calls it with the draft the words it is putting in
 * would make — because a second caller that set the words and then asked the
 * render what they were would send the draft of a render that has not happened
 * (Astra F1 on g1-s52). It guards a save in flight in a ref rather than in
 * render state, for the same reason: two presses in one render would both see
 * `sending` false. And it answers what happened rather than that it tried, by a
 * conservative reading of what the route said (D1, Astra F1 on g1-s56).
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
  onReread,
}: {
  goal: Row;
  backlog: Backlog;
  onClose: () => void;
  /** The ledger as it stands after the act, which is what the page re-reads. */
  onDone: (edited: Backlog, id: string) => void;
  /**
   * The ledger as it stands after a save nobody could confirm, where the page
   * that opened this sheet has somewhere to put it.
   *
   * It is a read and not a resend: an act this page could not confirm may have
   * landed, so the row it shows may be stale, and the one honest thing to do
   * about that is to look (D1). The sheet stays open, because the words saying
   * the save was not confirmed are in it.
   */
  onReread?: (after: Backlog) => void;
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
  // Whether a save is in flight, held where a press in the same render can see
  // it. `sending` is for the panel and for the button's word; this is the guard.
  const inFlight = useRef(false);
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

  /**
   * Send this draft, and answer what the ledger did with it.
   *
   * The draft is the argument and not the render's, which is the whole point:
   * the delta is measured from what the sheet opened with to the draft the
   * caller is sending, so a press that sets words and sends them in one act
   * sends those words. Everything Save had is here — its validation, its
   * once-only sign-in retry, its busy guard on the panel, and its closing of the
   * sheet when the ledger confirms — and nothing that was not.
   */
  const submit = async (next: EditDraft): Promise<Outcome> => {
    const asked = changedIn(opened, next);
    const stop = blockedForEdit(next, asked);
    if (stop !== "") {
      // This page's own no, in the words the note under the button already
      // carries. Nothing was sent, so nothing is said about the ledger.
      return refusedSave(stop);
    }
    // Held in a ref and read synchronously: a second press in the same render
    // as the first must see the first one, and render state cannot show it.
    if (inFlight.current) {
      return refusedSave(IN_FLIGHT);
    }
    inFlight.current = true;
    setSending(true);
    setRefusal("");
    try {
      const edited = await editGoal(goal.ref.id, asked);
      inFlight.current = false;
      setSending(false);
      onDone(edited, goal.ref.id);
      return SAVED;
    } catch (error: unknown) {
      inFlight.current = false;
      setSending(false);
      // The remedy for this one is in this page: the sign-in sheet opens and
      // the act is retried once, and what this press answers is what that
      // retry answered — so a card cannot say a save was refused for want of a
      // human and then have it land behind the words. A sign-in the human walks
      // away from leaves this press unanswered, and the card goes on saying the
      // save is in hand: that says less than the truth rather than more, and
      // Save at the foot of the sheet is still theirs.
      if (error instanceof BacklogError && error.signIn && !retried.current) {
        retried.current = true;
        return new Promise<Outcome>((settle) => {
          askToSignIn(() => {
            void submit(next).then(settle);
          });
        });
      }
      const outcome = outcomeOf(error);
      setRefusal(outcome.words);
      if (outcome.kind === "unresolved" && onReread !== undefined) {
        // Nothing is sent again, ever, by this page: an act nobody could confirm
        // may have landed. What is done instead is one read, so the page stops
        // showing a row the ledger may have moved.
        loadBacklog()
          .then((after) => {
            onReread(after);
          })
          .catch(() => {
            // The words above already say the save was not confirmed; a failed
            // read on top of it has nothing to add.
          });
      }
      return outcome;
    }
  };

  const send = () => {
    void submit(draft);
  };

  /**
   * Put the Partner's words in one of the three and send the sheet, as one act.
   *
   * The next draft is built here, set here, and sent here, all from the same
   * value: the sheet never asks a later render what it now holds, because there
   * is no later render between the two halves of one press (D2).
   */
  const putWordsAndSave = (field: string, text: string): Promise<Outcome> => {
    const at = WRITABLE_FIELDS[field];
    if (at === undefined) {
      return Promise.resolve(refusedSave(`${field} is not a field of this sheet`));
    }
    const next = { ...draft, [at]: text };
    setDraft(next);
    return submit(next);
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
      save={putWordsAndSave}
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
        <FieldProposals opening={opening} field="Intent" value={draft.intent} saves />
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
        <FieldProposals opening={opening} field="Next step" value={draft.nextStep} saves />
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
        <FieldProposals opening={opening} field="Labels" value={draft.labels} saves />
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
