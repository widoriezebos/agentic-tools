import type {
  Book,
  Chapter,
  DocumentFile,
  DocumentPayload,
  Goal,
  Pane,
  ProjectRecord,
  Question,
  SittingRow,
} from "./api";
import { ACTIONS, ASK, type Kind } from "./writing";
import type { HelpId } from "../help/terms";
import { documentPath, goalPath } from "../routes";

/**
 * What the briefing shows, and what the reader shows around one document,
 * worked out from what the server answered.
 *
 * The rules are here rather than in the components so that the ones worth
 * arguing about are readable and tested: what a book's own words are, which
 * records a goal shows, how designs group by status, which records are a
 * record's siblings, and where previous and next go. Nothing here fetches, and
 * nothing here invents a row the payload did not carry.
 */

/** One line of a section: a title that may open, and the facts beside it. */
export type Row = {
  key: string;
  title: string;
  /** Where the title opens, or null for a row this build cannot open. */
  to: string | null;
  /** The record's own first words, or "" where it has none. */
  summary: string;
  status: string;
  /**
   * The ledger goals this row's record names, as the head wrote them. Empty
   * is the project as a whole, which is a scope and not a missing field: the
   * row carries what the record says about itself, and the page decides what
   * to draw from it.
   */
  goals: string[];
  /**
   * One quiet clause beside the status, or "" where the row has none. A
   * design's is how much of the work it names has landed; nothing else has one
   * yet.
   */
  note: string;
  /** The checkout-relative path, shown in mono and never wrapped. */
  path: string;
};

/**
 * One tab of the strip under the header: a section of this page, by the name
 * the address opens it under, and the term that explains it.
 *
 * The same four words head both pages, and they do not mean the same thing on
 * both: Decisions under Project is everything this project has decided, and
 * Decisions on a goal page is the handful whose Goals line names that goal. So
 * the explanation is chosen where the strip is built, not from the tab's name.
 */
export type PageSection = { id: string; title: string; help?: HelpId };

/** One collapsed run of the checkout's other documents. */
export type DocumentGroup = { id: string; title: string; files: DocumentFile[] };

/** One entry of a book's table of contents, titled as the index wrote it. */
export type TocEntry = { key: string; title: string; to: string | null; summary: string };

/** A book as the briefing opens it: in its own words, then its reading order. */
export type BookBriefing = {
  id: string;
  title: string;
  /** Where the whole book is read, or null where the home has no index. */
  to: string | null;
  status: string;
  /** The index's first paragraph, which is what the book says it is. */
  lede: string;
  chapters: TocEntry[];
};

/** The designs: what governs, open; what is finished, in one run each. */
export type DesignRuns = { open: Row[]; runs: { status: string; rows: Row[] }[] };

/**
 * One goal's slice plan, as the records that have one record it.
 *
 * There is no slice-plan owner in the engine: the repository has slice
 * admission and a first-slicing marker, and the master says a browser-only
 * checklist cannot stand in for the missing owner. So this is a read of two
 * things that do exist — the goal's own slicing boundary, and the list each
 * governing design writes under a Slices heading — and it is read-only for
 * exactly that reason.
 */
export type SlicePlan = {
  /** When slicing started and which seat started it, or null. */
  started: { at: string; machine: string; lineage: string } | null;
  /** Each design that lists slices, with what it lists and where it is. */
  designs: { key: string; title: string; to: string; status: string; slices: string[] }[];
};

/** What the tab says when no governing design records a plan. */
export const NO_SLICE_PLAN = "No slice plan is recorded; the slice-plan owner arrives with gate 5 (master)";

/**
 * The block a goal page opens with: the goal's own id, its state and the
 * ledger's own reason for it. A goal the ledger does not carry is said to be
 * missing rather than invented.
 */
export type GoalBriefing = { id: string; title: string; state: string; intent: string; found: boolean; count: number };

/** The aside's first block: what in this scope is waiting for a human. */
export type NeedsYou = { questions: number; designs: number };

/** The aside's second block: the size and health of what was read. */
export type CheckoutFacts = { records: number; homes: number; goals: number; problems: number };

/**
 * One kind of record counted by what it is about: the project's own — the
 * ones whose head names no goal — and the ones that name at least one.
 *
 * A record naming three goals is one record here and not three: this counts
 * records, and the grouping below is where one record stands under each goal
 * it names.
 */
export type ScopeCount = { own: number; underGoals: number };

/** The three kinds the scope control narrows, each counted both ways. */
export type ScopeCounts = { decisions: ScopeCount; designs: ScopeCount; questions: ScopeCount };

/** Every row of the three narrowed tabs, whatever the scope says: Find's reach. */
export type Everything = { decisions: Row[]; designs: Row[]; questions: Row[] };

export type Briefing = {
  /** The goal this briefing is scoped to, or null for the whole project. */
  goal: GoalBriefing | null;
  /** The two books, opened in full. Empty under a goal: they are the project's. */
  books: BookBriefing[];
  decisions: Row[];
  designs: DesignRuns;
  questions: Row[];
  /** The goal's slice plan. Null for the project, which has no one plan. */
  slices: SlicePlan | null;
  needsYou: NeedsYou;
  checkout: CheckoutFacts;
  /**
   * What the scope control narrowed away, still here.
   *
   * Find crosses every scope, so the box has to be able to search rows the
   * control is not showing; and a goal page's tail line names how many
   * project-wide records of its kind there are, which is a fact about the
   * project rather than about the goal. Both read from here, so neither
   * re-derives a selection this module already made.
   */
  across: Everything;
  /** The three kinds counted by scope, over the whole project either way. */
  scopes: ScopeCounts;
};

