import { useEffect } from "react";

import { draftOf, draftSource, offersDraft, type Field } from "./drafting";
import { usePartner } from "./store";
import { useOpening } from "./Suggestion";

/**
 * "Ask about this" in a sheet's head.
 *
 * It is the same affordance every object on a page has, for the one thing that
 * had none: the sheet the human is filling in. Pressing it takes what is in
 * the fields at that moment, attaches it above the composer as a removable
 * chip, opens the drawer and puts the caret in the field. Nothing is sent —
 * the question is still the human's to write — and nothing about the sheet
 * reaches the Partner until they write it.
 *
 * It is offered only while there is something to offer. An empty form is not a
 * draft, and a chip saying "Draft: New goal" with nothing behind it would be
 * an offer of nothing.
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
  const { askAbout, offerFields } = usePartner();
  // A sheet that does not mint its own — the Project pane's, which hand their
  // fields over to be read and have nothing writable — still has one opening,
  // because what it hands over is still one draft of one thing.
  const mine = useOpening();
  const id = opening ?? mine;
  const draft = draftOf(id, sheet, fields, writable);
  const offers = offersDraft(draft);
  // Every render, because what these two functions close over is this render's
  // state: a registration kept from an earlier render would read, or write
  // into, the draft as it stood a keystroke ago.
  useEffect(() => {
    offerFields(id, {
      sheet,
      read: () => draftOf(id, sheet, fields, writable),
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
