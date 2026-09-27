import type { Proposal, ProposalRead, ProposalState } from "./api";
import { BacklogError, type Budget, type Row } from "../backlog/api";
import { LANDED_NOT_RECORDED, outcomeOf, type GoalEdit } from "../backlog/editing";
import { prefillFor, type BudgetSource } from "../backlog/moves";
import { derivedTier, type Answer, type NewGoal, type Risk } from "../backlog/opening";

/**
 * What the human may do with the acts the Partner proposed, and when.
 *
 * The Partner cannot act. It has no session, no cookie and no way to obtain one,
 * and the act routes refuse a cookie-less request while a Partner is configured.
 * What it can do is name one of this interface's ten acts with its arguments;
 * the card renders it, the human ticks what they want, and their press is the
 * act. This file is the whole of the rules around that press — what a line says,
 * which lines may be sent, what the run does with each answer, and what the card
 * offers afterwards — so that every one of them can be read, and tested, without
 * a browser.
 *
 * Four rules here are the ones a human would notice if they were wrong.
 *
 * **A refusal is passed; an unknown answer stops.** A refusal is a no with
 * nothing behind it, so the run goes on to the next line. An answer that does not
 * say what happened may have landed, and the next line may depend on it, so the
 * run stops there and the rest say "not run".
 *
 * **Nothing is sent twice.** A line is sent at most once per press. A line whose
 * answer said the act landed but its proof was not recorded is recorded as
 * applied and offers no Try again at all: sending it again would make a second
 * approval record rather than repair the first one's missing proof. And a line
 * another tab left at `applying` is reconciled before it is sent: a goal that
 * already carries the act is recorded applied with nothing sent for it.
 *
 * **What was read is what is approved.** An approve and an edit carry the goal as
 * the Partner read it. Before the first line the page reads the canonical branch
 * once; a line whose goal has moved since, or whose budget would now differ from
 * the one on the card, is refused on the line without being sent.
 *
 * **The version decides who writes.** Every press sends the version of the entry
 * it last rendered. A write the server refuses with the entry means somebody else
 * moved that line, and the runner then shows what they did rather than sending
 * an act of its own.
 */

/* ------------------------------------------------------------- the lines -- */

/** What identifies one line: the turn it arrived in, and its place in it. */
export function lineID(turn: string, index: number): string {
  return `${turn}#${String(index)}`;
}

/** What identifies one card: the answer it is under. */
export function cardID(turn: string): string {
  return turn;
}

/**
 * The budget one approve line displayed, and where it came from.
 *
 * It is kept with the line rather than computed at the press, and that is the
 * whole of Astra's S58-08: the card shows one tuple and a fresh prefill can
 * answer another — a goal's own budget changed without its intent, next step,
 * tier or labels changing — so a press that confirmed A would submit B. What the
 * run sends is this, and a fresh prefill that differs refuses the line unsent.
 */
export type Displayed = { budget: Budget | null; source: BudgetSource };

/** Every approve line's displayed tuple, by the line's own id. */
export type Displayeds = Readonly<Record<string, Displayed>>;

/** What the page holds about one line beyond what the server persists. */
export type Mark = {
  /** Whether the human has this line ticked. Every offered line starts ticked. */
  ticked: boolean;
  /**
   * True for a line a stopped run never reached. It is the page's own state and
   * not the server's: nothing was sent, so there is nothing to record.
   */
  notRun: boolean;
  /** Why this line will not be sent, in words, or "" where it will be. */
  refusedUnsent: string;
  /**
   * What the act answered where the conversation could not write it down, and
   * null where there is nothing of the kind.
   *
   * It is kept apart from every other mark because the act's answer WINS. A
   * write that failed after the act is a failure to record, not a failure to
   * act: the approval landed, and a line that said only "the conversation could
   * not record this" and offered Try again invited a second one (Sol S58-C-02).
   * So the answer is held here, the line reads as that answer, and what the line
   * offers is decided by it.
   */
  unrecorded: { state: ProposalState; words: string } | null;
};

export type Marks = Readonly<Record<string, Mark>>;

const TICKED: Mark = { ticked: true, notRun: false, refusedUnsent: "", unrecorded: null };

export function markOf(marks: Marks, id: string): Mark {
  return marks[id] ?? TICKED;
}

/** One line of a card: the action, what the page holds about it, and its words. */
export type Line = Proposal & {
  id: string;
  mark: Mark;
  /** The tuple this line displays, on an approve, and null on everything else. */
  displayed: Displayed | null;
};

/** One card: the answer's actions, in the order they were proposed. */
export type Card = {
  id: string;
  turn: string;
  lines: readonly Line[];
  /** Folded to one line by Dismiss, or by a newer answer standing under it. */
  folded: boolean;
  /** True on the newest answer that carries a card, which never folds itself. */
  newest: boolean;
};

/** One turn's proposals, as the transcript and the running turn carry them. */
export type Carried = { turn: string; proposals: readonly Proposal[] };

/**
 * Every card this conversation carries, oldest first.
 *
 * `folded` is the presentation rule of D8, and it is the only thing in here that
 * depends on the other cards: a card of any answer but the newest that still has
 * a waiting line folds to one line, because an old Apply must not be the first
 * thing in view and a conversation must not bury a decision. A card the human
 * dismissed folds too, and a card with nothing waiting is read as it stands —
 * folding a finished card would be hiding the record of what was done.
 */
export function cardsIn(
  carried: readonly Carried[],
  marks: Marks,
  displayed: Displayeds,
  dismissedCards: readonly string[],
): readonly Card[] {
  const held = carried.filter((one) => one.turn !== "" && one.proposals.length > 0);
  const newest = newestWaiting(held, marks);
  return held.map((one) => {
    const lines = one.proposals.map((proposal) => {
      const id = lineID(one.turn, proposal.index);
      return {
        ...proposal,
        id,
        mark: markOf(marks, id),
        displayed: proposal.verb === "approve-goal" ? (displayed[id] ?? null) : null,
      };
    });
    const isNewest = one.turn === newest;
    return {
      id: cardID(one.turn),
      turn: one.turn,
      lines,
      folded: dismissedCards.includes(one.turn) || (!isNewest && waitingIn(lines) > 0),
      newest: isNewest,
    };
  });
}

