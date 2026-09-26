import { X } from "lucide-react";

import { lifeOf, takeBackLabel, type Attachment } from "./attachments";
import { usePartner } from "./store";
import { registerFor } from "./subject";
import { DRAFT_REGISTER, suggestionsFor } from "./suggestions";
import { Hint } from "../shell/controls";

/**
 * The questions worth asking, and the thing they would be about.
 *
 * Both live inside the composer card now. The suggestions are pills between
 * the Seeing row and the field, and they are there only while the field is
 * empty: a half-written sentence is the human's, and three questions floating
 * over it are three questions in the way. They leave when the human types and
 * come back when the field is empty again.
 *
 * A chip is one attachment: the thing a human put above the composer by one
 * act, with the × that takes it back and the one line that says how long it
 * lives. Everything above the composer is one of these, and nothing else is —
 * where the human is standing has no × and is not a chip; it is the Seeing
 * line beside them.
 *
 * A pill sends when the composer is empty and inserts at the cursor when
 * something is half-written, which is the whole of contract 5 — and with the
 * pills gone while a draft stands, the second half of that rule is now only
 * reachable from the shortcut that puts a question in by other means.
 */

/**
 * Whether the pills are shown: the field is empty, and nothing but whitespace
 * counts as written. It is a predicate rather than a condition inside the
 * render so that the rule can be read, and tested, on its own.
 */
export function showsSuggestions(draft: string): boolean {
  return draft.trim() === "";
}

export function Suggestions() {
  const { suggest, busy, capture, chosen, passage } = usePartner();
  // Which register: the subject's kind where there is one, a selected passage
  // where that is all there is, and the section otherwise — which is what the
  // board and the landing page are asked about.
  const suggestions = suggestionsFor(registerFor(chosen ?? passage, capture.section));
  if (suggestions.length === 0) {
    return null;
  }
  return (
    <div className="ms-partner-suggestions">
      {suggestions.map((suggestion) => (
        <Hint key={suggestion.text} label={`Asks about ${suggestion.scope}`}>
          <button
            type="button"
            className="ms-partner-suggestion"
            disabled={busy}
            onClick={() => {
              suggest(suggestion.text);
            }}
          >
            {suggestion.text}
          </button>
        </Hint>
      ))}
    </div>
  );
}

/**
 * The two questions a handed-over sheet is worth asking, beside its own chip.
 *
 * They are the ordinary suggested questions and go the ordinary way: with an
 * empty composer each one sends, and with something half-written its words go
 * in at the cursor, because the sentence a human was composing is theirs. They
 * stand beside the chip rather than among the pills above the field because
 * they are about that chip: they ask the Partner to write in the sheet the chip
 * hands over, and they are gone the moment the chip is.
 */
export function DraftQuestions() {
  const { suggest, busy } = usePartner();
  return (
    <>
      {suggestionsFor(DRAFT_REGISTER).map((question) => (
        <Hint key={question.text} label={`Asks about ${question.scope}`}>
          <button
            type="button"
            className="ms-partner-suggestion"
            disabled={busy}
            onClick={() => {
              suggest(question.text);
            }}
          >
            {question.text}
          </button>
        </Hint>
      ))}
    </>
  );
}

/**
 * One attachment, as a chip: what it is, the × that takes it back, and — on
 * hover and on focus — the one line that says how long it lives.
 *
 * The line is there because a human should never have to guess why a chip is
 * on screen or when it will leave. It comes from the attachment's own
 * lifetime, declared by the act that made it, so a chip cannot say one thing
 * while the list does another.
 *
 * The tooltip wraps the whole chip rather than the ×, so that the words are
 * enough to hover over; focus reaches it through the × inside, which is the
 * part of a chip a keyboard can land on, and the × keeps its own name.
 */
export function AttachmentChip({ attachment, onRemove }: { attachment: Attachment; onRemove: () => void }) {
  return (
    <Hint label={lifeOf(attachment)}>
      <span
        className={
          attachment.kind === "draft"
            ? "ms-partner-subject-chip ms-partner-draft-chip"
            : "ms-partner-subject-chip"
        }
      >
        <span className="ms-partner-subject-what">{attachment.label}</span>
        <button
          type="button"
          className="ms-partner-subject-clear"
          aria-label={takeBackLabel(attachment)}
          onClick={onRemove}
        >
          <X size={12} strokeWidth={2} aria-hidden="true" />
        </button>
      </span>
    </Hint>
  );
}
