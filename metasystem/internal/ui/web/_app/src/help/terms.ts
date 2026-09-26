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
  | "design-work"
  | "questions"
  | "documents"
  | "slices"
  | "goal-decisions"
  | "goal-designs"
  | "goal-questions"
  | "project-scope"
  | "overview"
  | "overview-needs-you"
  | "overview-changed"
  | "overview-work"
  | "overview-memory"
  | "overview-health"
  | "project"
  | "backlog"
  | "fleet"
  | "decisions-section"
  | "needs-your-choice"
  | "inbox"
  | "new-here"
  | "waiting-for-approval"
  | "not-now"
  | "act-selected"
  | "return-to-queue"
  | "silence"
  | "ruling"
  | "review-condition"
  | "application-section"
  | "what-concluded"
  | "known-problems"
  | "concluded-new"
  | "last-engine"
  | "settings"
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
  | "lane-unknown"
  | "edit-goal"
  | "priority"
  | "tier"
  | "seat"
  | "fleet-needs-you"
  | "this-seat"
  | "machine-holds"
  | "launch-machine"
  | "temporary-word"
  | "review-by"
  | "engine"
  | "presence"
  | "standing"
  | "reachable"
  | "unreachable"
  | "unknown"
  | "rung"
  | "phase"
  | "box"
  | "bound"
  | "chain"
  | "arc"
  | "waits-for"
  | "holds"
  | "stickies"
  | "suggestion"
  | "proposal"
  | "ask-the-partner"
  | "not-offered"
  | "private-store"
  | "sitting"
  | "deposit"
  | "the-table"
  | "no-recommendation";

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
  "design-work": {
    term: "Work",
    text: "The goals this design names, with their ledger states. When every one has landed, the design can be marked done.",
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
  "project-scope": {
    term: "What this page is showing",
    text: "Scope is a filter, not two lists. Project is the records whose head names no goal, which are the small set that shapes everything and what this page opens on. Goals is the rest, grouped under the goal each one names. All is both, flat. Find searches every scope whichever one is chosen.",
  },
  overview: {
    term: "Overview",
    text: "What needs you, and what changed since you last looked, with links to the records behind it.",
  },
  "overview-needs-you": {
    term: "Needs you",
    text: "What is waiting on you and nobody else: work nobody has authorized, questions nobody has answered, drafts nobody has accepted, designs whose work has all landed, and what the steward addressed to you. When there is none of it, the page says so.",
  },
  "overview-changed": {
    term: "Since your last visit",
    text: "What happened while you were away, measured from the end of your last visit rather than from your last page load. Coming back and refreshing during a visit never narrows the window, so nothing slips past between two reads.",
  },
  "overview-work": {
    term: "Work now",
    text: "What the fleet is doing at this moment: what a seat has claimed, what it would take next, what is held and why, and how the lanes stand. Every count opens the board.",
  },
  "overview-memory": {
    term: "The project's memory",
    text: "What this project has written down: its intent and its doctrine, the decisions it has made, the designs governing work, and the questions still open. Each count opens the part of Project it belongs to.",
  },
  "overview-health": {
    term: "Health",
    text: "Whether the machinery itself is sound: this checkout's ledger against the canonical tip, what the records check refuses, and any message the steward could not deliver. One line when all three answered.",
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
    text: "The machines working on this project and whether each has been heard from lately, with the goals each one holds. A goal whose holder has gone quiet is flagged here; nothing on this page acts on it.",
  },
  "decisions-section": {
    term: "Decisions",
    text: "Two questions on one page: what needs your choice, and what you decided. The first is everything waiting on a human, each row with what happens if you do nothing; the second is your rulings, the decisions recorded, the questions answered and the goals you approved.",
  },
  "needs-your-choice": {
    term: "Needs your choice",
    text: "Everything waiting on a human and nobody else, complete rather than capped, in two blocks: what is asked of you, which is a few things with a few different consequences, and what is waiting for your approval, which is the queue of work nobody has authorized yet.",
  },
  inbox: {
    term: "Inbox",
    text: "Everything waiting on a human and nobody else, in one group per kind: a seat's question, an open question of the register, a draft, a design whose work has landed, an approval that expired, a goal a seat paused or a fence stopped, a ruling whose review has come due, what the steward addressed to you, and the queue of goals nobody has authorized. A group says how many, how old the newest one is, and how many are new; one group is open at a time, and what you leave open is what opens next time.",
  },
  "new-here": {
    term: "New",
    text: "Recorded after your last visit to this page, by the dates the records themselves carry. Reading this page is the visit, and reading it again within half an hour goes on comparing against the same moment, so a refresh never hides what you have not read. The dates are uneven and this is honest rather than exact: an instant is compared as an instant, a date without a time counts as new from the day your window opened, and a record that carries no date at all is never marked new. On a first visit it means the last day. Decisions keeps this mark separately from the Overview's, so reading one never moves the other's boundary.",
  },
  "waiting-for-approval": {
    term: "Waiting for your approval",
    text: "The goals nobody has authorized, which is a queue rather than a list: many, mostly weeks old, fine to wait. Approving one admits it so a seat may claim it, and the approval carries the budget shown beside it — the tuple the goal already has, or the project's law for its tier.",
  },
  "not-now": {
    term: "Not now",
    text: "A goal you paused, with the reason you gave and the date you gave it. It is a decision rather than something waiting on you, so it is here and not in the inbox. Return to queue lifts the pause; a pause that names a blocker lifts by itself when that blocker is done.",
  },
  "act-selected": {
    term: "Acting on what you selected",
    text: "One publication per goal, sent in the order shown, because this engine has no act over many goals. The first refusal stops the run and says which goal it stopped at in the engine's own words; the goals after it were never sent, and the page reads the ledger again to say what landed.",
  },
  "return-to-queue": {
    term: "Return to queue",
    text: "Lifts one pause. The goal goes back to approved where its approval still stands, and to the queue otherwise. A pause a seat made can be lifted here too; one that waits for a blocker returns by itself when every blocker is done.",
  },
  silence: {
    term: "If you do nothing",
    text: "What the machinery does when nobody answers. It is a statement about the engine rather than an encouragement, and it is never invented: where a record has no recorded consequence of being left, the row says exactly that.",
  },
  ruling: {
    term: "Ruling",
    text: "A standing decision a human made, kept in the register in their own words with its context, its accountable owner and, where it is temporary, the condition under which it is reviewed. The register is append-only: a ruling that is overturned is superseded by a later one, never rewritten.",
  },
  "review-condition": {
    term: "Review condition",
    text: "When a ruling comes back for a decision. Most rulings stand until a human says otherwise; a temporary, experimental, delegated-authority or assumption-dependent one carries a date or a named event, and the date is what this page can judge. An event is shown as written and judged by the steward, not here.",
  },
  "application-section": {
    term: "Application",
    text: "What this workspace has concluded, what is known to be wrong with it, and what it says it is. The first is the ledger's own record of work that ended, week by week; the second is the known-issues register, open rows first; the third is one line of links into the reader.",
  },
  "what-concluded": {
    term: "What concluded",
    text: "Every goal the ledger has concluded, newest first, with the one sentence written when it concluded. It is the record of work that ended and not a statement of what the application can do now: a conclusion sometimes records an administrative end, such as a duplicate withdrawn or a requirement absorbed into another goal, and it dates the conclusion rather than a capability that still stands. What this build does today is read from its own documents and its behaviour, not from this list.",
  },
  "known-problems": {
    term: "Known problems",
    text: "The project's known-issues register, one row per defect or limitation, open rows first. A row is concluded when its status begins with FIXED, RESOLVED, RETIRED, CLOSED or ACCEPTED, and the word stays visible, because an accepted limitation still exists. Nothing here is interpreted beyond that word, and a row this build could not read as six columns is counted in its own line rather than dropped.",
  },
  "concluded-new": {
    term: "Since your last visit",
    text: "Concluded after your last visit to this page, by the dates the records themselves carry. Reading this page is the visit, and reading it again within half an hour goes on comparing against the same moment, so a refresh never hides what you have not read. A goal whose conclusion carries no date is never marked new. On a first visit it means the last day. This page keeps the mark separately from the Overview's and Decisions', so reading one never moves another's boundary.",
  },
  "last-engine": {
    term: "Last published engine",
    text: "The engine build this seat wrote into its own presence record the last time it ticked, with the generation and the moment it was published. It is a record at one tick and not a statement about what is running now: a seat that has not ticked since it was rebuilt still publishes the build it had then. A seat that has published nothing says so rather than showing a blank.",
  },
  settings: {
    term: "Settings",
    text: "This workspace's own configuration rather than the project's records: which agent answers as the Project Partner, who is signed in, how the interface looks, and where this checkout is. Only the appearance can be changed from here in this build.",
  },
  "private-store": {
    term: "Private store",
    text: "Where this interface keeps your own material, outside every checkout so that no agent and no examiner reads it: your notepad, and your conversations with the Project Partner. It is kept to a size by a sweep that runs when the server starts and once a day after that — a conversation is trimmed from its oldest end, and the Partner's record of the wire is rotated. No directory here is ever removed, and nothing in your project is touched.",
  },
  partner: {
    term: "Project Partner",
    text: "The agent you work with across this whole interface. Ask it about anything you see. It reads the same records you do and explains them; it does not write and it does not act. Your conversation with it is kept privately outside this checkout and trimmed from its oldest end as it grows, so it is not a place to keep anything: the records are the memory that survives, which is why what you settle goes into one.",
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
    term: "Review",
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
    text: "A goal that was split into smaller goals. It stands here beside the goals it became, not in Done, because it was never delivered as one piece.",
  },
  // Unknown is a place in the projection and a column nowhere. It still needs
  // a sentence: the board discloses what landed there, and a human — or the
  // Partner reading the interface's own manifest — has to be able to find out
  // what being there means.
  "lane-unknown": {
    term: "Unknown",
    text: "A goal record this build cannot place in any lane. It is disclosed under the board with the reason it carries, never hidden and never a column of its own.",
  },
  "edit-goal": {
    term: "Edit",
    text: "Rewrites a goal's intent, next step and labels. Only a goal still queued that nobody has approved is edited here: an approval is a human's word on a particular intent, so changing one means withdrawing the approval first, a claimed goal is edited at the seat working it, and a paused one returns to the queue first. Only the fields you change are sent, so an edit made elsewhere in the meantime survives.",
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
  "waits-for": {
    term: "Waits for",
    text: "The goals this one waits for. It parks until every one of them is done, and a seat cannot claim it before then. Removing a goal that is not yet done is a decision only a person can record.",
  },
  holds: {
    term: "Holds",
    text: "The goals that are waiting for this one. Each of them stays parked until this goal is done, so finishing it is what lets them be claimed.",
  },
  presence: {
    term: "Presence",
    text: "A small record each armed machine publishes every steward tick, saying that it is alive and what it is running. It carries no goals and no words, so it can be read by every seat and grants none of them anything.",
  },
  standing: {
    term: "Standing",
    text: "What this seat concludes about another machine from its presence record: reachable, unreachable or unknown. It is judged as the page is read, against this seat's own clock, and never stored.",
  },
  reachable: {
    term: "Reachable",
    text: "A machine whose presence record is recent enough to trust: it published within the window this seat judges by, which is the longer of half an hour and three of that machine's own ticks.",
  },
  unreachable: {
    term: "Unreachable",
    text: "A machine whose presence record has gone stale. It may be off, asleep, or unable to reach the remote; the record says only that nothing new has arrived, and the goals it holds stay held.",
  },
  unknown: {
    term: "Unknown",
    text: "A machine this seat cannot judge: one that has published no presence at all, one whose record cannot be read, or one dated further ahead than a clock difference explains.",
  },
  "fleet-needs-you": {
    term: "Needs you",
    text: "The goals held by a machine this seat has not heard from lately, or by one that has published no presence at all. The goal stays claimed and nothing here changes that: reassigning it or lifting its fence is a decision a person makes at a terminal.",
  },
  "this-seat": {
    term: "This seat",
    text: "The checkout this interface is serving: what it is called on the fleet, whether supervision is armed on it, what it last published about itself, and what the steward last recorded about its health.",
  },
  "machine-holds": {
    term: "Holds",
    text: "The goals whose claim names this machine, read from the accepted ledger. It is what the ledger says and not what the machine says: a presence record carries no goals, so a machine nobody has heard from still holds what it claimed.",
  },
  "launch-machine": {
    term: "Launch a machine",
    text: "A new machine of this fleet, on this host: a full clone of this checkout beside it, with its own engine built, this seat's roster and configuration copied, its own nickname, the fleet's ledger fetched, and a steward armed. It starts no session; it joins, publishes presence and waits.",
  },
  "temporary-word": {
    term: "Your authorization",
    text: "The words you enroll the machine with, recorded on its identity as a temporary enrollment. It is the lawful path for a human who is not at that checkout's terminal, and the interface adds no authority of its own: what enrolls the machine is what you typed.",
  },
  "review-by": {
    term: "Review by",
    text: "The date the machine's temporary enrollment is due to be re-approved. It stops nothing and nothing enforces it: a person re-approves the machine, or stops it, at a terminal. This field refuses a date in the past, because a review already overdue the moment a machine joins says nothing.",
  },
  engine: {
    term: "Engine",
    text: "The build a machine was running when it last published, with the enrolment generation beside it. Two seats on different builds can read the same ledger by different rules, which is what makes it worth showing.",
  },
  rung: {
    term: "Rung",
    text: "Which way a machine managed to publish its presence at the remote: a metasystem ref, a branch per machine, or a branch per machine without force. A higher rung means the remote refused the one before it.",
  },
  phase: {
    term: "Phase",
    text: "What a machine is in the middle of: the role of the job it is running, the round that job is on, and the goal it serves. A critic's round counts against the limit its chain froze; a build has no round limit of its own, so it shows a round and no denominator.",
  },
  box: {
    term: "Box",
    text: "What a goal is allowed to spend, and what it has spent: attempts, and the job minutes its delegates have reserved. An open job is counted at its full cap rather than at what it has used so far, so a box can look nearly spent and then give minutes back when a job ends early.",
  },
  bound: {
    term: "Bound",
    text: "When a job's reserved minutes run out. It is a bound and not an estimate: nothing here predicts when work will finish, and a job that reaches its cap is stopped rather than finished. Read it as the latest this job can still be running, never as how long is left.",
  },
  stickies: {
    term: "Stickies",
    text: "Your own reminders, jotted while you work. They are yours and nobody else's: they live on this seat, outside the project's records and outside every checkout, so no agent and no reviewer reads them. A sticky can say what it is about — the goal or the document you were looking at — and then it comes back on that page. Open ones are newest first; what you mark done is kept, out of the way.",
  },
  chain: {
    term: "Chain",
    text: "The jobs that belong to one piece of work, newest first: a build, the critic that reviewed it, the round that answered the findings. Each carries the cap it reserved and when it ran, and never a consumed charge — settling what a job actually spent belongs to the engine that dispatched it.",
  },
  suggestion: {
    term: "Suggestion",
    text:
      "Words your Project Partner offered for one field of the sheet you are filling in. It is an offer and nothing else: the field keeps what you wrote until you press Use this, and Undo puts back what was there for as long as the field still holds the suggestion. Saving is still yours, and the Partner never saves.",
  },
  proposal: {
    term: "The Partner proposes",
    text:
      "Words your Project Partner has offered for this field, under the field itself. Use this puts them in, whole, and Undo puts back what was there for as long as the field still holds them; Dismiss folds the offer away. Nothing is saved by any of it: Save is yours, at the foot of this sheet. Where more than one has been offered for this field, the newest is here and the earlier ones are in the Partner's own column.",
  },
  "ask-the-partner": {
    term: "Ask the Partner",
    text:
      "Asks your Project Partner for words for this field. It writes the request into the Partner's composer and takes you there; nothing is sent until you press Enter, and you can change the request first. What comes back appears under this field as a proposal you decide about.",
  },
  "not-offered": {
    term: "Not offered",
    text:
      "Your Project Partner prepared words for a field, and they were not offered to you. It happens when the field is not one the open sheet lets the Partner write for, or when the sheet's draft was left out of the question. The card says which, and the words are there to read; nothing was put in any field and nothing was saved.",
  },
  sitting: {
    term: "Sitting",
    text:
      "A working conversation on one record, with your Project Partner. It has no memory of its own: the record is the memory, and what you decide goes into it as you go, by your own press. The test of a sitting is that you could stop right now and a fresh partner could go on from the record alone. The Partner's first turn brings what the records already hold about the subject; it never decides anything, and only records rule.",
  },
  deposit: {
    term: "What the Partner offers the record",
    text:
      "A fact, a decision or an open question your Project Partner has offered for the record of this sitting. It is an offer and nothing else: nothing is written until you press Record it, and the words are yours to change before you do. A decision is recorded with the reason you gave and a fact with the anchor where it can be checked, so Record it waits until each has one.",
  },
  "the-table": {
    term: "The table",
    text:
      "The four kinds of working material this sitting has put into its record: Facts, Proposals, Decisions and Open questions. It is not a second store — it is those four sections of the record itself, so everything here is something you recorded and anything you did not record is not here. The counts beside your Project Partner are the same four.",
  },
  "no-recommendation": {
    term: "No recommendation",
    text:
      "Your Project Partner lays out options with their consequences and does not say which to pick. That is deliberate: when every option comes pre-weighed, choosing the recommended one every time is approval by habit, and the values in this project are yours. Ask it to weigh them and it will; unasked, it brings the cases at the edge, the conflicts and the consequences, and leaves the choice where it belongs.",
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
  application: "application-section",
};

export function sectionHelp(id: string | undefined): HelpId | null {
  if (id === undefined) {
    return null;
  }
  return SECTION_HELP[id] ?? null;
}

/**
 * The term that says what a section is, for a reader that needs one for every
 * section rather than only for the six the header explains.
 *
 * The header's control is above; this is the same register, extended by the
 * two sections the header leaves alone: the Project Partner is explained on
 * its own drawer, and Settings is the workspace's own configuration. A reader
 * that has to describe the whole interface — the manifest the Partner is
 * given — needs a sentence for those two as well, and it is this one rather
 * than a second account written for it.
 */
const SECTION_TERM: Readonly<Record<string, HelpId>> = {
  ...SECTION_HELP,
  brain: "partner",
  settings: "settings",
};

export function sectionTerm(id: string | undefined): HelpId | null {
  if (id === undefined) {
    return null;
  }
  return SECTION_TERM[id] ?? null;
}

/** What a term says, where a surface shows the sentence rather than an icon. */
export function helpText(id: HelpId | null): string {
  return id === null ? "" : HELP[id].text;
}