/** The newest answer that carries a card with a waiting line, or "". */
function newestWaiting(carried: readonly Carried[], marks: Marks): string {
  for (let at = carried.length - 1; at >= 0; at -= 1) {
    const lines = carried[at].proposals.map((proposal) => ({
      ...proposal,
      id: lineID(carried[at].turn, proposal.index),
      mark: markOf(marks, lineID(carried[at].turn, proposal.index)),
      displayed: null,
    }));
    if (waitingIn(lines) > 0) {
      return carried[at].turn;
    }
  }
  return "";
}

/** How many lines of this card are still waiting for the human. */
export function waitingIn(lines: readonly Line[]): number {
  return lines.filter((line) => waiting(line)).length;
}

/** True for a line the human has still not answered. */
export function waiting(line: Line): boolean {
  return line.offered && line.state === "waiting";
}

/** One card by its id, or nothing where the conversation has no such card. */
export function cardIn(cards: readonly Card[], id: string): Card | undefined {
  return cards.find((card) => card.id === id);
}

/** The lines this card would send: ticked, offered, waiting and sendable. */
export function sendable(card: Card): readonly Line[] {
  return card.lines.filter((line) => waiting(line) && line.mark.ticked && excludedFor(line) === "");
}

/** How many lines are ticked, which is what Apply counts. */
export function ticked(card: Card): number {
  return sendable(card).length;
}

/**
 * Why this line cannot be sent at all, or "".
 *
 * One reason today, and it is the bulk sheet's own: an approve whose budget the
 * interface cannot prefill has five empty fields, and nothing here invents a
 * budget. It is listed and excluded rather than dropped, so the human sees it
 * named and approves it alone where the sheet can ask them for the tuple.
 */
export function excludedFor(line: Line): string {
  if (line.verb !== "approve-goal") {
    return "";
  }
  return line.displayed?.budget == null ? NEEDS_ITS_BUDGET : "";
}

export const NEEDS_ITS_BUDGET = "needs its budget first: approve it alone";

/* ------------------------------------------------------------- the words -- */

/**
 * The word on the line: the goal action's public name under the `goal` object.
 *
 * It is the word `metasystem goal ACTION` uses and the word the pages' buttons
 * now use, so the card, the rows, the chip, the buttons and the terminal say one
 * thing for one act. The route id is what the message persists and the runner
 * dispatches on, which is why a word changed here touches no persisted proposal.
 */
const WORD: Readonly<Record<string, string>> = {
  "open-goal": "Open",
  "approve-goal": "Approve",
  "withdraw-goal": "Unapprove",
  "set-goal-priority": "Prioritize",
  "block-goal": "Block",
  "unblock-goal": "Unblock",
  "park-goal": "Pause",
  "unpark-goal": "Resume",
  "edit-goal": "Edit",
  "abandon-goal": "Abandon",
};

export function verbWord(verb: string): string {
  return WORD[verb] ?? verb;
}

/** Every act a card can carry, for a reader that needs the whole list. */
export const VERB_WORDS = WORD;

/** One labelled argument of a line, as the card shows it. */
export type Argument = { label: string; value: string };

/**
 * Every argument this act will carry, whole, in the words the sheets use.
 *
 * It is rendered by the interface from the validated action and never from the
 * Partner's prose: the Partner's own words are its "why", shown separately as
 * its words. An approve shows the intent and the next step whole from what was
 * read, as the Decisions open row does before its Approve, and the budget it
 * would carry with where that budget came from. An abandon shows the intent
 * whole from what was read too, then the reason, the successor where one was
 * named, and what becomes of the goals that wait for the one being abandoned,
 * which is the sentence below.
 */
export function argumentsOf(line: Line): readonly Argument[] {
  const said: Argument[] = [];
  const fields = line.fields ?? {};
  const add = (label: string, value: string | undefined) => {
    if (value !== undefined && value.trim() !== "") {
      said.push({ label, value });
    }
  };
  switch (line.verb) {
    case "open-goal":
      add("Intent", fields.intent);
      add("First next step", fields.nextStep);
      said.push({ label: "Tier", value: tierWords(fields) });
      add("Basis", fields.basis);
      add("Labels", fields.labels);
      add("Waits for", fields.blockedBy);
      add("Holds up", fields.blocks);
      break;
    case "approve-goal":
      add("Intent", line.read?.intent);
      add("Next step", line.read?.nextStep);
      said.push({ label: "Budget", value: budgetWords(line) });
      break;
    case "edit-goal":
      add("Intent", fields.intent);
      add("Next step", fields.nextStep);
      add("Labels", fields.labels === "" ? "none" : fields.labels);
      break;
    case "withdraw-goal":
      add("Reason", fields.reason);
      break;
    case "park-goal":
      add("Reason", fields.because);
      break;
    case "abandon-goal":
      // The intent whole, first, from what the Partner read — as an approve's
      // line shows it, and for a stronger reason: the heading above shows the
      // TITLE, which is the intent cut at its first sentence, and this press
      // ends the goal for good. A human deciding that reads the goal's own
      // account of itself and not an abbreviation of it (g1-s64 D4, Sol
      // S64-C-01).
      add("Intent", line.read?.intent);
      add("Reason", fields.because);
      add("Successor", fields.successor);
      add("Holds up", dependentsWords(line));
      break;
    case "set-goal-priority":
      add("Priority", fields.priority);
      add("Position", fields.sequence);
      break;
    case "block-goal":
    case "unblock-goal":
      add("The goal it waits for", fields.blocker);
      break;
    default:
      break;
  }
  return said;
}

