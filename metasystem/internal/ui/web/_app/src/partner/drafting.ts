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
};

/** How much of one field's value the chip shows before it trails off. */
const CHIP_FIELD = 40;

/** How many of a sheet's fields the chip names. A chip is a label, not a form. */
const CHIP_FIELDS = 2;

/**
 * The draft as it will travel: the sheet's name, the opening it came from, the
 * fields that have something in them, and the ones that may be written into.
 *
 * An empty field says nothing, so its value is not carried — the Partner
 * reading "Intent:" followed by nothing would be reading a claim the human
 * never made. Its name still travels among the writable ones, because a field
 * nobody has filled in is a field the Partner can be asked for words for.
 */
export function draftOf(opening: string, sheet: string, fields: Field[], writable: string[] = []): SheetDraft {
  return {
    sheet,
    opening,
    fields: fields
      .map((field) => ({ name: field.name, value: field.value.trim() }))
      .filter((field) => field.value !== ""),
    writable: writable.filter((name) => name.trim() !== ""),
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
 * What the chip says: the sheet, then the first of its fields, so a human
 * scanning the composer reads "Draft: New goal · refund-worker · Refunds are
 * issued within a day…" and knows both which sheet and which draft.
 */
export function draftLabel(draft: SheetDraft): string {
  const said = draft.fields.slice(0, CHIP_FIELDS).map((field) => firstWords(field.value));
  return ["Draft: " + draft.sheet, ...said].join(" · ");
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
