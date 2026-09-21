import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { failureMessage, loadWorkspace, type Workspace } from "./workspace";

/**
 * What this workspace is, read once on load and again on Retry.
 *
 * The chip, the tab title, and Settings' About card read the same state, so
 * there is one request and one answer rather than three of each. A workspace
 * that cannot be read is a state, not an error boundary: the sections render
 * either way, because none of them needs the answer.
 */

export type WorkspaceState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "known"; workspace: Workspace };

type Identity = { workspace: WorkspaceState; retry: () => void };

const IdentityContext = createContext<Identity>({ workspace: { state: "loading" }, retry: () => {} });

export function useWorkspaceState(): Identity {
  return useContext(IdentityContext);
}

export function IdentityProvider({ children }: { children: ReactNode }) {
  const [workspace, setWorkspace] = useState<WorkspaceState>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const aborter = new AbortController();
    loadWorkspace(aborter.signal)
      .then((described) => {
        setWorkspace({ state: "known", workspace: described });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setWorkspace({ state: "failed", message: failureMessage(error) });
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const retry = useCallback(() => {
    setWorkspace({ state: "loading" });
    setAttempt((previous) => previous + 1);
  }, []);

  const value = useMemo(() => ({ workspace, retry }), [workspace, retry]);
  return <IdentityContext.Provider value={value}>{children}</IdentityContext.Provider>;
}