/**
 * What an abandon line says about the goals that wait for the goal, or "" where
 * the read carries none.
 *
 * Both halves of the sentence are the ENGINE's, not this page's. With a successor
 * the engine does not refuse a goal with live dependents: it repoints every one
 * of them at the successor inside the same act, so the line says where they will
 * wait instead. Without one it refuses the abandon until each dependent has been
 * waived or abandoned, neither of which a browser can do, so the line says so
 * before the press rather than letting the human learn it from a refusal
 * (g1-s64 D2, Astra S64-02).
 *
 * The dependents are the ones the Partner read, carried on the line — which is
 * the only reason the sentence can be said at all, since no record stores the
 * other direction of the relation. A goal nothing waits for carries no list, and
 * says nothing: a line reading "0 goals wait for it" would be an argument about
 * an absence.
 */
export function dependentsWords(line: Line): string {
  const dependents = line.read?.dependents ?? [];
  if (dependents.length === 0) {
    return "";
  }
  const successor = (line.fields?.successor ?? "").trim();
  // "1 goal waits for it" and "2 goals wait for it": the verb agrees with the
  // count, because the line is read as a sentence and not as a label.
  const wait =
    dependents.length === 1 ? "1 goal waits for it" : `${String(dependents.length)} goals wait for it`;
  const becomes =
    successor === ""
      ? "the engine will refuse until they are waived or abandoned at a terminal"
      : `they will wait for ${successor} instead`;
  return `${wait}: ${dependents.join(", ")} — ${becomes}`;
}

/**
 * The tier, derived on the card by the new-goal sheet's own function, with the
 * two answers it came from and the two that scale the proof.
 *
 * The Partner names no tier: it answers the four questions and the interface
 * derives what they imply, exactly as the sheet does, and sends what it derived.
 */
export function tierWords(fields: Readonly<Record<string, string>>): string {
  const risk = riskOf(fields);
  return (
    `${String(derivedTier(risk))}, from severity ${risk.severity} · novelty ${risk.novelty} · ` +
    `exposure ${risk.exposure} · accumulation ${risk.accumulation}`
  );
}

/** The four answers, as the sheet's own type carries them. */
export function riskOf(fields: Readonly<Record<string, string>>): Risk {
  return {
    severity: answerOf(fields.severity),
    novelty: answerOf(fields.novelty),
    exposure: answerOf(fields.exposure),
    accumulation: answerOf(fields.accumulation),
    basis: fields.basis ?? "",
  };
}

function answerOf(said: string | undefined): Answer {
  return said === "2" || said === "3" ? said : "1";
}

/** The words the approve sheet uses for where a prefilled budget came from. */
const SOURCE_WORDS: Readonly<Record<BudgetSource, string>> = {
  goal: "the tuple this goal already carries",
  project: "the project's budget law for this goal's tier",
  "last-approved": "the goal approved most recently",
  none: "nothing: this project declares no budget law and no goal has been approved yet",
};

/**
 * The budget this approve line would carry, in the sheet's five words, with
 * where it came from — or the sentence that says it has none.
 */
export function budgetWords(line: Line): string {
  const displayed = line.displayed;
  if (displayed == null || displayed.budget === null) {
    return NEEDS_ITS_BUDGET;
  }
  const budget = displayed.budget;
  return (
    `${budget.elapsedLimit} elapsed · ${String(budget.attemptLimit)} attempts · ` +
    `${String(budget.reservedJobMinutesLimit)} reserved job-minutes · ` +
    `${String(budget.activeJobLimit)} active jobs · ${String(budget.reviewRoundLimit)} review rounds, ` +
    `from ${SOURCE_WORDS[displayed.source]}`
  );
}

/** What the card heads itself with: how many actions this answer proposed. */
export function cardHead(card: Card): string {
  const offered = card.lines.filter((line) => line.offered).length;
  return offered === 1 ? "The Partner proposes" : `The Partner proposes ${String(offered)} actions`;
}

/** The one line an older card folds to, which reopens it. */
export function foldedLine(card: Card): string {
  const waitingLines = waitingIn(card.lines);
  return waitingLines === 0
    ? "The Partner proposed earlier"
    : `The Partner proposed earlier · ${String(waitingLines)} waiting`;
}

/** What the foot counts, once anything has happened. */
export function footLine(card: Card): string {
  const counted: string[] = [];
  for (const [state, word] of [
    ["applied", "applied"],
    ["refused", "refused"],
    ["unresolved", "unresolved"],
    ["dismissed", "dismissed"],
  ] as const) {
    const many = card.lines.filter((line) => line.offered && line.state === state).length;
    if (many > 0) {
      counted.push(`${String(many)} ${word}`);
    }
  }
  const notRun = card.lines.filter((line) => line.mark.notRun).length;
  if (notRun > 0) {
    counted.push(`${String(notRun)} not run`);
  }
  return counted.join(" · ");
}

/**
 * What one line says about where it stands.
 *
 * `inFlight` is whether THIS page is running the line right now. It is the whole
 * difference between two things that look the same in the record: a line at
 * `applying` while a run is going is in flight, and the same line found at
 * `applying` by a page that has just loaded is one a page went away in the middle
 * of. The second says so, and never says fresh — an approve applied twice is two
 * approval records, so a card that had forgotten would invite the press that
 * makes one.
 */
export function lineState(line: Line, inFlight = false): string {
  if (!line.offered) {
    return NOT_OFFERED;
  }
  // The act's own answer, where the conversation could not write it down. It is
  // read before everything else because it is the only thing here that knows
  // what the LEDGER did; the persisted line is still at `applying`, and saying
  // so would hide the act that landed.
  if (line.mark.unrecorded !== null) {
    return `${said(line.mark.unrecorded.state, line.mark.unrecorded.words)}; ${COULD_NOT_RECORD}`;
  }
  if (line.mark.refusedUnsent !== "") {
    return line.mark.refusedUnsent;
  }
  if (line.mark.notRun) {
    return NOT_RUN;
  }
  if (line.state === "applying") {
    return inFlight ? APPLYING : WAS_IN_FLIGHT;
  }
  return said(line.state, line.words ?? "");
}

/** One settled state in the words a human reads, with what it carried. */
function said(state: ProposalState, carried: string): string {
  // The words are absent on a plain outcome and present on one that carried a
  // sentence; the server leaves an empty one out, so both read as none here.
  const words = carried.trim();
  switch (state) {
    case "applied":
      return words === "" ? APPLIED : `applied; ${words}`;
    case "refused":
      return `refused: ${words}`;
    case "unresolved":
      return `unresolved: ${words}; check the goal before trying again`;
    case "dismissed":
      return DISMISSED;
    default:
      return "";
  }
}

