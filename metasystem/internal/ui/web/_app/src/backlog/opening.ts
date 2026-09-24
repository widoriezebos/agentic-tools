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
  /**
   * The goals this one unblocks: each of them parks until this one is done.
   * It is a list because the relation is one — one defect can hold several
   * goals up — and a field that took one id would have been asking a human to
   * open the same goal twice.
   */
  blocks: string[];
  /** The goals this one waits for: it parks until every one of them is done. */
  blockedBy: string[];
};

export const emptyIntake: Intake = {
  id: "",
  intent: "",
  nextStep: "",
  tier: "",
  why: "",
  labels: "",
  blocks: [],
  blockedBy: [],
};

/**
 * The intake a sheet opens on when something already knows what the goal is
 * for: a design's own title, where the goal is being opened from that design's
 * page. It is a starting point and nothing more — the human edits the line
 * freely, and the rest of the intake is as empty as it ever was.
 */
export function intakeFor(intent: string): Intake {
  return { ...emptyIntake, intent: intent.trim() };
}

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
  blocks: string[];
  blockedBy: string[];
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
    blocks: [...intake.blocks],
    blockedBy: [...intake.blockedBy],
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
 * Nothing about proof is here. A server that has found no human behind this
 * browser says so above the fields, and the act is still offered: the route
 * answers such a press with the sign-in this page can open, and the sheet
 * sends the same act again once a human is behind it. What is left is the
 * human's own to fix, and each reason names the field rather than saying that
 * something is missing.
 */
