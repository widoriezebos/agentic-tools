import type { Deposit, Sitting } from "./api";
import { documentPath } from "../routes";

/**
 * What a sitting is, from the browser's side: the four sections of one record,
 * and the rules around the one press that writes into them.
 *
 * A sitting keeps no working material of its own. The record IS the memory —
 * "Facts", "Proposals", "Decisions", "Open questions", four plain headings the
 * document reader already renders — and everything here reads or composes that
 * record's own source. It is a file of functions rather than a component
 * because every rule in it is a rule a human would notice if it were wrong, and
 * each one can be read, and tested, without a browser.
 *
 * Two rules are the ones that would cost a human something.
 *
 * A Record press composes its entry from ONE current reading of the record and
 * writes it under that reading's revision. The document edit replaces the whole
 * source under a revision check: it has no append, and it serializes nothing.
 * So two presses against one reading would refuse the second or, overlapping,
 * lose the first entry — Astra's F1. The answer is here and in the recorder
 * beside it: one reading, refreshed from every write, and one press at a time.
 *
 * A decision is not recorded without a reason and a fact is not recorded
 * without an anchor. Not because the Partner must supply them — it may not know
 * the anchor — but because an entry that says a choice was made and cannot say
 * why is exactly the entry the sitting exists to prevent. The card asks the
 * human for the one thing missing, after their own edit.
 *
 * Two more rules were Sol's, and both are about a card outliving the moment it
 * was offered in.
 *
 * A card belongs to the record it was offered against, which the server stamped
 * on it. End the sitting on A, start one on B, and A's cards are still on the
 * transcript: pressing one of them would put A's words into B's record. So a
 * card whose subject is not the sitting now standing offers no press at all —
 * only its words, to copy.
 *
 * And what the record holds is what says a card was recorded. The mark a press
 * leaves on a card is this browser's memory, and a reload has none: every
 * persisted deposit came back as a fresh card offering Record it a second time,
 * so the same entry could be written twice. So each entry carries the identity
 * of the deposit it was recorded from, and a card's recorded state is read from
 * the record's own reading rather than remembered.
 */

/* ------------------------------------------------------- the four sections -- */

/** The four sections of a sitting's record, in the order the table shows them. */
export const SECTIONS = ["Facts", "Proposals", "Decisions", "Open questions"] as const;

export type Section = (typeof SECTIONS)[number];

/**
 * The fifth section, which is not a pile: what the sitting came to, written when
 * the human ends it (g1-s55 D2).
 *
 * It is not on the table and it is not a list. The four piles are the working
 * material a sitting accumulates, entry by entry; this is the one thing it
 * concludes, written once and replaced rather than appended to — so a sitting
 * closed twice leaves one Outcome and not two.
 */
export const OUTCOME = "Outcome";

/** Where one deposit is written: one of the four piles, or the Outcome. */
export type Written = Section | typeof OUTCOME;

/** Which section each kind of deposit is written into. */
const SECTION_OF: Readonly<Record<string, Written>> = {
  fact: "Facts",
  proposal: "Proposals",
  decision: "Decisions",
  question: "Open questions",
  outcome: OUTCOME,
};

/**
 * Where a deposit of this kind lands.
 *
 * A case is missing from the map on purpose and never asked for it: a case lands
 * on no section by itself (D1), and what lands is the decision or the open
 * question the human makes of it. The card for one shows no destination, because
 * it has none until a human presses one of its two buttons.
 */
export function sectionOf(kind: string): Written {
  return SECTION_OF[kind] ?? "Facts";
}

/** Whether one written section is one of the four piles the table shows. */
export function isPile(section: Written): section is Section {
  return (SECTIONS as readonly string[]).includes(section);
}

/** What each kind calls the clause beside its words, as the card labels it. */
const CLAUSE_OF: Readonly<Record<string, string>> = {
  fact: "Anchor",
  decision: "Reason",
  question: "Consequence",
  proposal: "Consequence",
  // A case card's clause is the consequence of leaving it open, because that is
  // the one its own press records: Leave open writes the case as an open
  // question with it. The other clause a case carries — the clause it would
  // become — belongs to the Decide sheet, which is where it is read and edited.
  case: "Consequence",
  // An outcome carries no clause. It is a whole section, and a labelled line
  // beside it would be a field with nothing to put in it.
  outcome: "",
};

export function clauseOf(kind: string): string {
  return CLAUSE_OF[kind] ?? "Anchor";
}

