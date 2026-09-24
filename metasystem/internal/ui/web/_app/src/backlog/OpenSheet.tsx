import { ChevronRight } from "lucide-react";
import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { actingAs } from "./acting";
import { BacklogError, openGoal, type Backlog } from "./api";
import {
  blockedForOpen,
  chosenStop,
  derivedTier,
  emptyRisk,
  goalOf,
  idRefusal,
  intakeFor,
  keepsDerived,
  openNote,
  overridesTier,
  slugFrom,
  tierLine,
  unopenable,
  ANSWERS,
  ID_RULE,
  INTENT_RULE,
  NEXT_STEP_RULE,
  SCORES,
  type Answer,
  type Intake,
  type Risk,
  type Score,
} from "./opening";
import { laneTitle } from "./lanes";
import { Panel } from "./Panel";
import { Button, Hint } from "../shell/controls";
import { GoalPicker, type PickableGoal } from "../shell/GoalPicker";
import { useSession } from "../shell/identity";
import { TokenField } from "../shell/TokenField";
import { failureMessage } from "../shell/workspace";

/**
 * A new goal, before it is opened, as one screen.
 *
 * The common case is three answers: what the goal is called, what done looks
 * like, and what to take on first. They are the whole form a human sees when
 * the sheet opens, with the button that ends it in view below them. Nothing
 * else has gone — the four risk answers and their basis are what the engine
 * derives the rigor tier from and refuses an open without, and the labels and
 * the blocker are still here — but each waits behind a disclosure until it is
 * wanted, which is what "the common case is three answers" means when it is
 * made real rather than merely said.
 *
 * Two things are stated here and nowhere else in the browser. The id is
 * suggested from the intent's first words until a human names it themselves,
 * because a goal's id is what every other record refers to and typing it
 * twice is work the browser can do once; and it is checked against the
 * engine's own rule and the ids this board already carries before the act is
 * sent, because a refusal read under the field is worth more than the same
 * refusal read after a round trip.
 *
 * Neither of the two fields behind More is free text any more, and one of
 * them never should have been. Unblocks names a goal the ledger already
 * carries, so it is a picker over what this page has loaded rather than a box
 * to guess an id into: what is chosen is a goal, what is chosen is on screen,
 * and a goal that has already ended is refused here rather than by the engine
 * after the act. Labels stays free words — a label is invented the first time
 * it is used — but it shows what the board already carries and marks a word
 * it does not, so a typo is visible before it is a label nobody meant. Both
 * fields are the shell's, because the priority sheet and the record sheets
 * ask for the same two things.
 *
 * There is no banner about proof. An act that needs a human opens the sign-in
 * sheet by itself and sends the same act again, as every act on this board
 * does now, so a warning in front of a form a human has not filled in yet is
 * a warning about nothing. The one thing that does stop the act before it
 * starts is a ledger that could not be read at all, and that is one muted
 * line under the head.
 *
 * There is still no priority here and no arc. `goal open` has no flag for
 * either. A new goal arrives where the engine appends it and is placed with
 * the same re-rank the board already publishes; an arc is `goal set-arc`, a
 * separate verb with its own membership rules. Offering them would be
 * promising a record this act cannot write.
 */
