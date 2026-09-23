import { useEffect, useRef } from "react";

import { Button } from "./controls";
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
  const { draft, setDraft, send, busy, sending, store } = usePartner();
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
          if (event.key === "Escape" && onEscape !== undefined) {
            event.preventDefault();
            onEscape();
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
