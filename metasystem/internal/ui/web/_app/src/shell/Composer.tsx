import { useEffect, useRef } from "react";

import { Button } from "./controls";

/**
 * Where a human writes to their Project Partner.
 *
 * Sending is what is not connected in this build, so Send is what is disabled
 * and says why; the field itself takes what is typed and keeps it. A human who
 * has found where to type should be able to write the sentence they came to
 * write, and the drawer carries that sentence from the closed bar into the
 * open panel rather than dropping it on the way.
 *
 * The reason is on screen, not in a tooltip, and the field names it through
 * aria-describedby.
 */
export const COMPOSER_REASON = "Your Project Partner is not connected in this build.";

export const COMPOSER_LABEL = "Message to your Project Partner";
/** The hint in the empty box: an invitation to type, not the name of the box. */
export const COMPOSER_HINT = "Ask your Project Partner, or think out loud…";

export function Composer({
  draft,
  onDraft,
  onEscape,
  takeCaret = false,
}: {
  draft: string;
  onDraft: (draft: string) => void;
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
  const field = useRef<HTMLTextAreaElement | null>(null);

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
        onChange={(event) => {
          onDraft(event.target.value);
        }}
        onKeyDown={(event) => {
          if (event.key === "Escape" && onEscape !== undefined) {
            event.preventDefault();
            onEscape();
          }
        }}
      />
      <div className="ms-composer-foot">
        <span className="ms-composer-reason" id="composer-reason">
          {COMPOSER_REASON}
        </span>
        <Button primary disabled>
          Send
        </Button>
      </div>
    </div>
  );
}
