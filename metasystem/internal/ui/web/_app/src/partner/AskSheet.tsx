import { useEffect } from "react";

import { draftOf, draftSource, offersDraft, type Field } from "./drafting";
import { usePartner } from "./store";

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
 * While this sheet is on screen it also says how to read its fields, so that a
 * question sent with the sheet still open carries the draft as it stands — the
 * chip's promise is the sheet's fields, not a copy of them taken minutes ago.
 * It is read at the moment a question is sent and at no other: the fields are
 * not watched, not streamed, and never read at all for a sheet whose "Ask
 * about this" nobody pressed.
 */
export function AskAboutSheet({ sheet, fields }: { sheet: string; fields: Field[] }) {
  const { askAbout, offerFields } = usePartner();
  const draft = draftOf(sheet, fields);
  const offers = offersDraft(draft);
  useEffect(() => {
    offerFields(sheet, () => draftOf(sheet, fields));
    return () => {
      offerFields(sheet, null);
    };
  });
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