/**
 * Which records the Project page is showing.
 *
 * Scope is a filter with a sensible default and not two lists: the project's
 * own records are the small set that shapes everything, the ones under goals
 * are one control away, and both are always found by Find. A goal page has no
 * scope at all — it is one goal's records by definition.
 */
export type ScopeFilter = "project" | "goals" | "all";

/** The scope a page opens on when the browser remembers nothing. */
export const DEFAULT_SCOPE: ScopeFilter = "project";

/** The control, in the order it is offered, with the word each choice reads by. */
export const SCOPES: readonly { id: ScopeFilter; title: string }[] = [
  { id: "project", title: "Project" },
  { id: "goals", title: "Goals" },
  { id: "all", title: "All" },
];

/** What the control is called, for anyone who cannot see that it is a control. */
export const SCOPE_LABEL = "What this page is showing";

/** A remembered value that is not one of the three is no preference at all. */
export function scopeOf(value: string | null): ScopeFilter {
  return SCOPES.some((scope) => scope.id === value) ? (value as ScopeFilter) : DEFAULT_SCOPE;
}

/** The two books, in reading order, with the words the briefing calls them. */
const BOOKS: readonly { id: string; kind: string; title: string; word: string }[] = [
  { id: "intent", kind: "intent", title: "Intent", word: "intent" },
  { id: "doctrine", kind: "doctrine", title: "Doctrine", word: "doctrine" },
];

/** The statuses a design is finished in, each collapsed into its own run. */
export const FINISHED = ["done", "superseded"];

/**
 * What the two sections no kind names are called. The tab and the block read
 * from the same word, so the strip and the heading beneath it cannot drift
 * apart; the other sections take their word from kindTitle.
 */
export const QUESTIONS_TITLE = "Open questions";
export const DOCUMENTS_TITLE = "Documents";
export const SLICES_TITLE = "Slices";
export const SITTINGS_TITLE = "Sittings";

/** What those sections are called in the address, and in the strip. */
export const QUESTIONS_TAB = "questions";
export const DOCUMENTS_TAB = "documents";
export const SLICES_TAB = "slices";
export const SITTINGS_TAB = "sittings";

/** The term that explains each book. A book with none carries no help. */
const BOOK_HELP: Readonly<Record<string, HelpId>> = { intent: "intent", doctrine: "doctrine" };

/**
 * The strip under the header: this page's own sections, in the order the page
 * offers them, each named by the word the address opens it under.
 *
 * It is derived from the briefing rather than written twice, so a section the
 * payload does not produce — a book on a goal page, the checkout's documents
 * anywhere but the project — is absent from the strip for the same reason it
 * is absent from the page. The ledger's goals are not here at all: a goal is
 * the Backlog's, and the tabs of a page are what is on that page.
 *
 * The goal itself is not a tab either. It is what a goal page is about rather
 * than one of the things on it, so it stands above the strip where the page's
 * own title would, and every tab beneath it is scoped to it.
 */
export function pageSections(briefing: Briefing): PageSection[] {
  const rows: PageSection[] = [];
  const goal = briefing.goal !== null;
  for (const book of briefing.books) {
    rows.push({ id: book.id, title: book.title, help: BOOK_HELP[book.id] });
  }
  rows.push({
    id: tabForKind("decision"),
    title: kindTitle("decision"),
    help: goal ? "goal-decisions" : "decisions",
  });
  rows.push({ id: tabForKind("design"), title: kindTitle("design"), help: goal ? "goal-designs" : "designs" });
  rows.push({ id: QUESTIONS_TAB, title: QUESTIONS_TITLE, help: goal ? "goal-questions" : "questions" });
  if (briefing.goal === null) {
    // Sittings are the project's and not a goal's: a sitting is on one record,
    // and the records it is on are read from the records themselves rather than
    // from a ledger goal. A goal page narrows records by what they say they are
    // about, and a sitting says nothing about a goal at all.
    rows.push({ id: SITTINGS_TAB, title: SITTINGS_TITLE, help: "sittings" });
    rows.push({ id: DOCUMENTS_TAB, title: DOCUMENTS_TITLE, help: "documents" });
  } else {
    // Slices are a goal's and only a goal's: the project as a whole has no
    // one plan, and a tab that showed every design's list at once would be a
    // list of lists rather than a plan.
    rows.push({ id: SLICES_TAB, title: SLICES_TITLE, help: "slices" });
  }
  return rows;
}

/**
 * The tab a kind's records are on, so that an action offered beside the
 * reading lands where what it creates will appear, and so that the strip and
 * the action cannot name that tab differently. A kind this page has no
 * section for answers with the empty string, which no strip carries.
 */
const KIND_TABS: Readonly<Record<string, string>> = {
  intent: "intent",
  doctrine: "doctrine",
  decision: "decisions",
  design: "designs",
};

export function tabForKind(kind: string): string {
  return KIND_TABS[kind] ?? "";
}

/**
 * The one act a tab offers: what the button is called, and which kind it
 * writes. A null kind is the register's own act, which has no kind because a
 * question is a row rather than a page.
 */
export type NewAction = { label: string; kind: Kind | null };

/**
 * The act at the trailing end of the strip, on the tab that is open.
 *
 * There is one, and it belongs to what is being read: on Decisions it writes a
 * decision, on Doctrine a chapter of the doctrine. A column of four buttons
 * standing beside every tab offered three things the reader was not looking at
 * and, at project level, offered to file them under a goal — which is the
 * confusion this replaces.
 *
 * Documents and Slices offer nothing: the first is the checkout's other
 * Markdown, which this surface does not write, and the second is read out of
 * the designs that record a plan, which is where a slice is added.
 */
