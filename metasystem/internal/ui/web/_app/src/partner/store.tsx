import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation } from "react-router";

import { isBusy, loadPartner, PartnerError, sendTurn, stopTurn } from "./api";
import { keyFor, pageOf } from "./asking";
import { busy, emptyStore, loaded, received, refused, retrying, unavailable, asked, type Store } from "./conversation";
import { onPartnerEvent, onStreamOpen } from "../notifications/stream";
import { useSubject } from "../shell/about";

/**
 * The conversation, held once for the whole page.
 *
 * There is one conversation, one draft and one running turn, and the drawer
 * and the focused page both read them from here. That is not tidiness: the two
 * are two views of one exchange, and the shell used to keep one draft while
 * the focused page kept another, so expanding the drawer stranded whatever was
 * half-written in it.
 *
 * The transcript is read when the page loads and again on every reconnect of
 * the one stream, and at no other time. Everything between arrives as beats on
 * that stream. Nothing here polls and nothing here sets a timer.
 */

type Partner = {
  store: Store;
  /** True while a turn is running, which is what disables the composer. */
  busy: boolean;
  /** What the human has written, shared by both composers. */
  draft: string;
  setDraft: (draft: string) => void;
  /**
   * Send the draft, or the text given. A refusal keeps it; acceptance clears
   * it. The text is passed by the composer's Enter, which reads the field
   * itself: a keystroke and the state it produced are two frames, and the
   * frame that sends must not be the one before.
   */
  send: (text?: string) => void;
  /** Stop the running turn. */
  stop: () => void;
  /** True while a send is in flight, so Send cannot be pressed twice. */
  sending: boolean;
};

const PartnerContext = createContext<Partner>({
  store: emptyStore,
  busy: false,
  draft: "",
  setDraft: () => {},
  send: () => {},
  stop: () => {},
  sending: false,
});

export function usePartner(): Partner {
  return useContext(PartnerContext);
}

export function PartnerProvider({ children }: { children: ReactNode }) {
  const [store, setStore] = useState<Store>(emptyStore);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  // The key this draft was minted with. It survives a refusal, so pressing
  // Send again after a 503 is the same turn rather than a second one.
  const key = useRef("");
  const location = useLocation();
  const subject = useSubject();

  const read = useCallback((signal?: AbortSignal) => {
    loadPartner(signal)
      .then((snapshot) => {
        setStore((held) => loaded(held, snapshot));
      })
      .catch((error: unknown) => {
        if (signal?.aborted === true) {
          return;
        }
        setStore((held) => unavailable(held, reasonOf(error)));
      });
  }, []);

  // The conversation, once, when the page loads.
  useEffect(() => {
    const aborter = new AbortController();
    read(aborter.signal);
    return () => {
      aborter.abort();
    };
  }, [read]);

  // The turn's beats, and the re-read that joins them to the transcript. A
  // reconnect means beats may have been missed while the connection was down,
  // so the snapshot is read again and the beats after it are joined by turn
  // and sequence, dropping the ones already counted.
  useEffect(() => onPartnerEvent((event) => {
    setStore((held) => received(held, event));
  }), []);
  useEffect(() => onStreamOpen(() => {
    read();
  }), [read]);

  // Where the human is, as the question carries it. It is composed here
  // because it belongs to the moment of asking: navigating afterwards cannot
  // retarget a question that has already been sent.
  const about = useMemo(() => pageOf(location.pathname, subject), [location.pathname, subject]);

  const running = busy(store);
  const send = useCallback((written?: string) => {
    const text = (written ?? draft).trim();
    if (text === "" || running || sending) {
      return;
    }
    key.current = keyFor(key.current);
    const minted = key.current;
    setSending(true);
    setStore(retrying);
    sendTurn(minted, text, about)
      .then((accepted) => {
        // Accepted: the draft goes, the key goes with it, and the question is
        // on screen before the first beat arrives.
        key.current = "";
        setDraft("");
        setStore((held) => asked(held, accepted.turn, minted, text, about, new Date().toISOString()));
      })
      .catch((error: unknown) => {
        // Refused: the draft stays, and so does the key, so pressing Send
        // again is this turn again rather than a second one.
        if (isBusy(error)) {
          setStore((held) => refused(held, reasonOf(error), ""));
          return;
        }
        setStore((held) => refused(held, reasonOf(error), installOf(error)));
      })
      .finally(() => {
        setSending(false);
      });
  }, [draft, running, sending, about]);

  const turn = store.live.turn;
  const stop = useCallback(() => {
    if (turn === "") {
      return;
    }
    stopTurn(turn)
      .then((snapshot) => {
        setStore((held) => loaded(held, snapshot));
      })
      .catch((error: unknown) => {
        setStore((held) => refused(held, reasonOf(error), ""));
      });
  }, [turn]);

  const value = useMemo(
    () => ({ store, busy: running, draft, setDraft, send, stop, sending }),
    [store, running, draft, send, stop, sending],
  );

  return <PartnerContext.Provider value={value}>{children}</PartnerContext.Provider>;
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function installOf(error: unknown): string {
  return error instanceof PartnerError ? error.install : "";
}
