import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation } from "react-router";

import {
  endSitting,
  isBusy,
  loadPartner,
  PartnerError,
  sendTurn,
  startSitting,
  stopTurn,
  type CapturedSticky,
  type Page,
  type Sitting,
  type Subject,
} from "./api";
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
import { valueIn, type SheetDraft } from "./drafting";
import {
  cardIn,
  folded,
  holding,
  NOWHERE,
  reach,
  offeredIn,
  undoable,
  undone,
  usable,
  used,
  type InHand,
  type Marks,
  type Offered,
  type Registered,
} from "./suggesting";
import { keyFor } from "./asking";
import { recorder, type Reading, type Recorder } from "./recording";
import {
  cardIn as depositIn,
  cardsIn as depositsIn,
  countsIn,
  entriesIn,
  entryOf,
  marked,
  missing,
  type Card as DepositCard,
  type Counts,
  type Entry,
  type Marks as DepositMarks,
} from "./sitting";
import { busy, emptyStore, loaded, nameOf, received, refused, retrying, unavailable, asked, type Store } from "./conversation";
import type { Chosen } from "./subject";
import { onPartnerEvent, onStreamOpen } from "../notifications/stream";
import { useAboutLine, useSubject } from "../shell/about";
import { editDocument, isStale, loadDocument } from "../project/api";
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
   * Offer this sheet's fields because the sheet opened, and do nothing else:
   * the drawer stays as the human left it and the caret stays in the field they
   * are typing in.
   *
   * It is the hand-over, and it is why there is no hidden first press any more
   * (g1-s52 D1). The master's rule is that unsaved edits are shared "only as
   * explicitly identified draft context"; the chip above the composer, with its
   * ×, is that identification, and it stands whether or not the drawer is open.
   * What it must not do is what Ask does — open the drawer and take the caret —
   * because the human opened a sheet to write in it.
   */
  handOver: (draft: SheetDraft) => void;
  /**
   * This sheet's chip, brought up to date — and nothing where the human has
   * taken it back.
   *
   * It is how the chip says which field the caret is in without the × being
   * undone by moving the caret: a draft nobody is offering is not re-offered by
   * bringing it up to date.
   */
  noteDraft: (draft: SheetDraft) => void;
  /** Stop offering this sheet's draft, which is what its unmount does. */
  dropDraft: (sheet: string) => void;
  /**
   * One opening of an editor, while it is on screen: how it reads its own
   * fields right now, so that a question carries the draft as the sheet stands
   * rather than as it stood when it was handed over, and how to put words into
   * one of its fields. Null takes the registration back, which is what the
   * sheet's own unmount does.
   *
   * It is keyed by the opening the sheet minted when it mounted, not by the
   * sheet's name: two openings of "Edit goal" are two different goals, and a
   * suggestion prepared for one must never be usable in the other.
   */
  offerFields: (opening: string, registered: Registered | null) => void;
  /**
   * Every suggestion this conversation carries, oldest first, each with where
   * it stands. The cards in the transcript and the link beside a field are
   * both read from this one list.
   */
  offered: readonly Offered[];
  /**
   * Use this: the suggestion's words replace that field's whole value, and
   * what the field held is kept for Undo. It does nothing where the editor
   * opening it was prepared for has gone — a closed sheet, or a sheet of the
   * same name opened again — because there is nothing left to write into.
   */
  use: (id: string) => void;
  /** Undo: what the field held at the moment of use goes back. */
  undo: (id: string) => void;
  /** Dismiss: the card folds to one line. */
  dismiss: (id: string) => void;
  /** The folded line, pressed: the card is back. */
  reopen: (id: string) => void;
  /** Open the drawer at one card, which is what a field's link does. */
  show: (id: string) => void;
  /** The card the drawer was last opened at, or "". */
  showing: string;
  /**
   * A field says what it now holds, so that Undo is offered only while the
   * field still holds the suggestion's words. Nothing changes where the answer
   * is the same as it was.
   */
  noteField: (opening: string, field: string, value: string) => void;
  /**
   * The writable field the caret is in, and the opening it belongs to.
   *
   * It is one field for the whole page because a human writes in one field at a
   * time. The chip says it, the capture carries it, and the Partner is told it,
   * so that a request naming no field is about the field they were in.
   */
  writing: InHand;
  /** A writable field says the caret is in it, which is what its focus does. */
  noteWriting: (opening: string, field: string) => void;
  /**
   * Put these words in the composer and take the caret there, sending nothing.
   *
   * It is what a field's own "Ask the Partner" does. With something half-written
   * the words go in at the cursor rather than over them, because the sentence a
   * human was composing is theirs.
   */
  fillComposer: (text: string) => void;
  /** How many times a card has been asked for; the shell opens the drawer. */
  revealed: number;
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

  /**
   * The sitting this conversation is, or null. It is the server's answer, read
   * from the snapshot, so a reload and a second tab agree about which record is
   * under discussion.
   */
  sitting: Sitting | null;
  /**
   * Start a sitting: on the record named, or on a draft the server creates under
   * the title given. The opening turn is the server's, and it is in the
   * transcript the answer carries.
   */
  startSitting: (asked: { purpose: string; subject?: Subject; title?: string }) => Promise<void>;
  /** End the sitting. What was recorded stays in the record. */
  endSitting: () => void;
  /** What a Start or End press was refused with, in the server's own words. */
  sittingRefusal: string;
  /** True while a Start or End press is in flight. */
  sittingBusy: boolean;

  /**
   * Every deposit this conversation carries, oldest first, each with where it
   * stands. The card on the transcript is rendered from this one list.
   */
  deposits: readonly DepositCard[];
  /** The words of one card, as the human now has them. */
  editDeposit: (id: string, text: string) => void;
  /** The clause of one card: its anchor, its reason or its consequence. */
  editClause: (id: string, clause: string) => void;
  /**
   * Record it. It composes the entry from the sitting's current reading of the
   * record, writes the whole source under that reading's revision, and refreshes
   * the reading from what the write answered. Presses are serialized: two in a
   * row land both entries, and two at once land both in the order they were
   * pressed.
   */
  recordDeposit: (id: string) => void;
  /** Dismiss: the card folds to one line. */
  dismissDeposit: (id: string) => void;
  /** The folded line, pressed: the card is back. */
  reopenDeposit: (id: string) => void;
  /**
   * The record's four sections as the sitting's current reading holds them: what
   * the table shows and what the drawer's four counts count. They are the record
   * and not a second store, so a card nobody recorded is not among them.
   */
  table: { counts: Counts; entries: readonly Entry[]; revision: string };
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
  handOver: () => {},
  noteDraft: () => {},
  dropDraft: () => {},
  offerFields: () => {},
  offered: [],
  use: () => {},
  undo: () => {},
  dismiss: () => {},
  reopen: () => {},
  show: () => {},
  showing: "",
  noteField: () => {},
  writing: NOWHERE,
  noteWriting: () => {},
  fillComposer: () => {},
  revealed: 0,
  noteSheet: () => () => {},
  capture: { section: "", path: "" },
  moved: false,
  refresh: () => {},
  suggest: () => {},
  offerInsert: () => {},
  wanted: 0,
  returnFocus: () => {},
  sitting: null,
  startSitting: async () => {},
  endSitting: () => {},
  sittingRefusal: "",
  sittingBusy: false,
  deposits: [],
  editDeposit: () => {},
  editClause: () => {},
  recordDeposit: () => {},
  dismissDeposit: () => {},
  reopenDeposit: () => {},
  table: { counts: { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0 }, entries: [], revision: "" },
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
  // What the human has done with each suggestion: what the field held before
  // Use this, whether it still holds the words, and whether the card is
  // folded. It is the page's own state and not the server's — the server
  // admitted the suggestion, and what happens to a field afterwards is the
  // human's.
  const [marks, setMarks] = useState<Marks>({});
  // Which editor openings are on screen, by the id each minted when it
  // mounted. It is state rather than a ref because a card has to change what it
  // says when its sheet closes; the registration's own functions are in the ref
  // below, which is refreshed on every render and re-renders nothing.
  const [open, setOpen] = useState<readonly string[]>([]);
  const [showing, setShowing] = useState("");
  const [revealed, setRevealed] = useState(0);
  // Which writable field the caret is in, of which opening. It is state and not
  // a ref because the chip has to change what it says when the caret moves.
  const [writing, setWriting] = useState<InHand>(NOWHERE);
  // What the human has made of each deposit: the words as they now stand, the
  // clause, whether a press is in flight, where the record took it, and whether
  // the card is folded. It is the page's own state and not the server's — the
  // server admitted the deposit, and what happens to it afterwards is the
  // human's until they press Record it.
  const [depositMarks, setDepositMarks] = useState<DepositMarks>({});
  // The sitting's one current reading of its record: source and revision, taken
  // when the sitting starts and replaced by the answer of every successful
  // write. The table reads it, and every Record press composes from it.
  const [reading, setReading] = useState<Reading | null>(null);
  const [sittingRefusal, setSittingRefusal] = useState("");
  const [sittingBusy, setSittingBusy] = useState(false);
  // The key this draft was minted with. It survives a refusal, so pressing
  // Send again after a 503 is the same turn rather than a second one.
  const key = useRef("");
  // Where Ask took the caret from, so Escape can put it back.
  const cameFrom = useRef<HTMLElement | null>(null);
  // How the composer on screen inserts at its own cursor.
  const insert = useRef<((text: string) => void) | null>(null);
  // How each open editor reads its own fields and writes into them, by the
  // opening it minted when it mounted. The reading happens at one moment and no
  // other — the instant a question is sent — and the writing only when a human
  // presses Use this or Undo.
  const openings = useRef(new Map<string, Registered>());
  // The one recorder of this sitting, which owns the reading and the queue. It
  // is a ref because it is not something a render reads: what a render reads is
  // the reading above, which the recorder hands back after every write.
  const recording = useRef<Recorder | null>(null);
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
  const notepad = captured(stickies.notepad, stickies.panelIsOpen, stickies.doneIsOpen, subject);
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
    for (const registered of openings.current.values()) {
      next = refreshDraft(next, registered.read());
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

  /**
   * The sheet opened, so its draft is offered: the same attachment "Ask about
   * this" makes, and nothing else.
   *
   * Not opening the drawer and not taking the caret is the whole of the
   * difference, and it is the point (g1-s52 D1). A human opened a sheet to write
   * in it; a hand-over that moved their caret or covered their work would be the
   * interface deciding what they are doing. The chip above the composer is what
   * makes the sharing explicit rather than silent, and it stands wherever the
   * composer is shown.
   */
  const handOver = useCallback((draft: SheetDraft) => {
    setAttachments((held) => attach(held, attachedDraft(draft)));
  }, []);

  /**
   * This sheet's chip, brought up to date, and nothing where nobody is offering
   * it. It is the refresh the send path uses, reached from the sheet so that the
   * chip can say which field the caret is in.
   */
  const noteDraft = useCallback((draft: SheetDraft) => {
    setAttachments((held) => refreshDraft(held, draft));
  }, []);

  const dropDraft = useCallback((sheet: string) => {
    setAttachments((held) => remove(held, idFor("draft", sheet)));
  }, []);

  /**
   * One opening of an editor, for as long as it is on screen.
   *
   * It is called on every render of the sheet, because the functions it carries
   * close over the sheet's own state and a stale one would read or write last
   * keystroke's draft. The list of open ids is what the cards watch, and it
   * changes only when the membership does: passing the same opening again
   * leaves the state as it was, so re-registering re-renders nothing.
   */
  const offerFields = useCallback((opening: string, registered: Registered | null) => {
    if (registered === null) {
      openings.current.delete(opening);
      setOpen((held) => (held.includes(opening) ? held.filter((one) => one !== opening) : held));
      return;
    }
    openings.current.set(opening, registered);
    setOpen((held) => (held.includes(opening) ? held : [...held, opening]));
  }, []);

  /**
   * Every suggestion the conversation carries, with where each one stands: the
   * answers already written down, then the answer arriving now. It is composed
   * here, from the transcript and the marks, so that the card in the drawer and
   * the link beside a field cannot disagree about one suggestion.
   */
  const offered = useMemo(
    () =>
      offeredIn(
        [
          ...store.messages
            .filter((message) => (message.suggestions ?? []).length > 0)
            .map((message) => ({ turn: message.turn, suggestions: message.suggestions ?? [] })),
          { turn: store.live.turn, suggestions: store.live.suggestions },
        ],
        marks,
        open,
      ),
    [store.messages, store.live.turn, store.live.suggestions, marks, open],
  );

  /**
   * Use this. The setter is the sheet's own, so what a field ends up holding is
   * decided by the sheet that owns it; what comes back is what it held before,
   * which is what Undo puts back.
   */
  const use = useCallback((id: string) => {
    const card = cardIn(offered, id);
    const registered = reach(openings.current, card);
    if (card === undefined || registered === null || !usable(card, [...openings.current.keys()])) {
      return;
    }
    const previous = registered.set(card.field, card.text);
    setMarks((held) => used(held, id, previous));
  }, [offered]);

  /**
   * Undo. It asks the sheet what the field holds before it writes, and not only
   * the mark the card is showing: the mark is kept up to date by the link beside
   * that field, and a field whose link is not on screen — a sheet's disclosure
   * folded away — would otherwise have the human's own words replaced in the
   * name of undoing ours. The field's own answer decides.
   */
  const undo = useCallback((id: string) => {
    const card = cardIn(offered, id);
    const registered = reach(openings.current, card);
    if (card === undefined || registered === null || !undoable(card, [...openings.current.keys()])) {
      return;
    }
    if (valueIn(registered.read(), card.field) !== card.text.trim()) {
      setMarks((held) => holding(held, offered, card.opening, card.field, ""));
      return;
    }
    registered.set(card.field, card.mark.previous ?? "");
    setMarks((held) => undone(held, id));
  }, [offered]);

  const dismiss = useCallback((id: string) => {
    setMarks((held) => folded(held, id, true));
  }, []);

  const reopen = useCallback((id: string) => {
    setMarks((held) => folded(held, id, false));
  }, []);

  /** A field's link: the drawer opens, and it opens at this card. */
  const show = useCallback((id: string) => {
    setShowing(id);
    setRevealed((at) => at + 1);
  }, []);

  const noteField = useCallback((opening: string, field: string, value: string) => {
    setMarks((held) => holding(held, offered, opening, field, value));
  }, [offered]);

  /**
   * A writable field says the caret is in it.
   *
   * The last one stands: blurring a field does not put the human nowhere, it
   * leaves them where they were, and a request written in the composer is about
   * the field they came from. An answer that has not changed changes no state, so
   * clicking about inside one field re-renders nothing.
   */
  const noteWriting = useCallback((opening: string, field: string) => {
    setWriting((held) => (held.opening === opening && held.field === field ? held : { opening, field }));
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

  /**
   * A field's own "Ask the Partner": the request goes in the composer and the
   * caret goes with it, and nothing is sent.
   *
   * It respects a half-written question for the same reason a suggested question
   * does — the sentence is the human's — so an empty composer is filled and one
   * with words in it takes them at the cursor. What it never does is send:
   * "Suggest a better Intent" is a starting point a human edits, not a question
   * the interface asks on their behalf (g1-s52 D4).
   */
  const fillComposer = useCallback((text: string) => {
    if (chipped(draft) === "send") {
      setDraft(text);
      wantComposer();
      return;
    }
    const at = insert.current;
    if (at === null) {
      setDraft((held) => insertAt(held, held.length, held.length, text).text);
    } else {
      at(text);
    }
    wantComposer();
  }, [draft, wantComposer]);

  const offerInsert = useCallback((at: ((text: string) => void) | null) => {
    insert.current = at;
  }, []);

  /* ------------------------------------------------------------- the sitting -- */

  const sitting = store.sitting;

  /**
   * The sitting's reading, taken when the subject changes and at no other time.
   *
   * It is read once per subject rather than on every render, because it is a
   * reading: a second one taken behind a human's back would move the revision a
   * Record press is about to write under. Every later reading comes from a write
   * this page made, or from the reread a conflict asks for.
   */
  const subjectID = sitting?.subject.id ?? "";
  useEffect(() => {
    if (subjectID === "") {
      recording.current = null;
      setReading(null);
      return;
    }
    let alive = true;
    loadDocument(subjectID)
      .then((document) => {
        if (!alive) {
          return;
        }
        const taken = { id: subjectID, revision: document.revision, source: document.source };
        recording.current = recorder(
          taken,
          async (id, source, revision) => {
            const saved = await editDocument(id, source, revision);
            return { revision: saved.revision, source: saved.source };
          },
          async (id) => {
            const again = await loadDocument(id);
            return { revision: again.revision, source: again.source };
          },
          isStale,
        );
        setReading(taken);
      })
      .catch((error: unknown) => {
        if (alive) {
          setSittingRefusal(reasonOf(error));
        }
      });
    return () => {
      alive = false;
    };
  }, [subjectID]);

  const begin = useCallback(
    async (asked: { purpose: string; subject?: Subject; title?: string }) => {
      setSittingBusy(true);
      setSittingRefusal("");
      try {
        const answered = await startSitting({ ...asked, about: capture });
        setStore((held) => loaded(held, answered));
      } catch (error: unknown) {
        setSittingRefusal(reasonOf(error));
        throw error;
      } finally {
        setSittingBusy(false);
      }
    },
    [capture],
  );

  const end = useCallback(() => {
    setSittingBusy(true);
    setSittingRefusal("");
    endSitting()
      .then((answered) => {
        setStore((held) => loaded(held, answered));
      })
      .catch((error: unknown) => {
        setSittingRefusal(reasonOf(error));
      })
      .finally(() => {
        setSittingBusy(false);
      });
  }, []);

  /**
   * Every deposit the conversation carries, with where each one stands: the
   * answers already written down, then the answer arriving now. It is composed
   * here, from the transcript and the marks, so the card on the transcript and
   * the table cannot disagree about one deposit.
   */
  const deposits = useMemo(
    () =>
      depositsIn(
        [
          ...store.messages
            .filter((message) => (message.deposits ?? []).length > 0)
            .map((message) => ({ turn: message.turn, deposits: message.deposits ?? [] })),
          { turn: store.live.turn, deposits: store.live.deposits },
        ],
        depositMarks,
      ),
    [store.messages, store.live.turn, store.live.deposits, depositMarks],
  );

  const changeMark = useCallback(
    (id: string, change: (mark: DepositMarks[string]) => DepositMarks[string]) => {
      setDepositMarks((held) => {
        const card = depositIn(deposits, id);
        if (card === undefined) {
          return held;
        }
        return { ...held, [id]: change(held[id] ?? marked(card)) };
      });
    },
    [deposits],
  );

  const editDeposit = useCallback(
    (id: string, text: string) => {
      changeMark(id, (mark) => ({ ...mark, text, refusal: "" }));
    },
    [changeMark],
  );

  const editClause = useCallback(
    (id: string, clause: string) => {
      changeMark(id, (mark) => ({ ...mark, clause, refusal: "" }));
    },
    [changeMark],
  );

  const dismissDeposit = useCallback(
    (id: string) => {
      changeMark(id, (mark) => ({ ...mark, dismissed: true }));
    },
    [changeMark],
  );

  const reopenDeposit = useCallback(
    (id: string) => {
      changeMark(id, (mark) => ({ ...mark, dismissed: false }));
    },
    [changeMark],
  );

  /**
   * Record it.
   *
   * The card that is pressed must be admitted, must still need nothing, and must
   * not already be in flight — a second press of one card is one entry, not two.
   * Everything else is the recorder's: the entry is composed from the reading as
   * it stands when the press runs, the write goes under that reading's revision,
   * and the reading is refreshed from what the write answered.
   *
   * A conflict keeps the card exactly as the human has it and says so; pressing
   * again is one more press, now against the record the reread brought back.
   */
  const recordDeposit = useCallback(
    (id: string) => {
      const card = depositIn(deposits, id);
      const held = recording.current;
      if (card === undefined || held === null || !card.offered) {
        return;
      }
      if (card.mark.recording || card.mark.recorded !== "" || missing(card.kind, card.mark) !== "") {
        return;
      }
      changeMark(id, (mark) => ({ ...mark, recording: true, refusal: "" }));
      const entry = entryOf(card, nameOf(store.human), stampOf(new Date()));
      void held.press(entry, card.kind).then((outcome) => {
        if (outcome.kind !== "failed") {
          setReading(outcome.reading);
        }
        setDepositMarks((marks) => {
          const mark = marks[id];
          if (mark === undefined) {
            return marks;
          }
          if (outcome.kind === "recorded") {
            return { ...marks, [id]: { ...mark, recording: false, recorded: outcome.section, refusal: "" } };
          }
          return { ...marks, [id]: { ...mark, recording: false, refusal: outcome.reason } };
        });
      });
    },
    [deposits, changeMark, store.human],
  );

  /**
   * The table: the record's four sections as the sitting's reading holds them.
   *
   * It is derived from the one reading and nothing else, which is what makes it
   * the record rather than a second store: an entry is on the table because the
   * record carries it, and a card nobody pressed Record it on is not.
   */
  const table = useMemo(
    () => ({
      counts: countsIn(reading?.source ?? ""),
      entries: entriesIn(reading?.source ?? ""),
      revision: reading?.revision ?? "",
    }),
    [reading],
  );

  const value = useMemo(
    () => ({
      store, busy: running, draft, setDraft, send, stop, sending,
      attachments, detach, chosen, ask, clearChosen, passage, askPassage,
      sheetDraft, askAbout, handOver, noteDraft, dropDraft, offerFields, noteSheet,
      offered, use, undo, dismiss, reopen, show, showing, noteField, revealed,
      writing, noteWriting, fillComposer,
      capture, moved, refresh, suggest, offerInsert,
      wanted, returnFocus,
      sitting, startSitting: begin, endSitting: end, sittingRefusal, sittingBusy,
      deposits, editDeposit, editClause, recordDeposit, dismissDeposit, reopenDeposit, table,
    }),
    [store, running, draft, send, stop, sending, attachments, detach, chosen, ask,
      clearChosen, passage, askPassage, sheetDraft, askAbout, handOver, noteDraft,
      dropDraft, offerFields, noteSheet,
      offered, use, undo, dismiss, reopen, show, showing, noteField, revealed,
      writing, noteWriting, fillComposer,
      capture, moved, refresh, suggest, offerInsert, wanted, returnFocus,
      sitting, begin, end, sittingRefusal, sittingBusy,
      deposits, editDeposit, editClause, recordDeposit, dismissDeposit, reopenDeposit, table],
  );

  return <PartnerContext.Provider value={value}>{children}</PartnerContext.Provider>;
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/**
 * The date an entry is stamped with, as a record writes one: the day, in this
 * browser's own time zone.
 *
 * A record's section is read by a human, and a human reads a date. The instant
 * is not kept: the transcript already holds the turn the deposit arrived in, to
 * the second, and a record's entry is not a second copy of that.
 */
function stampOf(now: Date): string {
  const two = (value: number) => String(value).padStart(2, "0");
  return `${String(now.getFullYear())}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
}

function installOf(error: unknown): string {
  return error instanceof PartnerError ? error.install : "";
}
