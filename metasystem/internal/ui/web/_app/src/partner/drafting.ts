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
  /** Its fields in the order the sheet asks them, the empty ones dropped. */
  fields: Field[];
};

/** How much of one field's value the chip shows before it trails off. */
const CHIP_FIELD = 40;

/** How many of a sheet's fields the chip names. A chip is a label, not a form. */
const CHIP_FIELDS = 2;

/**
 * The draft as it will travel: the sheet's name, and the fields that have
 * something in them. An empty field says nothing, so it is not carried — the
 * Partner reading "Intent:" followed by nothing would be reading a claim the
 * human never made.
 */
export function draftOf(sheet: string, fields: Field[]): SheetDraft {
  return {
    sheet,
    fields: fields
      .map((field) => ({ name: field.name, value: field.value.trim() }))
      .filter((field) => field.value !== ""),
  };
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