export const NOT_OFFERED = "Not offered";
export const NOT_RUN = "not run";
export const APPLYING = "applying…";
export const APPLIED = "applied";
export const DISMISSED = "dismissed";

/**
 * What a line left at `applying` by a page that went away says.
 *
 * It never says fresh, and that is the whole of why the state is persisted: an
 * approve applied twice is two approval records, so a card that had forgotten it
 * was applied would invite the one press that writes one.
 */
export const WAS_IN_FLIGHT = "was being applied when the page left; check the goal before applying again";

/** The buttons. They are written once because the card and its tests say them. */
export const APPLY = "Apply";
export const SELECT_ALL = "Select all";
export const DISMISS = "Dismiss";
export const TRY_AGAIN = "Try again";
export const ASK_THE_PARTNER = "Ask the Partner";
export const CONTINUE = "Continue with the rest";
export const OPEN_THE_GOAL = "Open the goal";
export const SIGN_IN_TO_APPLY = "Sign in to apply";
export const COULD_NOT_RECORD = "the conversation could not record this";
/** What the line says when a run stopped because that write could not be made. */
export const COULD_NOT_START = "not applied: the conversation could not record that it was being applied";

/** What Apply says, with how many it would send. */
export function applyLabel(card: Card): string {
  const many = ticked(card);
  return many <= 1 ? APPLY : `${APPLY} ${String(many)}`;
}

/** What the foot says above the buttons while the human is still choosing. */
export function selectedLine(card: Card): string {
  return `${String(ticked(card))} selected`;
}

/**
 * Whether this line offers Try again.
 *
 * A refused line does, because a refusal is a no with nothing behind it. An
 * unresolved line does, because the human has read the goal and decided. A line
 * whose answer said the act LANDED offers none at all: sending it again would
 * make a second act rather than repair the first one's missing proof
 * (Astra S58-05).
 */
export function offersTryAgain(line: Line, inFlight = false): boolean {
  if (!line.offered) {
    return false;
  }
  // An act whose answer could not be written down offers what that answer
  // offers. An applied one offers nothing: the act landed, and a second press
  // would make a second act rather than repair the record (Sol S58-C-02).
  if (line.mark.unrecorded !== null) {
    return line.mark.unrecorded.state !== "applied";
  }
  if (line.mark.refusedUnsent !== "") {
    return true;
  }
  // A line a page went away in the middle of offers it too, and for the same
  // reason the words beside it give: check the goal, and then press. A line THIS
  // page is running does not — it is in flight.
  if (line.state === "applying") {
    return !inFlight;
  }
  return line.state === "refused" || line.state === "unresolved";
}

/**
 * Whether this line can still be put away.
 *
 * Every state a line can be moved OUT of: waiting, and the one a page went away
 * in the middle of. A line left at `applying` with nothing offering to settle it
 * would otherwise stay on the card for good, which is why the outcome route
 * admits `applying` to `dismissed` at all.
 */
export function dismissable(line: Line): boolean {
  return line.offered && (line.state === "waiting" || line.state === "applying");
}

/** How many lines of this card can still be put away. */
export function dismissableIn(lines: readonly Line[]): number {
  return lines.filter((line) => dismissable(line)).length;
}

/** Whether the card offers Continue with the rest: a run stopped short. */
export function offersContinue(card: Card): boolean {
  return card.lines.some((line) => line.mark.notRun);
}

/**
 * What "Ask the Partner" puts in the composer: the verb, the subject and what
 * was said, in the words a human would have typed.
 */
export function askLine(line: Line): string {
  const said = lineState(line);
  return `About your proposed ${verbWord(line.verb)} on ${line.goal} — ${said}. What else could be done?`;
}

/* --------------------------------------------------------------- the run -- */

/** What one sent line came back as, in the four answers the runner tells apart. */
export type Answered =
  | { kind: "applied"; words: string }
  | { kind: "refused"; words: string }
  | { kind: "unresolved"; words: string }
  | { kind: "sign-in"; words: string };

/**
 * What a failed act means to the runner.
 *
 * It is the page's one mapping, `outcomeOf`, with two things the runner needs
 * that a save did not: a refusal a sign-in would remedy is its own answer,
 * because it ends the run at that line and opens the sheet; and the answer that
 * says the act landed and its proof did not is recorded as APPLIED with its own
 * words, because that is what it says.
 */
export function answeredOf(error: unknown): Answered {
  if (error instanceof BacklogError && error.signIn) {
    return { kind: "sign-in", words: NOT_APPLIED_SIGN_IN };
  }
  if (error instanceof BacklogError && error.code === LANDED_NOT_RECORDED) {
    return { kind: "applied", words: error.message };
  }
  const outcome = outcomeOf(error);
  return outcome.kind === "refused"
    ? { kind: "refused", words: outcome.words }
    : { kind: "unresolved", words: outcome.words };
}

export const NOT_APPLIED_SIGN_IN = "not applied: sign in to apply";

/**
 * Whether the run goes on to the next ticked line after this answer.
 *
 * A refusal is passed, because nothing landed behind it and the next line does
 * not depend on it. An unresolved answer stops the run, because the act may have
 * landed and the next line may depend on it. A sign-in refusal stops it too: the
 * run never waits on a sheet, so the line says what it needs and the rest say
 * "not run" until the human signs in or presses Continue (g1-s58 D7).
 */
export function goesOn(answered: Answered): boolean {
  return answered.kind === "applied" || answered.kind === "refused";
}

/**
 * The state one answer writes onto its line, and the words with it.
 *
 * A sign-in refusal is written as `refused`: nothing was published, which is
 * what refused means, and the words say what the human does about it.
 */
export function recorded(answered: Answered): { state: ProposalState; words: string } {
  return {
    state: answered.kind === "sign-in" ? "refused" : answered.kind,
    words: answered.words,
  };
}

