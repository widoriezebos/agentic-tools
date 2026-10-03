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
  | "fleet-verdict"
  | "fleet-needs-you"
  | "lane-pause"
  | "lane-resume"
  | "machine-stop"
  | "machine-holds"
  | "launch-machine"
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
  | "proposed-action"
  | "apply-proposed"
  | "unresolved-act"
  | "ask-the-partner"
  | "not-offered"
  | "private-store"
  | "sitting"
  | "deposit"
  | "the-case"
  | "the-outcome"
  | "critique"
  | "send-to-critique"
  | "finding-card"
  | "section-card"
  | "answer-the-round"
  | "the-board"
  | "sittings"
  | "no-recommendation"
  | "reviews"
  | "review-room"
  | "landing-gate"
  | "the-desk"
  | "the-finding"
  | "the-walks"
  | "the-verdict"
  | "the-candidate"
  | "the-brief"
  | "sitting-room"
  | "shaping-desk"
  | "shaping-walks";

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
    text: "The machines working on this project: whether each has been heard from lately, what it is doing, and the goals it holds. A goal whose holder has gone quiet is flagged, and Needs you says what a person does about it at a terminal.",
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
    term: "Paused",
    text: "A goal you paused, with the reason you gave and the date you gave it, called Not now until the verbs landed. It is a decision rather than something waiting on you, so it is here and not in the inbox. Resume lifts the pause; a pause that names a blocker lifts by itself when that blocker is done.",
  },
  "act-selected": {
    term: "Acting on what you selected",
    text: "One publication per goal, sent in the order shown, because this engine has no act over many goals. The first refusal stops the run and says which goal it stopped at in the engine's own words; the goals after it were never sent, and the page reads the ledger again to say what landed.",
  },
  "return-to-queue": {
    term: "Resume",
    text: "Lifts one pause, called Return to queue until the verbs landed. The goal goes back to approved where its approval still stands, and to the queue otherwise. A pause a seat made can be lifted here too; one that waits for a blocker returns by itself when every blocker is done.",
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
    text: "What is known to be wrong with this workspace, what it has concluded, and what it says it is. The first is the known-issues register, open rows first, because an open problem is what a human can do something about today; the second is the ledger's own record of work that ended, week by week; the third is one line of links into the reader.",
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
    text: "Goals that are not approved: nobody has authorized them yet. A goal here may be waiting on intake or on a human's approval. Drag it to Ready for Work to approve it.",
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
  "fleet-verdict": {
    term: "Verdict",
    text: "One line on whether anything on this computer needs you: All good only when Needs you is empty and every section of this page could be read; otherwise how many things need you, or which section could not be read. Checked on this computer: its seats, its landing lane and this checkout's questions. Questions asked on other computers are not checked here.",
  },
  "fleet-needs-you": {
    term: "Needs you",
    text: "Everything on this page that waits for you, newest first: a seat of this computer that is stuck, the landing lane paused or unable to run, a branch that came back and was not handed in again, a red proof nothing has answered, a question this checkout's seats asked you, this computer's own health, and goals held by a machine this seat has not heard from. Each is one line with one thing to do: a button that opens the goal, the question or the proof's log, Resume for a paused lane, Stop for a stuck seat's machine, or the one command to type at a terminal on this computer. Resume and Stop act in your name, so they ask you to sign in first, and Stop asks once more before it stops.",
  },
  "lane-pause": {
    term: "Pause",
    text: "Pauses this computer's landing lane in your name: no landing agent starts and nothing waiting lands until the lane is resumed, and the lane says who paused it and since when. It runs metasystem landing stop as you; Resume undoes it.",
  },
  "lane-resume": {
    term: "Resume",
    text: "Ends a pause of this computer's landing lane: its landing agent starts again when there is work. It runs metasystem landing start with you as the person it asks for. Resuming a lane that is not paused changes nothing.",
  },
  "machine-stop": {
    term: "Stop",
    text: "Stops MetaSystem on one machine of this computer: its seat and every job on it end, and its steward does not start it again until metasystem system start is run at that machine's checkout. It runs metasystem machine stop with you as the person it asks for, and asks a second press first. The machine serving this page is stopped at a terminal, never from here.",
  },
  "machine-holds": {
    term: "Holds",
    text: "The goals whose claim names this machine, read from the accepted ledger. It is what the ledger says and not what the machine says: a presence record carries no goals, so a machine nobody has heard from still holds what it claimed.",
  },
  "launch-machine": {
    term: "Launch a machine",
    text: "A new machine of this fleet, on this host: a full clone of this checkout beside it, with its own engine built, this seat's roster and configuration copied, its own nickname, the fleet's ledger fetched, and a steward armed. It starts no session; it joins, publishes presence and waits.",
  },
  engine: {
    term: "Engine",
    text: "The build a machine was running when it last published, and its enrolment generation. Two seats on different builds can read the same ledger by different rules, which is what makes it worth showing.",
  },
  rung: {
    term: "Rung",
    text: "Which way a machine managed to publish its presence at the remote: a metasystem ref, a branch per machine, or a branch per machine without force. A higher rung means the remote refused the one before it.",
  },
  phase: {
    term: "Doing",
    text: "What a machine is doing, and for how long: building, reviewing with the round of its limit, proving with the sections done of those planned, waiting to land, or idle. Stalled is a seat whose card stopped moving, with its stage and the time since its last progress: it made no progress past the stall bound, or the process writing it is gone. A stalled seat is not counted as working, and Needs you says what to do about it. It is read from this computer's board where the board has a card for a goal the machine holds, and otherwise from the job records the machine keeps or published. A dash is a machine this seat has not heard from, so what it is doing is not known here.",
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
      "Words your Project Partner offered for one field of the sheet you are filling in. It is an offer and nothing else: the field keeps what you wrote until you press Use this, and Undo puts back what was there for as long as the field still holds the suggestion and you have not saved it. Every save is a press of yours, and the Partner never saves.",
  },
  proposal: {
    term: "The Partner proposes",
    text:
      "Words your Project Partner has offered for this field, under the field itself. Use this puts them in, whole, and Undo puts back what was there for as long as the field still holds them; Dismiss folds the offer away. Use and save puts them in and sends the sheet in one press, and then says what happened: saved, refused in the ledger's own words, or not confirmed, in which case nothing is sent again. Save at the foot of this sheet is still yours either way. Where more than one has been offered for this field, the newest stands here and the earlier ones unfold under it.",
  },
  "proposed-action": {
    term: "The Partner proposes",
    text: "Acts the Project Partner has named for you to apply. It cannot act itself: each line says which act it would make, on which goal, with every argument it would carry, and your press is the act, under your own sign-in. Tick the ones you want and press Apply; a card you do not answer stays proposed, and Dismiss folds it away.",
  },
  "apply-proposed": {
    term: "Apply",
    text: "Sends the ticked actions in order, one act each, never retried. A refused line says why in the engine's own words and the run goes on; a line whose answer does not say what happened stops the run, and the lines after it say not run until you press Continue with the rest.",
  },
  "unresolved-act": {
    term: "Unresolved",
    text: "The act may have landed and nobody here can say. Nothing is sent again by itself: read the goal, and then press Try again if it did not land. An act that landed but whose authority proof was not recorded says so instead, and offers no Try again at all.",
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
  "the-case": {
    term: "A case at the edge",
    text:
      "A case your Project Partner has put in front of you: the awkward instance a rule has to survive — a page read for an hour without a keystroke, a laptop that slept, a session made before the rule changed. It is not recorded as it stands, because a case is a question and not an entry. Decide settles it: a small sheet opens with the clause as the Partner heard it and a line for your reason, and the decision goes into the record with that reason. Leave open records it as an open question, with what follows from leaving it open. Dismiss leaves nothing behind, which is also an answer.",
  },
  "the-outcome": {
    term: "What this sitting came to",
    text:
      "The closing draft your Project Partner writes when you end a sitting: the outcome as you decided it, the constraints, the open questions with their consequences, and what the table holds. Nothing is weighed and nothing new is settled — it is drafted from the record's own sections and from this conversation. You read it, edit it, and press Record it: it becomes the record's Outcome section, replacing any Outcome the record already had, and the sitting ends then. Ending without recording it is allowed, and it says so: what you recorded during the sitting stays, and the outcome is not written.",
  },
  critique: {
    term: "Critique",
    text:
      "An independent critic's reading of this design, in rounds: a design critique has two, and the second is the failsafe round — there is no third. Each round's findings stand here as cards, read from that round's own return. You decide every one, and Answer the round hands your decisions to the engine, which closes the critique or asks for the next round by its own rules. When it closes, the design gains its Dispositions table at the foot.",
  },
  "send-to-critique": {
    term: "Send to critique",
    text:
      "Starts the configured critique lane on this design, as `metasystem design review` does at a terminal. The goal that funds it must be approved; the reader budget is the most tool calls the critic may make, and the engine refuses a review without one. Sending a design that is already being read rejoins that round rather than buying another.",
  },
  "finding-card": {
    term: "A finding",
    text:
      "One finding of one round: how severe, whether it is material, the claim, and the evidence it rests on. Fold asks your Project Partner for the section the finding names, and Use writes it and records the finding accepted with its amendment. Refute records it refuted with your reason. Defer records a finding that is not material as noted, and a material one as out of scope, with the evidence that it is outside the brief. Each press writes one row of the round's decisions file; nothing a press writes goes into the design.",
  },
  "section-card": {
    term: "A section, drafted anew",
    text:
      "Old and new side by side, the changed lines marked. Use replaces exactly that section of the design, checked against the version you read; a heading that is not in the design, or is in it twice, cannot be told apart and nothing is written. If the design changed since you read it, the section is read again and compared before Use is offered again.",
  },
  "answer-the-round": {
    term: "Answer the round",
    text:
      "Hands the round's decisions to the engine. A design nothing you folded changed is closed; a design a fold changed gets the next round, the failsafe; on the final round the engine closes by its own rules — cleanly, on fixture obligations it publishes on the goal, or it leaves the critique open for a person. What it did is shown in its own words.",
  },
  reviews: {
    term: "Reviews",
    text:
      "A human's review of one goal's built work, kept as a record of its own beside the designs: which goal, which commits were examined, the findings with the answer given to each, and the verdict the review ended with. The interface creates it when you press Review it; what is in it is what you recorded.",
  },
  "review-room": {
    term: "The review room",
    text:
      "The review of one goal's work before it lands, in four steps: what you are looking at, what the reviewer found, try it, and your verdict. The reviewer is your Project Partner: it was not in the room that shaped this work; it reads the goal, the change, the critics' records and the proof, and it recommends one decision per finding. The verdict is yours. Leave whenever you like: everything stays as it is, and the goal's card on the board is the way back.",
  },
  "landing-gate": {
    term: "The landing gate",
    text:
      "Whether a goal in Review lands by itself or waits for you. Below the tier set in Settings (Waits for a person from tier) it becomes eligible once the time set there (Lands by itself after) has passed since it reached Review or since the last thing a person did on it, and the seat that holds it lands it on its next turn; the card counts the time down and says when it is only waiting for that seat. At or above that tier it lands only on your word at its branch's current tip: a review sitting that ends clear to land, or Land without a sitting with your reason, which the Decide sheet asks for. While a review sitting stands, nothing lands, whatever the tier, and the card says whose sitting holds it.",
  },
  "the-desk": {
    term: "The desk",
    text:
      "The candidate's own code, not this checkout's: a file at the lines under discussion with the changed ones marked, the change as a whole with each file's counts, one file's diff, or a record's section. The strip above it lists everything that has been on the desk, newest first, and pressing one brings it back. Select lines to Ask about them, to make a Finding anchored where they are, or to write a Remark: a private sticky on those lines at the commit the desk read them at, which stays on the board and leaves the lines once the desk reads other code. The evidence puts the screenshots and reports the build recorded on the desk, and a drawing your Partner makes can be put here and kept in the record.",
  },
  "the-finding": {
    term: "A finding",
    text:
      "Something the reviewer raises about this version, in layers: how much it matters (Blocks landing, Worth fixing, or a Note), the problem in plain words, why it matters, and the decision the reviewer recommends, with the evidence folded underneath. Your decision is one of four, each saying what follows: Must fix before landing goes to the builder as a correction when you send the goal back; Fix after landing opens a follow-up goal; Not a problem records your reason; I accept this risk records your acceptance and your reason on this review, in your name. A finding you leave undecided follows the reviewer's recommendation when you give your verdict, and you are told so first.",
  },
  "the-walks": {
    term: "The walks",
    text:
      "Five questions the interface asks the reviewer in your name: what was asked, what was built, how it was examined, how it was proven, and how it behaves. Each answer opens in the conversation, and what it points at opens beside it.",
  },
  "the-verdict": {
    term: "The verdict",
    text:
      "How this review ends. Looks good, land it records your verdict on the goal, and the seat that holds it lands it on its next turn; where a finding must still be fixed, you are first shown what landing means for it and asked for your reason. Send it back gives the builder your must-fix decisions as a correction, and the goal leaves Review until it comes back fixed. Either is bound to the version on screen: if the review changed before it was recorded, nothing is recorded and you decide again. End without a verdict ends the sitting and releases your hold; the goal still waits for a decision to land.",
  },
  "the-candidate": {
    term: "Try it",
    text:
      "This exact version of the goal's work, started on this computer on a port of its own, so you can click through it before you decide. Start runs it, Open it takes you there in a new tab, and Stop stops it. If what runs is not the version you are reviewing, the line says so.",
  },
  "the-brief": {
    term: "The correction brief",
    text:
      "What Send it back gives the builder: each finding that must be fixed before landing, with why it matters, where it sits and what the reviewer found, and your own words where you wrote what must change. It is published beside the review, and the builder revises from it once.",
  },
  "the-board": {
    term: "The board",
    text:
      "The working material this sitting has put into its record, pile by pile: Facts, Proposals, Decisions and Open questions for a sitting that shapes a record, Facts, Findings, Decisions and Open questions for a review. It is not a second store — it is those sections of the record itself, so everything here is something you recorded and anything you did not record is not here. An anchor puts what it names back on the desk. Beside the piles stand your Remarks, each saying where it was made, with Record as a fact and, in a review, Make a finding; and the Drawings the record keeps, each with the question it was drawn for.",
  },
  sittings: {
    term: "Sittings",
    text:
      "The records this project has sat on, newest first: what each sitting recorded into its record, how much of each pile it holds, when its last entry was written, and whether a sitting stands on it right now. It is a view over those records and not a second store — a record is here because its own entries say a sitting put them there. Opening a row takes you into that sitting's room, and starts a sitting on the record where none stands; the press is your consent to that.",
  },
  "no-recommendation": {
    term: "No recommendation",
    text:
      "Your Project Partner lays out options with their consequences and does not say which to pick. That is deliberate: when every option comes pre-weighed, choosing the recommended one every time is approval by habit, and the values in this project are yours. Ask it to weigh them and it will; unasked, it brings the cases at the edge, the conflicts and the consequences, and leaves the choice where it belongs.",
  },
  "sitting-room": {
    term: "The room for a sitting",
    text:
      "Where you shape an intent or a design with your Project Partner beside you. The desk on the left shows one thing at a time — a section of the record, or the code as this checkout has it today — and the conversation stays on the right. The Partner brings what the records hold and points at the code; it lays out the cases and never says which way to decide. Step out whenever you like; the room keeps the desk, the conversation and your unfinished words, and the record's page becomes the door back.",
  },
  "shaping-desk": {
    term: "The desk",
    text:
      "The record you are shaping, section by section, and the code as this checkout has it now — the same files your Partner reads, uncommitted edits included. Nothing on it is pinned: when you come back it reads its item again, so it shows what the code does today. The strip above it lists everything that has been on the desk, and pressing one brings it back. Select lines to Ask about them or to make a Fact anchored where they are.",
  },
  "shaping-walks": {
    term: "The walks",
    text:
      "Four questions the interface asks your Partner in your name: Records, what earlier rulings, decisions and open questions touch this subject; Today, what the code does now, with the lines on the desk; Cases, the cases at the edge, each a card you decide or leave open; Open, what this sitting has not settled and what would settle it. None of them weighs anything for you.",
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