/** Whether this kind of card has a clause beside its words at all. */
export function clausedKind(kind: string): boolean {
  return clauseOf(kind) !== "";
}

/**
 * The heading id the document reader gives each of the four sections.
 *
 * Four literals rather than a slug function, because there is exactly one thing
 * they have to agree with — the ids the reader itself writes, lower case with
 * the spaces hyphenated (internal/ui/markdown) — and four spellings cannot
 * drift from that the way a second implementation of the rule could.
 */
const ANCHOR_OF: Readonly<Record<Section, string>> = {
  Facts: "facts",
  Proposals: "proposals",
  Decisions: "decisions",
  "Open questions": "open-questions",
};

/**
 * Where one entry of the table leads: its own record, opened at the pile it
 * stands in (g1-s53 D7).
 *
 * At the pile, and not at the line. The reader anchors headings and only
 * headings — a list item carries no id — so an entry-level anchor would need the
 * reader to mint ids for list items and the table to know which item of the
 * source this entry was: new mechanism on both sides, for a scroll of a few
 * lines inside a pile that is already short. The heading exists, it is where the
 * entry is, and pressing an entry opens the record there.
 *
 * A record carrying two headings of one name would have the reader number the
 * second, and this leads to the first. That is also the heading a recorded entry
 * is written under, so the link and the write agree about which pile is meant.
 */
export function entryPath(record: string, section: Section): string {
  return `${documentPath(record)}#${ANCHOR_OF[section]}`;
}

/**
 * One entry of a section, as the record carries it and the table reads it back.
 *
 * `when` and `who` are the two facts D7 asks of every entry beside its words
 * and its clause, and they are first in the line for a reason: the words and
 * the clause are free text a human edits, and free text cannot be told from
 * free text by a separator. So the line carries the dated, attributed head
 * first and the words to the end, and the clause goes on its own nested line
 * where nothing follows it either.
 */
export type Entry = {
  when: string;
  who: string;
  text: string;
  /** The anchor, the reason or the consequence, whichever the kind carries. */
  clause: string;
  /** Which section it stands in: one of the four piles, or the Outcome. */
  section: Written;
  /**
   * Which deposit this entry was recorded from, or "" for a line a human wrote
   * by hand. It is what makes a card's recorded state a fact about the record
   * rather than a memory of this browser's, so a reload cannot offer Record it
   * a second time for an entry the record already carries.
   */
  mark: string;
};

/** The separator between an entry's dated head and its words. */
const DOT = " · ";

/**
 * The mark that says which deposit an entry was recorded from.
 *
 * It rides the end of the entry's own line, after the words, in brackets: short,
 * plainly a mark rather than prose, and the last thing on the line so that
 * nothing a human writes in front of it can be mistaken for it. The table's
 * reader takes it off again, so what a human reads in the table is their words
 * and not the bookkeeping.
 *
 * It is a visible mark and not an HTML comment on purpose. The document reader
 * shows html as source text, deliberately — nothing in a record is interpreted
 * — so a comment would be rendered as a blob of code beside every entry, and
 * teaching the reader to hide comments would teach it to hide text from the
 * human reading a record.
 */
const MARKED = /\s*\[d:([^\]\s]+)\]\s*$/;

export function markOf(id: string): string {
  return ` [d:${id}]`;
}

/** How one entry is written into a section. */
export function lineOf(entry: Entry, kind: string): string {
  const mark = entry.mark.trim() === "" ? "" : markOf(entry.mark.trim());
  const head = `- ${entry.when}${DOT}${entry.who}${DOT}${oneLine(entry.text)}${mark}`;
  const clause = entry.clause.trim();
  return clause === "" ? `${head}\n` : `${head}\n  - ${clauseOf(kind)}: ${oneLine(clause)}\n`;
}

/**
 * An entry's own words on one line.
 *
 * A record's section is a list, and a list item that carried a blank line would
 * end the item and leave the rest of the words as prose beside it. So the words
 * are flattened when they are written, and the entry the table reads back is
 * the line it wrote.
 */
function oneLine(said: string): string {
  return said.split(/\s+/).filter((part) => part !== "").join(" ");
}

/** The line that opens a section. */
function headingOf(section: Section): string {
  return `## ${section}`;
}

