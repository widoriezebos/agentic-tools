import { NavLink } from "react-router";
import { X } from "lucide-react";

import { idFor } from "./attachments";
import { usePartner } from "./store";
import { chipLabel, effectiveSubject, kindWord } from "./subject";
import { useSubject } from "../shell/about";

/**
 * The thing under discussion, pinned beside the conversation.
 *
 * The drawer is for a quick question and the focused page is for a long one,
 * and a long one is exactly where the subject scrolls away: twenty messages
 * later "it" is a word with nothing on screen behind it. So the focused page
 * keeps the subject in view with its identity, the reading it was taken from,
 * and its summary as the pages show it.
 *
 * It shows the chosen subject where there is one and the page's own otherwise,
 * which is the same rule the chip above the composer follows: one subject, two
 * places it is visible.
 */
export function PinnedSubject() {
  const { chosen, passage, clearChosen, detach } = usePartner();
  const page = useSubject();
  // A passage is pinned where nothing else was chosen: it is what the human
  // put there, and the panel shows the quote itself below.
  const attached = chosen ?? passage;
  const takeBack =
    chosen !== null
      ? clearChosen
      : passage === null
        ? null
        : () => {
            detach(idFor("passage"));
          };
  const subject = effectiveSubject(attached, page);
  if (subject === null) {
    return (
      <aside className="ms-partner-pinned ms-partner-pinned--empty" aria-label="What this is about">
        <p className="ms-partner-pinned-none">
          Nothing is chosen, so questions are about the page you are on. Choose Ask about this on a card, a row, a lane
          or an item to pin it here.
        </p>
      </aside>
    );
  }
  return (
    <aside className="ms-partner-pinned" aria-label="What this is about">
      <div className="ms-partner-pinned-head">
        <span className="ms-partner-pinned-kind">{kindWord(subject.kind)}</span>
        {takeBack !== null && (
          <button
            type="button"
            className="ms-partner-subject-clear"
            aria-label={`Stop asking about ${chipLabel(subject)}`}
            onClick={takeBack}
          >
            <X size={12} strokeWidth={2} aria-hidden="true" />
          </button>
        )}
      </div>
      <p className="ms-partner-pinned-title">
        {subject.to === undefined || subject.to === "" ? (
          subject.title === "" ? subject.id : subject.title
        ) : (
          <NavLink className="ms-partner-pinned-link" to={subject.to}>
            {subject.title === "" ? subject.id : subject.title}
          </NavLink>
        )}
      </p>
      {subject.id !== subject.title && <p className="ms-partner-pinned-id ms-mono">{subject.id}</p>}
      {subject.source !== "" && <p className="ms-partner-pinned-source">{subject.source}</p>}
      {subject.summary !== "" && <p className="ms-partner-pinned-summary">{subject.summary}</p>}
      {subject.quote !== undefined && subject.quote !== "" && (
        <blockquote className="ms-partner-pinned-quote">{subject.quote}</blockquote>
      )}
    </aside>
  );
}
