import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { loadSession, needsAHandle, type SessionState, type SessionStatus } from "./session";
import { SignInSheet } from "./SignInSheet";
import { failureMessage, loadWorkspace, type Workspace } from "./workspace";

/**
 * What this workspace is, and who the server is acting as, read once on load
 * and again on Retry.
 *
 * The chip, the tab title, and Settings' About card read the same workspace
 * state, so there is one request and one answer rather than three of each. A
 * workspace that cannot be read is a state, not an error boundary: the
 * sections render either way, because none of them needs the answer.
 *
 * The session is read beside it, and again whenever a sign-in or a sign-out
 * answers with a new one — never on a timer. It is here rather than in the
 * board because who the server acts as is the shell's question: the header
 * shows it, the board's acts depend on it, and an act refused for want of one
 * opens the sheet this provider owns.
 */

export type WorkspaceState =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "known"; workspace: Workspace };

type Identity = {
  workspace: WorkspaceState;
  retry: () => void;
  session: SessionStatus;
  /**
   * Open the sign-in sheet. An act that was refused for want of a human
   * passes itself, and is run once when a sign-in lands — once, because the
   * caller is what remembers it and the sheet forgets it either way.
   */
  askToSignIn: (retry?: () => void) => void;
  /** What a sign-in or a sign-out answered, which is what the page shows. */
  settled: (state: SessionState) => void;
};

const IdentityContext = createContext<Identity>({
  workspace: { state: "loading" },
  retry: () => {},
  session: { state: "loading" },
  askToSignIn: () => {},
  settled: () => {},
});

export function useWorkspaceState(): Identity {
  return useContext(IdentityContext);
}

/** Who the server is acting as, and the two ways the page changes it. */
export function useSession(): Identity {
  return useContext(IdentityContext);
}

export function IdentityProvider({ children }: { children: ReactNode }) {
  const [workspace, setWorkspace] = useState<WorkspaceState>({ state: "loading" });
  const [session, setSession] = useState<SessionStatus>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [asking, setAsking] = useState(false);
  // The act waiting on a sign-in. It is a ref rather than state because it is
  // never rendered and must not make the sheet render again when it changes.
  const waiting = useRef<(() => void) | null>(null);

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

  useEffect(() => {
    const aborter = new AbortController();
    loadSession(aborter.signal)
      .then((state) => {
        setSession({ state: "known", session: state });
      })
      .catch((error: unknown) => {
        if (!aborter.signal.aborted) {
          setSession({ state: "failed", message: failureMessage(error) });
        }
      });
    return () => {
      aborter.abort();
    };
  }, [attempt]);

  const retry = useCallback(() => {
    setWorkspace({ state: "loading" });
    setSession({ state: "loading" });
    setAttempt((previous) => previous + 1);
  }, []);

  const askToSignIn = useCallback((again?: () => void) => {
    waiting.current = again ?? null;
    setAsking(true);
  }, []);

  const settled = useCallback((state: SessionState) => {
    setSession({ state: "known", session: state });
    setAsking(false);
    const again = waiting.current;
    waiting.current = null;
    if (state.signedIn && again !== null) {
      again();
    }
  }, []);

  const value = useMemo(
    () => ({ workspace, retry, session, askToSignIn, settled }),
    [workspace, retry, session, askToSignIn, settled],
  );
  return (
    <IdentityContext.Provider value={value}>
      {children}
      <SignInSheet
        open={asking}
        status={session}
        needsHandle={needsAHandle(session)}
        onOpenChange={(next) => {
          if (!next) {
            waiting.current = null;
          }
          setAsking(next);
        }}
        onSignedIn={settled}
      />
    </IdentityContext.Provider>
  );
}
