import type { NewRecord } from "./api";

/**
 * What a contribution will be, worked out before it is made.
 *
 * The page a human reads in the sheet is composed here from the same grammar
 * the server writes with — the title, the four required head lines, the
 * references, and the kind's own empty sections — so the preview is the file
 * and not a picture of one. The one thing this side cannot know is the id,
 * which the server mints, and the preview says so rather than inventing a
 * number that would then be wrong.
 *
 * The rules are here rather than in the component because they are what is
 * worth arguing about, and because a rule in a component is a rule no test
 * reaches.
 */

/** The three kinds a human creates here. Doctrine is not one of them. */
const KINDS = ["decision", "design", "intent"] as const;

export type Kind = (typeof KINDS)[number];

/** The four statuses a record carries, in the order they are lived. */
export const STATUSES = ["draft", "accepted", "superseded", "done"];

/** What each kind opens with, empty, for a human to write into. */
const SECTIONS: Readonly<Record<Kind, string[]>> = {
  intent: ["Users", "Outcomes", "Constraints", "Open questions"],
  decision: ["Context", "Decision", "Consequences"],
  design: ["Outcome", "Scope", "What changes", "Verification"],
};

/** The home each kind is written in, relative to the project's state root. */
const HOMES: Readonly<Record<Kind, string>> = {
  intent: "docs/intent",
  decision: "docs/decisions",
  design: "plans/designs",
};

/** What each action is called, where it is offered and where it is confirmed. */
export const ACTIONS: Readonly<Record<Kind, { offer: string; eyebrow: string; confirm: string }>> = {
  decision: { offer: "Record a decision", eyebrow: "New record · Decision", confirm: "Create draft" },
  design: { offer: "New design", eyebrow: "New record · Design", confirm: "Create draft" },
  intent: { offer: "Write the intent chapter", eyebrow: "New record · Intent", confirm: "Create draft" },
};

/** The id line's stand-in, which says what will be there instead of guessing. */
const MINTED = "(a fresh id, minted when this is written)";

/** How long a file name this makes. The title is where the words live. */
const SLUG_LIMIT = 60;

/**
 * The file name a title yields: lower case, a hyphen for everything that is
 * not an unaccented letter or a digit, runs of hyphens collapsed, the ends
 * trimmed, and at most SLUG_LIMIT characters.
 *
 * It is the server's rule, written again here so that the sheet can say where
 * the file will land before it exists. The server decides; this agrees.
 */
export function slugOf(title: string): string {
  let name = "";
  for (const character of title.toLowerCase()) {
    if (/[a-z0-9]/.test(character)) {
      name += character;
    } else if (!name.endsWith("-")) {
      name += "-";
    }
  }
  return trimHyphens(trimHyphens(name).slice(0, SLUG_LIMIT));
}

function trimHyphens(value: string): string {
  let from = 0;
  let to = value.length;
  while (from < to && value[from] === "-") {
    from += 1;
  }
  while (to > from && value[to - 1] === "-") {
    to -= 1;
  }
  return value.slice(from, to);
}

/** A draft of a record, as the sheet holds it while a human fills it in. */
export type Draft = { kind: Kind; title: string; goals: string[]; affects: string[]; cites: string[] };

/**
 * A draft of one kind, about one goal where the page it was opened from is a
 * goal's, and about the project as a whole otherwise. Goals are optional
 * throughout: a record that names none is about the whole.
 */
export function emptyDraft(kind: Kind, goal: string | null): Draft {
  return { kind, title: "", goals: goal === null ? [] : [goal], affects: [], cites: [] };
}

export function asked(draft: Draft): NewRecord {
  return {
    kind: draft.kind,
    title: draft.title.trim().replace(/\s+/g, " "),
    goals: draft.goals,
    affects: draft.affects,
    cites: draft.cites,
  };
}

/**
 * The page as it will be written. The head's order is the memory system's: the
 * three required keys, then the goals where the record names any, then the
 * references, in the order the resolver reads them.
 *
 * A record about the project as a whole carries no Goals line at all, because
 * an empty key would declare nothing — the server writes it the same way.
 */
export function pageFor(draft: Draft): string {
  const title = draft.title.trim().replace(/\s+/g, " ");
  const head = [`- Kind: ${draft.kind}`, `- Id: ${MINTED}`, "- Status: draft"];
  if (draft.goals.length > 0) {
    head.push(`- Goals: ${draft.goals.join(" ")}`);
  }
  if (draft.cites.length > 0) {
    head.push(`- Cites: ${draft.cites.join(" ")}`);
  }
  if (draft.affects.length > 0) {
    head.push(`- Affects: ${draft.affects.join(" ")}`);
  }
  const body = SECTIONS[draft.kind].map((section) => `\n## ${section}\n`).join("");
  return `# ${title}\n\n${head.join("\n")}\n${body}`;
}

/** Where the file will land, relative to the project's own state root. */
export function fileFor(draft: Draft): string {
  const name = slugOf(draft.title);
  return `${HOMES[draft.kind]}/${name === "" ? "…" : name}.md`;
}

/**
 * The one line under the buttons: what will be written, and where. It is the
 * whole promise the sheet makes, in a sentence, so that a human confirming has
 * read the consequence rather than trusted the button.
 */
export function noteFor(draft: Draft): string {
  const chapter =
    draft.kind === "intent" ? " It is listed in the intent index's reading order." : "";
  return `Writes ${fileFor(draft)} with a fresh id and Status: draft, then opens it.${chapter}`;
}

/** What the sheet says a status change will do, in the same voice. */
export function statusNote(path: string, status: string): string {
  return `Rewrites the Status line of ${path} to ${status}, and no other byte of the file.`;
}

/** What is true of a draft before it can be written, said as one sentence. */
export function incomplete(draft: Draft): string {
  if (draft.title.trim() === "") {
    return "A record needs a title.";
  }
  if (slugOf(draft.title) === "") {
    return "This title yields no file name; it needs a letter or a digit.";
  }
  return "";
}
