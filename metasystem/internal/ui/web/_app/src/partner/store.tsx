import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation } from "react-router";

import { isBusy, loadPartner, PartnerError, sendTurn, stopTurn, type Page } from "./api";
import { captureOf, readingMoved } from "./capture";
import { chipped, insertAt } from "./composing";
import type { SheetDraft } from "./drafting";
import { keyFor } from "./asking";
import { busy, emptyStore, loaded, received, refused, retrying, unavailable, asked, type Store } from "./conversation";
import type { Chosen } from "./subject";
import { onPartnerEvent, onStreamOpen } from "../notifications/stream";
import { useAboutLine, useSubject } from "../shell/about";

/**
 * The conversation, held once for the whole page.
 *
 * There is one conversation, one draft, one running turn and one subject, and
 * the drawer and the focused page both read them from here. That is not
 * tidiness: the two are two views of one exchange, and the shell used to keep
 * one draft while the focused page kept another, so expanding the drawer
 * stranded whatever was half-written in it. The subject is here for the same
 * reason and one more: "this" has to keep meaning the same thing while a human
 * navigates, and a subject derived from the mounted pane disappears with it.
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

  /** The subject a human chose, or null while the subject follows the page. */
  chosen: Chosen | null;
  /** Make this the subject and take the caret to the composer. */
  ask: (chosen: Chosen) => void;
  /** Clear the chosen subject, which is what the chip's × does. */
  clearChosen: () => void;

  /**
   * A sheet a human handed over with "Ask about this", or null. It stands as a
   * chip above the composer until it is removed or another sheet replaces it,
   * exactly as a chosen subject does: what "this" means, and what the draft
   * beside it is, both stay put while a human writes their question.
   */
  sheetDraft: SheetDraft | null;
  /** Offer this sheet's fields, and take the caret to the composer. */
  askAbout: (draft: SheetDraft) => void;
  /** Stop offering it, which is what that chip's × does. */
  clearSheetDraft: () => void;
  /**
   * Say that this sheet is open, for as long as it is. The capture carries
   * its name, the Seeing line ends with it, and the message a question becomes
   * keeps it. It returns the way to take it back, so it is used as an effect.
   */
  noteSheet: (name: string) => () => void;
  /**
   * The capture the next question would carry, composed from the page as it
   * stands. It is what the "Seeing:" sheet asks about and what Send sends.
   */
  capture: Page;
  /** True when the page's reading has moved since the last question. */
  moved: boolean;
  /** Take the next capture from the page as it stands now, and send nothing. */
  refresh: () => void;
  /**
   * A suggested question. With an empty composer it sends; with a draft
   * present it inserts its words at the cursor and sends nothing.
   */
  suggest: (text: string) => void;
  /** The composer says how to insert at its cursor while it is on screen. */
  offerInsert: (insert: ((text: string) => void) | null) => void;
  /** How many times the composer has been asked for; the shell watches it. */
  wanted: number;
  /** Put the caret back where Ask took it from. */
  returnFocus: () => void;
};

const nothing: Partner = {
  store: emptyStore,
  busy: false,
  draft: "",
  setDraft: () => {},
  send: () => {},
  stop: () => {},
  sending: false,
  chosen: null,
  ask: () => {},
  clearChosen: () => {},
  sheetDraft: null,
  askAbout: () => {},
  clearSheetDraft: () => {},
  noteSheet: () => () => {},
  capture: { section: "", path: "" },
  moved: false,
  refresh: () => {},
  suggest: () => {},
  offerInsert: () => {},
  wanted: 0,
  returnFocus: () => {},
};

const PartnerContext = createContext<Partner>(nothing);

export function usePartner(): Partner {
  return useContext(PartnerContext);
}

/**
 * While this sheet is on screen, the capture says so: the Seeing line ends
 * with "· New goal sheet open", the question carries the name, and the message
 * it becomes keeps it.
 *
 * A sheet that shows the capture itself passes "" and names nothing, because a
 * sheet naming itself in the very block it is displaying would be telling the
 * human about the act of looking rather than about the page.
 */
export function useOpenSheet(name: string): void {
  const { noteSheet } = usePartner();
  useEffect(() => {
    if (name === "") {
      return;
    }
    return noteSheet(name);
  }, [noteSheet, name]);
}

