import { useCallback } from "react";

import { usePartner } from "./store";

/**
 * Which field the human is writing in, reported by the field itself.
 *
 * It is the answer to one question Wido asked on 2026-09-26: "do you know which
 * field I was editing when I started editing in the project partner panel?
 * Because you will have to." Without it, "make this shorter" names nothing: the
 * capture carries a sheet's fields in the order the sheet asks them, and nothing
 * says which one the caret was in.
 *
 * The sheet reports it, because the sheet owns the fields. The store keeps the
 * last one, the chip says it, the capture carries it as the draft's `writing`,
 * and the Partner's context block says "The human was writing in Intent" or that
 * there was no field yet. The last one stands: blurring a field does not put the
 * human nowhere, it leaves them where they were, and a request written in the
 * composer is about the field they came from.
 */

/**
 * The handler a sheet gives each of its writable fields: this field now has the
 * caret.
 *
 * It is one hook rather than a wrapper component so that a sheet can put it on
 * whatever it already has. Focus bubbles, so a field's own container reports for
 * whatever is inside it — the text area, the token field's input, the field's own
 * "Ask the Partner" — and a sheet does not have to thread a prop through every
 * control it happens to use.
 */
export function useFieldInHand(opening: string): (field: string) => void {
  const { noteWriting } = usePartner();
  return useCallback(
    (field: string) => {
      noteWriting(opening, field);
    },
    [noteWriting, opening],
  );
}
