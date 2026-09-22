import { useCallback, useEffect, useState } from "react";

import { loadBacklog, type Backlog } from "./api";
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

  return { backlog, refresh, moved };
}
