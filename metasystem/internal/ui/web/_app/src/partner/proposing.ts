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
 * somebody else began — left at `applying`, or left `unresolved` by a press the
 * act layer refused because another one owns that act — is reconciled before it
 * is sent: a goal that already carries the act is recorded applied with nothing
 * sent for it. And a press the act layer refused stops the run where it stands,
 * because the act it could not make may be landing behind it.
 *
 * **The page never rebases.** The tab that received an act's answer is the only
 * thing that knows what the ledger did, and it keeps that answer rather than
 * writing it somewhere it no longer belongs. An outcome write refused because
 * another press moved the entry while the act was out — on the version, or on the
 * attempt, which is that press owning the line now — is never written again at the
 * version that came back. This tab holds its own answer on the line as an
 * unrecorded mark, shows the entry the refusal carried, and stops the run there,
 * leaving the lines behind it for Continue.
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
   *
   * `version` is the entry it was held at. What retires the answer is the
   * record carrying an account of its own — an entry somebody has SETTLED — and
   * not that number: a surface reading the mark raw showed a refusal over a goal
   * that was applied and offered a second act (Astra C-04), while one that
   * expired it on any newer version hid an approval that had landed behind
   * another tab's bookkeeping (Astra E-03).
   */
  unrecorded: { state: ProposalState; words: string; version: number } | null;
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
 *
 * `expandedCards` is the human's own press on the folded line, and it outranks
 * that automatic fold for the card it names: the fold is a rule about what is in
 * view and not a fact about the card, so a card nobody dismissed was folded
 * straight back by it and its arguments and controls could not be reached at all
 * (Astra C-03). A dismissal folds it again, because putting the card away is a
 * later act of the same human's.
 */
