import { useCallback, useEffect, useState } from "react";

import { loadBacklog, type Backlog } from "./api";
import { loadPane, type Pane } from "../project/api";
import { failureMessage } from "../shell/workspace";

/**
 * The backlog, read when the pane mounts and again on Refresh.
 *
 * Leaving the pane aborts a request in flight, so a reader who navigates away
 * mid-read leaves no state behind to arrive later. A backlog that cannot be
 * read is a state, not an error boundary: the rail and the header work either
 * way, and the reason is what a human acts on.
 *
 * An act answers with the backlog as the ledger then stood, and `moved` is
 * how that answer becomes what the page shows. It is not a second read and it
 * is not optimism: the server carried this clone's accepted ref forward
 * before it answered, so a card that moves is a card the ledger moved.
 */

export type BacklogState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "known"; backlog: Backlog };

export function useBacklog(): {
  backlog: BacklogState;
  refresh: () => void;
  moved: (after: Backlog) => void;
  /** Which read this is, so what is read beside the ledger refreshes with it. */
  attempt: number;
} {
  const [backlog, setBacklog] = useState<BacklogState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadBacklog(aborter.signal)
      .then((read) => {
        setBacklog({ state: "known", backlog: read });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setBacklog({ state: "failed", message: failureMessage(error) });
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const refresh = useCallback(() => {
    setBacklog({ state: "loading" });
    setAttempt((previous) => previous + 1);
  }, []);

  const moved = useCallback((after: Backlog) => {
    setBacklog({ state: "known", backlog: after });
  }, []);

  return { backlog, refresh, moved, attempt };
}

/**
 * The project's records, for the one thing the board wants from them: what
 * each card's slice plan says.
 *
 * It is a second read rather than a field of the backlog, because the two
 * resources answer different questions and are paid for differently. The slice
 * plan is read out of the checkout's design records, which means walking the
 * homes; folding that into the backlog payload would make every approval,
 * withdrawal and re-rank pay for a filesystem walk in order to answer with a
 * board. So the board reads it once when it mounts, beside the ledger.
 *
 * A project that cannot be read is not an error here and is not shown as one:
 * the board is the ledger's, and the slice plan is a reading beside it. The
 * cards simply carry no slice line, and everything else works.
 */
export function useSlicePlans(attempt: number): Pane | null {
  const [pane, setPane] = useState<Pane | null>(null);

  useEffect(() => {
    const aborter = new AbortController();
    loadPane(aborter.signal)
      .then((read) => {
        setPane(read);
      })
      .catch(() => {
        setPane(null);
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  return pane;
}
