import { useEffect, useRef, useState } from "react";

import { bringUp } from "./scrolling";
import { usePartner } from "./store";
import {
  cardHead,
  cardIn,
  closedLine,
  DISMISSED,
  EDITED_SINCE,
  mintOpening,
  NOT_OFFERED,
  refusedLine,
} from "./suggesting";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";

/**
 * The words the Partner offered, as a card under its answer: the conversation's
 * own record of what was proposed, and of what was refused.
 *
 * The card is the reusable result card the master asks for, first used here. It
 * is a card and not prose because a human has to be able to see whose words
 * these are: in the drawer they are the Partner's, set apart with the field's
 * name over them; in the field, after Use this, they are the human's draft.
 *
 * Where a human decides about them is not here any more. Since g1-s52 the offer
 * stands under the field it is for, inside the sheet — src/partner/FieldProposals.tsx
 * — because the drawer's default height hid a card behind the composer. This card
 * keeps every state it had, reads the same store, and gained the one the drawer is
 * now the only place for: a suggestion the service did not offer, with its reason.
 *
 * Nothing here writes to the ledger and nothing here saves. Use this puts the
 * words in the field the card names and stops; the sheet's own Save is still
 * the human's, and the record of an edit is still their act.
 */

/**
 * One opening's id, minted when the sheet mounts and never again while it
 * stands.
 *
 * It is what a suggestion belongs to. The sheet's name cannot be: close goal
 * A's edit sheet, open goal B's, and the name is the same while the draft is a
 * different goal's — so a card prepared for A would write into B.
 */
export function useOpening(): string {
  const [id] = useState(mintOpening);
  return id;
}

/**
 * One card, in the transcript, under the answer that offered it.
 *
 * Its five shapes are the five things that can be true of one suggestion: it was
 * not offered at all, it is waiting for the human, they used it, they folded it
 * away, or the editor it was for has been closed and all that is left to do with
 * the words is copy them.
 *
 * The card is the record. Since g1-s52 the offer itself stands under the field it
 * is for, inside the sheet, and this is where the conversation keeps it — both
 * read the same store state, so the two cannot disagree about one suggestion.
 */
export function SuggestionCard({ id }: { id: string }) {
  const { offered, use, undo, dismiss, reopen, showing } = usePartner();
  const card = cardIn(offered, id);
  const box = useRef<HTMLDivElement | null>(null);
  // The field's link opened the drawer at this card, so the column comes to it.
  useEffect(() => {
    if (showing === id) {
      bringUp(box.current);
    }
  }, [showing, id]);
  if (card === undefined) {
    return null;
  }
  // Nothing was offered. It is the one card with nothing to press: no field of
  // the draft the human handed over was named, so there is nowhere to put the
  // words and no offer to fold away. What it owes them is the reason.
  if (card.standing === "refused") {
    return (
      <div className="ms-suggestion ms-suggestion--refused" ref={box} data-suggestion={id}>
        <p className="ms-suggestion-head">
          <span>{NOT_OFFERED}</span>
          <Help id="not-offered" />
        </p>
        <p className="ms-suggestion-refused" role="status">
          {refusedLine(card)}
        </p>
        <p className="ms-suggestion-text">{card.text}</p>
      </div>
    );
  }
  if (card.standing === "dismissed") {
    return (
      <button
        type="button"
        className="ms-suggestion-folded"
        title={`Show the suggestion for ${card.field} again`}
        onClick={() => {
          reopen(id);
        }}
      >
        {DISMISSED}
      </button>
    );
  }
  return (
    <div className="ms-suggestion" ref={box} data-suggestion={id}>
      <p className="ms-suggestion-head">
        <span>{cardHead(card)}</span>
        <Help id="suggestion" />
      </p>
      <p className="ms-suggestion-text">{card.text}</p>
      <div className="ms-suggestion-foot">
        {card.standing === "waiting" && (
          <>
            <Button
              primary
              onClick={() => {
                use(id);
              }}
            >
              Use this
            </Button>
            <Button
              onClick={() => {
                dismiss(id);
              }}
            >
              Dismiss
            </Button>
          </>
        )}
        {card.standing === "used" && (
          <>
            <span className="ms-suggestion-said">Used</span>
            <Button
              onClick={() => {
                undo(id);
              }}
            >
              Undo
            </Button>
          </>
        )}
        {card.standing === "edited" && (
          <span className="ms-suggestion-said">Used · {EDITED_SINCE}</span>
        )}
        {card.standing === "closed" && (
          <>
            <span className="ms-suggestion-said">{closedLine(card)}</span>
            <Copy text={card.text} />
          </>
        )}
      </div>
    </div>
  );
}

/**
 * The words, to the clipboard, for a card nobody can act on any more.
 *
 * It is the one thing left to do with a suggestion whose editor has gone — and,
 * since Sol's first finding on the sitting, with a deposit offered to a record
 * this sitting is not on. A browser that refuses the clipboard says so rather
 * than leaving a button that looks as though it worked.
 */
export function Copy({ text }: { text: string }) {
  const [said, setSaid] = useState("");
  return (
    <>
      <Button
        onClick={() => {
          try {
            void navigator.clipboard.writeText(text).then(
              () => {
                setSaid("Copied");
              },
              () => {
                setSaid("This browser would not give the clipboard.");
              },
            );
          } catch {
            setSaid("This browser would not give the clipboard.");
          }
        }}
      >
        Copy
      </Button>
      {said !== "" && <span className="ms-suggestion-said">{said}</span>}
    </>
  );
}
