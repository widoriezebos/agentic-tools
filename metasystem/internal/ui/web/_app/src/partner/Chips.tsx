import { X } from "lucide-react";

import { usePartner } from "./store";
import { chipLabel, effectiveSubject, registerFor, type Chosen } from "./subject";
import { HOW_TO_ASK, suggestionsFor } from "./suggestions";
import { Hint } from "../shell/controls";
import { useSubject } from "../shell/about";

/**
 * What stands above the composer: what the question is about, and the three
 * questions worth asking about it.
 *
 * The subject chip is the answer to "what does 'this' mean right now". It is
 * the thing a human chose, with the × that gives the subject back to the page;
 * a page whose own subject is in force shows it too, without the ×, because
 * there is nothing to clear.
 *
 * A chip sends when the composer is empty and inserts at the cursor when
 * something is half-written, which is the whole of contract 5: the sentence a
 * human was composing is theirs.
 */
export function Chips({ first = false }: { first?: boolean }) {
  const { chosen, clearChosen, suggest, busy, capture } = usePartner();
  const page = useSubject();
  const subject = effectiveSubject(chosen, page);
  // Which register: the subject's kind where there is one, and the section
  // otherwise — which is what the board and the landing page are asked about.
  const suggestions = suggestionsFor(registerFor(chosen, capture.section));
  if (subject === null && suggestions.length === 0 && !first) {
    return null;
  }
  return (
    <div className="ms-partner-chips">
      {subject !== null && <SubjectChip subject={subject} onClear={chosen === null ? null : clearChosen} />}
      {subject !== null && subject.quote !== undefined && subject.quote !== "" && (
        <p className="ms-partner-quote">{subject.quote}</p>
      )}
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
      {first && <p className="ms-partner-hint">{HOW_TO_ASK}</p>}
    </div>
  );
}

/** The chosen subject, with the × that gives the subject back to the page. */
function SubjectChip({ subject, onClear }: { subject: Chosen; onClear: (() => void) | null }) {
  return (
    <span className="ms-partner-subject-chip">
      <span className="ms-partner-subject-what">{chipLabel(subject)}</span>
      {subject.source !== "" && <span className="ms-partner-subject-source">{subject.source}</span>}
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