/**
 * Whether the run may compare against what it read.
 *
 * The page reads the canonical branch once before the first line, and the read
 * answers 200 from the accepted ledger whether or not that fetch succeeded. Only
 * `advanced` or `current` means the compare below is against the canonical branch
 * as of the press; on `failed` every line that needs a compare is refused unsent
 * with the fetch's own message, while the lines that need none run as they would
 * (Astra S58-07).
 */
export function canCompare(outcome: string): boolean {
  return outcome === "advanced" || outcome === "current";
}

/**
 * Which acts depend on what the goal says, and so need the compare.
 *
 * An abandon is the third, and it is here because it is the one act on this list
 * a press cannot undo: `goal reopen` is a terminal act, so a stale card must not
 * record that a goal will never be worked when its intent, next step, tier,
 * labels or live dependents have moved in another tab since the Partner read it
 * (g1-s64 D4, Astra S64-01). An approve and an edit are refused for the same
 * reason and were the only two before it.
 */
export function needsTheCompare(verb: string): boolean {
  return verb === "approve-goal" || verb === "edit-goal" || verb === "abandon-goal";
}

export function fetchFailedLine(message: string): string {
  return `the canonical branch could not be read: ${message}; try again`;
}

export const GOAL_CHANGED = "the goal changed since this was proposed; open it and ask again";

/**
 * Why this line will not be sent, in words, or "" where it will be.
 *
 * Four refusals, all of them unsent. The fetch that could not read the canonical
 * branch, for a line that depends on what the goal says. The goal whose intent,
 * next step, tier or labels differ from what the Partner read. The budget that a
 * fresh prefill now answers differently from the one the card displayed — which
 * is the press confirming one tuple and the run sending another. And, on an
 * abandon, the live dependents that are no longer the ones the card named: a goal
 * that has grown a dependent since is an abandon the engine would refuse or
 * repoint, and a goal that has lost one is an act the human read as something
 * else (g1-s64 D4).
 */
export function guardFor(
  line: Line,
  rows: readonly Row[],
  defaults: Partial<Record<string, Budget>>,
  fetchOutcome: string,
  fetchMessage: string,
): string {
  if (!needsTheCompare(line.verb)) {
    return "";
  }
  if (!canCompare(fetchOutcome)) {
    return fetchFailedLine(fetchMessage);
  }
  const row = rows.find((one) => one.ref.id === line.goal);
  if (row === undefined) {
    return GOAL_CHANGED;
  }
  const read = line.read;
  if (read === null || read === undefined || moved(read, row)) {
    return GOAL_CHANGED;
  }
  if (line.verb === "approve-goal" && !sameBudget(line.displayed, prefillFor(row, defaults, rows))) {
    return GOAL_CHANGED;
  }
  // The dependents are not a field of the row: they are the other direction of
  // the blocked relation, computed from the whole reading, which is why they are
  // compared here beside the budget rather than inside the field-by-field move
  // below — and why an approve and an edit still compare exactly what they
  // compared before.
  if (line.verb === "abandon-goal" && named(read.dependents) !== named(liveDependentsIn(rows, line.goal))) {
    return GOAL_CHANGED;
  }
  return "";
}

/**
 * The goals that wait for this one at the tip: the live rows whose own blockers
 * name it, in id order.
 *
 * The reading a run takes carries the closed rows beside the live ones, so the
 * live ones are picked out here by their own `where`: a goal already done or
 * abandoned waits for nothing, and counting it would refuse an abandon the engine
 * would have admitted. It is the same selection the engine makes inside the
 * transaction, which is the whole reason a compare against it means anything.
 */
export function liveDependentsIn(rows: readonly Row[], goal: string): string[] {
  return rows
    .filter((one) => one.where === "live" && one.blockedBy.includes(goal))
    .map((one) => one.ref.id)
    .sort();
}

/** One list of names as one string, for a compare that is about the whole list. */
function named(names: readonly string[] | undefined): string {
  return (names ?? []).join(" ");
}

/** Whether the goal has changed in any way the approve, the edit or the abandon is about. */
function moved(read: ProposalRead, row: Row): boolean {
  return (
    read.intent !== row.intent ||
    read.nextStep !== row.nextStep ||
    read.tier !== row.tier ||
    read.labels.join(" ") !== row.labels.join(" ")
  );
}

/** Whether the tuple the card displayed is the tuple a fresh prefill answers. */
function sameBudget(displayed: Displayed | null, fresh: Displayed): boolean {
  if (displayed === null) {
    return false;
  }
  return displayed.source === fresh.source && sameTuple(displayed.budget, fresh.budget);
}

/** Whether two budgets are the same five limits. Nothing is never a match. */
function sameTuple(one: Budget | null, other: Budget | null): boolean {
  if (one === null || other === null) {
    return false;
  }
  return (
    one.elapsedLimit === other.elapsedLimit &&
    one.attemptLimit === other.attemptLimit &&
    one.reservedJobMinutesLimit === other.reservedJobMinutesLimit &&
    one.activeJobLimit === other.activeJobLimit &&
    one.reviewRoundLimit === other.reviewRoundLimit
  );
}

/**
 * Whether the goal already carries what this line would do.
 *
 * It is asked of a line found at `applying` and of no other. Such a line was
 * begun by somebody — this page before it went away, or another tab whose write
 * this page learned — and the act may well have landed already. Learning that
 * must not itself authorise a fresh one: an approve applied twice is two approval
 * records, and the engine suppresses a duplicate proven approval but not a
 * session one, so nothing behind the browser would catch it (Astra A-01). The
 * press therefore looks in the reading the run took, and a goal that shows the
 * act's own effect settles the line without a send.
 *
 * Every effect below is the one that act's route writes, so a goal that shows it
 * is a goal the act reached. Two acts are absent and fall through to the send: an
 * open, whose effect is a goal that exists, and an unapprove, whose effect is an
 * approval that is gone. Both are refused by the engine the second time — the
 * goal already exists, the goal is not approved — so a second press answers with
 * a refusal rather than with a second act, which is the harm this is about.
 */
