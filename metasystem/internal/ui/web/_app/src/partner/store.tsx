import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation } from "react-router";

import {
  askWalk,
  closeSitting,
  endSitting,
  isBusy,
  keepRoom,
  loadPartner,
  PartnerError,
  sendTurn,
  startSitting,
  stopTurn,
  type CapturedSticky,
  type Deposit,
  type Snapshot,
  type Message,
  type Page,
  type ProposalState,
  type Sitting,
  type Subject,
} from "./api";
import { lookOnce, sendProposal, writeOutcome } from "./applying";
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
  retireOnOpeningClosed,
  retireOnSent,
  subjectIn,
  type Attachment,
} from "./attachments";
import { captureOf, readingMoved } from "./capture";
import { chipped, insertAt } from "./composing";
import { type SheetDraft } from "./drafting";
import {
  answered,
  cardIn,
  folded,
  holding,
  NOWHERE,
  reach,
  offeredIn,
  saving,
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
import { movesTheTable, recorder, type Reading, type Recorder } from "./recording";
import {
  cardIn as depositIn,
  cardsIn as depositsIn,
  clauseFrom,
  countsIn,
  editable,
  ENDED_WITHOUT,
  pressable,
  entriesIn,
  entryOf,
  marked,
  NOT_READ_YET,
  OUTCOME,
  recordedIn,
  standingOf,
  type Card as DepositCard,
  type Counts,
  type Entry,
  type Marks as DepositMarks,
  type Records,
} from "./sitting";
import {
  asked,
  busy,
  emptyStore,
  loaded,
  nameOf,
  proposalMoved,
  received,
  refused,
  retrying,
  unavailable,
  type Store,
} from "./conversation";
import { proposedFor, showsAt, type Proposed } from "./proposed";
import {
  askLine as askAboutLine,
  askReread,
  barLine,
  cardIn as proposalCardIn,
  cardsIn as proposalCardsIn,
  coverChanged,
  dismissable as dismissableLine,
  displayedFor,
  lineID as proposalLineID,
  markOf as proposalMarkOf,
  newestWaitingCard,
  noRun,
  nothingDeferred,
  releaseRun,
  runProposals,
  sendable as sendableIn,
  takeRun,
  waiting as waitingLine,
  waitingAcross,
  type Card as ProposalCard,
  type Displayeds,
  type Line as ProposalLine,
  type Marks as ProposalMarks,
  type Written,
} from "./proposing";
import { loadBacklog } from "../backlog/api";
import type { Chosen } from "./subject";
import { onPartnerEvent, onStreamOpen } from "../notifications/stream";
import { useAboutLine, useSubject } from "../shell/about";
import { useSession } from "../shell/identity";
import { editDocument, isStale, loadDocument } from "../project/api";
import { captured } from "../stickies/stickies";
import { useStickies } from "../stickies/store";
import { roomIdFromPath } from "../routes";
import {
  answerLine,
  EMPTY_DESK,
  localDeposit,
  outcomeShape,
  retipped,
  keepAfterSilence,
  keepDue,
  onDesk,
  roomOf,
  withDraft,
  withoutDraft,
  type AnswerKind,
  type Desk,
  type DeskItem,
  type Draft,
  type Drafts,
  type Face,
  type RoomState,
} from "../review/room";

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
  /** Stop the running answer; answers a refusal in words, or "" once it has settled. */
  stop: () => Promise<string>;
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
  /**
   * Stop offering this opening's draft, which is what its unmount does.
   *
   * By the opening and not by the sheet's name: two edit sheets can stand open
   * over two goals, and the first one to close must leave the other one's draft
   * exactly where it is (Astra F2 on g1-s56).
   */
  dropDraft: (opening: string) => void;
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
  /**
   * Use this and save, as one press: the words go into the field and the sheet
   * sends what it would then hold, and the card says which of the two happened.
   *
   * It does nothing where the opening has gone, where the card was already used,
   * or where the sheet offers no submission path — which is every sheet but the
   * one that has one, so every other proposal is offered Use this alone.
   */
  useAndSave: (id: string) => Promise<void>;
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
   * The drawer was closed, so nothing is being shown at any more.
   *
   * The target outlives the column that read it: the panel is mounted only
   * while the drawer is open, so a chip's press writes the card and the next
   * mount reads it — and without this, EVERY later mount read it. A human who
   * pressed a chip in the morning and opened the drawer with its own toggle in
   * the afternoon was taken back to that morning's answer, which is not where
   * they asked to be and not where a conversation opens (g1-s61, as built).
   *
   * It is the shell's call and not the drawer's own, because the shell is what
   * knows the drawer has closed: the header's toggle, the panel's, and Escape
   * are three presses and one place that answers them.
   */
  clearShowing: () => void;
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
   * Which conversation is on screen (g1-s65 D16, g1-s67 D1): the room's record
   * where the address is a room, or "" for the human's own, which is the one the
   * drawer shows. Every request and every beat is about this one.
   */
  conversation: string;
  /**
   * The review room's working state (g1-s65 D9): the desk's strip and the item
   * up, the face of the desk pane, and every unfinished card's words. It is kept
   * on the sitting's mark, at most once a second while a human types and always
   * when they leave the field, step out or leave the page.
   */
  room: RoomState;
  /** Put one thing on the desk, which is what an anchor, the strip and a walk do. */
  putOnDesk: (item: DeskItem) => void;
  /** Lay a whole desk down: a shaping room's first, its record's sections (g1-s67 D2). */
  openDesk: (desk: Desk) => void;
  /** Bring one item of the strip back up. */
  showOnDesk: (at: number) => void;
  /** Flip the desk pane to the board or back. */
  setFace: (face: Face) => void;
  /** Keep the room now, whatever the second says: leaving a field, stepping out, leaving the page. */
  /** Keep the room on its mark; answers whether the keep landed. */
  keepRoomNow: (leaving?: boolean) => Promise<boolean>;
  /** One of the room's walks, asked by the interface in the human's name (D6, g1-s67 D3). */
  walk: (part: string) => Promise<void>;
  /** The turn whose presenting the human stopped, or "". */
  stoppedPresenting: string;
  /** Stop the running walk putting things on the desk, for this turn. */
  stopPresenting: () => void;
  /**
   * A card the human makes from a selection on the desk, with its anchor filled
   * and the words left empty: a review's finding (D7), a shaping sitting's fact
   * (g1-s67 D4).
   */
  startCard: (anchor: string, kind: string) => void;
  /**
   * Answer one recorded finding: its Answer line rewritten by its mark through
   * the recorder (D8). It answers "" when the record took it, and the refusal
   * in words when it did not — a follow-up with no goal, an acceptance with no
   * reason, a record that moved.
   */
  answerFinding: (mark: string, answer: AnswerKind, detail: string) => Promise<string>;
  /** The reason an open Accept sheet holds, by card, or none where it is closed. */
  accepting: Readonly<Record<string, string>>;
  /** Open, fill or close (null) one finding's Accept sheet. */
  noteAccepting: (id: string, reason: string | null) => void;
  /** The unfinished words the room keeps, by card. */
  drafts: Drafts;
  /**
   * Review the new tip: the record's Reviewed line moves to the branch now and
   * the old tip is kept as Previously (D9). It answers "" or the refusal.
   */
  reviewNewTip: (current: string) => Promise<string>;

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
  startSitting: (asked: { purpose: string; subject?: Subject; title?: string }) => Promise<string>;
  /**
   * Ask the Partner to draft what this sitting came to (g1-s55 D2). The sitting
   * stands: the card it offers is admitted against this record, and the record
   * takes it when the human presses Record it.
   */
  closeSitting: (verdict?: string) => Promise<void>;
  /** End the sitting. What was recorded stays in the record. */
  endSitting: () => void;
  /**
   * End it without recording an outcome, which is allowed and said: the line
   * below is what it leaves behind, and it is shown where the press was made.
   */
  endWithoutRecording: () => Promise<void>;
  /** What the last end-without-recording left behind, or "". */
  sittingEnded: string;
  /** What a Start, Close or End press was refused with, in the server's words. */
  sittingRefusal: string;
  /** True while a Start, Close or End press is in flight. */
  sittingBusy: boolean;

  /**
   * Every deposit this conversation carries, oldest first, each with where it
   * stands against the sitting now standing and the record as it now reads. The
   * card on the transcript is rendered from this one list.
   */
  deposits: readonly DepositCard[];
  /** The words of one card, while they are still the human's to change. */
  editDeposit: (id: string, text: string) => void;
  /** The clause of one card: its anchor, its reason or its consequence. */
  editClause: (id: string, clause: string) => void;
  /**
   * Record it. It composes the entry from the sitting's current reading of the
   * record, writes the whole source under that reading's revision, and refreshes
   * the reading from what the write answered. Presses are serialized: two in a
   * row land both entries, and two at once land both in the order they were
   * pressed.
   *
   * It writes only into the record the card was offered against, and only while
   * that is the record this sitting is on: a card left standing by a sitting that
   * has ended is not a way into the next sitting's record.
   *
   * `records` says what the press is writing where that is not the card's own
   * words: a case card's Decide records a decision and its Leave open records an
   * open question (g1-s55 D1). The entry still carries the card's own mark, so
   * the record says which deposit it came from and the card is a once-only press.
   * An outcome recorded ends the sitting, and nothing before it does.
   */
  recordDeposit: (id: string, records?: Records) => void;
  /** Dismiss: the card folds to one line. */
  dismissDeposit: (id: string) => void;
  /** The folded line, pressed: the card is back. */
  reopenDeposit: (id: string) => void;
  /**
   * Every card the conversation carries, oldest first, each with its lines and
   * where each one stands. The card in the transcript is rendered from this one
   * list, and so is the count on the closed bar.
   */
  proposals: readonly ProposalCard[];
  /** Tick or untick one line, which is how the human chooses which to apply. */
  tickProposal: (id: string, ticked: boolean) => void;
  /** Select all: every waiting line of this card that can be sent. */
  selectProposals: (card: string) => void;
  /**
   * Apply: run the ticked lines of this card in order, one act each, never
   * retried. A refusal is passed and the run goes on; an answer that does not say
   * what happened stops it, and the rest say "not run".
   */
  applyProposals: (card: string) => void;
  /** Continue with the rest: run from the first line a stopped run never reached. */
  continueProposals: (card: string) => void;
  /** Try again: send this one line again, as the human's press. */
  tryProposal: (id: string) => void;
  /** Dismiss: every waiting line is recorded dismissed and the card folds. */
  dismissProposals: (card: string) => void;
  /** The folded line, pressed: the card is open again. */
  reopenProposals: (card: string) => void;
  /** Ask the Partner: the line's words go in the composer, and nothing is sent. */
  askAboutProposal: (id: string) => void;
  /**
   * What this page holds about each proposed line beyond what the server
   * persists, by the line's own id.
   *
   * It is read outside the drawer for one reason: an act whose answer the
   * conversation could not write down is known HERE and nowhere in the record.
   * The entry still says `applying`, so a surface reading the record alone would
   * offer to send it again — and the inbox is such a surface (Astra C-02). The
   * store stands above the pages, so there is one answer for one line.
   */
  proposalMarks: ProposalMarks;
  /**
   * Forget what this page was holding about one line, because something newer
   * has said what happened to it.
   *
   * The mark above is ONE mark, read by the drawer and by the inbox, so the
   * surface that recorded or reconciled that newer outcome clears it here rather
   * than only in its own state: an inbox that cleared its own copy went on
   * importing the drawer's obsolete refusal over an entry the record had applied
   * (Astra C-04).
   */
  clearProposalMark: (id: string) => void;
  /** The card whose run is in flight, or "". A second Apply is refused while it is. */
  runningProposals: string;
  /** How many actions are waiting for the human, across every answer. */
  proposalsWaiting: number;
  /** What the closed bar says beside the composer, or "". */
  proposalsLine: string;
  /** Open the drawer at the newest card with a waiting line, which the bar does. */
  showProposals: () => void;
  /**
   * Open the drawer at the newest card carrying a line that still waits on this
   * human about one goal, which is what a chip on that goal's row does. On the
   * focused page, which has no drawer, the transcript comes to that card.
   */
  showProposedFor: (goal: string) => void;
  /**
   * The section's own offered re-read, and whether a sheet covers the work area,
   * told to the store from inside the shell.
   *
   * The store stands above the refresh and the work area — it has to, because the
   * conversation is read before either exists — so it cannot ask for them. This
   * is the same registration the composer's cursor and an editor's fields use:
   * something inside the tree says how, for as long as it is there.
   */
  offerReread: (reread: (() => void) | null) => void;
  /** A sheet says the work area is covered, or is not. */
  noteCovered: (covered: boolean) => void;

  /**
   * The record's four sections as the sitting's current reading holds them: what
   * the table shows and what the drawer's four counts count. They are the record
   * and not a second store, so a card nobody recorded is not among them.
   */
  table: {
    counts: Counts;
    entries: readonly Entry[];
    revision: string;
    /**
     * The record's whole source as the reading holds it. It is here because the
     * table is the one place that has to read something of the record that is
     * not an entry: the ledger goals its head names, which are the scope a
     * question asked out of this sitting carries (g1-s55 F1).
     */
    source: string;
  };
};