export function OpenSheet({
  backlog,
  intent = "",
  onClose,
  onDone,
}: {
  backlog: Backlog;
  /**
   * What the intent line opens with, where whatever opened the sheet already
   * knows what the goal is for. A design's own page passes its title; the
   * board passes nothing and the line starts empty, as it always did. Either
   * way it is a first draft — the human writes what done looks like.
   */
  intent?: string;
  onClose: () => void;
  /** The ledger as it stands after the act, which is what shows the card. */
  onDone: (opened: Backlog, id: string) => void;
}) {
  const [intake, setIntake] = useState<Intake>(() => intakeFor(intent));
  const [risk, setRisk] = useState<Risk>(emptyRisk);
  /** Whether a human has named the goal, after which nothing suggests an id. */
  const [named, setNamed] = useState(false);
  /**
   * Whether the risk disclosure is open: null until anything has an opinion,
   * which is what makes it closed when the sheet opens and lets the sheet
   * open it once without ever arguing with the human afterwards.
   */
  const [answersOpen, setAnswersOpen] = useState<boolean | null>(null);
  const [moreOpen, setMoreOpen] = useState(false);
  /**
   * Whether a human has asked to record a tier other than the derived one.
   *
   * The tier is text until then. It is derived — four answers already chose
   * it — so a select sitting under those four answers reads as a fifth
   * question, and a human who has just answered four is owed a statement
   * rather than another box. The override is a link away for the case the
   * engine allows, which is rare and deliberate.
   */
  const [overriding, setOverriding] = useState(false);
  /**
   * What the Unblocks field itself refuses, or "". It is the picker's own
   * sentence rather than the sheet's, because the picker is the only thing
   * that knows what was typed, what was chosen, and what the board carries;
   * the sheet keeps it so that the act can be held until the field is
   * answered, and shows it under that field rather than in the foot.
   */
  const [blocksRefusal, setBlocksRefusal] = useState("");
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const { session, askToSignIn } = useSession();
  const retried = useRef(false);
  const authority = actingAs(backlog.authority, session);

  // The intake as the act would take it: the typed id, or the suggestion
  // standing in for one nobody has typed. The suggestion is the value and not
  // a hint behind it, because a human who presses Open goal without touching
  // the field means the name they can read in it.
  const asked: Intake = { ...intake, id: named ? intake.id : slugFrom(intake.intent) };
  // Every goal the page has already loaded, which is every live one and every
  // closed one: what the id is checked against, what the picker offers, and
  // where the labels this board already uses come from.
  const loaded = [...backlog.rows, ...backlog.closed];
  const taken = loaded.map((row) => row.ref.id);
  const goals: PickableGoal[] = loaded.map((row) => ({
    id: row.ref.id,
    intent: row.intent,
    lane: laneTitle(row.lane),
    concluded: row.lane === "done" || row.lane === "abandoned" ? row.lane : "",
  }));
  const knownLabels = [...new Set(loaded.flatMap((row) => row.labels))].sort();
  const cannot = unopenable(backlog.ledger.state);
  const blocked = blockedForOpen(asked, risk);
  const refuseId = idRefusal(asked.id, taken);

  // Everything blockedForOpen names after the three fields lives inside the
  // risk disclosure, so the sheet opens it once, at the moment those three
  // are answered and the act is waiting on what is inside. It opens once and
  // not on every keystroke after: a panel that shut itself again the instant
  // the basis was typed would take the tier away mid-sentence. After that the
  // human's own word is the only one that moves it.
  if (answersOpen === null && asked.id.trim() !== "" && asked.intent.trim() !== "" && asked.nextStep.trim() !== "") {
    setAnswersOpen(true);
  }

  const send = () => {
    if (blocked !== "" || cannot !== "" || refuseId !== "" || blocksRefusal !== "") {
      return;
    }
    setSending(true);
    setRefusal("");
    const wanted = goalOf(asked, risk);
    openGoal(wanted)
      .then((opened) => {
        setSending(false);
        onDone(opened, wanted.id);
      })
      .catch((error: unknown) => {
        setSending(false);
        if (error instanceof BacklogError && error.signIn && !retried.current) {
          retried.current = true;
          askToSignIn(send);
          return;
        }
        setRefusal(failureMessage(error));
      });
  };

  const answer = (key: Score["key"], value: Answer) => {
    setRisk({ ...risk, [key]: value });
  };

  /** Take the override back: the select goes, and so does what it recorded. */
  const keepDerived = () => {
    setOverriding(false);
    setIntake({ ...intake, tier: "", why: "" });
  };

  return (
    <Panel
      form
      eyebrow="Opens in To Do, not yet approved"
      title="New goal"
      // What the Partner is given when the head's Ask is pressed: the three
      // answers of the common case first, so the chip reads the id and the
      // intent, then what the tier is derived from and what the disclosures
      // hold. Every one of them is a field a human typed; none is a secret.
      fields={[
        { name: "Id", value: asked.id },
        { name: "Intent", value: intake.intent },
        { name: "First next step", value: intake.nextStep },
        { name: "Tier", value: intake.tier === "" ? String(derivedTier(risk)) : intake.tier },
        { name: "Why these answers", value: risk.basis },
        { name: "Why that tier", value: intake.why },
        { name: "Labels", value: intake.labels },
        { name: "Unblocks", value: intake.blocks },
      ]}
      unproven=""
      refusal={refusal}
      note={cannot === "" ? (blocked === "" ? openNote(authority.human) : blocked) : ""}
      aside={cannot === "" ? undefined : <p className="ms-act-cannot">{cannot}</p>}
      onClose={onClose}
      act={
        <Button
          primary
          disabled={cannot !== "" || blocked !== "" || refuseId !== "" || blocksRefusal !== "" || sending}
          onClick={send}
        >
          {sending ? "Opening…" : "Open goal"}
        </Button>
      }
    >
      <Field id="ms-open-id" label="Id" hint={ID_RULE} refuse={refuseId}>
        <input
          id="ms-open-id"
          type="text"
          value={asked.id}
          placeholder="e.g. refund-worker"
          aria-describedby="ms-open-id-hint"
          onChange={(event) => {
            setNamed(event.target.value !== "");
            setIntake({ ...intake, id: event.target.value });
          }}
        />
      </Field>

      <Field id="ms-open-intent" label="Intent" hint={INTENT_RULE}>
        <textarea
          id="ms-open-intent"
          rows={2}
          value={intake.intent}
          placeholder="e.g. Refunds are issued within a day, with nobody touching the queue."
          aria-describedby="ms-open-intent-hint"
          onChange={(event) => {
            setIntake({ ...intake, intent: event.target.value });
          }}
        />
      </Field>

      <Field id="ms-open-nextStep" label="First next step" hint={NEXT_STEP_RULE}>
        <textarea
          id="ms-open-nextStep"
          rows={2}
          value={intake.nextStep}
          placeholder="e.g. Take the worker to a working end state; the approach is yours."
          aria-describedby="ms-open-nextStep-hint"
          onChange={(event) => {
            setIntake({ ...intake, nextStep: event.target.value });
          }}
        />
      </Field>

      <Disclosure
        label={`Tier ${String(derivedTier(risk))} · from the four answers below`}
        open={answersOpen ?? false}
        onOpen={setAnswersOpen}
      >
        <div className="ms-act-matrix">
          {SCORES.map((score) => (
            <Stops key={score.key} score={score} value={risk[score.key]} onChange={answer} />
          ))}
        </div>
        <Field
          id="ms-open-basis"
          label="Why these answers"
          hint="One line saying why those four answers are the answers."
        >
          <input
            id="ms-open-basis"
            type="text"
            value={risk.basis}
            placeholder="e.g. severity 1: reversible; novelty 2: new logic in an existing owner; exposure 2: every seat; accumulation 1: nothing compounds"
            aria-describedby="ms-open-basis-hint"
            onChange={(event) => {
              setRisk({ ...risk, basis: event.target.value });
            }}
          />
        </Field>
        <div className="ms-act-tier">
          <p className="ms-act-derived">{tierLine(risk)}</p>
          {overriding ? (
            <>
              <Field
                id="ms-open-tier"
                label="Tier"
                hint="The tier the answers derive is the usual one. Another is recorded with why, and a lower one is a human's own act."
              >
                <select
                  id="ms-open-tier"
                  value={intake.tier}
                  aria-describedby="ms-open-tier-hint"
                  onChange={(event) => {
                    const chosen = event.target.value as "" | Answer;
                    if (keepsDerived(risk, chosen)) {
                      keepDerived();
                      return;
                    }
                    setIntake({ ...intake, tier: chosen });
                  }}
                >
                  <option value="">the one these answers derive ({derivedTier(risk)})</option>
                  {ANSWERS.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </Field>
              {overridesTier(asked, risk) && (
                <Field
                  id="ms-open-why"
                  label="Why that tier"
                  hint="Why this goal is held to a rigor its own answers did not derive."
                >
                  <input
                    id="ms-open-why"
                    type="text"
                    value={intake.why}
                    placeholder="e.g. the answers derive 3, but the fix shape is known and carries no design unknowns"
                    aria-describedby="ms-open-why-hint"
                    onChange={(event) => {
                      setIntake({ ...intake, why: event.target.value });
                    }}
                  />
                </Field>
              )}
              <button type="button" className="ms-act-link" onClick={keepDerived}>
                Keep the derived tier
              </button>
            </>
          ) : (
            <button
              type="button"
              className="ms-act-link"
              onClick={() => {
                setOverriding(true);
              }}
            >
              Record a different tier…
            </button>
          )}
        </div>
      </Disclosure>

      <Disclosure label="More" open={moreOpen} onOpen={setMoreOpen}>
        <Field
          id="ms-open-labels"
          label="Labels"
          hint="Lowercase words the board narrows by, separated by spaces or commas. What the board already uses is suggested; anything else is marked new."
        >
          <TokenField
            id="ms-open-labels"
            value={intake.labels}
            known={knownLabels}
            placeholder="e.g. ui board"
            onChange={(labels) => {
              setIntake({ ...intake, labels });
            }}
          />
        </Field>
        <Field
          id="ms-open-blocks"
          label="Unblocks"
          hint="This goal parks with the chosen one recorded as its blocker, and returns when that one is done."
          refuse={blocksRefusal}
        >
          <GoalPicker
            id="ms-open-blocks"
            goals={goals}
            chosen={intake.blocks}
            // A goal cannot unblock itself, and the id in the field above is
            // what this goal is about to be called.
            exclude={asked.id}
            placeholder="e.g. refund-queue"
            onChoose={(blocks, why) => {
              setIntake({ ...intake, blocks });
              setBlocksRefusal(why);
            }}
          />
        </Field>
      </Disclosure>
    </Panel>
  );
}

/**
 * One field: what it is called, the field itself, what it is for, and what it
 * refuses. The hint is under the field rather than inside it as a placeholder,
 * because a placeholder is gone the moment it is needed most — while the
 * human is typing — and the placeholder is left to do the one thing it is
 * good at, which is showing an example.
 */
function Field({
  id,
  label,
  hint,
  refuse = "",
  children,
}: {
  id: string;
  label: string;
  hint: string;
  /** What this field itself refuses, before anything is sent, or "". */
  refuse?: string;
  children: ReactNode;
}) {
  return (
    <div className="ms-act-field">
      <label htmlFor={id}>{label}</label>
      {children}
      <p className="ms-act-hint" id={`${id}-hint`}>
        {hint}
      </p>
      {refuse !== "" && (
        <p className="ms-act-refuse" role="alert">
          {refuse}
        </p>
      )}
    </div>
  );
}

/**
 * A disclosure the sheet owns rather than the browser.
 *
 * A native `<details>` would be simpler and is what the rest of this build
 * uses, but the risk disclosure has to open itself when the act is waiting on
 * something inside it, and a browser that fires `toggle` for its own opening
 * as well as for a human's cannot tell the two apart. So the summary's click
 * is taken instead: Enter and Space on a summary are clicks, so the keyboard
 * is covered, and the element's open state is the one this sheet computed.
 */
function Disclosure({
  label,
  open,
  onOpen,
  children,
}: {
  label: string;
  open: boolean;
  onOpen: (open: boolean) => void;
  children: ReactNode;
}) {
  return (
    <details className="ms-act-disclosure" open={open}>
      <summary
        className="ms-act-disclosure-line"
        onClick={(event) => {
          event.preventDefault();
          onOpen(!open);
        }}
      >
        <ChevronRight className="ms-act-chevron" size={14} strokeWidth={2} aria-hidden="true" />
        <span>{label}</span>
      </summary>
      <div className="ms-act-disclosed">{children}</div>
    </details>
  );
}

/**
 * One risk answer, as one row: the score's name, the question it answers and
 * three pills on the same line, and under it the one phrase that is chosen.
 *
 * Every word here is the kit's; see SCORES in opening.ts for where each one
 * comes from. What changed is how many of them are on screen. Twelve
 * sentences laid out at once made this section taller than the form it
 * belongs to and read as a wall rather than as four questions, so the other
 * two phrases of each scale live in the pill's own tooltip — on hover and on
 * focus both, which is what the shell's Hint gives — and in the pill's
 * accessible name, so a screen reader hears what a 2 means without hovering
 * anything.
 *
 * The three stops are still a radio group and not a select: what a 1 means
 * beside what a 3 means is the answer, and a select shows one at a time.
 */
function Stops({
  score,
  value,
  onChange,
}: {
  score: Score;
  value: Answer;
  onChange: (key: Score["key"], value: Answer) => void;
}) {
  const stops = useRef<(HTMLButtonElement | null)[]>([]);
  const name = `ms-open-${score.key}-name`;
  const question = `ms-open-${score.key}-question`;

  const keys = (event: KeyboardEvent<HTMLDivElement>) => {
    const step = STEPS[event.key];
    if (step === undefined) {
      return;
    }
    event.preventDefault();
    const at = ANSWERS.indexOf(value);
    const next = (at + step + ANSWERS.length) % ANSWERS.length;
    onChange(score.key, ANSWERS[next]);
    stops.current[next]?.focus();
  };

  return (
    <div className="ms-act-score">
      <div className="ms-act-score-line">
        <p className="ms-act-score-asked">
          <span className="ms-act-score-name" id={name}>
            {score.name}
          </span>{" "}
          <span id={question}>{score.question}</span>
        </p>
        <div
          className="ms-act-pills"
          role="radiogroup"
          aria-labelledby={name}
          aria-describedby={question}
          onKeyDown={keys}
        >
          {ANSWERS.map((stop, at) => (
            <Hint key={stop} label={score.stops[at]}>
              <button
                type="button"
                className="ms-act-pill"
                role="radio"
                aria-checked={stop === value}
                // The number a human reads opens the name, and what the number
                // means follows it: a scale whose stops are named "1, 2, 3" to
                // a screen reader is a scale nobody can answer.
                aria-label={`${stop}, ${score.stops[at]}`}
                // The roving tab stop: one pill of the three is in the tab
                // order, and it is the chosen one, so Tab moves between
                // questions and the arrows move within one.
                tabIndex={stop === value ? 0 : -1}
                ref={(element) => {
                  stops.current[at] = element;
                }}
                onClick={() => {
                  onChange(score.key, stop);
                }}
              >
                {stop}
              </button>
            </Hint>
          ))}
        </div>
      </div>
      <p className="ms-act-chosen">{chosenStop(score, value)}</p>
    </div>
  );
}

/** Which way each key moves along a scale that is read left to right. */
const STEPS: Record<string, number | undefined> = {
  ArrowRight: 1,
  ArrowDown: 1,
  ArrowLeft: -1,
  ArrowUp: -1,
};
