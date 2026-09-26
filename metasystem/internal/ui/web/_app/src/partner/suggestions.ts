/**
 * The questions worth asking about each kind of thing, in one register.
 *
 * It is one file for the same reason the help texts are one file: these are
 * words a human reads, and words a human reads should be written once, in one
 * voice, where rewording one of them is one edit. The chips above the composer
 * read from here, and so does the empty drawer.
 *
 * Every question names its scope. Astra's fourth finding is why: "What needs
 * me here?" and "What changed today?" are only honest if both sides know what
 * set and what window they are about, and a question this page cannot scope is
 * not suggested at all rather than suggested and quietly answered about
 * something else.
 */

/** One suggested question: what it asks, and what it is asking about. */
export type Suggestion = {
  /** The question, exactly as it is sent. */
  text: string;
  /** What it is scoped to, which travels with it and which the chip explains. */
  scope: string;
};

/**
 * The register, by the kind of thing the question is about.
 *
 * The keys are the ones subject.ts resolves to: the five subject kinds, and
 * the sections a page with no subject falls back to.
 */
const REGISTER: Readonly<Record<string, readonly Suggestion[]>> = {
  goal: [
    { text: "Why is it here?", scope: "this goal's own record at the accepted tip: its lane, its state, and what put it there" },
    { text: "What would move it?", scope: "this goal's blockers, approval and next step" },
    { text: "What does its design say?", scope: "the designs whose Goals line names this goal" },
  ],
  design: [
    { text: "Has its work shipped?", scope: "the goals this design names, and where each one stands at the accepted tip" },
    { text: "What does it leave out?", scope: "this design's own text, as it stands in the checkout" },
    { text: "Which goals does it name?", scope: "this design's Goals line" },
  ],
  decision: [
    { text: "What did it decide, and why?", scope: "this decision's own text, as it stands in the checkout" },
  ],
  lane: [
    { text: "What is in here, and why?", scope: "the rows this lane is showing, after the board's filters" },
  ],
  board: [
    { text: "What needs me here?", scope: "the rows on this board that nobody has authorised, after its filters" },
    { text: "What changed today?", scope: "since local midnight, through the changes this server knows" },
    { text: "What is next up?", scope: "Ready for Work, in priority order" },
  ],
  overview: [
    { text: "What needs me here?", scope: "the Needs-you set on this page" },
    { text: "What changed today?", scope: "since local midnight, through the changes this server knows" },
    { text: "What is next up?", scope: "Ready for Work, in priority order" },
  ],
  document: [
    { text: "Summarise this", scope: "this document's own text, as it stands in the checkout" },
    { text: "What does it rest on?", scope: "the records this one cites, and the records that cite it" },
  ],
  record: [
    { text: "Summarise this", scope: "this record's own text, as it stands in the checkout" },
    { text: "What does it rest on?", scope: "the records this one cites, and the records that cite it" },
  ],
  passage: [
    { text: "What does this mean?", scope: "the passage you selected, and the document it came from" },
    { text: "What does it rest on?", scope: "the records the document it came from cites" },
  ],
  project: [
    { text: "What is in this tab?", scope: "the records this tab is listing" },
    { text: "What changed today?", scope: "since local midnight, through the changes this server knows" },
  ],
  draft: [
    {
      text: "Suggest a better wording",
      scope: "the fields of the sheet you handed over, as they stand; what comes back is a suggestion you decide about",
    },
    {
      text: "Suggest the next step",
      scope: "the sheet you handed over, as it stands; what comes back is a suggestion you decide about",
    },
  ],
};

/**
 * The register a handed-over sheet asks from, which is the one register that is
 * not about a kind of thing on a page: it is about the draft a human is
 * filling in, and the two questions are the two that ask the Partner to write
 * rather than to explain.
 */
export const DRAFT_REGISTER = "draft";

/** The questions for one register, or none where this build suggests none. */
export function suggestionsFor(register: string): readonly Suggestion[] {
  return REGISTER[register] ?? [];
}

/** Every register this build carries, for the guard that reads them all. */
export function registers(): string[] {
  return Object.keys(REGISTER);
}

/**
 * How to make a thing the subject, said once, in the first minute.
 *
 * The chips teach the questions; this teaches the gesture. It is a sentence
 * rather than a chip because it is not a question, and an empty drawer that
 * offered it as one would send it.
 */
export const HOW_TO_ASK =
  "Right-click a card and choose Ask about this, or select a passage and press Ask.";

/** Cmd or Ctrl, by what the browser is running on. */
export function composerShortcut(platform: string): string {
  return /mac/i.test(platform) ? "Cmd+J" : "Ctrl+J";
}
