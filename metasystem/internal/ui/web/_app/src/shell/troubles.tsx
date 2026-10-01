import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { held, released, type Pending, type Trouble } from "./troubling";
import { browserStore, readPendingTroubles, writePendingTroubles, type Store } from "../storage";

/**
 * The trouble context (g1-s68 D2): what every trouble line presses, mounted
 * above every provider.
 *
 * It stands above the providers because two of the lines that most need the
 * control are drawn by providers that stand above the Partner's: the bell's
 * panel and the sign-in sheet. Beneath those, the Partner's context is the
 * no-op one, and a control there would send nothing. So the Partner's store
 * registers its one `ask` here, the line presses that, and until it is
 * registered the line shows no control at all.
 *
 * It also holds the two things a press needs that are nobody's page: the
 * troubles waiting for a running answer, which the line that was pressed and
 * the chip above the composer both read, and which are kept in the browser's
 * store so a reload finds them still waiting, and the secrets the page holds at
 * this moment — the code being typed, a token field's value — which every
 * trouble is scrubbed of before it is kept or sent.
 */

/** What the Partner's store does with one press: send it, or hold it while its conversation answers. */
export type TroubleAsk = (trouble: Trouble, origin: string) => void;

type Troubles = {
  /** The Partner's ask, or null until the Partner has registered one. */
  ask: TroubleAsk | null;
  /** The Partner registers its ask, and takes it back (null) when it leaves. */
  register: (ask: TroubleAsk | null) => void;
  /** Every press waiting for its conversation's answer to finish. */
  pending: readonly Pending[];
  hold: (entry: Pending) => void;
  release: (id: string) => void;
  /** Every secret the page holds right now. */
  secrets: () => readonly string[];
  /** Say this value is a secret for as long as the returned release is not called. */
  holdSecret: (read: () => string) => () => void;
};

const nothing: Troubles = {
  ask: null,
  register: () => {},
  pending: [],
  hold: () => {},
  release: () => {},
  secrets: () => [],
  holdSecret: () => () => {},
};

const TroublesContext = createContext<Troubles>(nothing);

export function useTroubles(): Troubles {
  return useContext(TroublesContext);
}

/** The context with these readings and no others, for a render that needs one state. */
export function TroublesAs({ held: given, children }: { held: Partial<Troubles>; children: ReactNode }) {
  return <TroublesContext.Provider value={{ ...nothing, ...given }}>{children}</TroublesContext.Provider>;
}

export function TroubleProvider({ children, store = browserStore() }: { children: ReactNode; store?: Store | null }) {
  // A function in state is set through the updater form, or React calls it.
  const [ask, setAsk] = useState<TroubleAsk | null>(null);
  // A question left waiting is kept where a reload finds it: the page that
  // loads next starts with it, in its own conversation, ready to send.
  const [pending, setPending] = useState<readonly Pending[]>(() => readPendingTroubles(store));
  useEffect(() => {
    writePendingTroubles(pending, store);
  }, [pending, store]);
  const readers = useRef(new Set<() => string>());

  const register = useCallback((next: TroubleAsk | null) => {
    setAsk(() => next);
  }, []);
  const hold = useCallback((entry: Pending) => {
    setPending((list) => held(list, entry));
  }, []);
  const release = useCallback((id: string) => {
    setPending((list) => released(list, id));
  }, []);
  const secrets = useCallback(() => [...readers.current].map((read) => read()), []);
  const holdSecret = useCallback((read: () => string) => {
    readers.current.add(read);
    return () => {
      readers.current.delete(read);
    };
  }, []);

  const value = useMemo(
    () => ({ ask, register, pending, hold, release, secrets, holdSecret }),
    [ask, register, pending, hold, release, secrets, holdSecret],
  );
  return <TroublesContext.Provider value={value}>{children}</TroublesContext.Provider>;
}

/**
 * This value is a secret while the component that holds it is on screen: a
 * trouble composed now has it replaced before it is kept or sent.
 */
export function useSecret(value: string): void {
  const { holdSecret } = useTroubles();
  const current = useRef(value);
  current.current = value;
  useEffect(() => holdSecret(() => current.current), [holdSecret]);
}