/**
 * Every entry the record's four sections carry, in the order they stand.
 *
 * It reads what this build writes and nothing cleverer: a list item whose head
 * is a date, a name and words, with an optional nested clause line under it. A
 * line a human wrote by hand that does not fit is read as an entry with no
 * clause and whatever it says as its words, because showing a human the line
 * they typed is better than hiding it.
 */
export function entriesIn(source: string): readonly Entry[] {
  const found: Entry[] = [];
  let section: Section | null = null;
  let last: Entry | null = null;
  for (const line of source.split("\n")) {
    const heading = /^#{1,6}\s+(.*)$/.exec(line);
    if (heading !== null) {
      const named = heading[1].trim();
      section = (SECTIONS as readonly string[]).includes(named) ? (named as Section) : null;
      last = null;
      continue;
    }
    if (section === null) {
      continue;
    }
    const clause = /^\s+[-*]\s+(?:Anchor|Reason|Consequence):\s*(.*)$/.exec(line);
    if (clause !== null && last !== null) {
      last.clause = clause[1].trim();
      continue;
    }
    const item = /^[-*]\s+(.*)$/.exec(line);
    if (item === null) {
      continue;
    }
    // The mark comes off first, so the words the table shows are the words and
    // the identity is read from where it was written.
    const marked = MARKED.exec(item[1]);
    const said = marked === null ? item[1] : item[1].slice(0, marked.index);
    const mark = marked === null ? "" : marked[1];
    const parts = said.split(DOT);
    const entry: Entry =
      parts.length >= 3
        ? {
            when: parts[0].trim(),
            who: parts[1].trim(),
            text: parts.slice(2).join(DOT).trim(),
            clause: "",
            section,
            mark,
          }
        : { when: "", who: "", text: said.trim(), clause: "", section, mark };
    found.push(entry);
    last = entry;
  }
  return found;
}

/**
 * Every entry the record carries that a deposit was recorded from, by that
 * deposit's identity.
 *
 * It is what makes Record it a once-only press across a reload: the card is
 * recorded because the record says so, and what it shows is what the record
 * says rather than what this browser remembers typing. An entry whose mark the
 * record carries twice is read as the first of them, because the first is the
 * one the press landed.
 */
export function recordedIn(source: string): ReadonlyMap<string, Entry> {
  const found = new Map<string, Entry>();
  for (const entry of entriesIn(source)) {
    if (entry.mark !== "" && !found.has(entry.mark)) {
      found.set(entry.mark, entry);
    }
  }
  // And the Outcome, which is the one recorded thing that is not an entry of a
  // pile. It is read for the same reason every entry's mark is: the outcome card
  // must say "recorded" because the record says so, and a reload must not offer
  // to write a second Outcome over the one already there.
  const outcome = outcomeIn(source);
  if (outcome !== null && outcome.mark !== "" && !found.has(outcome.mark)) {
    found.set(outcome.mark, outcome);
  }
  return found;
}

/* -------------------------------------------------------------- the outcome -- */

/**
 * The line that says where an Outcome came from.
 *
 * The section itself is the human's words, whole, so the bookkeeping cannot ride
 * the end of them the way an entry's does: an entry is one line and this is
 * paragraphs, and a mark stuck to the last sentence of a human's outcome would
 * be a mark inside their prose. So it is one dated, attributed line at the foot,
 * shaped like an entry's head — which is also what lets the record say who
 * closed this sitting and when.
 */
export const FROM_THE_SITTING = "Recorded from the sitting";

const OUTCOME_FOOT = new RegExp(
  `^[-*]\\s+${FROM_THE_SITTING}${DOT}(.*?)${DOT}(.*?)\\s*\\[d:([^\\]\\s]+)\\]\\s*$`,
);

export function outcomeFoot(entry: Entry): string {
  return `- ${FROM_THE_SITTING}${DOT}${entry.when}${DOT}${entry.who} ${markOf(entry.mark).trim()}`;
}

/** Where the record's Outcome section stands, or null where it has none. */
function outcomeAt(lines: readonly string[]): { at: number; end: number } | null {
  for (let index = 0; index < lines.length; index += 1) {
    const heading = /^#{1,6}\s+(.*)$/.exec(lines[index]);
    if (heading === null || heading[1].trim() !== OUTCOME) {
      continue;
    }
    let end = lines.length;
    for (let after = index + 1; after < lines.length; after += 1) {
      if (/^#{1,6}\s+/.test(lines[after])) {
        end = after;
        break;
      }
    }
    return { at: index, end };
  }
  return null;
}

