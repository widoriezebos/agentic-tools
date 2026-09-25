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
  newestFor,
  waitingFor,
  waitingLabel,
} from "./suggesting";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";

/**
 * The words the Partner offered, as a card under its answer — and, beside the
 * field they are for, the line that says one is waiting.
 *
 * The card is the reusable result card the master asks for, first used here. It
 * is a card and not prose because a human has to be able to see whose words
 * these are: in the drawer they are the Partner's, set apart with the field's
 * name over them; in the field, after Use this, they are the human's draft.
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
 * What a field says beside its label while suggestions wait for it, placed by
 * the sheet that owns the label.
 *
 * It says how many are waiting and takes the human to them; it never shows the
 * words. A suggestion waits in the drawer, and the field only says one is
 * there — which is the whole of what makes the draft the human's.
 *
 * It is also how a card learns that the human has typed since: the field says
 * what it now holds, and the card stops offering Undo once that is no longer
 * the words it offered.
 */
export function FieldSuggestions({
  opening,
  field,
  value,
}: {
  /** The opening this field belongs to, as the sheet minted it. */
  opening: string;
  /** The field, as its own label says it. */
  field: string;
  /** What the field holds right now. */
  value: string;
}) {
  const { offered, show, noteField } = usePartner();
  // Every render, because every render is a keystroke or a press: the value
  // above is what the field holds now, and nothing else tells the store.
  useEffect(() => {
    noteField(opening, field, value);
  });
  const waiting = waitingFor(offered, opening, field);
  if (waiting === 0) {
    return null;
  }
  const newest = newestFor(offered, opening, field);
  return (
    <button
      type="button"
      className="ms-field-suggestions"
      title={`Open your Project Partner at the newest suggestion for ${field}`}
      onClick={() => {
        show(newest);
      }}
    >
      {waitingLabel(waiting)}
    </button>
  );
}

/**
 * One card, in the transcript, under the answer that offered it.
 *
 * Its four shapes are the four things that can be true of one suggestion: it is
 * waiting for the human, they used it, they folded it away, or the editor it
 * was for has been closed and all that is left to do with the words is copy
 * them.
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
 * The words, to the clipboard, for a card whose editor has gone.
 *
 * It is the one thing left to do with a suggestion nobody can use any more, and
 * a browser that refuses the clipboard says so rather than leaving a button
 * that looks as though it worked.
 */
function Copy({ text }: { text: string }) {
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