export function newActionFor(tab: string): NewAction | null {
  switch (tab) {
    case tabForKind("intent"):
      return { label: ACTIONS.intent.offer, kind: "intent" };
    case tabForKind("doctrine"):
      return { label: ACTIONS.doctrine.offer, kind: "doctrine" };
    case tabForKind("decision"):
      return { label: ACTIONS.decision.offer, kind: "decision" };
    case tabForKind("design"):
      return { label: ACTIONS.design.offer, kind: "design" };
    case QUESTIONS_TAB:
      return { label: ASK, kind: null };
    default:
      return null;
  }
}

/** What the strip's refresh control is called, for anyone who sees an icon. */
export const REFRESH = "Refresh";

/**
 * What stands at the trailing end of the strip on one tab, in the order it is
 * met: the act this tab offers, where it has one, and then the refresh.
 *
 * The refresh is on every tab, because reading the checkout again is a thing
 * to do whichever section is open. It used to be a button on a line of its own
 * above the strip, which put it at the very top of the scrolling region where
 * the work area's own edge clipped it; the strip is sticky, so here it stays
 * on the screen instead. Nothing refetches on its own: this control and a
 * fresh mount are the only two ways this pane reads again.
 */
export function stripActions(tab: string): { newAction: NewAction | null; refresh: string } {
  return { newAction: newActionFor(tab), refresh: REFRESH };
}

/** What each tab has none of, in the words its own empty section says it in. */
const NOTHING: Readonly<Record<string, string>> = {
  intent: "chapters",
  doctrine: "chapters",
  decisions: "decisions",
  designs: "designs",
  [QUESTIONS_TAB]: "open questions",
  [SITTINGS_TAB]: "sittings",
};

/**
 * What an empty tab says, which is what it has none of rather than that
 * something is missing. On a goal page it says so of that goal: a project with
 * four hundred decisions and none about this goal has not "recorded nothing".
 */
export function nothingLine(tab: string, goal: boolean): string {
  const word = NOTHING[tab];
  if (word === undefined) {
    return "";
  }
  return goal ? `No ${word} about this goal yet.` : `No ${word} yet.`;
}

/** True when a selection shows this record. A null id is everything. */
function selected(goals: string[], id: string | null): boolean {
  return id === null || goals.includes(id);
}

/** True when this scope shows a record whose head names these goals. */
function inScope(goals: readonly string[], scope: ScopeFilter): boolean {
  switch (scope) {
    case "project":
      return goals.length === 0;
    case "goals":
      return goals.length > 0;
    default:
      return true;
  }
}

/** The rows of one list this scope shows, keeping the list's own order. */
export function narrowed(rows: readonly Row[], scope: ScopeFilter): Row[] {
  return rows.filter((row) => inScope(row.goals, scope));
}

/** One list counted both ways, which is what a tab's head says. */
export function countScope(rows: readonly Row[]): ScopeCount {
  return {
    own: rows.filter((row) => row.goals.length === 0).length,
    underGoals: rows.filter((row) => row.goals.length > 0).length,
  };
}

/**
 * What one tab's head says beside its count: the part of this kind the open
 * scope is not showing, so a human reading twelve designs knows that
 * forty-three more exist rather than that the project has twelve.
 *
 * All says nothing, because All leaves nothing out. Neither does a scope whose
 * other side is empty: "0 under goals" is furniture, not a fact worth a line.
 */
export function scopeNote(scope: ScopeFilter, counted: ScopeCount): string {
  if (scope === "project") {
    return counted.underGoals === 0 ? "" : `${String(counted.underGoals)} under goals`;
  }
  if (scope === "goals") {
    return counted.own === 0 ? "" : `${String(counted.own)} project-wide`;
  }
  return "";
}

/**
 * The quiet figure beside a tab's name: how many this scope shows, and then
 * what it is leaving out. It is one string and not two elements, because the
 * two halves are one statement — twelve of these, and forty-three more — and
 * a gap between two spans does not say "and".
 */
export function countText(shown: number, note: string): string {
  return note === "" ? String(shown) : `${String(shown)} · ${note}`;
}

/**
 * A tab's head as one line, which is what it reads as on the screen: the
 * kind, how many this scope shows, and what it is leaving out —
 * "Designs 12 · 43 under goals".
 */
export function countLine(title: string, shown: number, note: string): string {
  return `${title} ${countText(shown, note)}`;
}

/** What the head says instead of a scope note while Find is narrowing. */
export const FOUND_NOTE = "found in every scope";

/** What a search that found nothing says, in the words that were typed. */
export function noMatchLine(typed: string): string {
  return `Nothing on this tab matches “${typed.trim()}”, in any scope.`;
}

/** One goal's group in the Goals scope: what to call it, and what is under it. */
export type GoalGroup = { id: string; title: string; to: string; rows: Row[] };

/**
 * The goal-scoped rows, grouped under the goals they name.
 *
 * A record that names three goals stands under each of the three, because it
 * is about each of them: a grouping that filed it under the first would hide
 * it from the other two, which is the thing a human on a goal's group came
 * here to find. The order is the ledger's own, which every other listing of
 * goals in this interface uses; a goal the ledger does not carry keeps its
 * place at the end under the id it is named by, rather than being dropped
 * along with the records that name it.
 */