const nothing: Partner = {
  store: emptyStore,
  busy: false,
  draft: "",
  setDraft: () => {},
  send: () => {},
  stop: async () => "",
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
  useAndSave: async () => {},
  undo: () => {},
  dismiss: () => {},
  reopen: () => {},
  show: () => {},
  showing: "",
  clearShowing: () => {},
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
  conversation: "",
  room: { desk: EMPTY_DESK, face: "desk", drafts: {} },
  putOnDesk: () => {},
  openDesk: () => {},
  showOnDesk: () => {},
  setFace: () => {},
  keepRoomNow: async () => true,
  walk: async () => {},
  stoppedPresenting: "",
  stopPresenting: () => {},
  startCard: () => {},
  answerFinding: async () => "",
  accepting: {},
  noteAccepting: () => {},
  drafts: {},
  reviewNewTip: async () => "",
  sitting: null,
  startSitting: async () => "",
  closeSitting: async () => {},
  endSitting: () => {},
  endWithoutRecording: async () => {},
  sittingEnded: "",
  sittingRefusal: "",
  sittingBusy: false,
  deposits: [],
  editDeposit: () => {},
  editClause: () => {},
  recordDeposit: () => {},
  dismissDeposit: () => {},
  reopenDeposit: () => {},
  proposals: [],
  tickProposal: () => {},
  selectProposals: () => {},
  applyProposals: () => {},
  continueProposals: () => {},
  tryProposal: () => {},
  dismissProposals: () => {},
  reopenProposals: () => {},
  askAboutProposal: () => {},
  proposalMarks: {},
  clearProposalMark: () => {},
  runningProposals: "",
  proposalsWaiting: 0,
  proposalsLine: "",
  showProposals: () => {},
  showProposedFor: () => {},
  offerReread: () => {},
  noteCovered: () => {},
  table: {
    counts: { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0, Findings: 0 },
    entries: [],
    revision: "",
    source: "",
  },
};

