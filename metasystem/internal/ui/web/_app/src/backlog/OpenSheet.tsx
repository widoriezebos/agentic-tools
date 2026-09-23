import { useRef, useState, type ReactNode } from "react";

import { actingAs } from "./acting";
import { BacklogError, openGoal, type Backlog } from "./api";
import {
  blockedForOpen,
  derivedTier,
  emptyRisk,
  goalOf,
  intakeFor,
  INTENT_RULE,
  NEXT_STEP_RULE,
  openNote,
  overridesTier,
  ANSWERS,
  type Answer,
  type Intake,
  type Risk,
} from "./opening";
import { Panel } from "./Panel";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";
import { failureMessage } from "../shell/workspace";

/**
 * A new goal, before it is opened.
 *
 * Intake is the one act on this board with no card to start from, and the one
 * where what the engine requires and what a board would guess differ most.
 * The two lines a human writes carry the rule they are written under, beside
 * the field rather than in a manual: an intent is what done looks like, and a
 * next step is intent, constraints and freedoms and never a script — because
 * a goal written as steps binds its executor to the author's context and goes
 * stale the moment reality shifts.
 *
 * The four risk answers are fields because the engine derives the rigor tier
 * from them and refuses an open without them and their basis. The tier is
 * therefore offered as "the one they derive" unless a human overrides it, and
 * an override is recorded with why, which is the engine's own rule.
 *
 * There is no priority here and no arc. `goal open` has no flag for either. A
 * new goal arrives where the engine appends it and is placed with the same
 * re-rank the board already publishes; an arc is `goal set-arc`, a separate
 * verb with its own membership rules. Offering them would be promising a
 * record this act cannot write.
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
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);
  const { session, askToSignIn } = useSession();
  const retried = useRef(false);
  const authority = actingAs(backlog.authority, session);
  const blocked = blockedForOpen(intake, risk);

  const send = () => {
    if (blocked !== "") {
      return;
    }
    setSending(true);
    setRefusal("");
    const asked = goalOf(intake, risk);
    openGoal(asked)
      .then((opened) => {
        setSending(false);
        onDone(opened, asked.id);
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

  const line = (
    key: "id" | "intent" | "nextStep" | "labels" | "blocks" | "why",
    label: string,
    hint: string,
    rule?: string,
  ) => (
    <div className="ms-act-field" key={key}>
      <label htmlFor={`ms-open-${key}`}>{label}</label>
      <input
        id={`ms-open-${key}`}
        type="text"
        value={intake[key]}
        placeholder={hint}
        onChange={(event) => {
          setIntake({ ...intake, [key]: event.target.value });
        }}
      />
      {rule !== undefined && <p className="ms-act-rule">{rule}</p>}
    </div>
  );

  return (
    <Panel
      eyebrow="Intake → To Do"
      title="New goal"
      unproven={authority.proven ? "" : authority.reason}
      refusal={refusal}
      note={blocked === "" ? openNote(authority.human) : blocked}
      onClose={onClose}
      act={
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          Open goal
        </Button>
      }
    >
      {line("id", "Id", "a short name, as every seat will refer to it")}
      {line("intent", "Intent", "what done looks like", INTENT_RULE)}
      {line("nextStep", "First next step", "what to take on, and what is free", NEXT_STEP_RULE)}
      {line("labels", "Labels (optional)", "ui board")}
      {line("blocks", "Unblocks (optional)", "the goal this one clears the way for")}

      <p className="ms-act-source">
        The four risk answers are how the engine works out the rigor this goal is held to. Severity and novelty derive
        the tier; exposure and accumulation scale the proof rather than lifting it.
      </p>
      <div className="ms-act-budget">
        <Answers
          label="Severity"
          value={risk.severity}
          onChange={(value) => {
            setRisk({ ...risk, severity: value });
          }}
        />
        <Answers
          label="Novelty"
          value={risk.novelty}
          onChange={(value) => {
            setRisk({ ...risk, novelty: value });
          }}
        />
        <Answers
          label="Exposure"
          value={risk.exposure}
          onChange={(value) => {
            setRisk({ ...risk, exposure: value });
          }}
        />
        <Answers
          label="Accumulation"
          value={risk.accumulation}
          onChange={(value) => {
            setRisk({ ...risk, accumulation: value });
          }}
        />
      </div>
      <div className="ms-act-field">
        <label htmlFor="ms-open-basis">Basis</label>
        <input
          id="ms-open-basis"
          type="text"
          value={risk.basis}
          placeholder="why those four answers are the answers"
          onChange={(event) => {
            setRisk({ ...risk, basis: event.target.value });
          }}
        />
      </div>
      <div className="ms-act-field">
        <label htmlFor="ms-open-tier">Tier</label>
        <select
          id="ms-open-tier"
          value={intake.tier}
          onChange={(event) => {
            setIntake({ ...intake, tier: event.target.value as "" | Answer });
          }}
        >
          <option value="">the one these answers derive ({derivedTier(risk)})</option>
          {ANSWERS.map((answer) => (
            <option key={answer} value={answer}>
              {answer}
            </option>
          ))}
        </select>
      </div>
      {overridesTier(intake, risk) && line("why", "Why that tier", "why this goal is held to another rigor")}
    </Panel>
  );
}

function Answers({
  label,
  value,
  onChange,
}: {
  label: string;
  value: Answer;
  onChange: (value: Answer) => void;
}): ReactNode {
  const id = `ms-open-${label.toLowerCase()}`;
  return (
    <div className="ms-act-field">
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        value={value}
        onChange={(event) => {
          onChange(event.target.value as Answer);
        }}
      >
        {ANSWERS.map((answer) => (
          <option key={answer} value={answer}>
            {answer}
          </option>
        ))}
      </select>
    </div>
  );
}
