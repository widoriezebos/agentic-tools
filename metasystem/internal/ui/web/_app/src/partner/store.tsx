import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation } from "react-router";

import { isBusy, loadPartner, PartnerError, sendTurn, stopTurn, type CapturedSticky, type Page } from "./api";
import {
  attach,
  attachedDraft,
  attachedPassage,
  attachedSubject,
  draftIn,
  idFor,
  passageIn,
  refreshDraft,
  remove,
  retireOnSent,
  retireOnSheetClosed,
  subjectIn,
  type Attachment,
} from "./attachments";
import { captureOf, readingMoved } from "./capture";
import { chipped, insertAt } from "./composing";
import type { SheetDraft } from "./drafting";
import { keyFor } from "./asking";
import { busy, emptyStore, loaded, received, refused, retrying, unavailable, asked, type Store } from "./conversation";
import type { Chosen } from "./subject";
import { onPartnerEvent, onStreamOpen } from "../notifications/stream";
import { useAboutLine, useSubject } from "../shell/about";
import { captured } from "../stickies/stickies";
import { useStickies } from "../stickies/store";

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
 * Everything above the composer is one list of attachments, held here, each
 * with the lifetime the act that made it declared. The store keeps the list
 * and nothing else: the subject, the passage and the offered sheet are views
 * over it, so there is one place an attachment is made and one place it is
 * retired. src/partner/attachments.ts holds the rules.
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

  /**
   * Everything above the composer, in the order it was attached. It is what
   * the chips render from and what the capture is composed from, so what the
   * Partner is told is exactly what the human can see.
   */
  attachments: readonly Attachment[];
  /** Take one back, which is what every chip's × does. */
  detach: (id: string) => void;

  /** The subject a human chose, or null while the subject follows the page. */
  chosen: Chosen | null;
  /**
   * Make this the subject and take the caret to the composer. It stays until
   * it is cleared or replaced; a selected passage is the other act, below,
   * because it lives for one question rather than until it is taken back.
   */
  ask: (chosen: Chosen) => void;
  /** Clear the chosen subject, which is what the chip's × does. */
  clearChosen: () => void;

  /** A passage a human selected, or null. It goes with the next question. */
  passage: Chosen | null;
  /** Attach this passage, and take the caret to the composer. */
  askPassage: (passage: Chosen) => void;

  /**
   * A sheet a human handed over with "Ask about this", or null. It stands as a
   * chip above the composer for as long as its sheet is open: cancelled or
   * opened, the thing it described is gone and the chip would describe
   * nothing.
   */
  sheetDraft: SheetDraft | null;
  /** Offer this sheet's fields, and take the caret to the composer. */
  askAbout: (draft: SheetDraft) => void;
  /**
   * How an open sheet reads its own fields right now, so that a question
   * carries the draft as the sheet stands rather than as it stood when it was
   * handed over. A sheet that has handed nothing over is still never read.
   */
  offerFields: (sheet: string, read: (() => SheetDraft) | null) => void;
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
  attachments: [],
  detach: () => {},
  chosen: null,
  ask: () => {},
  clearChosen: () => {},
  passage: null,
  askPassage: () => {},
  sheetDraft: null,
  askAbout: () => {},
  offerFields: () => {},
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
  // Everything above the composer, in the order it was attached. One list,
  // because a subject, a passage and an offered sheet are one kind of thing —
  // context a human put there by one act — and three states were three places
  // for a rule about when they leave to be written and forgotten.
  const [attachments, setAttachments] = useState<readonly Attachment[]>([]);
  // The sheets that are open over the work area. That is a different fact from
  // a sheet a human handed over: a sheet is open whether or not anybody
  // offered it, and it is this stack that says where the human is standing.
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
  // How each open sheet reads its own fields, by the sheet's own name. It is
  // read at one moment and no other: the instant a question is sent.
  const openFields = useRef(new Map<string, () => SheetDraft>());
  const location = useLocation();
  const subject = useSubject();
  const stickies = useStickies();
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

  // What the components that read one attachment read: a view over the list
  // rather than a state of its own, so nothing can hold a subject the list has
  // retired.
  const chosen = useMemo(() => subjectIn(attachments), [attachments]);
  const passage = useMemo(() => passageIn(attachments), [attachments]);
  const sheetDraft = useMemo(() => draftIn(attachments), [attachments]);

  // The capture: the page as it stands, with the attachments that are on it.
  // It is composed here because it belongs to the moment of asking, and it is
  // what the sheet shows, what the question carries, and what the message
  // keeps — one composition rather than three. It is composed from the list
  // the chips are drawn from, so the Partner is told what the human sees.
  // The innermost sheet is the one a human is standing in, and the one the
  // capture names.
  const sheet = sheets.at(-1) ?? "";
  // The human's own notepad, as the page is showing it. It is read here rather
  // than described by each pane, because what travels depends on something no
  // pane knows: whether the panel is open over it.
  const notepad = captured(stickies.notepad, stickies.panelIsOpen, subject);
  const noted = JSON.stringify(notepad);
  const compose = useCallback(
    (list: readonly Attachment[]) =>
      captureOf({
        pathname: location.pathname,
        page: subject,
        label,
        sheet,
        notepad: JSON.parse(noted) as { stickies: CapturedSticky[]; open: number },
        chosen: subjectIn(list),
        passage: passageIn(list),
        draft: draftIn(list),
      }),
    [location.pathname, subject, label, sheet, noted],
  );
  const capture = useMemo(() => compose(attachments), [compose, attachments]);

  /**
   * Every open sheet's draft, brought up to date from the sheet itself. A
   * sheet that handed nothing over is not read, and a list with nothing to
   * bring up to date comes back as it went in.
   */
  const refreshed = useCallback((held: readonly Attachment[]) => {
    let next = held;
    for (const fields of openFields.current.values()) {
      next = refreshDraft(next, fields());
    }
    return next;
  }, []);
  const moved = useMemo(() => readingMoved(baseline, subject), [baseline, subject]);

  const running = busy(store);
  const send = useCallback((written?: string) => {
    const text = (written ?? draft).trim();
    if (text === "" || running || sending) {
      return;
    }
    // An open sheet says what is in it now, so the question carries the draft
    // as the sheet stands rather than as it stood when it was handed over.
    const list = refreshed(attachments);
    const taken = list === attachments ? capture : compose(list);
    if (list !== attachments) {
      setAttachments(list);
    }
    key.current = keyFor(key.current);
    const minted = key.current;
    setSending(true);
    setStore(retrying);
    sendTurn(minted, text, taken)
      .then((accepted) => {
        // Accepted: the draft goes, the key goes with it, and the question is
        // on screen before the first beat arrives.
        key.current = "";
        setDraft("");
        setBaseline(taken);
        // The question carried them: the first of the two retiring events.
        // What goes with a question goes now; the subject and the sheets a
        // human is still filling in stand.
        setAttachments(retireOnSent);
        setStore((held) => asked(held, accepted.turn, minted, text, taken, new Date().toISOString()));
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
  }, [draft, running, sending, capture, attachments, compose, refreshed]);

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
   * What every Ask does besides attaching: the drawer opens and the caret goes
   * to the composer. Where it came from is remembered, because Escape from the
   * composer belongs back on the card it was opened from.
   */
  const wantComposer = useCallback(() => {
    const active = globalThis.document.activeElement;
    cameFrom.current = active instanceof HTMLElement ? active : null;
    setWanted((at) => at + 1);
  }, []);

  const detach = useCallback((id: string) => {
    setAttachments((held) => remove(held, id));
  }, []);

  /**
   * Ask about this: the thing becomes the subject, and stays the subject until
   * it is cleared or replaced. Navigating does not touch it.
   */
  const ask = useCallback((next: Chosen) => {
    setAttachments((held) => attach(held, attachedSubject(next)));
    wantComposer();
  }, [wantComposer]);

  const clearChosen = useCallback(() => {
    detach(idFor("subject"));
  }, [detach]);

  /**
   * Ask on a selection: the passage is attached for one question. A quote is
   * said once, and one that stayed would be carried by questions nobody meant
   * it for.
   */
  const askPassage = useCallback((next: Chosen) => {
    setAttachments((held) => attach(held, attachedPassage(next)));
    wantComposer();
  }, [wantComposer]);

  /**
   * Ask about this sheet: what is written in it right now becomes a chip above
   * the composer, and it lives as long as its sheet does. It is the same act
   * Ask on a card is, and it takes the same route — nothing is sent, and the
   * question is still the human's to write.
   */
  const askAbout = useCallback((draft: SheetDraft) => {
    setAttachments((held) => attach(held, attachedDraft(draft)));
    wantComposer();
  }, [wantComposer]);

  /** A sheet says how to read its fields, for as long as it is on screen. */
  const offerFields = useCallback((name: string, read: (() => SheetDraft) | null) => {
    if (read === null) {
      openFields.current.delete(name);
      return;
    }
    openFields.current.set(name, read);
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
      // The second of the two retiring events. A draft's life is its sheet's:
      // cancelled, the thing it described is gone; opened, it is a goal now.
      // Either way the chip would describe nothing (Wido, 2026-09-24). It is
      // the lifetime that decides, so a draft from another sheet stands and
      // nothing here knows what kinds of attachment there are.
      setAttachments((held) => retireOnSheetClosed(held, name));
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
      attachments, detach, chosen, ask, clearChosen, passage, askPassage,
      sheetDraft, askAbout, offerFields, noteSheet,
      capture, moved, refresh, suggest, offerInsert,
      wanted, returnFocus,
    }),
    [store, running, draft, send, stop, sending, attachments, detach, chosen, ask,
      clearChosen, passage, askPassage, sheetDraft, askAbout, offerFields, noteSheet,
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
