import { useEffect } from "react";

import { draftOf, draftSource, offersDraft, type Field } from "./drafting";
import { usePartner } from "./store";
import { useOpening } from "./Suggestion";
import { writingIn } from "./suggesting";

/**
 * The hand-over: a sheet with writable fields offers its draft by opening, and
 * "Ask about this" in its head hands back a draft the human took away.
 *
 * Opening the sheet is the hand-over (g1-s52 D1). It was a press, and the press
 * was invisible: a human asked the Partner to rewrite an intent, the Partner
 * called suggest, the service refused it because nothing had been handed over,
 * and the refusal went to an activity line nobody sees. So the chip appears when
 * the sheet opens, with its × — which is what makes the sharing explicit rather
 * than silent, the one thing the master's rule about unsaved edits asks for.
 *
 * What the hand-over must NOT do is what Ask does. The drawer stays as the human
 * left it and the caret stays in the field they are typing in: they opened a
 * sheet in order to write in it, and an interface that moved their caret would be
 * deciding what they are doing. The chip's × leaves the draft out of every later
 * question, and bringing the chip up to date never brings it back — a draft
 * nobody is offering is not re-offered because the caret moved.
 *
 * "Ask about this" stays, for the one thing it is now for: handing back a draft
 * the × took away. It is offered only while there is something to offer. An empty
 * form is not a draft, and a chip saying "Draft: New goal" with nothing behind it
 * would be an offer of nothing.
 *
 * While this sheet is on screen it also registers the one opening it is: how to
 * read its fields, so that a question sent with the sheet still open carries the
 * draft as it stands — the chip's promise is the sheet's fields, not a copy of
 * them taken minutes ago — and, where the sheet supplies one, how to put words
 * into one of them. The fields are read at the moment a question is sent and at
 * no other: they are not watched, not streamed, and never read at all for a
 * sheet whose "Ask about this" nobody pressed. Nothing is written into them
 * except by a human's own press of Use this.
 *
 * A sheet that supplies no writable fields is registered exactly the same way
 * and can be offered nothing: its draft is handed over to be read, and the
 * Partner is told so.
 */
export function AskAboutSheet({
  sheet,
  fields,
  opening,
  writable = [],
  set,
}: {
  sheet: string;
  fields: Field[];
  /** The opening the sheet minted at mount, where the sheet owns one. */
  opening?: string;
  /** Which of those fields the Partner may offer words for, empty ones too. */
  writable?: string[];
  /** Put words in one of them, and answer what it held. */
  set?: (field: string, text: string) => string;
}) {
  const { askAbout, handOver, noteDraft, dropDraft, offerFields, writing } = usePartner();
  // A sheet that does not mint its own — the Project pane's, which hand their
  // fields over to be read and have nothing writable — still has one opening,
  // because what it hands over is still one draft of one thing.
  const mine = useOpening();
  const id = opening ?? mine;
  // Which of this sheet's own fields the caret is in. Another opening's caret is
  // not this draft's, so it reads as none.
  const inHand = writingIn(writing, id);
  const draft = draftOf(id, sheet, fields, writable, inHand);
  const offers = offersDraft(draft);
  // Every render, because what these two functions close over is this render's
  // state: a registration kept from an earlier render would read, or write
  // into, the draft as it stood a keystroke ago.
  useEffect(() => {
    offerFields(id, {
      sheet,
      read: () => draftOf(id, sheet, fields, writable, inHand),
      set: (field: string, text: string) => set?.(field, text) ?? "",
    });
  });
  // And one registration is taken back exactly once: when the sheet goes.
  useEffect(
    () => () => {
      offerFields(id, null);
    },
    [offerFields, id],
  );
  // The hand-over, once, when a sheet with writable fields opens — and the draft
  // is dropped when it closes, because the thing it described has gone.
  //
  // A sheet with nothing writable is not handed over by opening: it has no field
  // the Partner may be asked to write, so the only reason to attach it is that a
  // human asked, which is the button below.
  const writes = writable.length > 0;
  useEffect(() => {
    if (!writes) {
      return;
    }
    handOver(draftOf(id, sheet, fields, writable, ""));
    return () => {
      dropDraft(sheet);
    };
    // The fields are read at this moment and not watched: the effect runs when
    // the sheet opens, and what it attaches is what was in it then. The chip is
    // brought up to date below and at every send, which is why they are not
    // among the dependencies — a keystroke must not re-run the hand-over.
  }, [handOver, dropDraft, id, sheet, writes]);
  // The chip says where the caret is, so it is brought up to date when the field
  // in hand changes — through the refresh that never resurrects a draft, so the
  // × is not undone by moving the caret.
  useEffect(() => {
    if (inHand === "") {
      return;
    }
    noteDraft(draftOf(id, sheet, fields, writable, inHand));
    // Same reason: the fields are read at this moment, not watched, so they are
    // not dependencies either.
  }, [noteDraft, id, sheet, inHand]);
  return (
    <button
      type="button"
      className="ms-sheet-ask"
      disabled={!offers}
      title={
        offers
          ? `Offer ${draftSource(draft)} to your Project Partner, as it stands`
          : "Fill something in, and this offers it to your Project Partner."
      }
      onClick={() => {
        askAbout(draft);
      }}
    >
      Ask about this
    </button>
  );
}