/**
 * The record's whole source with the sitting's outcome written under an
 * "Outcome" heading — replacing the one the record has (g1-s55 D2).
 *
 * Replacing, and not appending, because an outcome is what the sitting came to
 * and a record has one: every design this project writes is created with an
 * empty Outcome heading already in it (project/write.go's templates), so an
 * append would leave the heading above and the words below it under a second one.
 *
 * The heading is left exactly as the record spells it — level and all — and
 * everything outside the section is untouched, because this is a whole source
 * going through a revision-checked write and a composition that tidied anything
 * would be this browser rewriting a human's record around the one thing they
 * asked for.
 */
export function outcomeWritten(source: string, entry: Entry): string {
  const body = [...oneOrMore(entry.text), "", outcomeFoot(entry)];
  const lines = source.split("\n");
  const found = outcomeAt(lines);
  if (found === null) {
    const before = source.replace(/\s*$/, "");
    const opened = before === "" ? "" : `${before}\n\n`;
    return `${opened}## ${OUTCOME}\n\n${body.join("\n")}\n`;
  }
  // The blank lines before the next heading belong to the gap between sections,
  // so the section is replaced and the gap is kept.
  let last = found.end;
  while (last > found.at + 1 && lines[last - 1].trim() === "") {
    last -= 1;
  }
  return [...lines.slice(0, found.at + 1), "", ...body, ...lines.slice(last)].join("\n");
}

/**
 * The Outcome the record carries, as the entry it was written from, or null.
 *
 * The words are what stands between the heading and the foot, whole, so the card
 * shows the record's own outcome rather than what the Partner first offered. A
 * section with no foot is an Outcome a human wrote by hand: it is read, with no
 * mark, so nothing claims a deposit wrote it.
 */
export function outcomeIn(source: string): Entry | null {
  const lines = source.split("\n");
  const found = outcomeAt(lines);
  if (found === null) {
    return null;
  }
  const said = lines.slice(found.at + 1, found.end);
  let when = "";
  let who = "";
  let mark = "";
  const words: string[] = [];
  for (const line of said) {
    const foot = OUTCOME_FOOT.exec(line);
    if (foot === null) {
      words.push(line);
      continue;
    }
    when = foot[1].trim();
    who = foot[2].trim();
    mark = foot[3];
  }
  const text = words.join("\n").replace(/^\s*\n/, "").replace(/\s*$/, "");
  if (text === "" && mark === "") {
    return null;
  }
  return { when, who, text, clause: "", section: OUTCOME, mark };
}

/**
 * The words of an outcome as its section carries them: every line, with the
 * blank lines kept, because an outcome is paragraphs and flattening it would
 * turn the whole of a sitting's result into one line.
 */
function oneOrMore(said: string): readonly string[] {
  return said.replace(/\s*$/, "").replace(/^\s*\n/, "").split("\n");
}

/** How many entries each section holds, which is what the drawer's strip says. */
export type Counts = Readonly<Record<Section, number>>;

export function countsIn(source: string): Counts {
  const counted: Record<Section, number> = { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0 };
  for (const entry of entriesIn(source)) {
    if (isPile(entry.section)) {
      counted[entry.section] += 1;
    }
  }
  return counted;
}

/**
 * The record's whole source with one entry appended to its section.
 *
 * The entry goes at the END of that section, before the next heading of any
 * level, so the section reads in the order the sitting went. A section the
 * record does not carry yet is opened at the foot of the document, because a
 * record that has never been sat on has none of the four and the first deposit
 * is what creates one.
 *
 * Everything else about the source is left exactly as it was. This is a whole
 * source going through a revision-checked write, so a composition that
 * reformatted anything would be this browser silently rewriting a human's
 * record around the one line they asked for.
 */
