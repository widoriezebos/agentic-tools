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
 */
export function AskAboutSheet({ sheet, fields }: { sheet: string; fields: Field[] }) {
  const { askAbout } = usePartner();
  const draft = draftOf(sheet, fields);
  const offers = offersDraft(draft);
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
