import type { Block, Problem } from "./api";

/**
 * Editing one document, as state.
 *
 * The component owns the text area, the fetches and the focus; everything that
 * can be said about the editor without a browser is said here, so the rules
 * that are worth arguing about — when the editor is dirty, what a refusal
 * means, what the human is told — are the rules a test reaches.
 *
 * The shape is small on purpose. This is step one of editing in place: a text
 * area over the source, a preview of it, and a save that refuses to write over
 * a file that changed underneath. There is no formatting, no history and no
 * merge; what is typed is what lands.
 *
 * Text only changes while the source is showing, so the preview can never be
 * a rendering of something other than what is in the box: switching to the
 * preview renders what is there, and switching back leaves the text alone.
 */

/** What the editor is showing: the source itself, or its rendering. */
export type Mode = "source" | "preview";

export type Editor = {
  /** The text as it stands in the box. */
  source: string;
  /** The text the file carried when this editor opened. */
  opened: string;
  /** The revision the next save is made against. */
  revision: string;
  mode: Mode;
  /** True while a save or a render is in flight; nothing else may start. */
  pending: boolean;
  /** What the preview last rendered to. */
  blocks: Block[];
  /** The one line under the bar. */
  message: string;
  /** The head problems the project refused a save over. */
  problems: Problem[];
};

/** What happened, in the words the editor answers to. */
export type Outcome =
  /** A human typed. */
  | { kind: "typed"; source: string }
  /** A human asked for the source back. */
  | { kind: "source" }
  /** A render or a save has been asked for and is in flight, saying which. */
  | { kind: "sending"; saying: string }
  /** A render came back. */
  | { kind: "rendered"; blocks: Block[] }
  /** A save or a render was refused. */
  | { kind: "refused"; status: number; said: string; problems: Problem[] };

/** What a file changed underneath is called, in the words a human can act on. */
export const CHANGED_ON_DISK =
  "This file changed on disk since you opened it. Copy your text, reload the page, and edit again.";

/** What the project refusing a record is called; the problems follow it. */
export const REFUSED_BY_THE_PROJECT = "The project refuses what this would write.";

/** The two statuses that have words of their own. */
const CONFLICT = 409;
const UNPROCESSABLE = 422;

/** An editor over one document, as the read route answered it. */
export function opening(source: string, revision: string): Editor {
  return {
    source,
    opened: source,
    revision,
    mode: "source",
    pending: false,
    blocks: [],
    message: "",
    problems: [],
  };
}

/** True when the box holds something the file does not. */
export function dirty(editor: Editor): boolean {
  return editor.source !== editor.opened;
}

/**
 * The editor after one outcome.
 *
 * A save that lands is not among them: the page goes back to reading with the
 * document the server answered with, and the next edit of it opens over those
 * bytes at that revision. So this editor's floor never moves while it is open,
 * and dirty means what it said when it opened.
 *
 * Nothing starts while something is in flight: a second save over the same
 * revision would be refused by the server anyway, and a second render would
 * race the first. So an outcome that begins work is ignored while pending,
 * and the outcomes that end it are always taken.
 */
export function next(editor: Editor, outcome: Outcome): Editor {
  switch (outcome.kind) {
    case "typed":
      // Typing answers whatever the last attempt said: the sentence was about
      // the text as it was, and the text has moved on.
      return editor.pending || editor.mode !== "source"
        ? editor
        : { ...editor, source: outcome.source, message: "", problems: [] };
    case "source":
      return editor.pending ? editor : { ...editor, mode: "source" };
    case "sending":
      return editor.pending ? editor : { ...editor, pending: true, message: outcome.saying, problems: [] };
    case "rendered":
      return { ...editor, pending: false, mode: "preview", blocks: outcome.blocks, message: "" };
    case "refused":
      return { ...editor, pending: false, message: refusal(outcome.status, outcome.said), problems: outcome.problems };
  }
}

/**
 * The one line under the bar: what just happened, or, when nothing has, what
 * is true of the text. An editor with nothing to say says nothing.
 */
export function statusOf(editor: Editor): string {
  if (editor.message !== "") {
    return editor.message;
  }
  return dirty(editor) ? "Unsaved changes" : "";
}

/**
 * What a refusal says.
 *
 * Two of them have words of their own, because the server's sentence is about
 * the file and the human needs to be told what to do about it. Everything else
 * is said in the server's own words, which is the whole of what is known.
 */
export function refusal(status: number, said: string): string {
  if (status === CONFLICT) {
    return CHANGED_ON_DISK;
  }
  if (status === UNPROCESSABLE) {
    return REFUSED_BY_THE_PROJECT;
  }
  return said;
}

/**
 * What the reading page says after a save: that it happened, and when. The
 * clock is the reader's own, and the minute is as fine as it needs to be —
 * this answers "did that land", not "at which second".
 */
export function savedAt(at: Date): string {
  return `Saved ${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}