export function carriesAlready(line: Line, rows: readonly Row[]): boolean {
  const row = rows.find((one) => one.ref.id === line.goal);
  if (row === undefined) {
    return false;
  }
  const fields = line.fields ?? {};
  switch (line.verb) {
    // Approved, and with the tuple this line displayed: an approval carrying
    // another budget is not this act's effect, and the guard above refuses such a
    // line on its own because a fresh prefill now answers differently.
    case "approve-goal":
      return row.approved !== undefined && sameTuple(line.displayed?.budget ?? null, row.budget ?? null);
    // Every field the edit names, as the goal now reads. A field the action does
    // not carry is one it says nothing about, so it is not compared.
    case "edit-goal":
      return (
        (fields.intent === undefined || fields.intent === row.intent) &&
        (fields.nextStep === undefined || fields.nextStep === row.nextStep) &&
        (fields.labels === undefined || namesIn(fields.labels).join(" ") === row.labels.join(" "))
      );
    case "park-goal":
      return row.state === "parked";
    case "unpark-goal":
      return row.state !== "parked";
    case "set-goal-priority":
      return (
        row.priority === Number(fields.priority ?? "0") &&
        (fields.sequence === undefined || fields.sequence === "" || row.sequence === Number(fields.sequence))
      );
    case "block-goal":
      return row.blockedBy.includes(fields.blocker ?? "");
    case "unblock-goal":
      return !row.blockedBy.includes(fields.blocker ?? "");
    case "abandon-goal":
      return row.state === "abandoned";
    default:
      return false;
  }
}

/** What a line settled without a send says: the act was there to be found. */
export const ALREADY_CARRIED = "the goal already carries this, so nothing was sent";

/** The tuple each approve line would display, read once at the terminal beat. */
export function displayedFor(
  lines: readonly Line[],
  rows: readonly Row[],
  defaults: Partial<Record<string, Budget>>,
): Displayeds {
  const held: Record<string, Displayed> = {};
  for (const line of lines) {
    if (line.verb !== "approve-goal" || !line.offered) {
      continue;
    }
    const row = rows.find((one) => one.ref.id === line.goal);
    held[line.id] = row === undefined ? { budget: null, source: "none" } : prefillFor(row, defaults, rows);
  }
  return held;
}

/* ---------------------------------------------------------- the dispatch -- */

/** Which act one line sends, and with what. */
export type Dispatch =
  | { act: "approve"; id: string; budget: Budget }
  | { act: "withdraw"; id: string; reason: string }
  | { act: "priority"; id: string; priority: number; sequence: number | null }
  | { act: "block"; dependent: string; blocker: string }
  | { act: "unblock"; dependent: string; blocker: string }
  | { act: "park"; id: string; because: string }
  | { act: "unpark"; id: string }
  | { act: "edit"; id: string; edit: GoalEdit }
  | { act: "open"; goal: NewGoal }
  | { act: "abandon"; id: string; because: string; successor: string };

/**
 * What this line sends, composed from the route body's own fields — or null
 * where the line names an act this page cannot dispatch, which is a line the
 * catalogue and the act table would have to have disagreed about.
 *
 * An approve sends the tuple the line DISPLAYED and no other; the guard above is
 * what refuses a line whose fresh prefill differs.
 */
export function dispatchOf(line: Line): Dispatch | null {
  const fields = line.fields ?? {};
  switch (line.verb) {
    case "approve-goal": {
      const budget = line.displayed?.budget ?? null;
      return budget === null ? null : { act: "approve", id: line.goal, budget };
    }
    case "withdraw-goal":
      return { act: "withdraw", id: line.goal, reason: fields.reason ?? "" };
    case "set-goal-priority":
      return {
        act: "priority", id: line.goal,
        priority: Number(fields.priority ?? "0"),
        sequence: fields.sequence === undefined || fields.sequence === "" ? null : Number(fields.sequence),
      };
    case "block-goal":
      return { act: "block", dependent: line.goal, blocker: fields.blocker ?? "" };
    case "unblock-goal":
      return { act: "unblock", dependent: line.goal, blocker: fields.blocker ?? "" };
    case "park-goal":
      return { act: "park", id: line.goal, because: fields.because ?? "" };
    case "unpark-goal":
      return { act: "unpark", id: line.goal };
    case "edit-goal":
      return { act: "edit", id: line.goal, edit: editOf(fields) };
    case "open-goal":
      return { act: "open", goal: openOf(line.goal, fields) };
    // The successor is sent as it was proposed and empty where nobody named one,
    // which is the body saying there is none; what an empty one means to a goal
    // with live dependents is the engine's own rule.
    case "abandon-goal":
      return {
        act: "abandon", id: line.goal,
        because: fields.because ?? "", successor: fields.successor ?? "",
      };
    default:
      return null;
  }
}

/**
 * An edit's body: the fields the action names and no others.
 *
 * A field the action does not carry is not a field set to nothing — it is a
 * field this act says nothing about, which the engine leaves as it found it. The
 * labels are the whole list the route takes, composed by the Partner from the
 * goal as it was read, and an action that carries them empty clears them.
 */
export function editOf(fields: Readonly<Record<string, string>>): GoalEdit {
  const edit: GoalEdit = {};
  if (fields.intent !== undefined) {
    edit.intent = fields.intent;
  }
  if (fields.nextStep !== undefined) {
    edit.nextStep = fields.nextStep;
  }
  if (fields.labels !== undefined) {
    edit.labels = namesIn(fields.labels);
  }
  return edit;
}

/** An open's body: the flat shape the route reads, with the tier derived here. */
export function openOf(id: string, fields: Readonly<Record<string, string>>): NewGoal {
  const risk = riskOf(fields);
  return {
    id,
    intent: fields.intent ?? "",
    nextStep: fields.nextStep ?? "",
    // Derived and sent as derived: the Partner names no tier, and a zero would
    // ask the engine to derive it again from answers it already has.
    tier: derivedTier(risk),
    why: "",
    blocks: namesIn(fields.blocks),
    blockedBy: namesIn(fields.blockedBy),
    labels: namesIn(fields.labels),
    severity: Number(risk.severity),
    novelty: Number(risk.novelty),
    exposure: Number(risk.exposure),
    accumulation: Number(risk.accumulation),
    basis: risk.basis,
  };
}

