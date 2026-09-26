import type { Deposit, Sitting } from "./api";

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
 */

/* ------------------------------------------------------- the four sections -- */

/** The four sections of a sitting's record, in the order the table shows them. */
export const SECTIONS = ["Facts", "Proposals", "Decisions", "Open questions"] as const;

export type Section = (typeof SECTIONS)[number];

/** Which section each kind of deposit is written into. */
const SECTION_OF: Readonly<Record<string, Section>> = {
  fact: "Facts",
  proposal: "Proposals",
  decision: "Decisions",
  question: "Open questions",
};

export function sectionOf(kind: string): Section {
  return SECTION_OF[kind] ?? "Facts";
}

/** What each kind calls the clause beside its words, as the card labels it. */
const CLAUSE_OF: Readonly<Record<string, string>> = {
  fact: "Anchor",
  decision: "Reason",
  question: "Consequence",
  proposal: "Consequence",
};

export function clauseOf(kind: string): string {
  return CLAUSE_OF[kind] ?? "Anchor";
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
  /** Which of the four sections it stands in. */
  section: Section;
};

/** The separator between an entry's dated head and its words. */
const DOT = " · ";

/** How one entry is written into a section. */
export function lineOf(entry: Entry, kind: string): string {
  const head = `- ${entry.when}${DOT}${entry.who}${DOT}${oneLine(entry.text)}`;
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
    const parts = item[1].split(DOT);
    const entry: Entry =
      parts.length >= 3
        ? {
            when: parts[0].trim(),
            who: parts[1].trim(),
            text: parts.slice(2).join(DOT).trim(),
            clause: "",
            section,
          }
        : { when: "", who: "", text: item[1].trim(), clause: "", section };
    found.push(entry);
    last = entry;
  }
  return found;
}

/** How many entries each section holds, which is what the drawer's strip says. */
export type Counts = Readonly<Record<Section, number>>;

export function countsIn(source: string): Counts {
  const counted: Record<Section, number> = { Facts: 0, Proposals: 0, Decisions: 0, "Open questions": 0 };
  for (const entry of entriesIn(source)) {
    counted[entry.section] += 1;
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
      return deposit.consequence ?? "";
    default:
      return deposit.anchor ?? "";
  }
}

/** Where a card stands, in the order a human would read them. */
export type Standing = "refused" | "dismissed" | "recorded" | "recording" | "conflict" | "blocked" | "waiting";

export function standingOf(deposit: Deposit, mark: Mark): Standing {
  if (!deposit.offered) {
    return "refused";
  }
  if (mark.dismissed) {
    return "dismissed";
  }
  if (mark.recorded !== "") {
    return "recorded";
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

/** Every card this conversation carries, oldest first. */
export function cardsIn(carried: readonly Carried[], marks: Marks): readonly Card[] {
  const cards: Card[] = [];
  for (const one of carried) {
    one.deposits.forEach((deposit, at) => {
      const id = depositID(one.turn, at);
      const mark = marks[id] ?? marked(deposit);
      cards.push({ ...deposit, id, mark, standing: standingOf(deposit, mark) });
    });
  }
  return cards;
}

export function cardIn(cards: readonly Card[], id: string): Card | undefined {
  return cards.find((card) => card.id === id);
}

/** The entry one card would write, from the words as the human now has them. */
export function entryOf(card: Card, who: string, when: string): Entry {
  return {
    when,
    who,
    text: card.mark.text,
    clause: card.mark.clause,
    section: sectionOf(card.kind),
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
    default:
      return "A fact";
  }
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
