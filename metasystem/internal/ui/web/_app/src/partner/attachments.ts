import { draftClearLabel, draftLabel, draftSource, type SheetDraft } from "./drafting";
import { chipLabel, type Chosen } from "./subject";

/**
 * Everything above the composer, as one list.
 *
 * A human puts context above the composer by one act — Ask on a card, Ask on a
 * selection, "Ask about this" in a sheet's head — and each of those acts makes
 * an attachment: one source, one label, one lifetime declared at the point of
 * making, and one × that takes it back. This file is the list and the rules,
 * and nothing else decides when an attachment leaves.
 *
 * It exists because a draft chip outlived its sheet: the rule that should have
 * retired it was written in the store beside the sheet stack, as a special
 * case, and the next kind of attachment would have needed another one. So the
 * lifetime travels with the attachment, two events retire attachments by that
 * lifetime alone, and every chip can say out loud how long it lives.
 *
 * What is not here: the page. Being somewhere is not an act, it has no ×, and
 * it is never a chip — it is the Seeing line. Suggested questions are offers,
 * not attachments, and the chips in the transcript are history.
 */

/**
 * How long an attachment lives, said where it is made.
 *
 * Three lifetimes, because there are three answers a human would accept to
 * "when does this leave": when I say so, when the question goes, and when the
 * thing it stands for goes.
 */
export type Lifetime = "until-cleared" | "until-sent" | { sheet: string };

/** What every attachment carries, whatever it stands for. */
type Made = {
  /**
   * What identifies it in the list. It is what the attachment is, rather than
   * when it was made: there is one subject, one passage and one draft per
   * sheet, so the id and the rule that a new one replaces the old are the same
   * fact said once.
   */
  id: string;
  /** Where it came from, in the words a human would use. */
  source: string;
  /** What its chip says. */
  label: string;
  lifetime: Lifetime;
};

/**
 * One attachment: what it stands for, and the thing itself.
 *
 * The content is typed by the kind rather than held as something to look up
 * later, for the same reason a capture is a value: what the Partner is given
 * and what the human saw are composed from one thing, at one moment.
 */
export type Attachment =
  | (Made & { kind: "subject"; content: Chosen })
  | (Made & { kind: "passage"; content: Chosen })
  | (Made & { kind: "draft"; content: SheetDraft });

/** The id an attachment of this kind has: one subject, one passage, one draft per sheet. */
export function idFor(kind: Attachment["kind"], sheet = ""): string {
  return kind === "draft" ? `draft:${sheet}` : kind;
}

/**
 * The subject a human chose. It stays until they clear it or choose another,
 * and a navigation does not touch it: "this" has to keep meaning the same
 * thing while they walk around (g1-s29, contract 3).
 */
export function attachedSubject(chosen: Chosen): Attachment {
  return {
    id: idFor("subject"),
    kind: "subject",
    source: chosen.source,
    label: chipLabel(chosen),
    lifetime: "until-cleared",
    content: chosen,
  };
}

/**
 * A passage a human selected. It goes with the next question: a quote is said
 * once, and a quote that stayed would be attached to questions nobody meant it
 * for.
 */
export function attachedPassage(passage: Chosen): Attachment {
  return {
    id: idFor("passage"),
    kind: "passage",
    source: passage.source,
    label: chipLabel(passage),
    lifetime: "until-sent",
    content: passage,
  };
}

/**
 * A sheet a human handed over. It lives as long as its sheet: cancelled, the
 * thing it described is gone; opened, it is a goal now. Either way the chip
 * would describe nothing (Wido, 2026-09-24).
 */
export function attachedDraft(draft: SheetDraft): Attachment {
  return {
    id: idFor("draft", draft.sheet),
    kind: "draft",
    source: draftSource(draft),
    label: draftLabel(draft),
    lifetime: { sheet: draft.sheet },
    content: draft,
  };
}

/**
 * Add one, or replace the one it replaces.
 *
 * A subject replaces the subject, a passage replaces the passage, and a draft
 * replaces the draft of its own sheet — which is what the shared id says. A
 * replacement keeps the place the old one held, so a second Ask does not
 * reshuffle the row a human is reading; everything else goes on the end, so
 * the list reads in the order the acts were made.
 */
export function attach(list: readonly Attachment[], made: Attachment): readonly Attachment[] {
  const at = list.findIndex((held) => held.id === made.id);
  if (at < 0) {
    return [...list, made];
  }
  return [...list.slice(0, at), made, ...list.slice(at + 1)];
}

/**
 * The first of the two events: a question was sent, and carried what it
 * carried. Everything that goes with a question goes now.
 */
export function retireOnSent(list: readonly Attachment[]): readonly Attachment[] {
  return without(list, (held) => held.lifetime === "until-sent");
}

/**
 * The second: a sheet closed, however it closed. Everything whose life was
 * that sheet's goes with it, and a draft from another sheet stands.
 */
export function retireOnSheetClosed(list: readonly Attachment[], name: string): readonly Attachment[] {
  return without(list, (held) => typeof held.lifetime === "object" && held.lifetime.sheet === name);
}

/** The × on a chip: this one, by name, and nothing else. */
export function remove(list: readonly Attachment[], id: string): readonly Attachment[] {
  return without(list, (held) => held.id === id);
}

/**
 * This sheet's draft, as the sheet stands now — and the list untouched where
 * that sheet handed nothing over. A sheet nobody offered still reaches the
 * Partner with nothing but its name.
 */
export function refreshDraft(list: readonly Attachment[], draft: SheetDraft): readonly Attachment[] {
  const made = attachedDraft(draft);
  return list.some((held) => held.id === made.id) ? attach(list, made) : list;
}

/**
 * The one line a chip says about itself, on hover and on focus. A human never
 * has to guess why a chip is there or when it will leave.
 */
export function lifeOf(attachment: Attachment): string {
  const life = attachment.lifetime;
  if (life === "until-cleared") {
    return "stays until you clear it";
  }
  if (life === "until-sent") {
    return "goes with the next question";
  }
  return `goes with the ${life.sheet} sheet`;
}

/** What the × says it will do, in the words the act that made it would use. */
export function takeBackLabel(attachment: Attachment): string {
  return attachment.kind === "draft"
    ? draftClearLabel(attachment.content)
    : `Stop asking about ${attachment.label}`;
}

/* ------------------------------------------- what the rest of the app reads -- */

/** The chosen subject, or null while the subject follows the page. */
export function subjectIn(list: readonly Attachment[]): Chosen | null {
  return firstOf(list, "subject")?.content ?? null;
}

/** The selected passage, or null. */
export function passageIn(list: readonly Attachment[]): Chosen | null {
  return firstOf(list, "passage")?.content ?? null;
}

/**
 * The sheet a human handed over, or null. Where two sheets stand open and both
 * were offered, it is the one offered last, which is the one they are standing
 * in: a capture carries one draft, as a human is filling in one form.
 */
export function draftIn(list: readonly Attachment[]): SheetDraft | null {
  return firstOf([...list].reverse(), "draft")?.content ?? null;
}

/** The first attachment of one kind, typed by the kind that was asked for. */
function firstOf<K extends Attachment["kind"]>(
  list: readonly Attachment[],
  kind: K,
): Extract<Attachment, { kind: K }> | undefined {
  return list.find((held): held is Extract<Attachment, { kind: K }> => held.kind === kind);
}

/** Dropped, and the same list where there was nothing to drop. */
function without(list: readonly Attachment[], going: (held: Attachment) => boolean): readonly Attachment[] {
  const kept = list.filter((held) => !going(held));
  return kept.length === list.length ? list : kept;
}