export function appended(source: string, entry: Entry, kind: string): string {
  // The Outcome is written and replaced rather than appended to, so a press for
  // one goes through its own composition and never through this. It is asked
  // here rather than at the call site because this is the function that would
  // silently make a list of a section that is prose.
  if (entry.section === OUTCOME) {
    return outcomeWritten(source, entry);
  }
  const line = lineOf(entry, kind);
  const lines = source.split("\n");
  const heading = headingOf(entry.section);
  const at = lines.findIndex((one) => one.trim() === heading);
  if (at < 0) {
    const body = source.replace(/\s*$/, "");
    const opened = body === "" ? "" : `${body}\n\n`;
    return `${opened}${heading}\n\n${line}`;
  }
  // The end of this section: the next heading of any level, or the foot.
  let end = lines.length;
  for (let index = at + 1; index < lines.length; index += 1) {
    if (/^#{1,6}\s+/.test(lines[index])) {
      end = index;
      break;
    }
  }
  // Blank lines before the next heading belong to the gap between sections, not
  // to this section's list, so the entry goes above them.
  let last = end;
  while (last > at + 1 && lines[last - 1].trim() === "") {
    last -= 1;
  }
  const before = lines.slice(0, last);
  // A section with nothing in it yet needs the blank line a list stands after.
  if (last === at + 1) {
    before.push("");
  }
  return [...before, ...line.replace(/\n$/, "").split("\n"), ...lines.slice(last)].join("\n");
}

/* ----------------------------------------------------------- the two verbs -- */

/** What this build offers a sitting to be for. The other two are not here. */
export const PURPOSES = ["shape intent", "shape a design"] as const;

export type Purpose = (typeof PURPOSES)[number];

/** What each purpose is called where a human chooses one. */
export function purposeLabel(purpose: string): string {
  return purpose === "shape intent" ? "Shape intent" : "Shape a design";
}

/**
 * The record kinds a sitting is about (g1-s53 D1).
 *
 * The two the purposes name and no others: a sitting shapes intent or shapes a
 * design, and its subject is the record it shapes. A doctrine record, a recorded
 * decision, a question row or a plain file of the checkout is not a thing there
 * is a sitting for, and offering Start on one offered a conversation whose four
 * piles would be written into a record that has no business carrying them.
 *
 * A file that declares no head at all is not a record and is not sittable
 * either, which is why this is asked of the kind rather than of the file.
 */
export const SITTABLE_KINDS = ["intent", "design"] as const;

export function sittable(kind: string | null | undefined): boolean {
  return (SITTABLE_KINDS as readonly string[]).includes((kind ?? "").trim());
}

/** What the chip above the composer says while a sitting stands. */
export function sittingChip(sitting: Sitting): string {
  const named = sitting.subject.title.trim();
  return `Sitting: ${named === "" ? sitting.subject.id : named}`;
}

/* --------------------------------------------------------- the deposit card -- */

/** What identifies one card: the turn it arrived in, and its place in it. */
export function depositID(turn: string, at: number): string {
  return `deposit:${turn}#${String(at)}`;
}

/** What the human has done with one deposit, and what they have made of it. */
export type Mark = {
  /** The words as they now stand, which start as the Partner's. */
  text: string;
  /** The clause as it now stands: the anchor, the reason or the consequence. */
  clause: string;
  /** True while a press is in flight. */
  recording: boolean;
  /** Where the record took it, or "" while it has not. */
  recorded: string;
  /** What a refused press said, in the words a human reads, or "". */
  refusal: string;
  /** Folded to one line by Dismiss, and unfolded by pressing that line. */
  dismissed: boolean;
};

export type Marks = Readonly<Record<string, Mark>>;

/** One card's mark as it starts: the Partner's words, untouched. */
export function marked(deposit: Deposit): Mark {
  return {
    text: deposit.text,
    clause: clauseFrom(deposit),
    recording: false,
    recorded: "",
    refusal: "",
    dismissed: false,
  };
}

/** The one clause the deposit carries, whichever of the three its kind uses. */
export function clauseFrom(deposit: Deposit): string {
  switch (deposit.kind) {
    case "decision":
      return deposit.reason ?? "";
    case "question":
    case "proposal":
    case "case":
      return deposit.consequence ?? "";
    case "outcome":
      return "";
    default:
      return deposit.anchor ?? "";
  }
}

/**
 * The clause a case would become, as the Partner heard it: what the Decide
 * sheet opens with.
 *
 * The case's own words are the case — "a laptop sleeps with the page open" — and
 * the clause is the rule it would turn into. Where the Partner named no clause
 * the sheet opens with the case itself, because a human deciding a case they can
 * see is better served by their own words over the Partner's than by an empty
 * box (D1).
 */
export function clauseHeard(deposit: Deposit): string {
  const said = (deposit.clause ?? "").trim();
  return said === "" ? deposit.text : said;
}