export function goalGroups(pane: Pane, rows: readonly Row[]): GoalGroup[] {
  const groups: GoalGroup[] = [];
  const byID = new Map<string, GoalGroup>();
  const place = (id: string) => {
    let group = byID.get(id);
    if (group === undefined) {
      const goal = goalWithID(pane, id);
      group = { id, title: goal === null ? id : nameOf(goal), to: goalPath(id), rows: [] };
      byID.set(id, group);
      groups.push(group);
    }
    return group;
  };
  for (const goal of pane.goals) {
    place(goal.id);
  }
  for (const row of rows) {
    for (const id of row.goals) {
      place(id).rows.push(row);
    }
  }
  return groups.filter((group) => group.rows.length > 0);
}

/**
 * The rows a typed line finds.
 *
 * Every word typed has to be somewhere in what the row shows — its title, its
 * own first words, the goals it names, or its path — in any order and in any
 * of them, because a human searching a listing remembers a word of what a
 * record is about at least as often as they remember its title. Case is
 * nothing. A line of only spaces is not a search, and answers with the rows it
 * was given.
 */
export function found(rows: readonly Row[], typed: string): Row[] {
  const words = typed
    .toLowerCase()
    .split(/\s+/)
    .filter((word) => word !== "");
  if (words.length === 0) {
    return [...rows];
  }
  return rows.filter((row) => {
    const searched = `${row.title} ${row.summary} ${row.status} ${row.path} ${row.goals.join(" ")}`.toLowerCase();
    return words.every((word) => searched.includes(word));
  });
}

/** What the box that crosses every scope is called, and what it offers. */
export const FIND_LABEL = "Find";
export const FIND_PLACEHOLDER = "title or goal";

/** What one kind is called in the line a goal page ends its tab with. */
const PROJECT_WIDE: Readonly<Record<string, { one: string; many: string }>> = {
  decisions: { one: "project-wide decision", many: "project-wide decisions" },
  designs: { one: "project-wide design", many: "project-wide designs" },
  [QUESTIONS_TAB]: { one: "project-wide open question", many: "project-wide open questions" },
};

/**
 * The one muted line a goal page's tab ends with: how many records of this
 * kind are about the project as a whole rather than about any goal.
 *
 * A goal page shows its own records only, which is right and is also a way to
 * forget that the decisions everything rests on are one page away. The line
 * says how many there are and leads to them. Nothing to say is said with
 * nothing: a tab whose kind has no project-wide records carries no line.
 */
export function projectWideLine(tab: string, count: number): string {
  const words = PROJECT_WIDE[tab];
  if (words === undefined || count <= 0) {
    return "";
  }
  return `${String(count)} ${count === 1 ? words.one : words.many} →`;
}

/** What a goal reads by: its own title where the ledger has one, else its id. */
export function nameOf(goal: Goal): string {
  return goal.title === "" ? goal.id : goal.title;
}

/** The goal this id names, or null where the ledger does not carry it. */
export function goalWithID(pane: Pane, id: string): Goal | null {
  return pane.goals.find((candidate) => candidate.id === id) ?? null;
}

/**
 * The briefing: the whole project, or one goal of it.
 *
 * For the project the two books open with their own first paragraph and their
 * reading order. A goal page carries neither: the intent and the doctrine are
 * the project's, and repeating them under every goal would say nothing. What a
 * goal page carries is its own intent, from the ledger, and then the decisions,
 * designs and open questions whose Goals name it.
 *
 * The scope narrows the project's own page and nothing else. A goal page is
 * already one goal's records, and asking it for "the project's own" would ask
 * for the records that name no goal among the records that name this one,
 * which is nothing at all. The rows the scope leaves out are still carried,
 * under `across`, because Find crosses every scope and because a goal page's
 * tail line counts what is project-wide.
 */
export function briefingFor(pane: Pane, id: string | null, scope: ScopeFilter = "all"): Briefing {
  const narrow = (rows: Row[]) => (id === null ? narrowed(rows, scope) : rows);
  const everyDecision = recordRows(pane, "decision", id);
  const everyDesign = recordRows(pane, "design", id);
  const everyQuestion = questionRows(pane, id);
  const decisions = narrow(everyDecision);
  const designs = designRuns(narrow(everyDesign));
  const questions = narrow(everyQuestion);
  return {
    goal: id === null ? null : goalBriefing(pane, id),
    books: id === null ? BOOKS.map((book) => bookBriefing(pane, book.id, book.kind, book.title)) : [],
    decisions,
    designs,
    questions,
    slices: id === null ? null : slicePlan(pane, id),
    across: { decisions: everyDecision, designs: everyDesign, questions: everyQuestion },
    // The counts are the project's, on either page: a goal page's tail line
    // asks how many records are about the project as a whole, which is not a
    // question about this goal.
    scopes: {
      decisions: countScope(recordRows(pane, "decision", null)),
      designs: countScope(recordRows(pane, "design", null)),
      questions: countScope(questionRows(pane, null)),
    },
    // What is waiting is what is waiting in this scope of the project, and
    // narrowing the view does not answer a question. So it is counted over
    // everything this page is about, whatever the control is showing.
    needsYou: {
      questions: everyQuestion.filter((row) => row.status === "open").length,
      // An accepted design is one whose work has not shipped: shipping it is
      // what makes it done.
      designs: everyDesign.filter((row) => !FINISHED.includes(row.status) && row.status === "accepted").length,
    },
    checkout: {
      records: pane.records.length,
      homes: new Set(pane.records.map((record) => record.home)).size,
      goals: pane.goals.length,
      problems: pane.problems.length,
    },
  };
}

