/**
 * Asking what happened (g1-s68): what a trouble is, what it carries and what
 * it never carries, and whose a pending one is.
 *
 * Everything here is arithmetic over values, with no request, no clock of its
 * own and no element in it. The trouble line, the context above every provider
 * and the Partner's store read it, so what the line offers, what the chip shows
 * and what the server is sent are one composition.
 */

/**
 * The one question a press asks. The server composes the human's turn from its
 * own copy (partner.TroubleRequest); this one is what the page shows while the
 * turn is on its way, and the two are the same words.
 */
export const TROUBLE_REQUEST = "What just happened here, why, and how do I recover?";

/** The control at the end of every trouble line. */
export const ASK_WHAT_HAPPENED = "Ask what happened";

/** What a secret the page holds is replaced with, before a trouble is kept or sent. */
export const WITHHELD = "[withheld]";

/** The sentence a trouble carries, at most: the server's own bound. */
export const MAX_TROUBLE_TEXT = 2000;

/** What a broken conversation's line says instead of offering a control nobody could read the answer of. */
export const BROKEN_ROOM = "This room cannot show its conversation; ask from the drawer after a reload.";
export const BROKEN_DRAWER = "The conversation cannot be shown; ask after a reload.";

/** The act a refusal refused, as words: never the request's arguments. */
export type TroubleAct = { verb: string; object: string; target?: string };

/** Where on the page it happened. */
export type TroubleWhere = { section: string; path: string; subject?: string; kind?: string };

/** One thing that went wrong on screen, as the page knew it (g1-s68 §6). */
export type Trouble = {
  text: string;
  code?: string;
  where: TroubleWhere;
  act?: TroubleAct;
  at: string;
  tip?: string;
  signIn?: boolean;
};

/**
 * A press made while its conversation was answering: held, owned by the
 * conversation the press was made in, and offered and sent only there.
 */
export type Pending = {
  id: string;
  /** The trouble line the press was made on, so the line can say it is waiting. */
  origin: string;
  /** The turn key, minted at the press, so a retry is the same turn. */
  key: string;
  conversation: string;
  trouble: Trouble;
  /** What the line says while it waits: "waiting for the room on …". */
  label: string;
};

/**
 * Every secret the page holds, replaced. The longer ones first, so a secret
 * that contains another is not half kept.
 */
export function scrub(text: string, secrets: readonly string[]): string {
  const held = secrets
    .map((secret) => secret.trim())
    .filter((secret) => secret !== "")
    .sort((left, right) => right.length - left.length);
  let said = text;
  for (const secret of held) {
    said = said.split(secret).join(WITHHELD);
  }
  return said;
}

/** The sentence, held to the server's bound by characters, not bytes. */
function bounded(text: string): string {
  const characters = [...text];
  if (characters.length <= MAX_TROUBLE_TEXT) {
    return text;
  }
  return characters.slice(0, MAX_TROUBLE_TEXT - 1).join("") + "…";
}

/**
 * A trouble, from what the line knows: scrubbed, bounded, and with no field
 * nobody filled. Nothing the request sent travels — there is no field for it.
 */
export function troubleOf(said: Trouble, secrets: readonly string[]): Trouble {
  const trouble: Trouble = {
    text: bounded(scrub(said.text.trim(), secrets)),
    where: { section: said.where.section, path: said.where.path },
    at: said.at,
  };
  if (said.where.subject !== undefined && said.where.subject !== "") {
    trouble.where.subject = said.where.subject;
    if (said.where.kind !== undefined && said.where.kind !== "") {
      trouble.where.kind = said.where.kind;
    }
  }
  if (said.code !== undefined && said.code !== "") {
    trouble.code = scrub(said.code, secrets);
  }
  if (said.act !== undefined) {
    trouble.act = { verb: said.act.verb, object: said.act.object };
    if (said.act.target !== undefined && said.act.target !== "") {
      trouble.act.target = said.act.target;
    }
  }
  if (said.tip !== undefined && said.tip !== "") {
    trouble.tip = said.tip;
  }
  if (said.signIn === true) {
    trouble.signIn = true;
  }
  return trouble;
}

/** The clock time a trouble happened at, in the browser's own zone. */
function clockOf(at: string): string {
  const when = new Date(at);
  if (Number.isNaN(when.getTime())) {
    return "";
  }
  return `${String(when.getHours()).padStart(2, "0")}:${String(when.getMinutes()).padStart(2, "0")}`;
}

/**
 * What the chip under the turn says travelled: the act and its subject, or the
 * pane; the code; the time.
 */
export function chipLine(trouble: Trouble): string {
  const parts: string[] = [];
  if (trouble.act !== undefined && trouble.act.verb !== "") {
    parts.push(trouble.act.verb);
    const target = trouble.act.target ?? trouble.where.subject ?? "";
    if (target !== "") {
      parts.push(target);
    }
  } else {
    parts.push(trouble.where.section === "" ? "This page" : trouble.where.section);
    if (trouble.where.subject !== undefined && trouble.where.subject !== "") {
      parts.push(trouble.where.subject);
    }
  }
  if (trouble.code !== undefined && trouble.code !== "") {
    parts.push(trouble.code);
  }
  const clock = clockOf(trouble.at);
  if (clock !== "") {
    parts.push(clock);
  }
  return parts.join(" · ");
}

/** A pane that threw, in one sentence that names the error. */
export function boundaryText(error: Error): string {
  return `This pane could not be rendered: ${error.name}: ${error.message}`;
}

/** What a waiting trouble says about whose it is. */
export function waitingLine(conversation: string, title: string): string {
  if (conversation === "") {
    return "waiting for your conversation";
  }
  // A room is named by what it sits on: the record's file name, without the
  // review's own prefix, is the goal a review room reviews.
  const named =
    title !== "" ? title : (conversation.split("/").at(-1) ?? conversation).replace(/\.md$/u, "").replace(/^review-of-/u, "");
  return `waiting for the room on ${named}`;
}

/** The first trouble waiting in this conversation, or null. */
export function pendingIn(list: readonly Pending[], conversation: string): Pending | null {
  return list.find((entry) => entry.conversation === conversation) ?? null;
}

/** Hold one press; the same press again replaces itself. */
export function held(list: readonly Pending[], entry: Pending): readonly Pending[] {
  const at = list.findIndex((one) => one.id === entry.id);
  if (at < 0) {
    return [...list, entry];
  }
  return [...list.slice(0, at), entry, ...list.slice(at + 1)];
}

/** Let one go: it was sent, or the human took it back. */
export function released(list: readonly Pending[], id: string): readonly Pending[] {
  return list.some((one) => one.id === id) ? list.filter((one) => one.id !== id) : list;
}

/**
 * What a Send does in one conversation: its own pending trouble first, then
 * the draft, and nothing while it is answering. A trouble waiting in another
 * conversation is never this one's to send.
 */
export function sendChoice(
  list: readonly Pending[],
  conversation: string,
  draft: string,
  state: { busy: boolean; sending: boolean },
): { kind: "trouble"; pending: Pending } | { kind: "draft" } | { kind: "none" } {
  if (state.busy || state.sending) {
    return { kind: "none" };
  }
  const waiting = pendingIn(list, conversation);
  if (waiting !== null) {
    return { kind: "trouble", pending: waiting };
  }
  return draft.trim() === "" ? { kind: "none" } : { kind: "draft" };
}
