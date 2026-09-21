import type { ReactNode } from "react";

import type { Ledger } from "./api";
import { ageBetween, clockTime, dateAndTime, shortTip } from "./format";
import { Button, Skeleton } from "../shell/controls";

/**
 * Which tip this page is reading, when it was read, and what the server's
 * fetch loop last found.
 *
 * Every time here is absolute and comes from the response. The page sets no
 * timer, so a countdown would be a number that quietly stopped being true;
 * the time the next fetch is due is a fact, and Refresh is how a human asks
 * what happened.
 */
export function LedgerLine({
  ledger,
  observedAt,
  onRefresh,
}: {
  ledger: Ledger | null;
  observedAt: string;
  onRefresh: () => void;
}) {
  return (
    <div className="ms-ledger">
      <div className="ms-ledger-lines">
        <p className="ms-ledger-line">
          {ledger === null ? <Skeleton /> : <Read ledger={ledger} observedAt={observedAt} />}
        </p>
        {ledger !== null && ledger.stale && <p className="ms-ledger-stale">{staleLine(ledger, observedAt)}</p>}
      </div>
      <Button onClick={onRefresh}>Refresh</Button>
    </div>
  );
}

function Read({ ledger, observedAt }: { ledger: Ledger; observedAt: string }): ReactNode {
  return (
    <>
      {ledger.tip === "" ? (
        "No accepted tip in this clone"
      ) : (
        <>
          Accepted tip <span className="ms-mono">{shortTip(ledger.tip)}</span>, committed{" "}
          {dateAndTime(ledger.committedAt)}
        </>
      )}
      {" · observed "}
      {clockTime(observedAt)}
      {" · "}
      {fetchClause(ledger)}
    </>
  );
}

/** What the loop last did, and when it looks again. */
export function fetchClause(ledger: Ledger): string {
  const loop = ledger.fetch;
  const due = loop.nextAt === "" ? "server stopping" : `next fetch ${clockTime(loop.nextAt)}`;
  switch (loop.outcome) {
    case "never":
      return loop.nextAt === "" ? "server stopping" : `first fetch due ${clockTime(loop.nextAt)}`;
    case "running":
      return `fetching since ${clockTime(loop.startedAt)}`;
    case "advanced":
      return `fetched ${clockTime(loop.finishedAt)}, accepted ${shortTip(loop.tip)} · ${due}`;
    case "current":
      return `fetched ${clockTime(loop.finishedAt)}, ${loop.detail} · ${due}`;
    case "failed":
      return `fetch failed ${clockTime(loop.finishedAt)}: ${loop.message} · ${
        loop.nextAt === "" ? "server stopping" : `retry ${clockTime(loop.nextAt)}`
      }`;
  }
}

/**
 * Why an old tip is old. A tree that has not moved is not by itself a
 * problem; what the human needs is what the last look at the canonical branch
 * found, which is the difference between a quiet repository and a broken one.
 */
export function staleLine(ledger: Ledger, observedAt: string): string {
  const age = `The accepted tip is ${ageBetween(ledger.committedAt, observedAt)} old`;
  switch (ledger.fetch.outcome) {
    case "current":
    case "advanced":
      return `${age}; the last fetch found the canonical branch at this tip.`;
    case "failed":
      return `${age}; the last fetch failed: ${ledger.fetch.message}.`;
    default:
      return `${age}; no fetch has completed yet.`;
  }
}