export function blockedForOpen(intake: Intake, risk: Risk): string {
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

/**
 * What each of the two dependency pickers is for, in one sentence.
 *
 * They are two directions of one relation and read almost the same, so each
 * says who ends up waiting: the goals chosen under Blocks wait for this one,
 * and this one waits for the goals chosen under Blocked by. Naming the waiter
 * in both is what keeps a human from filling in the wrong field and finding
 * out when the board parks the wrong card.
 */
export const BLOCKS_RULE = "The chosen goals wait for this one.";
export const BLOCKED_BY_RULE = "This goal waits for the chosen ones and parks until they are done.";

/**
 * Which picker an engine refusal is about, or "" when it is about neither.
 *
 * The engine refuses an open when one of the goals it would have parked
 * cannot be parked - a breach-stopped claim is the case that exists today -
 * and the refusal names that goal. A sentence naming a goal belongs under the
 * field where that goal was chosen, two disclosures down, rather than in the
 * foot where a human would read it without knowing which of two fields to
 * look at. Blocks is checked first because it is the only direction the
 * engine parks other goals for.
 */
export function whichFieldRefused(said: string, intake: Intake): "blocks" | "blockedBy" | "" {
  if (intake.blocks.some((id) => said.includes(id))) {
    return "blocks";
  }
  if (intake.blockedBy.some((id) => said.includes(id))) {
    return "blockedBy";
  }
  return "";
}

/** What the sheet says the act will do, named for the human who will do it. */
export function openNote(human: string): string {
  const who = human === "" ? "the enrolled human" : `human:${human}`;
  return `Publishes goal open as ${who}, with origin human. The goal arrives in To Do, unapproved: opening it authorizes nothing.`;
}

/**
 * The intake rule the sheet quotes where a human is writing the two lines.
 *
 * Each is a hint under its own field now rather than a rule beside it, and
 * each is the shortest form of the rule that still carries it: a form read at
 * desk height is read once, and a sentence nobody finishes is a sentence that
 * taught nothing.
 */
export const INTENT_RULE = "One line saying what done looks like: the outcome, not the work.";
export const NEXT_STEP_RULE =
  "What to take on, and what is free. A different machine has to be able to claim this without asking you what you meant.";

/**
 * The id rule, in the engine's own terms, as the Id field's hint.
 *
 * `internal/goal/goal.go`'s validId is the authority: lowercase letters,
 * digits and hyphens, and at most a hundred characters. The sentence saying
 * what an id is for is the one the disabled button used to carry; it belongs
 * beside the field a human is filling rather than under the whole form.
 */
export const ID_RULE = "The short name every seat will use for this goal. Lowercase letters, digits and hyphens.";

/** What the engine's own validId accepts, repeated so the field can say no first. */
const KEBAB = /^[a-z0-9-]+$/;

/**
 * The ceiling an id actually has. internal/goal/goal.go carries two: validId
 * takes a hundred characters for filesystem safety, and boundGoal refuses
 * anything past MaxIdBytes, which is sixty-four. The tighter of the two is
 * the one a goal file must pass, so it is the one this field says.
 */
const ID_LIMIT = 64;

/**
 * Why this id cannot be this goal's, or "" when it can.
 *
 * Both refusals are the engine's own, said here before the act is sent
 * rather than after it is refused: an id that is not kebab-case, and an id
 * the ledger already carries. Nothing is said about an empty id — an
 * untouched field is not a mistake, and the foot already names it as the
 * thing the act is waiting on.
 */
export function idRefusal(id: string, taken: readonly string[]): string {
  const wanted = id.trim();
  if (wanted === "") {
    return "";
  }
  if (!KEBAB.test(wanted)) {
    return "An id is kebab-case [a-z0-9-]: lowercase letters, digits and hyphens, and nothing else.";
  }
  if (wanted.length > ID_LIMIT) {
    return `An id is at most ${String(ID_LIMIT)} characters; this one is ${String(wanted.length)}.`;
  }
  if (taken.includes(wanted)) {
    return `The backlog already carries ${wanted}, and ids are unique across all sections.`;
  }
  return "";
}

/** The most words a suggestion takes from the intent, and its longest form. */
const SLUG_WORDS = 6;
const SLUG_LENGTH = 40;

/**
 * An id suggested from the intent's first words.
 *
 * It is a suggestion and never a value: the sheet stops offering one the
 * moment a human types in the Id field, because the id is what every other
 * record will refer to and a browser must not quietly rewrite one a human
 * chose. The cut is at six words or forty characters, whichever comes first,
 * and it is made at a word rather than inside one — an id truncated
 * mid-word reads as a typo rather than as a name.
 */
export function slugFrom(intent: string): string {
  const words = intent
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .split(" ")
    .filter((word) => word !== "")
    .slice(0, SLUG_WORDS);
  // The first word is the floor: one word longer than the whole allowance is
  // cut rather than dropped, because an empty suggestion helps nobody.
  let slug = (words.at(0) ?? "").slice(0, SLUG_LENGTH);
  for (const word of words.slice(1)) {
    const next = `${slug}-${word}`;
    if (next.length > SLUG_LENGTH) {
      break;
    }
    slug = next;
  }
  return slug;
}

/**
 * Why this seat cannot open a goal at all, or "" when it can.
 *
 * It is one sentence and not four: a ledger the server could not project is
 * a ledger no act can append to, and the pane's own statement already says
 * which of the four ways it failed. Nothing here is about proof — a server
 * with no human behind it is still offered the act, and answers it with the
 * sign-in this page can open.
 */
export function unopenable(state: string): string {
  return state === "read" ? "" : "The accepted ledger cannot be read here, so no goal can be opened until it is.";
}

/** One risk answer as the sheet asks it: its name, its question, its stops. */
export type Score = {
  key: "severity" | "novelty" | "exposure" | "accumulation";
  name: string;
  /** The question this score answers, in the kit's own words. */
  question: string;
  /** What 1, 2 and 3 mean, in the kit's own words, in that order. */
  stops: readonly [string, string, string];
};

/**
 * The four questions, and what each answer means, in the kit's own words.
 *
 * Not one syllable of this is the browser's. The four questions are
 * docs/paper/06-proof-over-trust.md and 11-economy.md, and the meaning of
 * each stop is plans/severity-tiered-rigor-p2-design.md's STR4-RISK-RECORD-15
 * — the one place in the kit that says what a 1, a 2 and a 3 are for each
 * score. Two parentheticals of that passage are left out, because neither is
 * part of what the answer means: severity 2's "(the bounded/severe line of
 * round 1)" cites a critique round, and exposure 3's list of shared-law paths
 * is where the answer is usually true rather than what it says.
 *
 * The same passage's tier formula is NOT used here and must not be read from
 * it: it was superseded by goal tier-from-severity-and-novelty, and
 * derivedTier above follows internal/goal/file.go as it stands.
 */
export const SCORES: readonly Score[] = [
  {
    key: "severity",
    name: "Severity",
    question: "How severe could the harm be if the change is wrong?",
    stops: [
      "visible and reversible on one machine",
      "recoverable but it crosses a proof, authority, secrets, data or external-side-effect boundary",
      "irreversible, or it moves authority, secrets or a landing bar",
    ],
  },
  {
    key: "novelty",
    name: "Novelty",
    question: "How unfamiliar is the approach to the system and its independent examiners?",
    stops: [
      "an existing owner whose existing checks cover the change",
      "new logic inside an existing owner",
      "a new law, verb, schema, seam or role",
    ],
  },
  {
    key: "exposure",
    name: "Exposure",
    question: "How many users or systems can it affect?",
    stops: ["one machine or one fixture", "every seat of the fleet", "every dispatch or every landing"],
  },
  {
    key: "accumulation",
    name: "Accumulation",
    question: "How much change has accumulated since the last broad examination of the touched area?",
    stops: [
      "broadly examined since its last change",
      "several landings since",
      "the area's last broad examination predates the goal's own base",
    ],
  },
];

/**
 * What the chosen stop means: the one phrase of three that is on screen.
 *
 * The other two are a pill away, in the tooltip. Twelve sentences at once is
 * what made this section taller than the form it belongs to, and a human
 * choosing an answer is reading the one they chose.
 */
export function chosenStop(score: Score, value: Answer): string {
  return score.stops[Number(value) - 1];
}

/**
 * The tier, and the two answers it came from: read, never chosen.
 *
 * It names severity and novelty rather than "the four answers" because the
 * confusion it answers is exactly that one — four boxes were chosen and only
 * two of them decide this. The other two are not silent: they scale the
 * proof, which is the engine's business and not this sheet's.
 */
export function tierLine(risk: Risk): string {
  return `Tier ${String(derivedTier(risk))}, from severity ${risk.severity} and novelty ${risk.novelty}.`;
}

/**
 * Whether a tier chosen in the override select is no override at all.
 *
 * The empty option is the derived tier by name, and the derived tier's own
 * number is the derived tier by value; either is a human saying they want
 * what the answers already said, so the sheet takes the override back rather
 * than recording one that changes nothing and then asking why.
 */
export function keepsDerived(risk: Risk, chosen: "" | Answer): boolean {
  return chosen === "" || Number(chosen) === derivedTier(risk);
}