/** Where a card stands, in the order a human would read them. */
export type Standing =
  | "refused"
  | "dismissed"
  | "recorded"
  | "elsewhere"
  | "recording"
  | "conflict"
  | "blocked"
  | "waiting";

/**
 * Where one card stands, against the sitting now standing.
 *
 * `sitting` is the record the sitting is on, by its path, and "" where no
 * sitting is open at all. A card offered against another record — or against a
 * sitting that has since ended — stands "elsewhere": it is not this record's to
 * record, and the press is not offered for it. That is Sol's first finding: the
 * card was offered, so the press was enabled, and the words went into whichever
 * record the sitting happened to be on when it was pressed.
 *
 * Recorded comes first because it is the truer fact: an entry that landed in A's
 * record landed there whatever happened afterwards, and a card saying so while
 * the human sits on B is not a card offering anything.
 */
export function standingOf(deposit: Deposit, mark: Mark, sitting: string): Standing {
  if (!deposit.offered) {
    return "refused";
  }
  if (mark.dismissed) {
    return "dismissed";
  }
  if (mark.recorded !== "") {
    return "recorded";
  }
  if (sitting === "" || (deposit.subject?.id ?? "") !== sitting) {
    return "elsewhere";
  }
  if (mark.recording) {
    return "recording";
  }
  if (mark.refusal !== "") {
    return "conflict";
  }
  return missing(deposit.kind, mark) === "" ? "waiting" : "blocked";
}

/**
 * Whether one card's two fields are still the human's to change.
 *
 * They are not while the press is in flight. Sol's fifth finding: the fields
 * stayed editable through the write, and the press had already composed its
 * entry from the words as they stood when it ran — so a human who kept typing
 * watched the card say "recorded" over words the record never took. Frozen, the
 * words that were submitted are the words on the card.
 *
 * They are not once the record has taken them either, nor for a card that
 * belongs to another sitting, because in both cases there is nothing left here
 * to write.
 */
export function editable(standing: Standing): boolean {
  return standing === "waiting" || standing === "blocked" || standing === "conflict";
}

/**
 * What one card still needs before it may be recorded, in the words a human
 * reads — and "" when it needs nothing.
 *
 * A decision with no reason and a fact with no anchor are the two the design
 * names, and they are judged AFTER the human's edit: the Partner may not have
 * found the anchor, and the human may know it. An open question needs neither,
 * because a question the Partner attached no consequence to is still a question
 * worth recording.
 */
export function missing(kind: string, mark: Mark): string {
  if (mark.text.trim() === "") {
    return "This says nothing yet; write the entry before recording it.";
  }
  if (kind === "decision" && mark.clause.trim() === "") {
    return "A decision is recorded with the reason you gave. Write the reason before recording it.";
  }
  if (kind === "fact" && mark.clause.trim() === "") {
    return "A fact is anchored where it can be checked. Write the anchor before recording it.";
  }
  return "";
}

/** One card: the deposit, what identifies it, and where it stands. */
export type Card = Deposit & { id: string; mark: Mark; standing: Standing };

/** One turn's deposits, as the transcript and the running turn carry them. */
export type Carried = { turn: string; deposits: readonly Deposit[] };

/**
 * Every card this conversation carries, oldest first, against the sitting now
 * standing and the record as it now reads.
 *
 * `sitting` is the record the sitting is on, and `recorded` is what that record
 * already holds by deposit, which is what says a card was recorded. A card the
 * record claims shows the record's own words: they are what a fresh worker
 * reading the record would find, and they may not be what the Partner first
 * offered or what this browser last remembered typing.
 */
export function cardsIn(
  carried: readonly Carried[],
  marks: Marks,
  sitting: string,
  recorded: ReadonlyMap<string, Entry>,
): readonly Card[] {
  const cards: Card[] = [];
  for (const one of carried) {
    one.deposits.forEach((deposit, at) => {
      const id = depositID(one.turn, at);
      const mark = asRecorded(marks[id] ?? marked(deposit), recorded.get(id));
      cards.push({ ...deposit, id, mark, standing: standingOf(deposit, mark, sitting) });
    });
  }
  return cards;
}

/** One card's mark as the record has it, where the record has it at all. */
function asRecorded(mark: Mark, entry: Entry | undefined): Mark {
  if (entry === undefined) {
    return mark;
  }
  return {
    ...mark,
    text: entry.text,
    clause: entry.clause,
    recording: false,
    recorded: entry.section,
    refusal: "",
  };
}