export function PartnerProvider({ children }: { children: ReactNode }) {
  const [store, setStore] = useState<Store>(emptyStore);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  // The subject a human chose. It survives navigation, and it is cleared by
  // its own chip and by choosing another — never by arriving somewhere else.
  const [chosen, setChosen] = useState<Chosen | null>(null);
  // The sheet a human handed over, and the sheets that are open over the work
  // area. They are two different facts: a sheet is open whether or not anybody
  // offered it, and an offered draft outlives the sheet it came from, because
  // the question about it is still being written.
  const [sheetDraft, setSheetDraft] = useState<SheetDraft | null>(null);
  const [sheets, setSheets] = useState<readonly string[]>([]);
  // The capture the last question was sent with, which is what "the page has
  // moved since" is measured against. Refresh replaces it with the page as it
  // stands, which prepares the next capture and changes no stamp.
  const [baseline, setBaseline] = useState<Page | null>(null);
  const [wanted, setWanted] = useState(0);
  // The key this draft was minted with. It survives a refusal, so pressing
  // Send again after a 503 is the same turn rather than a second one.
  const key = useRef("");
  // Where Ask took the caret from, so Escape can put it back.
  const cameFrom = useRef<HTMLElement | null>(null);
  // How the composer on screen inserts at its own cursor.
  const insert = useRef<((text: string) => void) | null>(null);
  const location = useLocation();
  const subject = useSubject();
  const label = useAboutLine("");

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

  // The capture: the page as it stands, with whatever subject is in force. It
  // is composed here because it belongs to the moment of asking, and it is
  // what the sheet shows, what the question carries, and what the message
  // keeps — one composition rather than three.
  // The innermost sheet is the one a human is standing in, and the one the
  // capture names.
  const sheet = sheets.at(-1) ?? "";
  const capture = useMemo(
    () => captureOf({ pathname: location.pathname, page: subject, chosen, label, sheet, draft: sheetDraft }),
    [location.pathname, subject, chosen, label, sheet, sheetDraft],
  );
  const moved = useMemo(() => readingMoved(baseline, subject), [baseline, subject]);

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
    sendTurn(minted, text, capture)
      .then((accepted) => {
        // Accepted: the draft goes, the key goes with it, and the question is
        // on screen before the first beat arrives.
        key.current = "";
        setDraft("");
        setBaseline(capture);
        setStore((held) => asked(held, accepted.turn, minted, text, capture, new Date().toISOString()));
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
  }, [draft, running, sending, capture]);

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

  /**
   * Ask about this: the thing becomes the subject, the drawer opens, and the
   * caret goes to the composer. Where it came from is remembered, because
   * Escape from the composer belongs back on the card it was opened from.
   */
  const ask = useCallback((next: Chosen) => {
    const active = globalThis.document.activeElement;
    cameFrom.current = active instanceof HTMLElement ? active : null;
    setChosen(next);
    setWanted((at) => at + 1);
  }, []);

  const clearChosen = useCallback(() => {
    setChosen(null);
  }, []);

  /**
   * Ask about this sheet: what is written in it right now becomes a chip above
   * the composer, the drawer opens, and the caret goes to the field. It is the
   * same act Ask on a card is, and it takes the same route — nothing is sent,
   * and the question is still the human's to write.
   */
  const askAbout = useCallback((draft: SheetDraft) => {
    const active = globalThis.document.activeElement;
    cameFrom.current = active instanceof HTMLElement ? active : null;
    setSheetDraft(draft);
    setWanted((at) => at + 1);
  }, []);

  const clearSheetDraft = useCallback(() => {
    setSheetDraft(null);
  }, []);

  /**
   * A sheet says it is open, and says so again by its own name rather than by
   * an identity, because two sheets of the same name are the same answer to
   * "where is the human". They are held as a stack so that a sheet opened over
   * a sheet is the one that is named, and closing it names the one beneath.
   */
  const noteSheet = useCallback((name: string) => {
    setSheets((held) => [...held, name]);
    return () => {
      setSheets((held) => {
        const at = held.lastIndexOf(name);
        return at < 0 ? held : [...held.slice(0, at), ...held.slice(at + 1)];
      });
      // A draft's life is its sheet's: cancelled, the thing it described is
      // gone; opened, it is a goal now. Either way the chip would describe
      // nothing, so it goes with the sheet (Wido, 2026-09-24). A draft from
      // another sheet stands.
      setSheetDraft((draft) => (draft !== null && draft.sheet === name ? null : draft));
    };
  }, []);

  const returnFocus = useCallback(() => {
    const source = cameFrom.current;
    cameFrom.current = null;
    source?.focus();
  }, []);

  const refresh = useCallback(() => {
    setBaseline(capture);
  }, [capture]);

  /**
   * A suggestion respects the draft. With an empty composer the chip is the
   * question and sends; with something half-written in it the chip's words go
   * in at the cursor and nothing is sent, because the sentence a human was
   * composing is theirs.
   */
  const suggest = useCallback((text: string) => {
    if (chipped(draft) === "send") {
      send(text);
      return;
    }
    const at = insert.current;
    if (at === null) {
      // No composer is on screen to take a cursor, so the words go on the end,
      // which is where a caret nobody has placed would be.
      setDraft((held) => insertAt(held, held.length, held.length, text).text);
      return;
    }
    at(text);
  }, [draft, send]);

  const offerInsert = useCallback((at: ((text: string) => void) | null) => {
    insert.current = at;
  }, []);

  const value = useMemo(
    () => ({
      store, busy: running, draft, setDraft, send, stop, sending,
      chosen, ask, clearChosen, sheetDraft, askAbout, clearSheetDraft, noteSheet,
      capture, moved, refresh, suggest, offerInsert,
      wanted, returnFocus,
    }),
    [store, running, draft, send, stop, sending, chosen, ask, clearChosen,
      sheetDraft, askAbout, clearSheetDraft, noteSheet,
      capture, moved, refresh, suggest, offerInsert, wanted, returnFocus],
  );

  return <PartnerContext.Provider value={value}>{children}</PartnerContext.Provider>;
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function installOf(error: unknown): string {
  return error instanceof PartnerError ? error.install : "";
}