/** A frame's list field, as the frame writes one: names separated by commas. */
export function namesIn(said: string | undefined): string[] {
  return (said ?? "")
    .split(",")
    .map((name) => name.trim())
    .filter((name) => name !== "");
}

/* ------------------------------------------------------------- the count -- */

/**
 * How many actions are waiting for the human, counted across every answer.
 *
 * It is what the closed bar says beside the composer, and pressing it opens the
 * drawer at the newest card that has one. Across answers, because a proposal the
 * human has not answered is waiting whether or not it is the newest thing said
 * (g1-s58 D8).
 */
export function waitingAcross(cards: readonly Card[]): number {
  return cards.reduce((count, card) => count + waitingIn(card.lines), 0);
}

/** What the bar says, or "" where nothing is waiting. */
export function barLine(cards: readonly Card[]): string {
  const many = waitingAcross(cards);
  return many === 0 ? "" : `${String(many)} action${many === 1 ? "" : "s"} proposed`;
}

/** The newest card with a waiting line, which the bar's press opens at. */
export function newestWaitingCard(cards: readonly Card[]): string {
  for (let at = cards.length - 1; at >= 0; at -= 1) {
    if (waitingIn(cards[at].lines) > 0) {
      return cards[at].id;
    }
  }
  return "";
}

/* ------------------------------------------------------- the run, in order -- */

/** The canonical branch as the run read it once, before anything was sent. */
export type Looked = {
  rows: readonly Row[];
  defaults: Partial<Record<string, Budget>>;
  /** What the server's fetch of the canonical branch did, in its own word. */
  outcome: string;
  /** What it said where it failed. */
  message: string;
};

/**
 * What one write onto one line answered, in the three the run tells apart.
 *
 * `conflict` is somebody else having moved the line first, with the entry as they
 * left it. It is not a failure: it is a fact about the line, and the run's whole
 * reaction to it is to show that entry and send nothing for it.
 */
export type Written =
  | { kind: "written"; proposal: Proposal }
  | { kind: "conflict"; proposal: Proposal }
  | { kind: "failed"; words: string };

/**
 * What the run needs of the world, named so the run itself can be read and
 * tested without one.
 *
 * `record` is the name of the write, and it is the domain's word rather than a
 * synonym chosen to please a guard: what it does is write one state down where
 * the proposal is, and nothing here writes to a document.
 *
 * Everything impure is here: the read, the two writes, the act, the page's
 * re-read and the sign-in sheet. The order they happen in, and what each answer
 * does to the lines after it, is the function below — which is the part a human
 * would notice if it were wrong.
 */
export type RunPorts = {
  look: () => Promise<Looked>;
  record: (line: Line, state: ProposalState, words: string) => Promise<Written>;
  send: (line: Line) => Promise<Answered>;
  mark: (line: Line, change: Partial<Mark>) => void;
  /**
   * Show one line as somebody else left it.
   *
   * The line comes with it because a caller can be showing lines from more than
   * one answer: a card is one answer and closes over its turn, and the inbox
   * lists every proposal that still waits, from every answer that has one. The
   * entry alone says its index and not which answer it is the nth of.
   */
  reconcile: (proposal: Proposal, line: Line) => void;
  /** Ask the page in view to read again, now or when a sheet over it closes. */
  reread: () => void;
  /**
   * Open the sign-in sheet, handing over the lines a signed-in human would run
   * on from — this one and the ones after it.
   *
   * It hands over the LINES rather than a function that runs them, so that
   * running them goes back through the same guarded entry a press does: a
   * continuation that called this function again would be a second run nothing
   * had taken the guard for.
   */
  signIn: (rest: readonly Line[]) => void;
};

/**
 * One line as the route last returned it: the entry's own state, words and
 * version over the line the press read, with everything the page holds about it
 * left alone.
 */
function asStanding(line: Line, held: Proposal | undefined): Line {
  return held === undefined ? line : { ...line, ...held };
}

/** True for a state nothing more will happen to by itself. */
export function settledState(state: ProposalState): boolean {
  return state === "applied" || state === "refused" || state === "dismissed";
}

/** True for an entry somebody left applied: the ledger moved behind this run. */
function appliedEntry(proposal: Proposal): boolean {
  return proposal.state === "applied";
}

/**
 * The run: these lines, in this order, one act each, never retried.
 *
 * Read down it, the rules are:
 *
 *   - the canonical branch is read before the first line, so the compare below is
 *     against the branch as of the press rather than per line — and read again
 *     after any act of this run lands, before the next line that depends on what
 *     a goal says, because one of these acts can be what changed it;
 *   - a line the compare refuses is refused UNSENT and the run goes on, because
 *     nothing was published, which is exactly what a refusal means;
 *   - a line found at `applying` is reconciled before anything is sent: somebody
 *     began it, and a goal that already carries the act is recorded applied with
 *     no act of this run's own;
 *   - `applying` is written BEFORE the act. A write that failed sends nothing and
 *     stops the run; a write somebody else won means the line is theirs, and the
 *     run goes past it only where what they left is settled;
 *   - the act is sent once, and its answer is written onto the line — two writes
 *     per line and no more, so a reload during a run finds the line in flight;
 *   - a refusal is passed; anything that does not say what happened stops the run
 *     and the lines after it say "not run";
 *   - a refusal a sign-in would remedy ends the run at that line and NEVER waits:
 *     the lines from there are handed to the sign-in sheet, whose success runs
 *     them as a fresh press, and a sheet closed without signing in leaves the
 *     card exactly as it is;
 *   - the page in view reads again after each confirmed act and when the run ends.
 */