export function cardIn(cards: readonly Card[], id: string): Card | undefined {
  return cards.find((card) => card.id === id);
}

/**
 * What one press writes, where what it writes is not the card's own words.
 *
 * It is the case card's whole reason for existing (D1): the card carries a case,
 * and the two presses on it record something else — Decide records a decision
 * whose words are the clause and whose reason the human wrote, and Leave open
 * records an open question. So a press says what it is recording, and the entry
 * is composed from that rather than from the card.
 */
export type Records = { kind: string; text: string; clause: string };

/** What one card records by default: the words as the human now has them. */
export function recordsOf(card: Card): Records {
  return { kind: card.kind, text: card.mark.text, clause: card.mark.clause };
}

/** The entry one press would write, marked with the card that offered it. */
export function entryOf(card: Card, who: string, when: string, records?: Records): Entry {
  const said = records ?? recordsOf(card);
  return {
    when,
    who,
    text: said.text,
    clause: said.clause,
    section: sectionOf(said.kind),
    mark: card.id,
  };
}

/* --------------------------------------------------------------- the words -- */

/** What the card heads itself with. */
export function cardHead(kind: string): string {
  switch (kind) {
    case "decision":
      return "A decision";
    case "question":
      return "An open question";
    case "proposal":
      return "A proposal";
    case "case":
      return "A case at the edge";
    case "outcome":
      return "What this sitting came to";
    default:
      return "A fact";
  }
}

/* ------------------------------------------------------ the case's two presses -- */

/** The two presses a case offers, and what each one records (g1-s55 D1). */
export const DECIDE = "Decide";
export const LEAVE_OPEN = "Leave open";

/** What the Decide sheet is called, and what its two fields are. */
export const DECIDE_TITLE = "Decide this case";
export const DECIDE_CLAUSE = "The clause, as heard";
export const DECIDE_REASON = "Your reason";

/**
 * What the Decide sheet says above its two fields.
 *
 * It says the one thing the paper asks of a sitting's decisions: the words are
 * the Partner's as it heard them and the reason is the human's own, recorded
 * then and there rather than reconstructed later.
 */
export const DECIDE_SAID =
  "The clause is what your Partner heard; change it to what you mean. " +
  "The reason is yours, and it is recorded with the decision.";

/** What Leave open says it will do, so a press is not a surprise. */
export function leftOpenLine(clause: string): string {
  const said = clause.trim();
  const follows = said === "" ? "" : ` Its consequence is recorded with it: ${said}`;
  return `Records this case as an open question, in the Partner's words.${follows}`;
}

/** What a case says once the record has taken one of its two presses. */
export function caseSettledLine(where: string): string {
  return where === "Decisions"
    ? "Decided, and in the record."
    : "Left open, and in the record.";
}

/* -------------------------------------------------------------- ending it -- */

/**
 * The two ways a sitting ends (g1-s55 D2), and what each of them says.
 *
 * Ending it asks the Partner for the closing deposit and ends nothing: the
 * sitting stands, the outcome card appears, and the record takes it when the
 * human presses Record it. Ending without recording is allowed — it is a
 * human's own judgement that this sitting had no outcome worth writing — and it
 * says what it leaves behind, because the weakest thing about this design is
 * that it is easy to do by accident (g1-s55 §5).
 */
export const END_WITHOUT = "End without recording";
export const DRAFTING = "Drafting the outcome…";
export const ENDED_WITHOUT =
  "The sitting ended and no outcome was written into the record. Everything you recorded during it is still there.";
export const END_SAID =
  "Your Partner drafts what this sitting came to; you read it, edit it, and press Record it. " +
  "It becomes the record's Outcome section, and the sitting ends then.";

/** What the outcome card's press is called, and what it says afterwards. */
export const RECORD_OUTCOME = "Record it and end the sitting";

/* ------------------------------------------------------------- Ask it -- */

/** The press an open question on the table offers (g1-s55 D2). */
export const ASK_IT = "Ask it";

/**
 * The one question an open question of the table becomes in the register.
 *
 * Three things in one question, because the register's row IS one question and
 * the route takes one line: the question itself, what follows from leaving it
 * open, and the record this sitting was on. Astra's F1 is the second of those —
 * the consequence was the whole reason the table kept the question, and a row
 * that dropped it would send the question to the register stripped of the thing
 * that says why it matters.
 *
 * The source is said in the question's own words rather than as a scope, because
 * the question route scopes by ledger goal id and refuses a record path: a
 * record is not a goal, and the row would be refused on the first Ask.
 */