/**
 * One goal's slice plan.
 *
 * A design governs this goal when its own Goals line names it — the same
 * selection every other section of a goal page makes — and it records a plan
 * when it wrote one under a Slices heading. A design that governs the goal and
 * lists nothing is not part of the plan and is not listed as an empty one:
 * there is nothing recorded there to read.
 */
export function slicePlan(pane: Pane, id: string): SlicePlan {
  const goal = goalWithID(pane, id);
  return {
    started: goal?.sliced ?? null,
    designs: pane.records
      .filter((record) => record.kind === "design" && record.goals.includes(id) && record.slices.length > 0)
      .map((record) => ({
        key: record.path,
        title: record.title,
        to: documentPath(record.path),
        status: record.status,
        slices: record.slices,
      })),
  };
}

/** How many slices the whole plan records, across every design that has one. */
export function sliceCount(plan: SlicePlan): number {
  return plan.designs.reduce((total, design) => total + design.slices.length, 0);
}

/**
 * The plans for a set of goals, built once.
 *
 * The board asks for one per card, and the same derivation answers the goal
 * page's tab: there is one rule for what a goal's slice plan is, and both
 * surfaces read it rather than each deriving its own.
 */
export function slicePlans(pane: Pane, ids: readonly string[]): Map<string, SlicePlan> {
  return new Map(ids.map((id) => [id, slicePlan(pane, id)]));
}

/**
 * The one line a card carries about its slices: what is planned, and when
 * slicing began, with whichever of the two is known.
 *
 * There is no execution state in it, per slice or in total, and that is not an
 * omission. The ledger records that slicing started and nothing finer; a
 * slice's state would have to be read out of prose somebody wrote in a design
 * or a next step, and inventing "done" or "underway" from prose is exactly the
 * reconstruction the master refuses. The honest label for a slice's state is
 * none.
 */
export function sliceLine(count: number, startedAt: string): string {
  const parts: string[] = [];
  if (count > 0) {
    parts.push(`${String(count)} slice${count === 1 ? "" : "s"} planned`);
  }
  if (startedAt !== "") {
    parts.push(`slicing started ${startedAt}`);
  }
  return parts.join(" · ");
}

/* ----------------------------------------- what a design's work has done -- */

/** One goal a design names, as the design's own page shows it. */
export type NamedGoal = {
  id: string;
  name: string;
  to: string;
  /** The ledger's own word, or "" where the ledger does not carry this goal. */
  state: string;
  done: boolean;
};

/**
 * What a design's Goals line says about whether its work has landed.
 *
 * The link runs one way — a design names goals, and a goal names no design —
 * so this is the whole of what can be read: the states of the goals the design
 * itself named. It is derived on every read and stored nowhere, because a
 * conclusion that was derived and then stored would be two answers to one
 * question.
 */
export type DesignWork = {
  goals: NamedGoal[];
  done: number;
  /** True when it names at least one goal and every one of them is concluded. */
  landed: boolean;
};

/** The ledger state a goal that landed carries. */
export const GOAL_DONE = "done";

/** The status a design carries once its work has shipped. */
export const DESIGN_DONE = "done";

/** What the design's page calls the section, and what it says when it is full. */
export const WORK_TITLE = "Work";
export const WORK_LANDED = "All the work this design names has landed.";
export const MARK_DONE = "Mark done";

/**
 * The goals a design names, each with where the ledger says it stands.
 *
 * They are shown in ledger order — the payload's own, which is the live goals
 * first and the concluded ones after them — because that is the order every
 * other listing of goals in this interface uses, and a design's head is a set
 * rather than a sequence. A goal the ledger does not carry has no place in
 * that order, so it keeps the place the head gave it, at the end, and counts
 * as not done: the check verb refuses that record in the same breath, and
 * calling its work landed would be inventing a conclusion.
 */
export function designWork(pane: Pane | null, goals: readonly string[]): DesignWork {
  const placed = goals.map((id) => {
    const at = pane === null ? -1 : pane.goals.findIndex((candidate) => candidate.id === id);
    const goal = at < 0 || pane === null ? null : pane.goals[at];
    const state = goal === undefined || goal === null ? "" : goal.state;
    return {
      at: at < 0 ? Number.MAX_SAFE_INTEGER : at,
      goal: {
        id,
        name: goal === undefined || goal === null ? id : nameOf(goal),
        to: goalPath(id),
        state,
        done: state === GOAL_DONE,
      },
    };
  });
  const named = [...placed].sort((left, right) => left.at - right.at).map((entry) => entry.goal);
  const done = named.filter((goal) => goal.done).length;
  return { goals: named, done, landed: named.length > 0 && done === named.length };
}

/**
 * The one quiet clause a design carries about its work: how much of what it
 * named has landed. A design that names no goals says nothing — a standing
 * design is never nagged about work it never claimed — and the count reads as
 * a whole once every goal is in.
 */
export function workLine(work: DesignWork): string {
  const total = work.goals.length;
  if (total === 0) {
    return "";
  }
  const word = total === 1 ? "goal" : "goals";
  return work.landed ? `all ${String(total)} ${word} done` : `${String(work.done)} of ${String(total)} ${word} done`;
}

/**
 * What one record's row says beside its status. Only a design has one, and a
 * design that is already done has none: its own status has said it, and a
 * count beside that would be saying it twice.
 */
export function designNote(pane: Pane, record: ProjectRecord): string {
  if (record.kind !== "design" || record.status === DESIGN_DONE) {
    return "";
  }
  return workLine(designWork(pane, record.goals));
}

