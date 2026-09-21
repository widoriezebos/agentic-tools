import type { ReactNode } from "react";

/**
 * What the pane says when the ledger cannot be projected.
 *
 * The shell's empty states are a fixed table read by id, because they say the
 * same thing every time. These say what was observed this time: a count, a
 * tip, the engine's own refusal, when the next fetch is due. So the words come
 * from the response and the styling is the shell's, unchanged.
 *
 * A heading is a name without a full stop, a body is sentences, and the note
 * is the small line beneath. Nothing here asserts that records are absent.
 */
export function Statement({
  heading,
  body,
  note,
  children,
}: {
  heading: string;
  body: ReactNode;
  note?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="ms-empty">
      <h2 className="ms-empty-heading">{heading}</h2>
      <p className="ms-empty-body">{body}</p>
      {note !== undefined && <p className="ms-empty-slice">{note}</p>}
      {children}
    </div>
  );
}
