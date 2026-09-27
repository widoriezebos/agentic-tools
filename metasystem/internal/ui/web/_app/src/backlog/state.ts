import { useCallback, useEffect, useRef, useState } from "react";

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
  // `problem` is what a later read of the board was refused with, standing
  // beside the reading it could not replace. A read that answers — and an act
  // that answers with the ledger it left — replaces this state whole, so it
  // cannot outlive the reading it was recorded against.
  | { state: "known"; backlog: Backlog; problem?: string };

export function useBacklog(): {
  backlog: BacklogState;
  /**
   * Refresh, as the toolbar presses it: the board blanks and the server is
   * asked to fetch the canonical branch before it answers.
   */
  refresh: () => void;
  /**
   * The same read in place: what is on screen stays until the answer arrives,
   * and the server is not asked to fetch.
   *
   * It is what the board OFFERS to the shell, and the difference matters because
   * an offered re-read is made on somebody else's behalf — after a proposal
   * applied in the Partner's drawer, say — over a page that may have a sheet open
   * on it. A read that blanked the board would unmount that sheet's own columns
   * and whatever was typed into them (Astra S58-03).
   */
  again: () => void;
  moved: (after: Backlog) => void;
  /** Which read this is, so what is read beside the ledger refreshes with it. */
  attempt: number;
} {
  const [backlog, setBacklog] = useState<BacklogState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  // Whether the read this attempt makes asks the server to fetch the canonical
  // branch first. The mount observes; Refresh looks, which is what makes "is
  // this current" a question about now rather than about whenever the server's
  // own loop last looked; an offered re-read observes, because nobody pressed
  // anything and the loop's own cadence is the honest answer for it.
  const looking = useRef(false);

  useEffect(() => {
    const aborter = new AbortController();
    loadBacklog(aborter.signal, looking.current)
      .then((read) => {
        setBacklog({ state: "known", backlog: read });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          // A refused read keeps whatever the board already read, and says so
          // beside it. Failing the whole board instead would draw the error view
          // over a page that is still good, and unmount a sheet open over the
          // lanes and whatever was typed into it — which is the very thing
          // `again` exists to protect (Astra C-05). Only a first read's failure
          // has nothing on screen to keep.
          setBacklog((held) =>
            held.state === "known"
              ? { ...held, problem: failureMessage(error) }
              : { state: "failed", message: failureMessage(error) },
          );
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const refresh = useCallback(() => {
    looking.current = true;
    setBacklog({ state: "loading" });
    setAttempt((previous) => previous + 1);
  }, []);

  const again = useCallback(() => {
    looking.current = false;
    setAttempt((previous) => previous + 1);
  }, []);

  const moved = useCallback((after: Backlog) => {
    setBacklog({ state: "known", backlog: after });
  }, []);

  return { backlog, refresh, again, moved, attempt };
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