/**
 * True when the design's page offers to mark it done: it named work, all of
 * that work has landed, and the design does not already say so. The status
 * stays a human word — this is the evidence put in front of them, and the act
 * is still theirs.
 */
export function marksDone(work: DesignWork, status: string): boolean {
  return work.landed && status !== DESIGN_DONE;
}

/** Every slice of a plan, each with the design that lists it. */
export function slicesOf(plan: SlicePlan): { key: string; text: string; title: string; to: string }[] {
  return plan.designs.flatMap((design) =>
    design.slices.map((text, at) => ({
      key: `${design.key}-${String(at)}`,
      text,
      title: design.title,
      to: design.to,
    })),
  );
}

function goalBriefing(pane: Pane, id: string): GoalBriefing {
  const goal = goalWithID(pane, id);
  const count = pane.records.filter((record) => record.goals.includes(id)).length;
  if (goal === null) {
    return { id, title: id, state: "", intent: "", found: false, count };
  }
  return { id, title: nameOf(goal), state: goal.state, intent: goal.intent, found: true, count };
}

/**
 * One book, opened: what it says it is, and every chapter it names, followed
 * by the records of its kind the index does not name — which are records
 * nobody would otherwise see.
 */
function bookBriefing(pane: Pane, id: string, kind: string, title: string): BookBriefing {
  const book = id === "intent" ? pane.intent : pane.doctrine;
  const index = book.index;
  const chapters: TocEntry[] = [];
  const named = new Set<string>(index === null ? [] : [index.id]);
  for (const [position, chapter] of book.chapters.entries()) {
    if (chapter.id !== undefined) {
      named.add(chapter.id);
    }
    chapters.push(tocEntry(pane, chapter, position));
  }
  for (const record of pane.records) {
    if (record.kind === kind && !named.has(record.id)) {
      chapters.push({ key: record.path, title: record.title, to: documentPath(record.path), summary: record.summary });
    }
  }
  return {
    id,
    title,
    to: index === null ? null : documentPath(index.path),
    status: index === null ? "" : index.status,
    lede: index === null ? "" : index.summary,
    chapters,
  };
}

/**
 * One chapter, as the book named it. A chapter that binds a document opens
 * that document; a chapter that names a record opens the record's own reading
 * view, which is how a chapter of the intent that is itself a record is read.
 * A chapter naming a record no head declares opens nothing, which is what the
 * check verb refuses in the same breath.
 */
function tocEntry(pane: Pane, chapter: Chapter, position: number): TocEntry {
  const key = `chapter-${String(position)}`;
  if (chapter.path !== undefined && chapter.path !== "") {
    return { key, title: chapter.title, to: documentPath(chapter.path), summary: chapter.summary };
  }
  const record = recordWithID(pane, chapter.id);
  if (record === null) {
    return { key, title: chapter.title, to: null, summary: chapter.summary };
  }
  return {
    key,
    title: chapter.title === "" ? record.title : chapter.title,
    to: documentPath(record.path),
    summary: chapter.summary === "" ? record.summary : chapter.summary,
  };
}

/**
 * The designs, grouped by what they are for: what is being written and what
 * governs stays open; what shipped and what was replaced go into one collapsed
 * run each, so a briefing is what is live rather than what has accumulated.
 */
function designRuns(designs: Row[]): DesignRuns {
  return {
    open: designs.filter((row) => !FINISHED.includes(row.status)),
    runs: FINISHED.map((status) => ({ status, rows: designs.filter((row) => row.status === status) })).filter(
      (run) => run.rows.length > 0,
    ),
  };
}

function recordRows(pane: Pane, kind: string, goal: string | null): Row[] {
  return pane.records
    .filter((record) => record.kind === kind && selected(record.goals, goal))
    .map((record) => rowOf(pane, record, record.path));
}

/** One row: what it is called, where it stands, and where it lives. */
function rowOf(pane: Pane, record: ProjectRecord, key: string): Row {
  return {
    key,
    title: record.title,
    to: documentPath(record.path),
    summary: record.summary,
    status: record.status,
    goals: record.goals,
    note: designNote(pane, record),
    path: record.path,
  };
}

/** A question reads as a row too: its text and where it stands. */
function questionRows(pane: Pane, goal: string | null): Row[] {
  return pane.questions
    .filter((question: Question) => selected(question.goals, goal))
    .map((question) => ({
      key: question.id === "" ? question.question : question.id,
      title: question.question,
      to: null,
      summary: "",
      status: question.status,
      goals: question.goals,
      note: "",
      path: "",
    }));
}

function recordWithID(pane: Pane, id: string | undefined): ProjectRecord | null {
  if (id === undefined || id === "") {
    return null;
  }
  return pane.records.find((record) => record.id === id) ?? null;
}

/* ------------------------------------------------------------- the reader -- */

/** One step of the trail above a document. The last step opens nothing. */
export type Crumb = { label: string; to: string | null };

/** One neighbour in the left rail: where it opens, and where it stands. */
export type Sibling = { key: string; title: string; to: string; note: string; current: boolean };

/** The left rail: what this document sits among, and how to walk it. */
export type SiblingRail = {
  title: string;
  siblings: Sibling[];
  previous: Sibling | null;
  next: Sibling | null;
};

/** The word each kind reads by where a section of the briefing names it. */
const KIND_TITLES: Readonly<Record<string, string>> = {
  intent: "Intent",
  doctrine: "Doctrine",
  decision: "Decisions",
  design: "Designs",
};

export function kindTitle(kind: string): string {
  return KIND_TITLES[kind] ?? "Documents";
}

