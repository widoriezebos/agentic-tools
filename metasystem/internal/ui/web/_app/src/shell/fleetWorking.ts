import { useEffect, useMemo, useState } from "react";

import { loadFleet, type Page } from "../fleet/api";
import { workingLines } from "../fleet/fleet";
import { onFleetEvent, onStreamOpen } from "../notifications/stream";

/**
 * What the rail's Fleet row says, read by the shell (g1-s74 D5).
 *
 * The shell reads the fleet on mount and again on each `fleet` beat and each
 * stream open, the three the Fleet page already reads on, through the Fleet
 * page's own call site. The answer is held above the rail; a read that has not
 * landed, or one that failed, says nothing. The minutes in the words are the
 * page's, measured once when it lands: this keeps no clock of its own.
 */
export function useFleetWorking(): readonly string[] {
  const [read, setRead] = useState<{ page: Page; at: Date } | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadFleet(aborter.signal)
      .then((page) => {
        setRead({ page, at: new Date() });
      })
      .catch(() => {
        if (!aborter.signal.aborted) {
          setRead(null);
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  useEffect(() => {
    const again = () => {
      setAttempt((previous) => previous + 1);
    };
    const stopFleet = onFleetEvent(again);
    const stopOpen = onStreamOpen(again);
    return () => {
      stopFleet();
      stopOpen();
    };
  }, []);

  return useMemo(() => (read === null ? [] : workingLines(read.page, read.at)), [read]);
}
