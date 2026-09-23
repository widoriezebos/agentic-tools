import type { Look } from "./api";

/**
 * What an answer was read from, quietly.
 *
 * Astra's second finding is the rule: a count can hide a failed read behind a
 * number that sounds like success, so this is a list. One line per read — what
 * it was, the reading it was of, and whether it was read whole, in part or not
 * at all — with the returned excerpt behind a disclosure of its own, for
 * whoever wants to check the answer against it.
 *
 * The page the human was looking at is first and separate, and it is not one
 * of the N: it is the one reading the Partner did not choose. A failed read is
 * listed and is never counted either, because "looked at three things" has to
 * mean three things it actually saw.
 */
export function Looked({ looked }: { looked: readonly Look[] }) {
  if (looked.length === 0) {
    return null;
  }
  const chosen = looked.filter((look) => look.page !== true);
  const counted = chosen.filter((look) => look.outcome !== "failed").length;
  const failed = chosen.length - counted;
  return (
    <details className="ms-partner-looked">
      <summary className="ms-partner-looked-line">
        Looked at {counted} {counted === 1 ? "thing" : "things"}
        {failed > 0 && `, and ${failed === 1 ? "one read failed" : `${String(failed)} reads failed`}`}
      </summary>
      <ul className="ms-partner-looked-list">
        {looked.map((look, at) => (
          <li key={at} className="ms-partner-looked-item">
            <span className={`ms-partner-outcome ms-partner-outcome--${look.outcome}`}>{look.outcome}</span>
            <span className="ms-partner-looked-what">{look.what}</span>
            {look.source !== undefined && look.source !== "" && (
              <span className="ms-partner-looked-source">{look.source}</span>
            )}
            {look.excerpt !== undefined && look.excerpt !== "" && (
              <details className="ms-partner-excerpt">
                <summary className="ms-partner-excerpt-line">What came back</summary>
                <pre className="ms-partner-excerpt-body">
                  <code>{look.excerpt}</code>
                </pre>
              </details>
            )}
          </li>
        ))}
      </ul>
    </details>
  );
}