/** One goal a record is about, as a link to that goal's own page. */
export type About = { id: string; name: string; to: string; known: boolean };

/**
 * What a record says it is about, as links. A goal the ledger does not carry is
 * shown as the id it is and still opens its page, which says the ledger does
 * not have it — the check verb refuses that record in the same breath, and
 * hiding the name would hide the thing to act on.
 */
export function aboutOf(pane: Pane | null, goals: string[]): About[] {
  return goals.map((id) => {
    const goal = pane === null ? null : goalWithID(pane, id);
    return { id, name: goal === null ? id : nameOf(goal), to: goalPath(id), known: goal !== null };
  });
}

/* ----------------------------------------------- a record's own scope, here -- */

/** What the facts row calls the scope, what it says when there is none, and the act. */
export const ABOUT_LABEL = "About";
export const ABOUT_PROJECT = "this project";
export const ABOUT_EDIT = "Edit";
export const ABOUT_SAVE = "Save";
export const ABOUT_CANCEL = "Cancel";

/** The kinds that are about the project as a whole by definition. */
const BOOK_KINDS = ["intent", "doctrine"];

/**
 * True when this record's scope can be changed from its own page.
 *
 * Intent and doctrine show no such act: a chapter of either is about the
 * project as a whole by definition, the grammar refuses a Goals line on one,
 * and an Edit that could only ever be refused is a control that lies.
 */
export function scopeEditable(kind: string): boolean {
  return kind !== "" && !BOOK_KINDS.includes(kind);
}

/**
 * What the About row says: the project as a whole, one goal by its id and its
 * title, or several goals by their ids alone.
 *
 * One goal gets its title because there is room for it and because an id
 * alone does not say what the record is about. Several do not: three ids and
 * three titles is a paragraph in a facts row, and each id still opens the
 * goal's own page where the title is the heading.
 */
export type AboutRow = { project: boolean; goals: About[]; named: boolean };

export function aboutRow(pane: Pane | null, goals: string[]): AboutRow {
  const about = aboutOf(pane, goals);
  return { project: about.length === 0, goals: about, named: about.length === 1 };
}

/** The book whose reading order names this document, or null. */
function bookOf(pane: Pane, document: DocumentPayload): { id: string; title: string; book: Book } | null {
  for (const { id, title } of BOOKS) {
    const book = id === "intent" ? pane.intent : pane.doctrine;
    const named = book.chapters.some(
      (chapter) =>
        (chapter.path !== undefined && chapter.path === document.id) ||
        (chapter.id !== undefined && document.record !== null && chapter.id === document.record.id),
    );
    if (named || (book.index !== null && book.index.path === document.id)) {
      return { id, title, book };
    }
  }
  return null;
}

/**
 * The trail above a document: the section, what kind it is, and what it is
 * called. A chapter bound into a book is named by its book instead, because
 * that is the only place it belongs.
 *
 * The goals a record is about are not a step of the trail: a record may be
 * about several, and the About line beneath the title names all of them as
 * links rather than one of them as a place this document lives.
 */
export function crumbsFor(pane: Pane | null, document: DocumentPayload): Crumb[] {
  const crumbs: Crumb[] = [{ label: "Project", to: "/project" }];
  if (pane === null) {
    return [...crumbs, { label: document.title, to: null }];
  }
  const head = document.record;
  if (head === null) {
    const bound = bookOf(pane, document);
    if (bound !== null) {
      crumbs.push({ label: bound.title, to: bound.book.index === null ? null : documentPath(bound.book.index.path) });
      return [...crumbs, { label: document.title, to: null }];
    }
    return [...crumbs, { label: "Documents", to: null }, { label: document.title, to: null }];
  }
  crumbs.push({ label: kindTitle(head.kind), to: null });
  return [...crumbs, { label: document.title, to: null }];
}

/**
 * The left rail: what this document is read among.
 *
 * A chapter of a book — bound or a record of its own — is read among the
 * book's chapters, in reading order, because that is the order it was written
 * to be read in. Every other record is read among the records of its own kind:
 * the designs among the designs, the decisions among the decisions. A document
 * that declares nothing has no siblings and no rail.
 *
 * A record that is the only one of its kind keeps its rail all the same,
 * naming itself and nothing else, with no previous and no next. The rail says
 * what this document is read among, and "the only decision" is an answer; a
 * rail that vanished would leave the reader with one region fewer and no way
 * to know why.
 */
export function railFor(pane: Pane, document: DocumentPayload): SiblingRail | null {
  const bound = bookOf(pane, document);
  const rail = bound === null ? kindRail(pane, document) : bookRail(pane, bound, document);
  if (rail === null) {
    return null;
  }
  const at = rail.siblings.findIndex((sibling) => sibling.current);
  return {
    ...rail,
    previous: at > 0 ? rail.siblings[at - 1] : null,
    next: at >= 0 && at + 1 < rail.siblings.length ? rail.siblings[at + 1] : null,
  };
}

function bookRail(
  pane: Pane,
  bound: { id: string; title: string; book: Book },
  document: DocumentPayload,
): SiblingRail | null {
  const siblings: Sibling[] = [];
  const index = bound.book.index;
  if (index !== null) {
    siblings.push(siblingOf(index.path, index.title, index.status, document));
  }
  for (const [position, chapter] of bound.book.chapters.entries()) {
    if (chapter.path !== undefined && chapter.path !== "") {
      siblings.push(siblingOf(chapter.path, chapter.title, "", document, `chapter-${String(position)}`));
      continue;
    }
    const record = recordWithID(pane, chapter.id);
    if (record === null) {
      continue;
    }
    const title = chapter.title === "" ? record.title : chapter.title;
    siblings.push(siblingOf(record.path, title, record.status, document, `chapter-${String(position)}`));
  }
  return { title: bound.title, siblings, previous: null, next: null };
}

