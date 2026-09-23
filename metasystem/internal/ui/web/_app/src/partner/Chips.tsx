import { X } from "lucide-react";

import { usePartner } from "./store";
import { chipLabel, registerFor, type Chosen } from "./subject";
import { suggestionsFor } from "./suggestions";
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
 * The subject chip is the answer to "what does 'this' mean right now". It is
 * the thing a human chose, with the × that gives the subject back to the page;
 * a page whose own subject is in force shows it too, without the ×, because
 * there is nothing to clear.
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
  const { suggest, busy, capture, chosen } = usePartner();
  // Which register: the subject's kind where there is one, and the section
  // otherwise — which is what the board and the landing page are asked about.
  const suggestions = suggestionsFor(registerFor(chosen, capture.section));
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

/** The chosen subject, with the × that gives the subject back to the page. */
export function SubjectChip({ subject, onClear }: { subject: Chosen; onClear: (() => void) | null }) {
  return (
    <span className="ms-partner-subject-chip">
      <span className="ms-partner-subject-what">{chipLabel(subject)}</span>
      {onClear !== null && (
        <button
          type="button"
          className="ms-partner-subject-clear"
          aria-label={`Stop asking about ${chipLabel(subject)}`}
          onClick={onClear}
        >
          <X size={12} strokeWidth={2} aria-hidden="true" />
        </button>
      )}
    </span>
  );
}
