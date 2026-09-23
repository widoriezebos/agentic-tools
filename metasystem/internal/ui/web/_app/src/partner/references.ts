import type { Index } from "./api";
import { backlogPath, documentPath } from "../routes";

/**
 * Turning the names in an answer into links, and only the ones that point at
 * one thing.
 *
 * Astra's eighth finding is the rule: stable ids link, and a title links only
 * when it names one record and not three. An answer that mentions six goals is
 * six links and no rings until one of them is hovered; hovering never
 * navigates and never clears a filter; and a name this workspace does not
 * carry stays text, because a link that refuses is worse than no link.
 *
 * The index the resolution is made against rides the conversation's own
 * snapshot, so nothing here reaches the network and nothing here is a second
 * account of what the workspace holds.
 */

/** One resolved reference: what it is, what it is called, and where it opens. */
export type Reference = {
  kind: "goal" | "record";
  /** The identity: a goal's ledger id, or a record's checkout-relative path. */
  id: string;
  /** The words that were written, which is what the link reads as. */
  text: string;
  to: string;
};

/** A run of an answer: plain words, or one reference. */
export type Run = { text: string } | { reference: Reference };

/** One name this workspace answers to, and what it names. */
type Named = { name: string; reference: Reference };

/**
 * Every name, grouped by its first character and longest first inside each
 * group.
 *
 * Grouped, because a conversation is re-rendered whenever a beat arrives and a
 * workspace carries hundreds of names: without it every character of every
 * answer would be compared against every name. Longest first, because the scan
 * takes the first match at a position, so the order inside a group is the
 * whole of the precedence — a path wins over the record id inside it.
 */
export type Names = Map<string, Named[]>;

export function namesIn(index: Index): Names {
  const named: Named[] = [];
  for (const goal of index.goals ?? []) {
    if (goal === "") {
      continue;
    }
    named.push({ name: goal, reference: { kind: "goal", id: goal, text: goal, to: backlogPath(goal) } });
  }
  const records = index.records ?? [];
  // A title names one record or it names none: two records called "Reading"
  // are two things an answer's own words cannot tell apart, so neither links.
  const titles = new Map<string, number>();
  for (const record of records) {
    const title = (record.title ?? "").trim();
    if (title !== "") {
      titles.set(title, (titles.get(title) ?? 0) + 1);
    }
  }
  for (const record of records) {
    const reference: Reference = {
      kind: "record",
      id: record.path,
      text: record.path,
      to: documentPath(record.path),
    };
    named.push({ name: record.path, reference });
    if (record.id !== undefined && record.id !== "") {
      named.push({ name: record.id, reference: { ...reference, text: record.id } });
    }
    const title = (record.title ?? "").trim();
    if (title !== "" && titles.get(title) === 1) {
      named.push({ name: title, reference: { ...reference, text: title } });
    }
  }
  const grouped: Names = new Map();
  for (const one of named) {
    const first = one.name[0];
    const group = grouped.get(first);
    if (group === undefined) {
      grouped.set(first, [one]);
      continue;
    }
    group.push(one);
  }
  for (const group of grouped.values()) {
    group.sort((left, right) => right.name.length - left.name.length);
  }
  return grouped;
}

/**
 * Split one run of text into words and references.
 *
 * A name matches only where it stands on its own: a goal called `run` must not
 * light up inside "running", and a path must not match inside a longer one. So
 * the character before and after a match has to be one no name can contain.
 */
export function runsIn(text: string, names: Names): Run[] {
  const runs: Run[] = [];
  let plain = "";
  let at = 0;
  while (at < text.length) {
    const found = boundary(text[at - 1]) ? matchAt(text, at, names) : null;
    if (found === null) {
      plain += text[at];
      at += 1;
      continue;
    }
    if (plain !== "") {
      runs.push({ text: plain });
      plain = "";
    }
    runs.push({ reference: { ...found.reference, text: found.name } });
    at += found.name.length;
  }
  if (plain !== "") {
    runs.push({ text: plain });
  }
  return runs;
}

function matchAt(text: string, at: number, names: Names): Named | null {
  for (const candidate of names.get(text[at]) ?? []) {
    if (text.startsWith(candidate.name, at) && endsAt(text, at + candidate.name.length)) {
      return candidate;
    }
  }
  return null;
}

/**
 * True at the start of a name: the start of the text, or a character no id,
 * path or title of this workspace runs through.
 */
function boundary(character: string | undefined): boolean {
  return character === undefined || !/[A-Za-z0-9_\-./]/.test(character);
}

/**
 * True at the end of a name.
 *
 * It is the same rule with one exception, and the exception is the full stop:
 * a path runs through one and a sentence ends with one. So a full stop ends a
 * name when nothing follows it that a name could run into — which links
 * "g1-s26." at the end of a sentence and leaves "docs/reading.md.bak" alone.
 */
function endsAt(text: string, at: number): boolean {
  const character = text[at];
  if (character === ".") {
    return boundary(text[at + 1]);
  }
  return boundary(character);
}