export function askedFromSitting(entry: Entry, record: string): string {
  const said = [oneLine(entry.text)];
  const clause = oneLine(entry.clause);
  if (clause !== "") {
    said.push(`leaving it open: ${clause}`);
  }
  said.push(`from the sitting on ${record}`);
  return said.join(" — ");
}

/**
 * The ledger goals a record's own head names, which is the scope the question
 * route takes (g1-s55 F1).
 *
 * The record's goals and not the record: a question is scoped by ledger goal id,
 * so what a question out of this sitting is about is what the record it came out
 * of says it is about. A record that names none is about the project as a whole,
 * and the question is too.
 *
 * It is read from the head, which is the list before the first section heading.
 * Fields, as the resolver reads them (internal/project/record.go:199), so a head
 * naming two goals scopes the question to both.
 */
export function goalsIn(source: string): readonly string[] {
  for (const line of source.split("\n")) {
    if (/^#{2,6}\s+/.test(line)) {
      return [];
    }
    const named = /^[-*]\s+Goals:\s*(.*)$/i.exec(line);
    if (named !== null) {
      return named[1].split(/\s+/).filter((one) => one !== "");
    }
  }
  return [];
}

/** The one press that writes. */
export const RECORD_IT = "Record it";

/** What a card says once the record has taken it. */
export function recordedLine(where: string): string {
  return `Recorded in ${where}`;
}

/** What a card says while the press is in flight. */
export const RECORDING = "Recording…";

/** What a card heads itself with when nothing was offered at all. */
export const NOT_OFFERED = "Not offered";

/**
 * What a card offered in another sitting says, and the one thing left to do with
 * it.
 *
 * It says both of the two things it can honestly be — the entry went into that
 * record, or that sitting ended before anybody pressed — because this page
 * cannot tell them apart without reading a record it is not sitting on. What it
 * can say for certain is the only thing that matters: these words are not this
 * record's to record, and here they are to copy.
 */
export const ELSEWHERE = "recorded elsewhere or its sitting ended";

/**
 * What a press is told while the sitting's reading of its record is still being
 * taken.
 *
 * A sitting starts with its opening turn already answered, so a card can be on
 * screen in the moment between the sitting's subject arriving and this page
 * having read that record. Nothing is written from no reading; what the press
 * owes the human is to say so rather than to do nothing at all.
 */
export const NOT_READ_YET = "The record is being read. Press Record it again in a moment.";

export function elsewhereLine(deposit: Deposit): string {
  const named = (deposit.subject?.id ?? "").trim();
  const whose = named === "" ? "another sitting" : named;
  return `Offered to ${whose}: ${ELSEWHERE}. Copy the words to say them in this one.`;
}

/** Why nothing was offered, as the card says it. */
export function notOfferedLine(deposit: Deposit): string {
  const said = (deposit.notOffered ?? "").trim();
  return said === "" ? "This was not offered." : said;
}

/** The folded card, in one word that presses to unfold it. */
export const DISMISSED = "dismissed";

/** What the start sheet and the drawer's control are called. */
export const START = "Start a sitting";
export const END = "End the sitting";

/**
 * What the start sheet says when the draft it created outlived the refusal.
 *
 * A sitting started on a title creates the record first, so a refusal after that
 * has left a real record in the project. The sheet says which one, and pressing
 * Start again opens the sitting on it — because the alternative is a human
 * pressing Start twice and finding two drafts of one wish.
 */
export function draftMadeLine(path: string): string {
  return (
    `The draft was created before this was refused: ${path}. ` +
    `${START} again opens the sitting on that record rather than making a second one.`
  );
}

/**
 * What the transcript says above the one question nobody typed.
 *
 * A sitting's opening turn is submitted by this interface on the human's behalf.
 * It is asked in their name, so it stands where their questions stand — and it
 * says whose it is, because a turn a human did not write and cannot tell apart
 * from one they did is the interface putting words in their mouth.
 */
export const THE_INTERFACES = "asked by the interface, to open this sitting";

/** What the table is called, and what its empty state says. */
export const TABLE = "The table";
export const TABLE_EMPTY = "Nothing has been recorded in this sitting yet.";

/** What the table says about a record nobody is sitting on. */
export const NO_SITTING = "No sitting is open.";
