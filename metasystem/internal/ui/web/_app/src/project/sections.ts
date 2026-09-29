/**
 * One section of a record, found by its heading and replaced whole (g1-s66 D3),
 * and the two things a goal opened from a design takes from its source (D5).
 *
 * A heading identifies a section only when it occurs exactly once. The Outcome
 * writer takes the first match and appends where there is none, which is right
 * for a section every design is created with; a finding folded into a design
 * names a section a human wrote, and a heading that is absent, or written twice,
 * is a section this page cannot tell apart. So it is refused in words and the
 * draft is kept, and nothing is written. The writer checks the document's
 * revision and not the section, which is why a conflict re-reads the section
 * and shows the comparison again before offering Use (DocumentPane).
 *
 * A section runs from its heading to the next heading of its own level or
 * higher, as the Outcome's does, so its own sub-headings go with it; a heading
 * inside a code fence is text and not a heading.
 */

export type Found =
  | { state: "found"; at: number; end: number; level: number; text: string }
  | { state: "absent" }
  | { state: "duplicate"; count: number };

export type Replaced = { state: "replaced"; source: string } | { state: "refused"; said: string };

type Heading = { at: number; level: number; text: string };

const HEADING = /^(#{1,6})\s+(.*?)\s*#*\s*$/;
const FENCE = /^\s{0,3}(`{3,}|~{3,})/;

/** Every heading outside a code fence, in order. */
function headingLines(lines: readonly string[]): Heading[] {
  const found: Heading[] = [];
  let fence = "";
  lines.forEach((line, at) => {
    const marker = FENCE.exec(line);
    if (marker !== null) {
      if (fence === "") {
        fence = marker[1];
      } else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) {
        fence = "";
      }
      return;
    }
    if (fence !== "") {
      return;
    }
    const heading = HEADING.exec(line);
    if (heading !== null) {
      found.push({ at, level: heading[1].length, text: heading[2].trim() });
    }
  });
  return found;
}

/** The headings a finding may be folded into, as the document spells them. */
export function headingsOf(source: string): string[] {
  return headingLines(source.split("\n")).map((heading) => heading.text);
}

/** The one section under this heading, or why there is none to replace. */
export function sectionIn(source: string, heading: string): Found {
  const lines = source.split("\n");
  const all = headingLines(lines);
  const named = all.filter((one) => one.text === heading.trim());
  if (named.length === 0) {
    return { state: "absent" };
  }
  if (named.length > 1) {
    return { state: "duplicate", count: named.length };
  }
  const start = named[0];
  const next = all.find((one) => one.at > start.at && one.level <= start.level);
  const end = next === undefined ? lines.length : next.at;
  return { state: "found", at: start.at, end, level: start.level, text: lines.slice(start.at, end).join("\n") };
}

/** Why a heading cannot be replaced, in the words the section card shows. */
export function refusalFor(heading: string, found: Found): string {
  if (found.state === "absent") {
    return `The heading “${heading}” is not in this document as it stands; nothing was written and the draft is kept.`;
  }
  if (found.state === "duplicate") {
    return `The heading “${heading}” occurs ${String(found.count)} times in this document, so the section cannot be told apart; nothing was written and the draft is kept.`;
  }
  return "";
}

/**
 * The whole source with exactly that section replaced by the draft.
 *
 * The draft is the section's body anew under the document's own heading line:
 * a heading the draft opens with is the Partner restating the section and is
 * not written, so a draft that opens under another heading cannot remove this
 * section's heading or write a second copy of another's. A Partner that sent
 * the body alone keeps the heading too. The blank lines between this section
 * and the next are the gap between sections and are kept.
 */
export function sectionReplaced(source: string, heading: string, draft: string): Replaced {
  const found = sectionIn(source, heading);
  if (found.state !== "found") {
    return { state: "refused", said: refusalFor(heading, found) };
  }
  const lines = source.split("\n");
  let last = found.end;
  while (last > found.at + 1 && lines[last - 1].trim() === "") {
    last -= 1;
  }
  let body = draft.replace(/^\n+/, "").replace(/\s+$/, "");
  if (HEADING.test(body.split("\n")[0])) {
    body = body.replace(/^[^\n]*/, "").replace(/^\s+/, "");
  }
  const written = body === "" ? [lines[found.at]] : [lines[found.at], "", ...body.split("\n")];
  return { state: "replaced", source: [...lines.slice(0, found.at), ...written, ...lines.slice(last)].join("\n") };
}

/** One line of the comparison, marked where it changed. */
export type Compared = { text: string; changed: boolean };

/**
 * Old and new, line by line, with the lines that changed marked on each side:
 * a line is unchanged where it belongs to the longest run the two have in
 * common, and changed everywhere else.
 */
export function changedLines(before: string, after: string): { old: Compared[]; new: Compared[] } {
  const left = before.split("\n");
  const right = after.split("\n");
  const common: number[][] = Array.from({ length: left.length + 1 }, () => new Array<number>(right.length + 1).fill(0));
  for (let i = left.length - 1; i >= 0; i -= 1) {
    for (let j = right.length - 1; j >= 0; j -= 1) {
      common[i][j] = left[i] === right[j] ? common[i + 1][j + 1] + 1 : Math.max(common[i + 1][j], common[i][j + 1]);
    }
  }
  const kept = { old: new Set<number>(), new: new Set<number>() };
  let i = 0;
  let j = 0;
  while (i < left.length && j < right.length) {
    if (left[i] === right[j]) {
      kept.old.add(i);
      kept.new.add(j);
      i += 1;
      j += 1;
    } else if (common[i + 1][j] >= common[i][j + 1]) {
      i += 1;
    } else {
      j += 1;
    }
  }
  return {
    old: left.map((text, at) => ({ text, changed: !kept.old.has(at) })),
    new: right.map((text, at) => ({ text, changed: !kept.new.has(at) })),
  };
}

/**
 * The intent a goal opened from a design starts on: the first paragraph of its
 * Outcome, folded onto the one line the ledger keeps an intent on. A design
 * with no Outcome, or an empty one, has none to offer.
 */
export function outcomeParagraph(source: string): string {
  const found = sectionIn(source, "Outcome");
  if (found.state !== "found") {
    return "";
  }
  const paragraph: string[] = [];
  for (const line of found.text.split("\n").slice(1)) {
    if (line.trim() === "") {
      if (paragraph.length > 0) {
        break;
      }
      continue;
    }
    if (HEADING.test(line)) {
      break;
    }
    paragraph.push(line.trim());
  }
  return paragraph.join(" ");
}

/**
 * The next step a goal opened from a design starts on: continue from the record,
 * with the section that holds step 1 named where the design has one.
 */
export function nextStepFor(path: string, source: string): string {
  const step = headingsOf(source).find((heading) => /\bstep 1\b/i.test(heading));
  const number = step === undefined ? "" : (/^(\d+)\./.exec(step)?.[1] ?? "");
  return number === ""
    ? `Continue from the record ${path}: build step 1 as it says`
    : `Continue from the record ${path}: build step 1 as its §${number} says`;
}
