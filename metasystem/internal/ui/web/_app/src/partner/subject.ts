import type { Subject } from "../shell/about";

/**
 * What "this" means, and how long it keeps meaning it.
 *
 * Astra's third finding is what this file answers. A human chooses Ask on goal
 * A, opens its design from the answer, and asks "what would move it?" — and
 * before this slice there was no rule saying whether "it" was A, the design,
 * or whatever page happened to be mounted. The old context was derived from
 * the mounted pane and disappeared with it.
 *
 * The rule is one sentence: the subject follows the page until you choose one,
 * and a chosen subject stays until you clear it or choose another. Navigating
 * does not change it. Expanding the drawer into the focused page keeps it.
 * The drawer and the focused page read the same one, because there is one
 * conversation.
 */

/** The kinds of thing this slice can be about. */
export type ChosenKind = "goal" | "record" | "document" | "lane" | "overview" | "passage";

/**
 * One chosen subject: what it is, how to name it, where it came from, and what
 * the pinned panel shows of it.
 *
 * It carries its own summary rather than a way to look one up, because the
 * panel has to agree with what the question carries: the two are composed from
 * the same values at the moment of choosing, so a board that has moved since
 * cannot make them disagree.
 */
export type Chosen = {
  kind: ChosenKind;
  /** The identity: a goal id, a record's path, a lane id, an item's id. */
  id: string;
  /** What a human calls it, which is what the chip shows. */
  title: string;
  /** The reading it was taken from, as the pinned panel says it. */
  source: string;
  /** Its summary, as the page that offered it shows it. */
  summary: string;
  /** Where it opens, where this build has a page for it. */
  to?: string;
  /**
   * Where the page this was chosen from shows it: the board's own address with
   * this goal named, for instance. A message chip returns here rather than to
   * the subject's own page, because the question was asked from the page and
   * "these" means what that page was showing.
   */
  at?: string;
  /** A document's revision, where the subject carries one. */
  revision?: string;
  /** The passage itself, on a selection. */
  quote?: string;
  /** The heading the passage sits under, which is what a chip returns to. */
  anchor?: string;
};

/** What the chip says for a chosen subject: its kind and what it is. */
export function chipLabel(chosen: Chosen): string {
  return `${kindWord(chosen.kind)} ${chosen.title === "" ? chosen.id : chosen.title}`;
}

/** The word a human reads for each kind. */
export function kindWord(kind: ChosenKind): string {
  switch (kind) {
    case "goal":
      return "Goal";
    case "record":
      return "Record";
    case "document":
      return "Document";
    case "lane":
      return "Lane";
    case "overview":
      return "Item";
    case "passage":
      return "Passage";
  }
}

/**
 * The subject a question is about: the one chosen, else the page's own.
 *
 * A page that is about nothing in particular — the board, the landing page —
 * contributes no subject, and the question is then about the page, which is
 * what the block already says.
 */
export function effectiveSubject(chosen: Chosen | null, page: Subject): Chosen | null {
  if (chosen !== null) {
    return chosen;
  }
  if (page.subject === undefined || page.subject === "") {
    return null;
  }
  return {
    kind: page.kind === "goal" ? "goal" : "document",
    id: page.subject,
    title: page.title ?? page.subject,
    source: page.revision === undefined || page.revision === ""
      ? "the accepted tip this page rendered from"
      : `${page.subject}, revision ${page.revision}`,
    summary: "",
    revision: page.revision,
    to: page.returnTo,
  };
}

/**
 * Which register of suggested questions a subject asks for. A page with no
 * subject asks by section, which is what the board and the landing page are.
 */
export function registerFor(chosen: Chosen | null, section: string): string {
  if (chosen !== null) {
    return chosen.kind === "record" || chosen.kind === "document" ? documentRegister(chosen) : chosen.kind;
  }
  switch (section) {
    case "Backlog":
      return "board";
    case "Overview":
      return "overview";
    case "Project":
      return "project";
    default:
      return "page";
  }
}

/**
 * A record's register follows what kind of record it is: a design and a
 * decision are asked different questions, and both are documents.
 */
function documentRegister(chosen: Chosen): string {
  const path = chosen.id.toLowerCase();
  if (path.includes("/designs/") || path.includes("design")) {
    return "design";
  }
  if (path.includes("/decisions/") || path.includes("decision")) {
    return "decision";
  }
  return "document";
}
