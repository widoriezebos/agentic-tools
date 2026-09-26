/**
 * A sheet a human is filling in, offered to the Project Partner.
 *
 * A sheet is a subject like any other object on a page: "Ask about this" in
 * its head attaches what is written in it, right now, as a removable chip
 * above the composer, and the next question carries it marked as a draft the
 * human is filling in rather than anything the ledger has. Nothing about a
 * sheet reaches the Partner without that act — the fields are not watched, not
 * streamed, and not attached again after they change.
 *
 * A field that holds a secret is never offered here at all: the sign-in sheet
 * has no Ask, and the one-time code it takes is not a field any sheet hands
 * over.
 */

/** One field of a sheet, as a human reads it and as they have filled it in. */
export type Field = { name: string; value: string };

/** What was in a sheet at the moment a human offered it. */
export type SheetDraft = {
  /** What the sheet is called, as its head says it: "New goal". */
  sheet: string;
  /**
   * The one opening of that sheet this draft came from: the id it minted when
   * it mounted. Goal A's edit sheet and goal B's are two openings of a sheet of
   * one name, and a suggestion prepared for one must never reach the other, so
   * what a suggestion belongs to is this rather than the name.
   */
  opening: string;
  /** Its fields in the order the sheet asks them, the empty ones dropped. */
  fields: Field[];
  /**
   * Every field of this sheet the Partner may offer words for, including the
   * ones with nothing in them yet, in the order the sheet asks them.
   *
   * It is not the fields above with the names taken off. Those drop what is
   * empty — and an empty next step is exactly the field a human asks for words
   * for — and they include what the sheet hands over to be read rather than
   * written into: the goal an edit is of is context, not a field anybody may
   * rewrite. The sheet that owns the state says which are which.
   */
  writable: string[];
  /**
   * The writable field the human's caret was last in, or "" before any of them
   * has held it.
   *
   * It is what makes "make this shorter" mean something. A request that names no
   * field is about the field the human is writing in, and nothing else in a
   * draft says which that is: the fields travel in the order the sheet asks
   * them, not in the order a human moved through them. Wido, 2026-09-26: "do you
   * know which field I was editing when I started editing in the project partner
   * panel? Because you will have to."
   */
  writing: string;
};

/** How much of one field's value the chip shows before it trails off. */
const CHIP_FIELD = 40;

/**
 * How many of a sheet's fields the chip names, before and after a field has held
 * the caret. A chip is a label, not a form.
 *
 * It drops to one once a field is in hand, because the chip then has something
 * better to say with the room: which field a request naming none is about. What
 * the one remaining field is for is telling two drafts apart — the goal an edit
 * is of, the id a new goal is being opened under — and the words of the field the
 * human is writing in are on the screen in front of them, in the sheet itself
 * (g1-s52 Built, deferred).
 */
const CHIP_FIELDS = 2;
const CHIP_FIELDS_WRITING = 1;

/**
 * The draft as it will travel: the sheet's name, the opening it came from, the
 * fields that have something in them, and the ones that may be written into.
 *
 * An empty field says nothing, so its value is not carried — the Partner
 * reading "Intent:" followed by nothing would be reading a claim the human
 * never made. Its name still travels among the writable ones, because a field
 * nobody has filled in is a field the Partner can be asked for words for.
 */
export function draftOf(
  opening: string,
  sheet: string,
  fields: Field[],
  writable: string[] = [],
  writing = "",
): SheetDraft {
  return {
    sheet,
    opening,
    fields: fields
      .map((field) => ({ name: field.name, value: field.value.trim() }))
      .filter((field) => field.value !== ""),
    writable: writable.filter((name) => name.trim() !== ""),
    writing: writing.trim(),
  };
}

/**
 * What one field of a draft holds, as the sheet reads it now, or "" for a field
 * that is empty or is not this sheet's.
 */
export function valueIn(draft: SheetDraft, field: string): string {
  return draft.fields.find((one) => one.name === field)?.value ?? "";
}

/** True where there is something to offer: a sheet with a field filled in. */
export function offersDraft(draft: SheetDraft): boolean {
  return draft.fields.length > 0;
}

/**
 * What the chip says: the sheet, then the first of its fields, then where the
 * caret is — so a human scanning the composer reads "Draft: Edit goal · g1-s12 ·
 * writing in Intent" and knows which sheet, which draft, and what a request
 * naming no field will mean.
 *
 * Before any field has held the caret there is nothing to say about where the
 * human is, so the room goes to a second field instead: "Draft: New goal ·
 * refund-worker · Refunds are issued within a day… · writing in nothing yet".
 */
export function draftLabel(draft: SheetDraft): string {
  const room = draft.writing.trim() === "" ? CHIP_FIELDS : CHIP_FIELDS_WRITING;
  const said = draft.fields.slice(0, room).map((field) => firstWords(field.value));
  return ["Draft: " + draft.sheet, ...said, writingClause(draft.writing)].join(" · ");
}

/**
 * What the chip says about the field in hand.
 *
 * It is on the chip because the chip is what a human can see of what the Partner
 * is being told, and the field in hand is now part of that: a request that names
 * no field is answered about this one. Before any field has held the caret it
 * says so rather than saying nothing, because "none yet" is the case in which
 * the Partner has to ask instead of guessing.
 */
export function writingClause(writing: string): string {
  const said = writing.trim();
  return said === "" ? "writing in nothing yet" : "writing in " + said;
}

/** Where the chip says the draft came from, in the words a human would use. */
export function draftSource(draft: SheetDraft): string {
  return `the ${draft.sheet} sheet`;
}

/** What the × says it will stop the Partner being given. */
export function draftClearLabel(draft: SheetDraft): string {
  return `Stop offering ${draftSource(draft)}`;
}

/**
 * The first words of a value, on one line. A field a human typed a paragraph
 * into is still one chip, and the whole of it is in the draft the question
 * carries.
 */
function firstWords(value: string): string {
  const one = value.replace(/\s+/gu, " ").trim();
  if (one.length <= CHIP_FIELD) {
    return one;
  }
  const cut = one.slice(0, CHIP_FIELD);
  const space = cut.lastIndexOf(" ");
  return `${(space > CHIP_FIELD / 2 ? cut.slice(0, space) : cut).trimEnd()}…`;
}