const PartnerContext = createContext<Partner>(nothing);

export function usePartner(): Partner {
  return useContext(PartnerContext);
}

/**
 * The store with these readings and no others.
 *
 * It is the seam a render needs to show one piece of the interface over a state
 * no static render could reach: what the closed bar does with a draft handed
 * over is a fact about the bar, and the hand-over that puts it there is an
 * effect. Everything not named here is the store as it stands with no provider
 * at all — every reading empty, every act nothing — so a reading added to the
 * store later does not have to be spelled out again wherever this is used.
 */
export function PartnerAs({ held, children }: { held: Partial<Partner>; children: ReactNode }) {
  return <PartnerContext.Provider value={{ ...nothing, ...held }}>{children}</PartnerContext.Provider>;
}

/**
 * Every act the Partner proposed on one goal that still waits on this human,
 * newest answer first — for a row that is not in the conversation at all.
 *
 * It is a hook rather than a field of the context because it is asked per goal,
 * and a board of forty cards asks it forty times: each one reads the cards the
 * store already composed and picks its own lines out of them. Nothing is
 * fetched, and a page outside the provider reads an empty conversation and shows
 * nothing (g1-s61 D1).
 */
export function useProposedFor(goal: string): readonly Proposed[] {
  const { proposals } = usePartner();
  return useMemo(() => proposedFor(proposals, goal), [proposals, goal]);
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
  // The room's working state (g1-s65 D9): the desk and its strip, the face of
  // the desk pane, the findings the human made themselves, and the reasons of
  // open Accept sheets. Taken from the mark once per visit, kept on it after.
  const [desk, setDesk] = useState(EMPTY_DESK);
  const [face, setFaceState] = useState<Face>("desk");
  const [locals, setLocals] = useState<Readonly<Record<string, Draft>>>({});
  const [accepting, setAccepting] = useState<Readonly<Record<string, string>>>({});
  const [stoppedPresenting, setStoppedPresenting] = useState("");
  const roomTaken = useRef("");
  const lastKept = useRef("");
  const keptAt = useRef(0);
  // The sitting's one current reading of its record: source and revision, taken
  // when the sitting starts and replaced by the answer of every successful
  // write. The table reads it, and every Record press composes from it.
  const [reading, setReading] = useState<Reading | null>(null);
  // What the human has done with each proposed line: whether it is ticked,
  // whether a stopped run ever reached it, and why it will not be sent. The
  // server owns the state a line is IN; this is what the page owns about it.
  const [proposalMarks, setProposalMarks] = useState<ProposalMarks>({});
  // The budget each approve line displays, taken once when the answer ends and
  // kept with the line: what the human read is what the run sends.
  const [displayed, setDisplayed] = useState<Displayeds>({});
  // The cards the human folded away by pressing Dismiss. It is the page's own
  // state: what the record says is that every waiting line was dismissed, and
  // whether the card is folded or open again is how they are reading it.
  const [dismissedCards, setDismissedCards] = useState<readonly string[]>([]);
  // And the cards they opened again by pressing the folded line. It is the same
  // kind of state for the same reason, and it outranks the automatic fold for the
  // card it names: without it a card nobody dismissed was folded straight back by
  // that rule and could not be read at all (Astra C-03).
  const [expandedCards, setExpandedCards] = useState<readonly string[]>([]);
  // The card whose run is in flight, for the buttons. The guard that actually
  // refuses a second Apply is the ref below, taken synchronously.
  const [runningProposals, setRunningProposals] = useState("");
  // Whether a sheet is covering the work area, as the shell tells this store.
  // Both, because one of them is read inside a run that is already in flight and
  // the other has to wake the effect that makes a deferred read.
  const [covered, setCovered] = useState(false);
  const coveredNow = useRef(false);
  // The section's own offered re-read, and whether one is waiting for a sheet to
  // close. The store stands above the refresh, so something inside the shell
  // registers it, exactly as the composer registers its cursor.
  const reread = useRef<(() => void) | null>(null);
  const pendingReread = useRef(nothingDeferred());
  // The run in flight, held synchronously so a second Apply in the same frame
  // does nothing: a state flag is a render away and a press is not.
  const runFor = useRef(noRun());
  // The guarded entry, for the one caller that is not a press: the sign-in
  // sheet's success, which runs on from the line that asked for it.
  const runLinesAgain = useRef<((card: string, lines: readonly ProposalLine[]) => Promise<void>) | null>(null);
  const [sittingRefusal, setSittingRefusal] = useState("");
  const [sittingBusy, setSittingBusy] = useState(false);
  // What the last end-without-recording left behind. It is kept because it is
  // the one act of this section that writes nothing and has to say so.
  const [sittingEnded, setSittingEnded] = useState("");
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
  // Which conversation is on screen (g1-s65 D16, g1-s67 D1): the room's own
  // where the address is a room, else the human's own, which is the drawer's.
  // The drawer never shows a sitting. It is read here, once, and every request
  // below names it.
  const roomRecord = roomIdFromPath(location.pathname);
  const where = roomRecord;
  const whereNow = useRef(where);
  whereNow.current = where;
  const { askToSignIn } = useSession();
  const subject = useSubject();
  const stickies = useStickies();
  const label = useAboutLine("");

  const read = useCallback((signal?: AbortSignal) => {
    const asked = whereNow.current;
    loadPartner(signal, asked)
      .then((snapshot) => {
        // An answer about a conversation this page has since left is that
        // conversation's, and not what is on screen now.
        if ((snapshot.conversation ?? "") !== whereNow.current) {
          return;
        }
        setStore((held) => loaded({ ...held, conversation: asked }, snapshot));
        takeRoom(asked, snapshot);
      })
      .catch((error: unknown) => {
        if (signal?.aborted === true) {
          return;
        }
        setStore((held) => unavailable(held, reasonOf(error)));
      });
  }, []);

  // The conversation, once when the page loads and again whenever the page
  // moves to another one: the room's, a sitting's in the drawer, or the human's
  // own. What was on screen goes first, so nothing of one conversation is shown
  // under another's name.
  useEffect(() => {
    setStore({ ...emptyStore, conversation: where });
    setDepositMarks({});
    setLocals({});
    setAccepting({});
    const aborter = new AbortController();
    read(aborter.signal);
    return () => {
      aborter.abort();
    };
  }, [read, where]);

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
    sendTurn(minted, text, taken, where)
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
  }, [draft, running, sending, capture, attachments, compose, refreshed, where]);

  const turn = store.live.turn;
  const stop = useCallback(async (): Promise<string> => {
    if (turn === "") {
      return "";
    }
    try {
      const snapshot = await stopTurn(turn, where);
      setStore((held) => loaded(held, snapshot));
      return "";
    } catch (error: unknown) {
      setStore((held) => refused(held, reasonOf(error), ""));
      return reasonOf(error);
    }
  }, [turn, where]);

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

  /**
   * One opening of a sheet has gone, so the draft it handed over goes with it.
   *
   * It is the second of the two retiring events, and it is the lifetime that
   * decides: the attachment says which opening it lives by, so nothing here
   * knows what kinds of attachment there are, and a draft from another opening
   * of a sheet of the same name stands (Astra F2 on g1-s56).
   */
  const dropDraft = useCallback((opening: string) => {
    setAttachments((held) => retireOnOpeningClosed(held, opening));
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
   *
   * It asks for the field's RAW value and compares it whole. The draft the
   * Partner is told is trimmed, and a field holding the suggestion's words with
   * the human's own spacing around them is not the suggestion's words: undoing
   * it would throw their spacing away for a difference a trimmed comparison
   * cannot see (g1-s51 Built, deferred).
   */
  const undo = useCallback((id: string) => {
    const card = cardIn(offered, id);
    const registered = reach(openings.current, card);
    if (card === undefined || registered === null || !undoable(card, [...openings.current.keys()])) {
      return;
    }
    if (registered.raw(card.field) !== card.text) {
      setMarks((held) => holding(held, offered, card.opening, card.field, ""));
      return;
    }
    registered.set(card.field, card.mark.previous ?? "");
    setMarks((held) => undone(held, id));
  }, [offered]);

  /**
   * Use this and save, as one press.
   *
   * The sheet does both halves, because only the sheet can: it builds the draft
   * it would send from the words it is putting in, and sends that value rather
   * than waiting for a render to tell it what it now holds (Astra F1 on g1-s52).
   * What is done here is the bookkeeping either way — the words are in the field
   * the instant the press lands, which is why the mark is written before the
   * sending answers, and what the sending answered is written on the card when
   * it does. Undo is not offered in between or after: the words are already on
   * their way, and this interface has no undo for an act (g1-s56 D2).
   *
   * A proposal whose sheet offers no submission path is not offered this at all,
   * so reaching here without one is nothing happening rather than a press that
   * silently uses the words without saving them.
   */
  const useAndSave = useCallback(async (id: string) => {
    const card = cardIn(offered, id);
    const registered = reach(openings.current, card);
    if (card === undefined || registered?.save === undefined) {
      return;
    }
    if (!usable(card, [...openings.current.keys()])) {
      return;
    }
    const previous = registered.raw(card.field);
    setMarks((held) => saving(held, id, previous));
    const outcome = await registered.save(card.field, card.text);
    setMarks((held) => answered(held, id, outcome));
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

  // The drawer closed, so the card it was opened at is spent. The asking count
  // is left alone: it counts presses and never a state, and a close is not one
  // of them.
  const clearShowing = useCallback(() => {
    setShowing("");
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
      // And nothing else. Retiring the draft is the OPENING's event, not this
      // one: two sheets of one name are the same answer to "where is the human"
      // and two different drafts, so a name closing here would take the draft
      // of the one still standing (Astra F2 on g1-s56). It is done where the
      // opening is known, which is the sheet's own unmount in AskSheet.
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
    // The old sitting's recorder and reading go before the new one's is asked
    // for, not when it arrives: a table that went on showing A's entries under
    // B's chip would be showing a human a record they are not sitting on, and a
    // press composed from A's reading is exactly what must not reach B.
    recording.current = null;
    setReading(null);
    if (subjectID === "") {
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
      setSittingEnded("");
      try {
        const answered = await startSitting({ ...asked, about: capture });
        const opened = answered.conversation ?? "";
        // A sitting is a conversation of its own (g1-s65 D16), and every
        // sitting opens in its room, which its caller goes to (g1-s67 D1).
        if (opened === whereNow.current) {
          setStore((held) => loaded(held, answered));
        }
        return opened;
      } catch (error: unknown) {
        setSittingRefusal(reasonOf(error));
        throw error;
      } finally {
        setSittingBusy(false);
      }
    },
    [capture],
  );

  /**
   * End the sitting: the first half of it, which is the Partner drafting what the
   * sitting came to (g1-s55 D2).
   *
   * The sitting is still standing when this returns, deliberately: the closing
   * turn's own deposit is admitted against this record, and the mark comes off
   * when the outcome is recorded — or when the human says they are leaving
   * without it.
   */
  const close = useCallback(async (verdict = "") => {
    setSittingBusy(true);
    setSittingRefusal("");
    setSittingEnded("");
    try {
      const answered = await closeSitting(capture, where, verdict);
      setStore((held) => loaded(held, answered));
    } catch (error: unknown) {
      setSittingRefusal(reasonOf(error));
      throw error;
    } finally {
      setSittingBusy(false);
    }
  }, [capture, where]);

  const end = useCallback(async (): Promise<void> => {
    setSittingBusy(true);
    setSittingRefusal("");
    try {
      const answered = await endSitting(where);
      setStore((held) => loaded(held, answered));
    } catch (error: unknown) {
      setSittingRefusal(reasonOf(error));
      throw error;
    } finally {
      setSittingBusy(false);
    }
  }, [where]);

  const endWithout = useCallback(async () => {
    await end();
    setSittingEnded(ENDED_WITHOUT);
  }, [end]);

  /**
   * Every deposit the conversation carries, with where each one stands: the
   * answers already written down, then the answer arriving now. It is composed
   * here, from the transcript and the marks, so the card on the transcript and
   * the table cannot disagree about one deposit.
   */
  const deposits = useMemo(() => {
    const recorded = recordedIn(reading?.source ?? "");
    const offered = depositsIn(
      [
        ...store.messages
          .filter((message) => (message.deposits ?? []).length > 0)
          .map((message) => ({ turn: message.turn, deposits: message.deposits ?? [] })),
        { turn: store.live.turn, deposits: store.live.deposits },
      ],
      depositMarks,
      subjectID,
      recorded,
    );
    // The cards the human made from a selection on the desk: a review's
    // findings (g1-s65 D7) and a shaping sitting's facts (g1-s67 D4). They are
    // cards like any other — Record it, Dismiss, a finding's four answers —
    // and until one is recorded it lives in the room's drafts and nowhere else.
    const mine = Object.entries(locals).map(([id, local]) => {
      const deposit: Deposit = localDeposit(local, sitting?.subject);
      const entry = recorded.get(id);
      const held = depositMarks[id] ?? { ...marked(deposit), text: local.text, clause: local.clause };
      const mark = entry === undefined
        ? held
        : { ...held, text: entry.text, clause: entry.clause, recording: false, recorded: entry.section, refusal: "" };
      return { ...deposit, id, mark, standing: standingOf(deposit, mark, subjectID) };
    });
    return [...offered, ...mine];
  }, [store.messages, store.live.turn, store.live.deposits, depositMarks, subjectID, reading, locals, sitting]);

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

  /**
   * The words of one card, where they are still the human's to change.
   *
   * A card whose press is in flight is frozen: the entry the press composed is
   * the entry the record takes, and a field that went on accepting keystrokes
   * through the write let a human watch the card say "recorded" over words it
   * never carried (Sol's fifth finding). The field itself is read-only while the
   * press runs; this is the same rule where the state is kept, so nothing else
   * can write past it.
   */
  const changeWords = useCallback(
    (id: string, change: (mark: DepositMarks[string]) => DepositMarks[string]) => {
      const card = depositIn(deposits, id);
      if (card === undefined || !editable(card.standing)) {
        return;
      }
      changeMark(id, change);
    },
    [deposits, changeMark],
  );

  const editDeposit = useCallback(
    (id: string, text: string) => {
      changeWords(id, (mark) => ({ ...mark, text, refusal: "" }));
    },
    [changeWords],
  );

  const editClause = useCallback(
    (id: string, clause: string) => {
      changeWords(id, (mark) => ({ ...mark, clause, refusal: "" }));
    },
    [changeWords],
  );

  const dismissDeposit = useCallback(
    (id: string) => {
      // A finding the human made and dismissed was never anybody's but theirs,
      // so it goes whole; an offered card folds to its line as it always did.
      if (id.startsWith(LOCAL)) {
        setLocals((held) => {
          const next = { ...held };
          delete next[id];
          return next;
        });
        return;
      }
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
   * The card that is pressed must be admitted, needing nothing, not already in
   * flight, not already in the record, and offered against the record this
   * sitting is on — waiting, or left in conflict by a record that moved. That last one is Sol's first finding: the press used to
   * ask only whether the card was offered, so a card left on the transcript by a
   * sitting that had ended wrote its words into whatever record the next sitting
   * was about. The recorder is asked for the same record by name, so the gate
   * holds even if a press gets past the card.
   *
   * Everything else is the recorder's: the entry is composed from the reading as
   * it stands when the press runs, the write goes under that reading's revision,
   * and the reading is refreshed from what the write answered — unless the
   * sitting has moved on since, in which case that answer is about another
   * record and no table on screen is its.
   *
   * A conflict keeps the card exactly as the human has it and says so; pressing
   * again is one more press, now against the record the reread brought back.
   */
  const recordDeposit = useCallback(
    (id: string, records?: Records) => {
      const card = depositIn(deposits, id);
      // A card in conflict is pressed again, without an edit: the record moved,
      // nothing was written, the human's words are still here and the reading has
      // been taken again, so this press is one more press against what the record
      // now says.
      if (card === undefined || !pressable(card.standing)) {
        return;
      }
      const into = card.subject?.id ?? "";
      const held = recording.current;
      // No reading of this record yet: a sitting's opening turn can be answered
      // before this page has read the record it is on. Nothing is composed from
      // no reading, and the card says so rather than doing nothing.
      if (held === null || held.reading().id !== into) {
        changeMark(id, (mark) => ({ ...mark, refusal: NOT_READ_YET }));
        return;
      }
      changeMark(id, (mark) => ({ ...mark, recording: true, refusal: "" }));
      const entry = entryOf(card, nameOf(store.human), stampOf(new Date()), records);
      // A review's Outcome opens with the verdict the human chose, and names
      // what they examined, so a nod cannot pass as a review (g1-s65 D10). The
      // verdict is the card's own, stamped by the server on the closing deposit,
      // so a card rebuilt after a reload records under the same shape (Sol
      // SOL-A-02, SOL-A-07). It is composed inside the recorder's queue, over
      // the record as it then reads, so a Clear meets every finding recorded
      // since the sheet opened.
      const shape = outcomeShape(card, sitting?.purpose, desk.items);
      void held.press(entry, records?.kind ?? card.kind, into, shape).then((outcome) => {
        if (movesTheTable(outcome, recording.current?.reading() ?? null)) {
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
        // The outcome recorded is where the sitting ends, and only after the
        // record has taken it (g1-s55 D2). A press that was refused or that met
        // a conflict leaves the sitting exactly where it was, because the human
        // has something left to do in it.
        if (outcome.kind === "recorded" && outcome.section === OUTCOME) {
          void end();
        }
      });
    },
    [deposits, changeMark, store.human, end, sitting, desk],
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
      source: reading?.source ?? "",
    }),
    [reading],
  );

  /* --------------------------------------------------------------- the room -- */

  /**
   * The room as the mark carried it back, taken once per visit to a
   * conversation: a reread on reconnect answers what this page last kept, and
   * taking it again would put back words typed since.
   */
  const takeRoom = useCallback((asked: string, snapshot: Snapshot) => {
    if (roomTaken.current === asked) {
      return;
    }
    roomTaken.current = asked;
    const kept = roomOf(snapshot.sitting?.room);
    setDesk(kept.desk);
    setFaceState(kept.face);
    const mine: Record<string, Draft> = {};
    const marks: Record<string, DepositMarks[string]> = {};
    const reasons: Record<string, string> = {};
    for (const [id, draft] of Object.entries(kept.drafts)) {
      if (id.startsWith(LOCAL)) {
        mine[id] = draft;
      } else {
        marks[id] = { text: draft.text, clause: draft.clause, recording: false, recorded: "", refusal: "", dismissed: false };
      }
      if (draft.sheet === "accept") {
        reasons[id] = draft.reason ?? "";
      }
    }
    setLocals(mine);
    setDepositMarks((held) => ({ ...marks, ...held }));
    setAccepting(reasons);
    lastKept.current = JSON.stringify(kept);
  }, []);

  const putOnDesk = useCallback((item: DeskItem) => {
    setDesk((held) => onDesk(held, item));
  }, []);

  const openDesk = useCallback((laid: Desk) => {
    setDesk(laid);
  }, []);

  const showOnDesk = useCallback((at: number) => {
    setDesk((held) => (at >= 0 && at < held.items.length ? onDesk(held, held.items[at]) : held));
  }, []);

  const setFace = useCallback((face: Face) => {
    setFaceState(face);
  }, []);

  const noteAccepting = useCallback((id: string, reason: string | null) => {
    setAccepting((held) => {
      if (reason === null) {
        const next = { ...held };
        delete next[id];
        return next;
      }
      return { ...held, [id]: reason };
    });
  }, []);

  /**
   * Every unfinished card's words (Astra S65-03): each card the human has not
   * recorded or dismissed whose words are not the Partner's any more, every
   * finding they made themselves, and an open Accept sheet's reason. A recorded
   * or dismissed card is not here, which is what clears its draft.
   */
  const drafts = useMemo(() => {
    let held: Drafts = {};
    for (const card of deposits) {
      if (!editable(card.standing)) {
        continue;
      }
      const mine = card.id.startsWith(LOCAL);
      const changed = card.mark.text !== card.text || card.mark.clause !== clauseFrom(card);
      const reason = accepting[card.id];
      if (!mine && !changed && reason === undefined) {
        continue;
      }
      const draft: Draft = { text: card.mark.text, clause: card.mark.clause };
      if (mine) {
        draft.kind = card.kind;
        draft.consequence = card.consequence ?? "";
      }
      if (reason !== undefined) {
        draft.sheet = "accept";
        draft.reason = reason;
      }
      held = withDraft(held, card.id, draft);
    }
    // A finding whose card has been recorded keeps an open Accept sheet's words.
    for (const [id, reason] of Object.entries(accepting)) {
      if (!(id in held)) {
        held = withDraft(held, id, { text: "", clause: "", sheet: "accept", reason });
      }
    }
    return held;
  }, [deposits, accepting]);

  const room = useMemo<RoomState>(() => ({ desk, face, drafts }), [desk, face, drafts]);

  /**
   * Keep the room on its mark (g1-s65 D9). While a human types it is kept at
   * most once a second, and a second after the last keystroke the words typed
   * last are kept (the human's grant of 2026-09-28); the leaving keeps them as
   * well — the field's blur, Step out, and the page going away — so the
   * keystroke before leaving is never the one that is lost.
   */
  const keepRoomNow = useCallback(async (leaving = false): Promise<boolean> => {
    const record = whereNow.current;
    const kept = JSON.stringify(roomNow.current);
    if (record === "" || roomRecord === "" || sittingNow.current === null) {
      return true;
    }
    // These very words are being kept, or were: the answer is that keep's.
    if (kept === lastKept.current) {
      return keeping.current;
    }
    lastKept.current = kept;
    keptAt.current = Date.now();
    const landed = keepRoom(record, JSON.parse(kept) as RoomState, leaving).then(
      () => true,
      () => {
        // The next change keeps it again, and Step out stays in the room and
        // says so: a keep that could not be made is words still on the screen.
        if (lastKept.current === kept) {
          lastKept.current = "";
        }
        return false;
      },
    );
    keeping.current = landed;
    return landed;
  }, [roomRecord]);

  const keeping = useRef<Promise<boolean>>(Promise.resolve(true));
  const roomNow = useRef<RoomState>(room);
  roomNow.current = room;
  const sittingNow = useRef<Sitting | null>(null);
  sittingNow.current = store.sitting;
  useEffect(() => {
    if (roomRecord === "" || roomTaken.current !== roomRecord) {
      return;
    }
    if (keepDue(keptAt.current, Date.now())) {
      void keepRoomNow();
    }
    return keepAfterSilence(() => {
      void keepRoomNow();
    });
  }, [room, roomRecord, keepRoomNow]);

  // The page going away keeps the room, asked of the browser to finish after
  // the page has gone.
  useEffect(() => {
    if (roomRecord === "") {
      return;
    }
    const leaving = () => {
      void keepRoomNow(true);
    };
    globalThis.addEventListener("pagehide", leaving);
    return () => {
      globalThis.removeEventListener("pagehide", leaving);
    };
  }, [roomRecord, keepRoomNow]);

  const walk = useCallback(async (part: string) => {
    setSittingRefusal("");
    try {
      const answered = await askWalk(whereNow.current, part, capture);
      setStore((held) => loaded(held, answered));
    } catch (error: unknown) {
      setStore((held) => refused(held, reasonOf(error), ""));
    }
  }, [capture]);

  const stopPresenting = useCallback(() => {
    setStoppedPresenting(store.live.turn);
  }, [store.live.turn]);

  const startCard = useCallback((anchor: string, kind: string) => {
    const id = `${LOCAL}${mintLocal()}`;
    setLocals((held) => ({ ...held, [id]: { text: "", clause: anchor, kind } }));
  }, []);

  const reviewNewTip = useCallback(async (current: string): Promise<string> => {
    const held = recording.current;
    const into = sittingNow.current?.subject.id ?? "";
    if (held === null || held.reading().id !== into) {
      return NOT_READ_YET;
    }
    const outcome = await held.rewrite(into, (source) => retipped(source, current),
      "This review names no branch tip to move.");
    if (movesTheTable(outcome, recording.current?.reading() ?? null)) {
      setReading(outcome.reading);
    }
    return outcome.kind === "recorded" ? "" : outcome.reason;
  }, []);

  const answerFinding = useCallback(async (mark: string, answer: AnswerKind, detail: string): Promise<string> => {
    const said = answerLine(answer, detail);
    if ("refusal" in said) {
      return said.refusal;
    }
    const held = recording.current;
    const into = sittingNow.current?.subject.id ?? "";
    if (held === null || held.reading().id !== into) {
      return NOT_READ_YET;
    }
    const outcome = await held.answer(mark, said.line, into);
    if (movesTheTable(outcome, recording.current?.reading() ?? null)) {
      setReading(outcome.reading);
    }
    if (outcome.kind === "recorded") {
      noteAccepting(mark, null);
      return "";
    }
    return outcome.reason;
  }, [noteAccepting]);

  /* ---------------------------------------------------------- the proposals -- */

  /**
   * Every card the conversation carries, with where each line stands: the answers
   * already written down, then the answer arriving now. It is composed here, from
   * the transcript and the page's own marks, so the card in the drawer and the
   * count on the closed bar cannot disagree about one action.
   */
  const proposals = useMemo(
    () =>
      proposalCardsIn(
        [
          ...store.messages
            .filter((message) => (message.proposals ?? []).length > 0)
            .map((message) => ({ turn: message.turn, proposals: message.proposals ?? [] })),
          { turn: store.live.turn, proposals: store.live.proposals },
        ],
        proposalMarks,
        displayed,
        dismissedCards,
        expandedCards,
      ),
    [store.messages, store.live.turn, store.live.proposals, proposalMarks, displayed,
      dismissedCards, expandedCards],
  );

  /**
   * The budget each approve line displays, read once when the answer ends.
   *
   * Once, and at the terminal beat, because both halves matter. The card's
   * buttons wake then — until then there is no message an outcome can be recorded
   * on — and the tuple the human reads has to be the tuple the run sends, so it is
   * taken here and kept with the line rather than computed again at the press
   * (Astra S58-08).
   *
   * A read that fails leaves every approve line with no budget, which the card
   * says: "needs its budget first: approve it alone". That is the truthful answer
   * for a page that could not read the project's law.
   */
  const settled = store.messages
    .filter((message) => (message.proposals ?? []).some((proposal) => proposal.verb === "approve-goal"))
    .map((message) => message.turn)
    .join(" ");
  useEffect(() => {
    if (settled === "") {
      return;
    }
    const aborter = new AbortController();
    loadBacklog(aborter.signal)
      .then((read) => {
        setDisplayed((held) => {
          const lines = proposalsIn(store.messages).filter((line) => held[line.id] === undefined);
          return lines.length === 0 ? held : { ...held, ...displayedFor(lines, read.rows, read.budgetDefaults) };
        });
      })
      .catch(() => {
        // No budget could be read, so no approve line has one and each says so.
        setDisplayed((held) => {
          const lines = proposalsIn(store.messages).filter((line) => held[line.id] === undefined);
          return lines.length === 0
            ? held
            : { ...held, ...Object.fromEntries(lines.map((line) => [line.id, { budget: null, source: "none" as const }])) };
        });
      });
    return () => {
      aborter.abort();
    };
    // The list of answers carrying an approve is what decides whether there is
    // anything to read for; the messages themselves change on every beat.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settled]);

  const tickProposal = useCallback((id: string, on: boolean) => {
    setProposalMarks((held) => ({ ...held, [id]: { ...proposalMarkOf(held, id), ticked: on } }));
  }, []);

  const selectProposals = useCallback((card: string) => {
    setProposalMarks((held) => {
      const found = proposalCardIn(proposals, card);
      if (found === undefined) {
        return held;
      }
      let next = held;
      for (const line of found.lines) {
        if (waitingLine(line)) {
          next = { ...next, [line.id]: { ...proposalMarkOf(next, line.id), ticked: true } };
        }
      }
      return next;
    });
  }, [proposals]);

  /**
   * The folded line, pressed: the card is open again, and stays open.
   *
   * Both writes, because a card folds for two different reasons. A dismissal is
   * taken back by dropping it from the dismissed list; the automatic fold of an
   * older answer that still has a waiting line is not a list to be dropped from
   * at all, so the expansion is recorded and the fold gives way to it. Pressing
   * the line of a card nobody had dismissed used to change nothing (Astra C-03).
   */
  const reopenProposals = useCallback((card: string) => {
    setDismissedCards((held) => held.filter((one) => one !== card));
    setExpandedCards((held) => (held.includes(card) ? held : [...held, card]));
  }, []);

  /**
   * The offered re-read, made now or when the sheet over the work area closes.
   *
   * Deferred while a sheet covers it, because the content under an open sheet is
   * what that sheet is rendered over: a read made then would unmount the sheet's
   * own columns and whatever the human had typed into them (Astra S58-03). The
   * ask is remembered and made the moment the cover clears.
   */
  const askTheReread = useCallback(() => {
    askReread(pendingReread.current, coveredNow.current, () => {
      reread.current?.();
    });
  }, []);

  useEffect(() => {
    coverChanged(pendingReread.current, covered, () => {
      reread.current?.();
    });
  }, [covered]);

  const offerReread = useCallback((ask: (() => void) | null) => {
    reread.current = ask;
  }, []);

  const noteCovered = useCallback((now: boolean) => {
    coveredNow.current = now;
    setCovered(now);
  }, []);

  const mark = useCallback((id: string, change: (held: ProposalMarks[string]) => ProposalMarks[string]) => {
    setProposalMarks((held) => ({ ...held, [id]: change(proposalMarkOf(held, id)) }));
  }, []);

  /**
   * The answer this page was holding for want of a record, dropped because
   * something newer says what happened. It is a callback of the store because
   * the mark is the store's: the inbox records outcomes on the same lines the
   * drawer does (Astra C-04).
   */
  const clearProposalMark = useCallback((id: string) => {
    mark(id, (held) => ({ ...held, unrecorded: null }));
  }, [mark]);

  /**
   * One state written onto one line, through the one route that writes them.
   *
   * It answers the entry as the server now holds it, so the next press sends the
   * version this one left. A write the server refused with the entry is handed
   * back as a conflict, and the caller must show that entry rather than send an
   * act of its own — which is why both are folded into the conversation here: an
   * entry somebody else left is the line as the route now holds it, exactly as the
   * inbox shows it, and a card that ignored it would put the line back where it
   * was (Astra C-06).
   *
   * The answer goes back to the caller, because only the press knows what a
   * failure means to it. A dismissal that could not be written did not happen.
   */
  const writeState = useCallback(
    async (line: ProposalLine, turn: string, state: ProposalState, words: string): Promise<Written> => {
      const answered = await writeOutcome(turn, line, state, words, "", whereNow.current);
      if (answered.kind !== "failed") {
        setStore((held) => proposalMoved(held, turn, answered.proposal));
      }
      return answered;
    },
    [],
  );

  /**
   * The run: the lines given, in order, one act each, never retried.
   *
   * The whole of it is here because the whole of it is one sequence, and the
   * rules it keeps are the ones a human would notice if they were wrong:
   *
   *   - one press, one run. A synchronous ref is taken before anything is read, as
   *     the edit sheet guards its own save (g1-s56 D1), so a second Apply while
   *     one is in flight does nothing.
   *   - the canonical branch is read once, fetch-first, before the first line. A
   *     fetch that failed refuses every line that depends on what the goal says,
   *     unsent, and the others run as they would (Astra S58-07).
   *   - `applying` is written BEFORE the act is sent. A write that fails sends
   *     nothing and stops the run; a write the server refuses with the entry means
   *     somebody else moved the line, and then the act is not sent either.
   *   - a refusal is passed and the next line is sent; anything that does not say
   *     what happened stops the run, and the lines after it say "not run".
   *   - the page in view reads again after each confirmed act and when the run
   *     ends, and never while a sheet of the human's is open on it.
   */
  const runLines = useCallback(
    async (card: string, lines: readonly ProposalLine[]) => {
      if (lines.length === 0 || !takeRun(runFor.current, card)) {
        return;
      }
      setRunningProposals(card);
      try {
        await runProposals(lines, {
          // The canonical branch, once, before anything is sent; the act through
          // the clients every button uses; and the outcome through the
          // conversation's own route. All three are the module two callers share,
          // because the inbox applies the same lines through the same runner
          // (g1-s60 D4).
          look: lookOnce,
          record: async (line, state, words, attempt) => {
            const answered = await writeOutcome(card, line, state, words, attempt, whereNow.current);
            if (answered.kind === "written") {
              setStore((held) => proposalMoved(held, card, answered.proposal));
            }
            return answered;
          },
          send: sendProposal,
          mark: (line, change) => {
            mark(line.id, (held) => ({ ...held, ...change }));
          },
          reconcile: (proposal) => {
            setStore((held) => proposalMoved(held, card, proposal));
          },
          reread: askTheReread,
          // The run never waits on a sheet. It has already ended here; signing
          // in runs the rest as a fresh press, through the same guarded entry,
          // and closing the sheet without signing in leaves the settled card.
          signIn: (rest) => {
            askToSignIn(() => {
              void runLinesAgain.current?.(card, rest);
            });
          },
        });
      } finally {
        releaseRun(runFor.current);
        setRunningProposals("");
      }
    },
    [mark, askTheReread, askToSignIn],
  );

  // The run, reached from the sign-in sheet's own success. It is a ref because
  // the sheet's callback outlives the render that opened it, and because the run
  // that hands it over is the run being defined.
  runLinesAgain.current = runLines;

  const applyProposals = useCallback((card: string) => {
    const found = proposalCardIn(proposals, card);
    if (found !== undefined) {
      void runLines(card, sendableIn(found));
    }
  }, [proposals, runLines]);

  const continueProposals = useCallback((card: string) => {
    const found = proposalCardIn(proposals, card);
    if (found === undefined) {
      return;
    }
    // From the first line the stopped run never reached, in the card's own order.
    const at = found.lines.findIndex((line) => line.mark.notRun);
    if (at < 0) {
      return;
    }
    void runLines(card, found.lines.slice(at).filter((line) => waitingLine(line) && line.mark.ticked));
  }, [proposals, runLines]);

  const tryProposal = useCallback((id: string) => {
    for (const card of proposals) {
      const line = card.lines.find((one) => one.id === id);
      if (line !== undefined) {
        void runLines(card.id, [line]);
        return;
      }
    }
  }, [proposals, runLines]);

  /**
   * Dismiss: every waiting line of this card is recorded dismissed, and the card
   * folds to its one line.
   *
   * The card folds whatever the writes did. A line somebody else settled in
   * another tab answers with a conflict, and the human's act here was to put the
   * card away rather than to move that line.
   */
  const dismissProposals = useCallback((card: string) => {
    const found = proposalCardIn(proposals, card);
    setDismissedCards((held) => (held.includes(card) ? held : [...held, card]));
    if (found === undefined) {
      return;
    }
    // Every line that can still be moved out of where it is, which includes a
    // line a page went away in the middle of: nothing else will settle that one,
    // and the route admits `applying` to `dismissed` for exactly this
    // (Sol S58-C-06). The version is the entry's own, so a line somebody else
    // moved first answers with a conflict and stays as they left it.
    const lines = found.lines.filter((line) => dismissableLine(line));
    void (async () => {
      const answers = await Promise.all(lines.map((line) => writeState(line, card, "dismissed", "")));
      for (const [at, answered] of answers.entries()) {
        if (answered.kind === "failed") {
          // The conversation could not write it down, so what the human pressed
          // did not happen: the line says so, and the card is open again over a
          // proposal that is still waiting (Astra C-06).
          mark(lines[at].id, (held) => ({ ...held, refusedUnsent: answered.words }));
          setDismissedCards((held) => held.filter((one) => one !== card));
        }
      }
      // And the page behind the drawer reads again, once however many lines were
      // written, so a mounted Decisions drops the row this press put away
      // (Astra C-07). It is the store's own deferred read, which waits for a
      // sheet over the work area to close.
      askTheReread();
    })();
  }, [proposals, writeState, mark, askTheReread]);

  const askAboutProposal = useCallback((id: string) => {
    for (const card of proposals) {
      const line = card.lines.find((one) => one.id === id);
      if (line !== undefined) {
        fillComposer(askAboutLine(line));
        return;
      }
    }
  }, [proposals, fillComposer]);

  const showProposals = useCallback(() => {
    const newest = newestWaitingCard(proposals);
    if (newest !== "") {
      setShowing(newest);
      setRevealed((at) => at + 1);
    }
  }, [proposals]);

  /**
   * A chip on a goal's own row, pressed: the conversation opens at the newest
   * card carrying a line that still waits about that goal.
   *
   * It is the bar's own path and not a second one — the card the drawer opens at,
   * and the count the shell watches — because there is one answer to "which card
   * is in view" and two mechanisms for it would be two answers. On the focused
   * page there is no drawer for the count to open; what the same two writes do
   * there is bring that card up in the transcript, which is the whole of what the
   * press is for (g1-s61 D3).
   */
  const showProposedFor = useCallback((goal: string) => {
    const card = showsAt(proposals, goal);
    if (card !== "") {
      setShowing(card);
      setRevealed((at) => at + 1);
    }
  }, [proposals]);

  const value = useMemo(
    () => ({
      store, busy: running, draft, setDraft, send, stop, sending,
      attachments, detach, chosen, ask, clearChosen, passage, askPassage,
      sheetDraft, askAbout, handOver, noteDraft, dropDraft, offerFields, noteSheet,
      offered, use, useAndSave, undo, dismiss, reopen, show, showing, clearShowing, noteField, revealed,
      writing, noteWriting, fillComposer,
      capture, moved, refresh, suggest, offerInsert,
      wanted, returnFocus,
      conversation: where, room, putOnDesk, openDesk, showOnDesk, setFace, keepRoomNow, walk,
      stoppedPresenting, stopPresenting, startCard, answerFinding, accepting, noteAccepting, drafts,
      reviewNewTip,
      sitting, startSitting: begin, closeSitting: close, endSitting: end,
      endWithoutRecording: endWithout, sittingEnded, sittingRefusal, sittingBusy,
      deposits, editDeposit, editClause, recordDeposit, dismissDeposit, reopenDeposit, table,
      proposals, tickProposal, selectProposals, applyProposals, continueProposals, tryProposal,
      dismissProposals, reopenProposals, askAboutProposal, runningProposals, proposalMarks,
      clearProposalMark,
      proposalsWaiting: waitingAcross(proposals), proposalsLine: barLine(proposals),
      showProposals, showProposedFor, offerReread, noteCovered,
    }),
    [store, running, draft, send, stop, sending, attachments, detach, chosen, ask,
      clearChosen, passage, askPassage, sheetDraft, askAbout, handOver, noteDraft,
      dropDraft, offerFields, noteSheet,
      offered, use, useAndSave, undo, dismiss, reopen, show, showing, clearShowing, noteField, revealed,
      writing, noteWriting, fillComposer,
      capture, moved, refresh, suggest, offerInsert, wanted, returnFocus,
      where, room, putOnDesk, openDesk, showOnDesk, setFace, keepRoomNow, walk,
      stoppedPresenting, stopPresenting, startCard, answerFinding, accepting, noteAccepting, drafts,
      reviewNewTip,
      sitting, begin, close, end, endWithout, sittingEnded, sittingRefusal, sittingBusy,
      deposits, editDeposit, editClause, recordDeposit, dismissDeposit, reopenDeposit, table,
      proposals, tickProposal, selectProposals, applyProposals, continueProposals, tryProposal,
      dismissProposals, reopenProposals, askAboutProposal, runningProposals, proposalMarks,
      clearProposalMark,
      showProposals, showProposedFor, offerReread, noteCovered],
  );

  return <PartnerContext.Provider value={value}>{children}</PartnerContext.Provider>;
}

/**
 * Every proposed line of every answer in the transcript, as the budget read needs
 * them: the lines alone, without the cards' own folding, because what is wanted is
 * which approve lines have no tuple yet.
 */
function proposalsIn(messages: readonly Message[]): ProposalLine[] {
  const lines: ProposalLine[] = [];
  for (const message of messages) {
    for (const proposal of message.proposals ?? []) {
      lines.push({
        ...proposal,
        id: proposalLineID(message.turn, proposal.index),
        mark: proposalMarkOf({}, proposalLineID(message.turn, proposal.index)),
        displayed: null,
      });
    }
  }
  return lines;
}

function reasonOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** What a finding the human made themselves is named by: never a deposit's own id. */
const LOCAL = "local-";

/** A new finding's id, unique enough for one room. */
function mintLocal(): string {
  const bytes = new Uint8Array(8);
  globalThis.crypto.getRandomValues(bytes);
  return [...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("");
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
