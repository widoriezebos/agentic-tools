import type { Look } from "./api";
import { sawLine } from "./capture";

/**
 * One meta line per answer, and what opens out of it.
 *
 * An answer stands on its own: the reading it was given and the things it read
 * are not the answer, so they are one muted line under it — "Saw tip 761586c ·
 * 21:34 · Looked at 2 things" — and nothing else stands between two turns.
 *
 * Astra's second finding is why the line opens rather than merely counting: a
 * count can hide a failed read behind a number that sounds like success. Open
 * it and there is a list, one line per read — what it was, the reading it was
 * of, and whether it was read whole, in part or not at all — with the returned
 * excerpt behind a disclosure of its own, for whoever wants to check the answer
 * against it. The forty-character hash and the ISO instant the summary shortens
 * are in that list, on the page's own entry, and in the Seeing sheet.
 *
 * The page the human was looking at is first and separate, and it is not one of
 * the N: it is the one reading the Partner did not choose. A failed read is
 * listed and is never counted either, because "looked at three things" has to
 * mean three things it actually saw.
 */
export function Looked({ looked }: { looked: readonly Look[] }) {
  if (looked.length === 0) {
    return null;
  }
  const page = looked.find((look) => look.page === true);
  const chosen = looked.filter((look) => look.page !== true);
  const counted = chosen.filter((look) => look.outcome !== "failed").length;
  const failed = chosen.length - counted;
  const saw = sawLine(page?.source ?? "");
  return (
    <details className="ms-partner-meta">
      <summary className="ms-partner-meta-line">
        {saw !== "" && <span className="ms-partner-meta-part">{saw}</span>}
        <span className="ms-partner-meta-part">
          Looked at {counted} {counted === 1 ? "thing" : "things"}
          {failed > 0 && `, and ${failed === 1 ? "one read failed" : `${String(failed)} reads failed`}`}
        </span>
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
