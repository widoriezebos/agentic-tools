import { useEffect, useRef } from "react";

import { Button } from "./controls";
import { insertAt } from "../partner/composing";
import { usePartner } from "../partner/store";

/**
 * Where a human writes to their Project Partner.
 *
 * Enter sends and Shift+Enter is a newline, which is what every conversation
 * does and what a human will try first. Send is disabled while a turn is
 * running and while a send is in flight, so one question is one turn; Stop is
 * in the transcript, beside the answer it would end.
 *
 * The draft is the store's, not this component's: the bar's field and the
 * panel's are two elements for one sentence, and the focused page is a third.
 * A refusal keeps it, so a question the server could not take is still there
 * to send again.
 *
 * Two things come in through the store rather than through a prop, because
 * the thing that needs them is not this component's parent. Ask on a card
 * takes the caret here, from wherever it was; and a suggestion chip pressed
 * while something is half-written inserts its words at this field's own
 * cursor, which only this field knows.
 */

export const COMPOSER_LABEL = "Message to your Project Partner";
/** The hint in the empty box: an invitation to type, not the name of the box. */
export const COMPOSER_HINT = "Ask your Project Partner, or think out loud…";

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
  const { draft, setDraft, send, busy, sending, store, wanted, offerInsert, returnFocus } = usePartner();
  const field = useRef<HTMLTextAreaElement | null>(null);
  const unavailable = store.state === "unavailable";

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
    <div className="ms-composer">
      <label className="ms-visually-hidden" htmlFor="composer">
        {COMPOSER_LABEL}
      </label>
      <textarea
        id="composer"
        ref={field}
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
          {busy ? "Answering…" : "Enter sends, Shift+Enter starts a line."}
        </span>
        <Button
          primary
          disabled={busy || sending || unavailable || draft.trim() === ""}
          onClick={() => {
            send();
          }}
        >
          Send
        </Button>
      </div>
    </div>
  );
}