function kindRail(pane: Pane, document: DocumentPayload): SiblingRail | null {
  const head = document.record;
  if (head === null) {
    return null;
  }
  const siblings = pane.records
    .filter((record) => record.kind === head.kind)
    .map((record) => siblingOf(record.path, record.title, record.status, document));
  return { title: kindTitle(head.kind), siblings, previous: null, next: null };
}

function siblingOf(path: string, title: string, note: string, document: DocumentPayload, key?: string): Sibling {
  return { key: key ?? path, title, to: documentPath(path), note, current: path === document.id };
}

/**
 * The id as a reader needs it: its ends, which is what tells two ULIDs apart,
 * with the whole of it kept for the title attribute. An id short enough to
 * read is left alone.
 */
export const ID_HEAD = 6;
export const ID_TAIL = 4;

export function shortID(id: string): string {
  return id.length <= ID_HEAD + ID_TAIL + 1 ? id : `${id.slice(0, ID_HEAD)}…${id.slice(-ID_TAIL)}`;
}

/* ---------------------------------------------------------- the documents -- */

/**
 * The checkout's other Markdown, in collapsed runs.
 *
 * A run is the first two segments of a file's directory, or the one segment it
 * has, or the checkout root for a file that sits in it: enough to tell
 * metasystem/docs from metasystem/plans without putting every subdirectory of
 * every home on the screen. The order is the payload's, which is path order.
 */
export const ROOT_GROUP = "The checkout root";

export function documentGroups(files: DocumentFile[]): DocumentGroup[] {
  const groups: DocumentGroup[] = [];
  const byID = new Map<string, DocumentGroup>();
  for (const file of files) {
    const id = groupOf(file.path);
    let group = byID.get(id);
    if (group === undefined) {
      group = { id, title: id === "" ? ROOT_GROUP : id, files: [] };
      byID.set(id, group);
      groups.push(group);
    }
    group.files.push(file);
  }
  return groups;
}

function groupOf(path: string): string {
  const segments = path.split("/");
  return segments.slice(0, Math.min(2, segments.length - 1)).join("/");
}

/**
 * What one tab of this page is listing, by the key each row is listed under.
 *
 * It travels with a question asked from the page, so the Project Partner is
 * told which records are on the screen rather than left to guess from a tab's
 * name. The keys are the rows' own — a record's checkout-relative path, a
 * question's id — because the server reads what each one is from its own read
 * of the project, and a title the page sent would be the page's word for it.
 */
export function listedIn(briefing: Briefing, tab: string): string[] {
  const book = briefing.books.find((candidate) => candidate.id === tab);
  if (book !== undefined) {
    return book.chapters.map((chapter) => chapter.key);
  }
  if (tab === tabForKind("decision")) {
    return briefing.decisions.map((row) => row.key);
  }
  if (tab === tabForKind("design")) {
    return [
      ...briefing.designs.open.map((row) => row.key),
      ...briefing.designs.runs.flatMap((run) => run.rows.map((row) => row.key)),
    ];
  }
  if (tab === QUESTIONS_TAB) {
    return briefing.questions.map((row) => row.key);
  }
  return [];
}

/* ------------------------------------------------------------- the sittings -- */

/**
 * What one Sittings row says beside its title, in the order a human reads it
 * (g1-s55 D3).
 *
 * The four piles as counts, because the counts are what says how much of a
 * sitting there is; the piles nobody wrote into are left out, because "0
 * proposals" is a fact about a section that does not exist rather than about
 * this sitting.
 */
export function pilesLine(counts: SittingRow["counts"]): string {
  const said = [
    counted(counts.facts, "fact"),
    counted(counts.proposals, "proposal"),
    counted(counts.decisions, "decision"),
    counted(counts.questions, "open question"),
  ].filter((one) => one !== "");
  return said.length === 0 ? NOTHING_RECORDED : said.join(" · ");
}

/** What a row says where its sitting has recorded nothing into the record yet. */
export const NOTHING_RECORDED = "nothing recorded yet";

/** What a row says where a sitting is open on that record right now. */
export const STANDS_NOW = "a sitting stands on this now";

/**
 * What the press on a row does, said where a human meets it.
 *
 * `stands` is whether a sitting stands at all, on whichever record. It decides
 * the words because it decides the press: there is one sitting on one
 * conversation, so a row pressed while another row's sitting stands cannot start
 * a second one — it would replace the standing mark and end that sitting without
 * its outcome. So such a press only opens the conversation, and the row says so
 * rather than promising a start it will not make.
 */
export function opensLine(row: SittingRow, stands: boolean): string {
  if (row.standing) {
    return `Open the conversation on ${row.record.path}`;
  }
  if (stands) {
    return "Open the conversation. A sitting stands on another record, so this starts nothing";
  }
  return `Open the conversation on ${row.record.path} and start a sitting on it`;
}

/** When a row's last entry was recorded, as the row says it, or "". */
export function lastAtLine(row: SittingRow): string {
  return row.lastAt === "" ? "" : `last entry ${row.lastAt}`;
}

/** One count, or "" for none of something, which is not worth a word. */
function counted(of: number, word: string): string {
  if (of === 0) {
    return "";
  }
  return `${String(of)} ${word}${of === 1 ? "" : "s"}`;
}
