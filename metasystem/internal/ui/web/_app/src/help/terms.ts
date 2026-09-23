/**
 * What every structure in this interface is for, in plain English.
 *
 * One register, and it is the only place an explanation is written. A name on
 * the screen — Doctrine, Ready for Work, Tier — is a word this project uses in
 * its own way, and a human meeting it for the first time should not have to
 * read the engine to find out what it means. The help control beside the name
 * says it, and says it in one voice because every sentence is here rather than
 * spread across the components that show them.
 *
 * An id is never shown. It names a term, so a component asks for the
 * explanation of a thing rather than carrying a copy of it, and rewording one
 * sentence is one edit in one file.
 */

export type HelpId =
  | "intent"
  | "doctrine"
  | "decisions"
  | "designs"
  | "questions"
  | "documents"
  | "slices"
  | "goal-decisions"
  | "goal-designs"
  | "goal-questions"
  | "overview"
  | "project"
  | "backlog"
  | "fleet"
  | "decisions-section"
  | "application"
  | "partner"
  | "lane-draft"
  | "lane-todo"
  | "lane-ready"
  | "lane-progress"
  | "lane-review"
  | "lane-waiting"
  | "lane-done"
  | "lane-abandoned"
  | "lane-split"
  | "priority"
  | "tier"
  | "seat"
  | "arc";

/** The name the popover heads itself with, and what that name is for. */
export type Term = { term: string; text: string };

export const HELP: Record<HelpId, Term> = {
  intent: {
    term: "Intent",
    text: "Why this project exists and what it must and must not become. It is the standing answer to what we are building and for whom. Every goal serves it, and when it changes, the change is written here first.",
  },
  doctrine: {
    term: "Doctrine",
    text: "The rules every design is held to, decided once so nobody argues them again per goal: the architecture, the principles, the shape of the code. A design that breaks doctrine is either wrong or a proposal to change doctrine.",
  },
  decisions: {
    term: "Decisions",
    text: "Choices that were made, and the reasons, kept so later work can rely on them instead of deciding again. One page per decision. A decision that is overturned is superseded, never deleted.",
  },
  designs: {
    term: "Designs",
    text: "How a piece of work will be built, written before it is built. A design says what the work does and does not do. When the work ships, the design's status becomes done.",
  },
  questions: {
    term: "Open questions",
    text: "What nobody has answered yet. One row per question, with when it was opened and, once settled, what answered it.",
  },
  documents: {
    term: "Documents",
    text: "The rest of the checkout's Markdown, listed by path. Nothing here declares what kind of record it is, so read it as writing, not as a record.",
  },
  slices: {
    term: "Slices",
    text: "The pieces a goal is built in, each small enough to ship on its own, listed by the design that governs it. The first slice is the smallest thing that works.",
  },
  "goal-decisions": {
    term: "Decisions for this goal",
    text: "The decisions whose Goals line names this goal. Project-wide decisions live under Project.",
  },
  "goal-designs": {
    term: "Designs for this goal",
    text: "The designs whose Goals line names this goal, with the slices each one planned.",
  },
  "goal-questions": {
    term: "Open questions for this goal",
    text: "The questions opened about this goal that nobody has answered yet.",
  },
  overview: {
    term: "Overview",
    text: "What needs you, and what changed since you last looked, with links to the records behind it.",
  },
  project: {
    term: "Project",
    text: "The project's memory: its intent, doctrine, decisions, designs and open questions, each a record the checkout carries, and the documents around them.",
  },
  backlog: {
    term: "Backlog",
    text: "The goals, as a board. A goal moves from To Do through Ready for Work, In Progress and Review to Done. Drag a card to approve or withdraw it, or to change its priority.",
  },
  fleet: {
    term: "Fleet",
    text: "The machines and seats working on this project: their sessions, jobs and health, with when each was last observed.",
  },
  "decisions-section": {
    term: "Decisions",
    text: "What the machinery is asking you to rule on: questions from every seat, approvals, delegations. Your answer is recorded with your name as the authority.",
  },
  application: {
    term: "Application",
    text: "What has been built and released, and the evidence that it works.",
  },
  partner: {
    term: "Project Partner",
    text: "The agent you work with across this whole interface. Ask it about anything you see. It reads the same records you do and can write with you.",
  },
  "lane-draft": {
    term: "Draft",
    text: "A goal someone wrote down that has not been through intake yet. It is not on the board's path until it is.",
  },
  "lane-todo": {
    term: "To Do",
    text: "Goals nobody has authorized yet. A goal here may be waiting on intake or on a human's approval. Drag it to Ready for Work to approve it.",
  },
  "lane-ready": {
    term: "Ready for Work",
    text: "Approved goals a seat may claim right now, in priority order. Drag one back to To Do to withdraw the approval.",
  },
  "lane-progress": {
    term: "In Progress",
    text: "Goals a seat has claimed and is working on.",
  },
  "lane-review": {
    term: "Review and Verification",
    text: "Work that is built and being checked before it lands.",
  },
  "lane-waiting": {
    term: "Waiting",
    text: "Work a blocker holds, with the reason and the state it waits from.",
  },
  "lane-done": {
    term: "Done",
    text: "Goals that landed, with what they concluded. The selector chooses how many days back to show.",
  },
  "lane-abandoned": {
    term: "Abandoned",
    text: "Work that was dropped, with the recorded reason.",
  },
  // Not a lane, and the one column head the design's own list does not name.
  // The board stands a goal retired by decomposition here rather than in Done,
  // and a column head with no explanation beside it is the one head a human
  // would have to guess at.
  "lane-split": {
    term: "Split into goals",
    text: "A goal retired by decomposition, standing here under the goals it became rather than in Done as a delivered outcome.",
  },
  priority: {
    term: "Priority",
    text: "A band and a position in it, shown as band:position. Band 1 comes before band 2, and inside a band the lower position comes first. Seats claim from the top of Ready for Work.",
  },
  tier: {
    term: "Tier",
    text: "How much proof a goal owes before it lands, 1 to 3, set by the worse of its risk answers on severity and novelty. Tier 3 is the most demanding.",
  },
  seat: {
    term: "Seat",
    text: "The machine and session that claimed the goal.",
  },
  arc: {
    term: "Arc",
    text: "The larger line of work a goal belongs to, named so related goals can be seen together.",
  },
};

/**
 * The explanation for one section of the rail, or null where the section has
 * none. Settings is the workspace's own configuration rather than one of the
 * project's structures, and the Project Partner is explained where it stands —
 * on its own drawer — rather than twice.
 */
const SECTION_HELP: Readonly<Record<string, HelpId>> = {
  overview: "overview",
  project: "project",
  backlog: "backlog",
  fleet: "fleet",
  decisions: "decisions-section",
  application: "application",
};

export function sectionHelp(id: string | undefined): HelpId | null {
  if (id === undefined) {
    return null;
  }
  return SECTION_HELP[id] ?? null;
}

/** What a term says, where a surface shows the sentence rather than an icon. */
export function helpText(id: HelpId | null): string {
  return id === null ? "" : HELP[id].text;
}