export function cardsIn(
  carried: readonly Carried[],
  marks: Marks,
  displayed: Displayeds,
  dismissedCards: readonly string[],
  expandedCards: readonly string[] = [],
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
      folded:
        dismissedCards.includes(one.turn) ||
        (!isNewest && waitingIn(lines) > 0 && !expandedCards.includes(one.turn)),
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
  const held = unrecordedOn(line);
  if (held !== null) {
    return `${said(held.state, held.words)}; ${COULD_NOT_RECORD}`;
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

/**
 * What this page holds about this line's own act that the record does not, or
 * null where the record has moved past it.
 *
 * The held answer stands above the record because it is the only thing that
 * knows what the ledger did — and it stands until the RECORD carries an account
 * of its own, which is an entry somebody has settled: `applied`, `refused` or
 * `dismissed`. A newer version is not such an account. Another tab's bookkeeping
 * about a request that published nothing moves the version and says nothing about
 * what became of this act, and retiring the answer on it hid the only
 * explanation there was — an approval that landed and could not be written down —
 * and offered the press that approves the goal twice (Astra E-03).
 *
 * The version the answer was held at is kept beside it as the account of where it
 * was held; what retires the answer is the settlement and not the number. Every
 * surface asks this rather than the mark, because the mark is ONE mark shared by
 * the drawer and the inbox, and a surface reading it raw showed the drawer's
 * obsolete refusal over an entry the record had applied (Astra C-04).
 */
export function unrecordedOn(line: Line): Mark["unrecorded"] {
  const held = line.mark.unrecorded;
  if (held === null || settledState(line.state)) {
    return null;
  }
  return held;
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
  const held = unrecordedOn(line);
  if (held !== null) {
    return held.state !== "applied";
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
  | { kind: "in-flight"; words: string }
  | { kind: "sign-in"; words: string };

/**
 * What a failed act means to the runner.
 *
 * It is the page's one mapping, `outcomeOf`, with two things the runner needs
 * that a save did not: a refusal a sign-in would remedy is its own answer,
 * because it ends the run at that line and opens the sheet; the answer that
 * says the act landed and its proof did not is recorded as APPLIED with its own
 * words, because that is what it says; and the refusal that says another press
 * owns this act is its own answer too, because it is not a no.
 */
export function answeredOf(error: unknown): Answered {
  if (error instanceof BacklogError && error.signIn) {
    return { kind: "sign-in", words: NOT_APPLIED_SIGN_IN };
  }
  if (error instanceof BacklogError && error.code === IN_FLIGHT) {
    return { kind: "in-flight", words: error.message };
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
 * The refusal code the act layer answers a second press with, while this
 * process is already executing the same act on the same goal (internal/ui/act).
 *
 * It is not a no. The request was refused at once, before any read and before
 * any publish, and the act that IS executing may land — so the tab that meets it
 * holds no result about that line at all. It writes `unresolved` in the
 * refusal's own sentence, never `refused`, which would say the act did not
 * happen; it stops the run there, because the act it could not make may be
 * changing the goal a line behind it is about (Astra E-01); and it never sends
 * again on its own (Astra A-01).
 */
export const IN_FLIGHT = "in-flight";

/**
 * Whether the run goes on to the next ticked line after this answer.
 *
 * A refusal is passed, because nothing landed behind it and the next line does
 * not depend on it. An unresolved answer stops the run, because the act may have
 * landed and the next line may depend on it. A sign-in refusal stops it too: the
 * run never waits on a sheet, so the line says what it needs and the rest say
 * "not run" until the human signs in or presses Continue (g1-s58 D7).
 *
 * And an act another press owns stops it, for the reason the unresolved answer
 * does. Nothing of THIS press was published, but the act that press owns may be
 * landing as the answer is read, and a line behind it can be about the very goal
 * it changes: going on would compare that line against the reading taken before
 * the act and send it against a goal nobody read (Astra E-01). The lines behind
 * it say "not run", and Continue is a fresh press, which reads fetch-first before
 * it compares anything.
 */
export function goesOn(answered: Answered): boolean {
  return answered.kind === "applied" || answered.kind === "refused";
}

/**
 * The state one answer writes onto its line, and the words with it.
 *
 * Two of the five are written as another state. A sign-in refusal is written as
 * `refused`: nothing was published, which is what refused means, and the words
 * say what the human does about it. An act another press owns is written
 * `unresolved`: this tab holds no result for it, and the act may yet land.
 */
export function recorded(answered: Answered): { state: ProposalState; words: string } {
  return { state: writtenAs(answered.kind), words: answered.words };
}

function writtenAs(kind: Answered["kind"]): ProposalState {
  switch (kind) {
    case "sign-in":
      return "refused";
    case "in-flight":
      return "unresolved";
    default:
      return kind;
  }
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
 * is a goal the act reached. All ten are here, because the engine answers the
 * repeat of all ten applied under a browser session, and a page that read one of
 * them as a fresh act would be offering a press the engine would refuse.
 *
 * What a line is NOT carried by is an effect this reading cannot establish. An
 * open is carried by the goal it asked for whole — a goal of that id that differs
 * is not this act's effect — and an abandon that asked for a successor is carried
 * by nothing at all, because the row's abandoned clause says who, when and why and
 * never which goal carried the work (Astra F-04). Such a line is sent, and the
 * engine answers it: applied where the effect it asked for holds, and refused in
 * its own words where another one does.
 */
export function carriesAlready(line: Line, rows: readonly Row[]): boolean {
  const row = rows.find((one) => one.ref.id === line.goal);
  if (row === undefined) {
    return false;
  }
  const fields = line.fields ?? {};
  switch (line.verb) {
    // Approved, unexpired, and with the tuple this line displayed. An approval
    // carrying another budget is not this act's effect, and the guard above
    // refuses such a line on its own because a fresh prefill now answers
    // differently. An EXPIRED approval is not this act's effect either: the
    // engine's own no-op requires an unexpired one (internal/goal/approval.go),
    // so the approve writes a fresh approval, and a goal left carrying the
    // expired one is inadmissible for work (Astra D-03).
    case "approve-goal":
      return (
        row.approved !== undefined &&
        !row.approved.expired &&
        sameTuple(line.displayed?.budget ?? null, row.budget ?? null)
      );
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
    // No approval on the goal: what the unapprove takes off is not there. An
    // EXPIRED approval is not nothing — it is an approval this act would take off
    // — so a goal carrying one is not carried (Astra F-05).
    case "withdraw-goal":
      return row.approved === undefined;
    // The goal this open asked for, as the goal that exists: its intent, its next
    // step, the tier the line DERIVED and sends, its labels, and both directions
    // of the blocked relation — `holds` is the other direction of `blocks`, which
    // no record stores and the reading computes. A goal of that id that differs is
    // somebody else's, or an earlier one, and this act never reached it: the line
    // is sent and the engine refuses it as it does today (Astra F-05). The edges
    // are compared as relations rather than in the order either side lists them.
    case "open-goal": {
      const asked = openOf(line.goal, fields);
      return (
        asked.intent === row.intent &&
        asked.nextStep === row.nextStep &&
        asked.tier === row.tier &&
        named(asked.labels) === named(row.labels) &&
        named([...asked.blockedBy].sort()) === named([...row.blockedBy].sort()) &&
        named([...asked.blocks].sort()) === named([...row.holds].sort())
      );
    }
    // Abandoned, and only where this line asked for no successor. The reading does
    // not say WHICH successor an abandon recorded, so an abandon that asks for one
    // is carried by nothing here and is sent to the engine, which compares the
    // successor it recorded with the one asked for (Astra F-04).
    case "abandon-goal":
      return row.state === "abandoned" && (fields.successor ?? "") === "";
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
  /**
   * Write one state onto one line, under this run's own attempt.
   *
   * The attempt is the same on every write of one run, and the server is what
   * compares it: this side carries it and holds what comes back.
   */
  record: (line: Line, state: ProposalState, words: string, attempt: string) => Promise<Written>;
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
 * True for a line somebody BEGAN: an act may have landed behind it.
 *
 * Two states say so. `applying` is a line this page went away in the middle of,
 * or one another tab is applying now. `unresolved` is a line whose act answered
 * something that does not say what happened — including the refusal that says
 * another press owns it, which is exactly an act that may be landing as this is
 * read. A press on either reconciles against the reading before it sends, so
 * that learning of an act is never what authorises a second one (Astra A-01).
 */
function begun(line: Line): boolean {
  return line.state === "applying" || line.state === "unresolved";
}

/**
 * One attempt: the token the press that makes this run owns its lines by.
 *
 * A press is an attempt, and the attempt owns the line. Every run of the runner —
 * a press on the card, an inbox press, a bulk press, Continue, Try again — makes
 * one of these for its lifetime and carries it on every write it makes. The server
 * stores it on the entry when the line moves to `applying`, and refuses a settle
 * write carrying any other, because the press that owns the line is the only one
 * that may say what became of it. The page never compares it: that compare is the
 * server's, and this side's whole part is one token per press.
 *
 * Sixteen hex characters from the browser's own generator. A clone without one
 * falls back to the weaker source rather than sending nothing, because an empty
 * attempt is the write of a press that claims no ownership at all.
 */
export function newAttempt(): string {
  const bytes = new Uint8Array(8);
  const random = globalThis.crypto as { getRandomValues?: (into: Uint8Array) => void } | undefined;
  if (random?.getRandomValues === undefined) {
    for (let at = 0; at < bytes.length; at += 1) {
      bytes[at] = Math.floor(Math.random() * 256);
    }
  } else {
    random.getRandomValues(bytes);
  }
  return [...bytes].map((byte) => byte.toString(16).padStart(2, "0")).join("");
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
 *   - an outcome write the server refuses is never written again at the version it
 *     comes back with: the answer is held on the line, the entry is shown, and the
 *     run stops there, because the press that owns the line now may be applying an
 *     act a line behind this one is about;
 *   - a refusal is passed; anything that does not say what happened stops the run
 *     and the lines after it say "not run";
 *   - a refusal a sign-in would remedy ends the run at that line and NEVER waits:
 *     the lines from there are handed to the sign-in sheet, whose success runs
 *     them as a fresh press, and a sheet closed without signing in leaves the
 *     card exactly as it is;
 *   - the page in view reads again after each confirmed act and when the run ends.
 */
export async function runProposals(lines: readonly Line[], ports: RunPorts): Promise<void> {
  // This press's own attempt, carried on every write below. Continue and Try
  // again come back through here, so each of them owns the lines it moves.
  const attempt = newAttempt();
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
  /**
   * One line the reading shows the goal already carries: recorded applied, with
   * no act of this run's own, and what the record answered kept as every other
   * outcome write's is.
   */
  const settleCarried = async (one: Line): Promise<void> => {
    const settled = await ports.record(one, "applied", ALREADY_CARRIED, attempt);
    if (settled.kind === "conflict") {
      ports.reconcile(settled.proposal, one);
    } else if (settled.kind === "failed") {
      ports.mark(one, { unrecorded: { state: "applied", words: ALREADY_CARRIED, version: one.version } });
    } else {
      ports.mark(one, { unrecorded: null });
    }
  };
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
    // A line somebody BEGAN is one an act may have landed behind, and learning
    // of it must not authorise a second one. So it is reconciled against the
    // reading this run took — fetch-first, and taken again where an act of this
    // run has moved the ledger since — and a goal that already carries the act
    // settles the line without being sent (Astra A-01).
    let carried = false;
    if (begun(line)) {
      if (stale) {
        looked = await ports.look();
        stale = false;
      }
      // Only a read that SUCCEEDED can say what the act did. The read answers
      // 200 from the accepted ledger whether or not its own fetch of the
      // canonical branch landed, so a failed one can hand this run rows older
      // than the act: the cache says parked, the goal has resumed since, and the
      // line would be settled applied for good over a goal that is running
      // (Astra D-02). So a failed read settles nothing and sends nothing. The
      // record is moved off `applying`, because nothing else will settle it, and
      // moved to `unresolved` in the read's own words; a line that already says
      // unresolved is left saying it. The three verbs that compare are refused
      // unsent above by this same rule.
      if (!canCompare(looked.outcome)) {
        const unread = fetchFailedLine(looked.message);
        if (line.state === "applying") {
          const said = await ports.record(line, "unresolved", unread, attempt);
          if (said.kind === "conflict") {
            ports.reconcile(said.proposal, line);
          }
        }
        ports.mark(line, { refusedUnsent: unread });
        continue;
      }
      carried = carriesAlready(line, looked.rows);
    }
    if (carried && line.state === "applying") {
      await settleCarried(line);
      continue;
    }
    const started = await ports.record(line, "applying", "", attempt);
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
        // Their settled entry establishes what happened to this line, so an
        // answer this page was holding for want of a record is history now
        // (Astra C-04).
        ports.mark(line, { unrecorded: null });
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
    // A line another press owns, whose act the reading already shows landed: the
    // entry is taken above rather than below because the record admits `applied`
    // only from `applying`, and it is settled here with no act of this run's own.
    if (carried) {
      await settleCarried(sending);
      continue;
    }
    const answered = await ports.send(sending);
    const written = recorded(answered);
    // The version the outcome was written AT, which is the version the entry
    // still stands at where that write did not land.
    const wroteAt = sending.version;
    const finished = await ports.record(sending, written.state, written.words, attempt);
    if (finished.kind !== "failed") {
      standing.set(line.id, finished.proposal);
    }
    // The page NEVER REBASES. A conflict here is the line not being this press's
    // to settle any more: another press moved the entry while this act was out —
    // refused on the version, or on the attempt where the entry belongs to that
    // press now. Writing the answer again at the version that came back is what
    // this page used to do over an entry left `unresolved`, on the reading that
    // nobody was executing; but `unresolved` says nothing of the kind. A third
    // press wrote it while the second press's act was landing, and the rebased
    // refusal settled the line refused over a park that had applied, taking that
    // press's own answer with it (Astra F-03).
    //
    // So the answer is held here, where every answer the record does not carry is
    // held, the view is reconciled to the entry the refusal came with, and the run
    // STOPS below — exactly as it does at an `in-flight` answer, and for the same
    // reason: the act that press owns may be changing the very goal a line behind
    // this one is about (Astra F-01). Continue is a fresh press, with its own
    // attempt and its own read.
    if (finished.kind === "conflict") {
      ports.reconcile(finished.proposal, sending);
      ports.mark(line, { unrecorded: { ...written, version: wroteAt } });
    } else if (finished.kind === "failed") {
      // The act happened and the conversation could not say so. What the act
      // answered is kept on the line, because that is the only thing here that
      // knows what the ledger did, and the line says both — until the record
      // carries an account of its own, which is what the version says
      // (Astra C-04).
      ports.mark(line, { unrecorded: { ...written, version: wroteAt } });
    } else {
      // The outcome IS written down now, so an answer the page was holding
      // because an earlier one could not be written is obsolete: the record says
      // what happened, and a line still reading "the conversation could not
      // record this" over it would offer a recovery nobody needs (Astra C-04).
      ports.mark(line, { unrecorded: null });
    }
    if (answered.kind === "applied") {
      // The ledger moved, so the reading every later compare rests on has.
      stale = true;
      ports.reread();
    }
    if (finished.kind === "conflict" || !goesOn(answered)) {
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
