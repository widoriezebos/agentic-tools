import type { About, Notepad, Sticky } from "./api";
import { documentPath, goalPath } from "../routes";
import type { Subject } from "../shell/about";

/**
 * What the notepad means, apart from how it is drawn.
 *
 * The order and the counts are the server's — every act answers the whole
 * list, so there is one opinion about them and it is not this file's. What is
 * here is everything the three surfaces have to agree on: which stickies
 * belong to the thing on the screen, what a chip says and where it goes, and
 * what the panel says when the server knows nobody.
 */

/** What the panel says when the server is acting as no human at all. */
export const SIGN_IN_LINE = "Sign in so your stickies are yours. Until then these are this seat's.";

/** The empty notepad, which is what every surface reads before the first load. */
export const emptyNotepad: Notepad = { schemaVersion: 1, human: "", stickies: [], counts: { open: 0, done: 0 } };

/**
 * Whether this keystroke saves what is in a composer.
 *
 * Enter saves and Shift+Enter breaks a line, as the Partner's composer does: a
 * sticky is written in two seconds or it is not written at all, and reaching
 * for a button is the half of that which never happens. Escape is not here —
 * it belongs to the sheet, which closes on it like every other sheet.
 */
export function savesOn(key: string, shift: boolean): boolean {
  return key === "Enter" && !shift;
}

/** What the disclosure over the struck-off ones says. */
export function doneLabel(many: number): string {
  return `Done (${String(many)})`;
}

/** The open ones, in the order the server answered with: newest first. */
export function openStickies(notepad: Notepad): Sticky[] {
  return notepad.stickies.filter((sticky) => sticky.doneAt === "");
}

/** The done ones, most recently struck off first. */
export function doneStickies(notepad: Notepad): Sticky[] {
  return notepad.stickies.filter((sticky) => sticky.doneAt !== "");
}

/**
 * What the page on screen is, as a sticky says it — or null where the page is
 * about nothing a sticky can name.
 *
 * The server knows two kinds of subject, a goal and a document, and a sticky
 * knows the same two under its own names. Everywhere else — the board, the
 * Fleet page, Overview — the composer offers no chip at all, which is D3's
 * "nothing elsewhere": a chip that named a page rather than a thing would be
 * a link nobody could follow back.
 */
export function aboutThePage(subject: Subject): About | null {
  const id = subject.subject ?? "";
  if (id === "") {
    return null;
  }
  if (subject.kind === "goal") {
    return { kind: "goal", id };
  }
  if (subject.kind === "document") {
    return { kind: "record", id };
  }
  return null;
}

/** True when two abouts name the same thing. */
export function sameAbout(one: About, other: About): boolean {
  return one.kind === other.kind && one.id === other.id;
}

/** The open stickies about one thing, in the order the notepad answered with. */
export function about(notepad: Notepad, named: About | null): Sticky[] {
  if (named === null) {
    return [];
  }
  return openStickies(notepad).filter((sticky) => sticky.about.some((one) => sameAbout(one, named)));
}

/**
 * What a chip says: a goal by its id, a document by its file name.
 *
 * A document's whole path in a chip is a chip that wraps over three lines in a
 * panel four hundred pixels wide, and the path is on the page the chip leads
 * to. The title attribute is not used for it either: the link's own address is
 * what a human checks, and a tooltip on a chip in a scrolling list is noise.
 */
export function chipWords(named: About): string {
  if (named.kind === "goal") {
    return named.id;
  }
  const cut = named.id.lastIndexOf("/");
  return cut < 0 ? named.id : named.id.slice(cut + 1);
}

/** Where a chip leads: the goal's own page, or the document in the reader. */
export function chipPath(named: About): string {
  return named.kind === "goal" ? goalPath(named.id) : documentPath(named.id);
}

/**
 * What the composer opens about: what "Add a sticky" asked for, else the thing
 * on the screen, else nothing.
 *
 * It is here rather than inside the composer because the answer depends on
 * something no field can see: the panel has to be rendered where the page says
 * what it is about. Opened from the header on a goal page there is no opening
 * chip at all, so the whole of the chip is the page's subject — which is only
 * the goal's if the panel stands inside the shell's own subject provider.
 */
export function composerAbout(opening: About | null, subject: Subject): About[] {
  const offered = opening ?? aboutThePage(subject);
  return offered === null ? [] : [offered];
}

/**
 * How many stickies one capture carries.
 *
 * It is the server's own bound, written here too because this is where the
 * cutting has to happen: a capture is kept — the turn's message is written into
 * the checkout's state root with the page it was asked from — so a notepad of
 * five hundred sent whole would be five hundred private reminders stored inside
 * the one place the notepad is deliberately not. The server holds the same
 * bound and does not take this file's word for it.
 */
export const MAX_CAPTURED = 25;

/**
 * What the Partner is given about the notepad, from one capture.
 *
 * D5, with Astra's F2 folded in: while the panel is open the capture carries
 * the stickies the PANEL shows, because a note about no subject — "the Fleet
 * page feels cramped" — is on no page at all and would otherwise never reach a
 * question. Closed, it carries what the page shows: the goal page's or the
 * document's. The open count travels either way, so "you have four open
 * stickies, none about this page" is an answer the Partner can give.
 *
 * What the panel SHOWS is not the whole notepad. The struck-off ones sit behind
 * a disclosure, and a capture that carried them while they were folded away
 * would be telling the Partner things the human cannot see on their own screen
 * — which is the opposite of what this capture is for. So the disclosure's own
 * state is asked for, and done stickies travel only while it stands open.
 */
export function captured(
  notepad: Notepad,
  panelIsOpen: boolean,
  doneIsOpen: boolean,
  subject: Subject,
): { stickies: { text: string; about: string[]; done?: boolean }[]; open: number } {
  const shown = panelIsOpen
    ? [...openStickies(notepad), ...(doneIsOpen ? doneStickies(notepad) : [])]
    : about(notepad, aboutThePage(subject));
  return {
    stickies: shown.slice(0, MAX_CAPTURED).map((sticky) => {
      const said: { text: string; about: string[]; done?: boolean } = {
        text: sticky.text,
        about: sticky.about.map(saidAs),
      };
      if (sticky.doneAt !== "") {
        said.done = true;
      }
      return said;
    }),
    open: notepad.counts.open,
  };
}

/** What one about is called in a block of prose the Partner reads. */
function saidAs(named: About): string {
  return named.kind === "goal" ? `goal ${named.id}` : named.id;
}
