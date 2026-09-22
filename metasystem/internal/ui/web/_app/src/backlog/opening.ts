/**
 * What `goal open` takes, and what it does not.
 *
 * The sheet asks for what the verb records and for nothing else. Two things a
 * board might expect to be here are not, because the verb has no flag for
 * either: a priority and an arc. A new goal is appended wherever the engine
 * appends it and is placed afterwards with the same re-rank the board already
 * publishes; an arc is `goal set-arc`, a separate verb with its own
 * membership rules. Offering either in this sheet would be promising a record
 * this act cannot write.
 *
 * What the verb does require, and a board would not guess, is the four risk
 * answers and the one line of basis behind them. The engine derives the rigor
 * tier from them and refuses an open without them, so they are fields here
 * rather than something the browser invents.
 */

/** One answer on the risk scale, as a select carries it. */
export type Answer = "1" | "2" | "3";

export const ANSWERS: readonly Answer[] = ["1", "2", "3"];

/** The four answers and the line that justifies them. */
export type Risk = {
  severity: Answer;
  novelty: Answer;
  exposure: Answer;
  accumulation: Answer;
  basis: string;
};

/** A goal as a human states it at intake, as typed rather than as parsed. */
export type Intake = {
  id: string;
  intent: string;
  nextStep: string;
  /** "" takes the tier the risk answers derive; anything else needs a why. */
  tier: "" | Answer;
  why: string;
  /** Space- or comma-separated, as typed; the empty string is no labels. */
  labels: string;
  /** The goal this one unblocks, where a human is opening a blocker. */
  blocks: string;
};

export const emptyIntake: Intake = {
  id: "",
  intent: "",
  nextStep: "",
  tier: "",
  why: "",
  labels: "",
  blocks: "",
};

export const emptyRisk: Risk = {
  severity: "1",
  novelty: "1",
  exposure: "1",
  accumulation: "1",
  basis: "",
};

/**
 * The rigor tier the risk answers imply: the worse of severity and novelty.
 * Exposure and accumulation do not lift it — they scale the proof instead —
 * which is internal/goal/file.go's own rule, repeated here only so the sheet
 * can say which tier a human is overriding before they are asked why.
 */
export function derivedTier(risk: Risk): number {
  return Math.max(Number(risk.severity), Number(risk.novelty));
}

/** True when the chosen tier is not the one the answers derive. */
export function overridesTier(intake: Intake, risk: Risk): boolean {
  return intake.tier !== "" && Number(intake.tier) !== derivedTier(risk);
}

/** The labels the sheet's one line means, separated however they were typed. */
export function labelsOf(typed: string): string[] {
  return typed
    .split(/[\s,]+/)
    .map((label) => label.trim())
    .filter((label) => label !== "");
}

/** What travels to the open route: the flat body the boundary reads. */
export type NewGoal = {
  id: string;
  intent: string;
  nextStep: string;
  tier: number;
  why: string;
  blocks: string;
  labels: string[];
  severity: number;
  novelty: number;
  exposure: number;
  accumulation: number;
  basis: string;
};

export function goalOf(intake: Intake, risk: Risk): NewGoal {
  return {
    id: intake.id.trim(),
    intent: intake.intent.trim(),
    nextStep: intake.nextStep.trim(),
    tier: intake.tier === "" ? 0 : Number(intake.tier),
    why: intake.why.trim(),
    blocks: intake.blocks.trim(),
    labels: labelsOf(intake.labels),
    severity: Number(risk.severity),
    novelty: Number(risk.novelty),
    exposure: Number(risk.exposure),
    accumulation: Number(risk.accumulation),
    basis: risk.basis.trim(),
  };
}

/**
 * Why the sheet's own button is disabled, or the empty string when it is not.
 *
 * A server that cannot act as the human says so first and in the route's own
 * words, so the reason is read before the press rather than after. The rest
 * are the human's own to fix, and each one names the field rather than saying
 * that something is missing.
 */
export function blockedForOpen(proven: boolean, reason: string, intake: Intake, risk: Risk): string {
  if (!proven) {
    return reason === "" ? "This interface cannot act as a human." : reason;
  }
  if (intake.id.trim() === "") {
    return "A goal is named by one id, which is how every other record and every seat refers to it.";
  }
  if (intake.intent.trim() === "") {
    return "The intent says what done looks like, in one line.";
  }
  if (intake.nextStep.trim() === "") {
    return "The next step states intent, constraints and freedoms — never a script of the how.";
  }
  if (risk.basis.trim() === "") {
    return "The basis is one line saying why those four risk answers are the answers.";
  }
  if (overridesTier(intake, risk) && intake.why.trim() === "") {
    return `The risk answers derive tier ${String(derivedTier(risk))}; choosing another one is recorded with why.`;
  }
  return "";
}

/** What the sheet says the act will do, named for the human who will do it. */
export function openNote(human: string): string {
  const who = human === "" ? "the enrolled human" : `human:${human}`;
  return `Publishes goal open as ${who}, with origin human. The goal arrives in To Do, unapproved: opening it authorizes nothing.`;
}

/** The intake rule the sheet quotes where a human is writing the two lines. */
export const INTENT_RULE = "One line saying what done looks like — the outcome, not the work.";
export const NEXT_STEP_RULE =
  "Intent, constraints and freedoms, never a script of the how: a different machine has to be able to claim this and execute it without asking you what you meant.";