export async function runProposals(lines: readonly Line[], ports: RunPorts): Promise<void> {
  for (const line of lines) {
    ports.mark(line, { notRun: false, refusedUnsent: "" });
  }
  let looked = await ports.look();
  // An act of this run can change the very goal a later line is about: an edit
  // of G lands, and the approve of G behind it would otherwise compare against
  // the reading taken before the run and approve work the card never showed
  // (Sol S58-C-01). So a confirmed act makes the reading stale, and the next
  // line that depends on what a goal says takes a fresh one before it compares.
  // At most one read per confirmed act, and none at all where no later line is
  // guarded.
  let stale = false;
  let stoppedAt = -1;
  // Each line's entry as the route last returned it. A line this run has written
  // is past the version the press read, and handing the press's own copy to the
  // sign-in sheet made the resumed run write a stale version, meet a settled
  // conflict, and skip the very act the human signed in for (Sol S58-C-03).
  const standing = new Map<string, Proposal>();
  for (const [at, line] of lines.entries()) {
    if (stale && needsTheCompare(line.verb)) {
      looked = await ports.look();
      stale = false;
    }
    const refusal = guardFor(line, looked.rows, looked.defaults, looked.outcome, looked.message);
    if (refusal !== "") {
      ports.mark(line, { refusedUnsent: refusal });
      continue;
    }
    // A line found at `applying` is one somebody began, and learning of it must
    // not authorise a second act. So it is reconciled against the reading this
    // run took — fetch-first, and taken again where an act of this run has moved
    // the ledger since — and a goal that already carries the act settles the line
    // without being sent (Astra A-01).
    if (line.state === "applying") {
      if (stale) {
        looked = await ports.look();
        stale = false;
      }
      if (carriesAlready(line, looked.rows)) {
        const settled = await ports.record(line, "applied", ALREADY_CARRIED);
        if (settled.kind === "conflict") {
          ports.reconcile(settled.proposal, line);
        } else if (settled.kind === "failed") {
          ports.mark(line, { unrecorded: { state: "applied", words: ALREADY_CARRIED } });
        } else {
          ports.mark(line, { unrecorded: null });
        }
        continue;
      }
    }
    const started = await ports.record(line, "applying", "");
    if (started.kind !== "failed") {
      standing.set(line.id, started.proposal);
    }
    if (started.kind === "conflict") {
      ports.reconcile(started.proposal, line);
      // Somebody else's applied entry means the LEDGER moved, exactly as this
      // run's own applied answer does, so the reading every later compare rests
      // on is stale and the next guarded line takes a fresh one (Astra C-01).
      stale = stale || appliedEntry(started.proposal);
      if (settledState(started.proposal.state)) {
        continue;
      }
      stoppedAt = at;
      break;
    }
    if (started.kind === "failed") {
      ports.mark(line, { refusedUnsent: COULD_NOT_START });
      stoppedAt = at;
      break;
    }
    // The line as the server now has it: the outcome's own write is against the
    // version this one left, so nobody else can slip between them.
    const sending = { ...line, version: started.proposal.version };
    const answered = await ports.send(sending);
    const written = recorded(answered);
    const finished = await ports.record(sending, written.state, written.words);
    if (finished.kind !== "failed") {
      standing.set(line.id, finished.proposal);
    }
    if (finished.kind === "conflict") {
      ports.reconcile(finished.proposal, sending);
      stale = stale || appliedEntry(finished.proposal);
    } else if (finished.kind === "failed") {
      // The act happened and the conversation could not say so. What the act
      // answered is kept on the line for the page's life, because that is the
      // only thing here that knows what the ledger did, and the line says both.
      ports.mark(line, { unrecorded: written });
    }
    if (answered.kind === "applied") {
      // The ledger moved, so the reading every later compare rests on has.
      stale = true;
      ports.reread();
    }
    if (!goesOn(answered)) {
      stoppedAt = at;
      if (answered.kind === "sign-in") {
        // The lines from here, as the route now holds them, so signing in
        // resumes with the act that asked for it rather than past it.
        ports.signIn(lines.slice(at).map((one) => asStanding(one, standing.get(one.id))));
      }
      break;
    }
  }
  if (stoppedAt >= 0) {
    for (const line of lines.slice(stoppedAt + 1)) {
      ports.mark(line, { notRun: true });
    }
  }
  ports.reread();
}

/* ------------------------------------------------- one run, one press, once -- */

/**
 * The guard that makes a press a run and a second press nothing.
 *
 * It is an object rather than a flag in state because a press is not a render: a
 * second Apply in the same frame would read a state that has not moved yet and
 * send every line twice. The edit sheet guards its own save the same way
 * (g1-s56 D1), and this is that rule where a test can reach it.
 */
export type RunGuard = { running: string };

export function noRun(): RunGuard {
  return { running: "" };
}

/** Take the run for this card, or answer false where one is already in flight. */
export function takeRun(guard: RunGuard, card: string): boolean {
  if (guard.running !== "") {
    return false;
  }
  guard.running = card;
  return true;
}

/** Give it back, however the run ended. */
export function releaseRun(guard: RunGuard): void {
  guard.running = "";
}

/**
 * True while the answer this card belongs to is still being written.
 *
 * Its buttons sleep until then. The outcome of a press is recorded on the
 * Partner's own message, and that message is appended when the whole answer has
 * ended — so an Apply pressed mid-answer would have nothing to record `applying`
 * on, and the route refuses it in words (Astra S58-04).
 */
export function busyAnswering(card: Card, running: string): boolean {
  return running !== "" && running === card.turn;
}

/* ------------------------------------------- a re-read that waits for a sheet -- */

/**
 * A re-read asked while a sheet covers the work area, waiting for it to close.
 *
 * The content under an open sheet is what that sheet is rendered over: a read
 * made then would unmount its columns and whatever the human had typed into
 * them, and an act applied from the Partner's drawer is exactly the thing that
 * asks for one at that moment (Astra S58-03). So the ask is remembered and made
 * when the cover clears.
 */
export type Deferred = { pending: boolean };

export function nothingDeferred(): Deferred {
  return { pending: false };
}

/** Ask for the re-read now, or remember it for when the cover clears. */
export function askReread(held: Deferred, covered: boolean, read: () => void): void {
  if (covered) {
    held.pending = true;
    return;
  }
  read();
}

/** The cover changed: make the read that was waiting, once, when it is gone. */
export function coverChanged(held: Deferred, covered: boolean, read: () => void): void {
  if (covered || !held.pending) {
    return;
  }
  held.pending = false;
  read();
}
