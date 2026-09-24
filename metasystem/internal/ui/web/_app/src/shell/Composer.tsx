import { Square } from "lucide-react";
import { useEffect, useRef } from "react";

import { Button } from "./controls";
import { showsSuggestions, Suggestions } from "../partner/Chips";
import { insertAt } from "../partner/composing";
import { useTypefaceOn } from "../partner/FontControl";
import { SeeingRow } from "../partner/Seeing";
import { usePartner } from "../partner/store";

/**
 * Where a human writes to their Project Partner: one card, and everything they
 * touch is in it.
 *
 * Top to bottom it is what the next question will carry, the questions worth
 * asking, the field, and the one row that says how to send and sends. It was
 * four things at four alignments — a line above the transcript, two chips
 * floating in it, a hint of its own and a button in the far corner — and the
 * card is what makes them one object: the eye goes there to act, and nowhere
 * else.
 *
 * Enter sends and Shift+Enter is a newline, which is what every conversation
 * does and what a human will try first. Stop takes Send's place while a turn
 * runs, so the one control that ends a turn is where the one control that
 * starts a turn was, rather than beside the answer it would end.
 *
 * The draft is the store's, not this component's: the bar's field and the
 * panel's are two elements for one sentence, and the focused page is a third.
 * A refusal keeps it, so a question the server could not take is still there
 * to send again.
 *
 * Two things come in through the store rather than through a prop, because
 * the thing that needs them is not this component's parent. Ask on a card
 * takes the caret here, from wherever it was; and a suggestion pill pressed
 * while something is half-written inserts its words at this field's own
 * cursor, which only this field knows.
 */

export const COMPOSER_LABEL = "Message to your Project Partner";
/** The hint in the empty box: an invitation to type, not the name of the box. */
export const COMPOSER_HINT = "Ask your Project Partner, or think out loud…";
/** How a question is sent, said once, under the field that sends it. */
export const COMPOSER_KEYS = "Enter to send · Shift+Enter for a new line";

export function Composer({
  onEscape,
  takeCaret = false,
}: {
  /** Escape here closes the drawer. The focused view passes nothing. */
  onEscape?: () => void;
  /**
   * Whether this composer takes the caret as it mounts, and takes it after
   * what is already written rather than before it. It is true only where the
   * human was typing in the bar this panel has just replaced, so opening the
   * drawer from its own button leaves the caret where the human put it.
   */
  takeCaret?: boolean;
}) {
  const { draft, setDraft, send, stop, busy, sending, store, wanted, offerInsert, returnFocus } = usePartner();
  const field = useRef<HTMLTextAreaElement | null>(null);
  // The card carries the conversation's chosen face and size, and the field is
  // the one thing in it that reads them: a custom property inherits, and the
  // rule that acts on it is the field's own.
  const card = useRef<HTMLDivElement | null>(null);
  const unavailable = store.state === "unavailable";

  useTypefaceOn(card);

  useEffect(() => {
    const element = field.current;
    if (!takeCaret || element === null) {
      return;
    }
    element.focus();
    element.setSelectionRange(element.value.length, element.value.length);
  }, [takeCaret]);

  // Ask, and Cmd/Ctrl+J, ask for the caret by counting. The first render is
  // not one of them, so a page that opens with the drawer open does not steal
  // the caret from whatever the human was reading.
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const element = field.current;
    if (element === null) {
      return;
    }
    element.focus();
    element.setSelectionRange(element.value.length, element.value.length);
  }, [wanted]);

  // How a suggestion goes in when there is already a draft: at the cursor,
  // where the human left it. The registration is taken back when this
  // composer leaves, so the other one is not written into.
  useEffect(() => {
    offerInsert((words: string) => {
      const element = field.current;
      if (element === null) {
        setDraft(insertAt(draft, draft.length, draft.length, words).text);
        return;
      }
      const written = insertAt(element.value, element.selectionStart, element.selectionEnd, words);
      setDraft(written.text);
      element.focus();
      element.setSelectionRange(written.caret, written.caret);
    });
    return () => {
      offerInsert(null);
    };
  }, [offerInsert, setDraft, draft]);

  return (
    <div className="ms-composer" ref={card}>
      <SeeingRow />
      {showsSuggestions(draft) && <Suggestions />}
      <label className="ms-visually-hidden" htmlFor="composer">
        {COMPOSER_LABEL}
      </label>
      <textarea
        id="composer"
        ref={field}
        rows={1}
        className="ms-composer-field"
        placeholder={COMPOSER_HINT}
        aria-describedby="composer-reason"
        value={draft}
        disabled={busy || unavailable}
        onChange={(event) => {
          setDraft(event.target.value);
        }}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            // Escape goes back to where Ask came from, where it came from
            // one; otherwise it closes the drawer, which is what it did
            // before there was anywhere else to go.
            returnFocus();
            onEscape?.();
            return;
          }
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            // The field's own value, not the draft this render closed over: a
            // keystroke and the state it produced are two frames, and Enter in
            // the same frame as the last character must still send it.
            send(event.currentTarget.value);
          }
        }}
      />
      <div className="ms-composer-foot">
        <span className="ms-composer-reason" id="composer-reason">
          {COMPOSER_KEYS}
        </span>
        {busy ? (
          <Button
            onClick={() => {
              stop();
            }}
          >
            <Square size={14} strokeWidth={1.75} aria-hidden="true" />
            Stop
          </Button>
        ) : (
          <Button
            primary
            disabled={sending || unavailable || draft.trim() === ""}
            onClick={() => {
              send();
            }}
          >
            Send
          </Button>
        )}
      </div>
    </div>
  );
}
