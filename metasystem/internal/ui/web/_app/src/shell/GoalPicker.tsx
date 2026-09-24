import { X } from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import { Chip } from "./controls";

/**
 * A field that must hold a goal the ledger already carries, so it is never
 * free text.
 *
 * A human typing an id into a box is a human guessing: the guess is checked
 * after the act, by the engine, and what comes back is a refusal about a
 * record they cannot see from here. So the field offers what the page has
 * already loaded — every live goal and every closed one — and the human
 * recognises the one they mean rather than spelling it. What they chose stays
 * on screen as a chip, because a field that shows an id and means a goal is a
 * field that can be misread.
 *
 * The pattern is the ARIA combobox, written by hand: Radix has no combobox,
 * and a listbox of buttons announced as a list of buttons is a control that
 * lies to anyone not looking at it. So the input says what it is, says whether
 * the list is open, and names the option the arrows are on; the options are
 * options; and the keyboard alone can do every part of it.
 *
 * Closed goals are listed rather than hidden. A human who is looking for a
 * goal that has already been done needs to learn that it is done — which is
 * the answer to their question — and a list that silently lacks it teaches
 * nothing. So it is there, greyed, disabled to a reader, and refused with a
 * sentence if it is chosen anyway.
 */

/** A goal as this field offers it: what it is, and what the page can say of it. */
export type PickableGoal = {
  id: string;
  intent: string;
  /** What the board calls the lane this goal stands in. */
  lane: string;
  /** "" while the goal is live; how it ended, once it has ended. */
  concluded: "" | "done" | "abandoned";
};

/** The longest the intent's opening runs in one option's line. */
const FIRST_WORDS = 80;

/**
 * The intent's first words, cut at a word rather than inside one.
 *
 * There is no first-sentence helper in this build to reuse, and an intent is
 * not reliably one sentence, so the cut is by length: what fits on a line,
 * ending where a word ends, with an ellipsis saying that the line continues.
 */
export function firstWords(intent: string): string {
  const line = intent.trim().replace(/\s+/g, " ");
  if (line.length <= FIRST_WORDS) {
    return line;
  }
  const cut = line.slice(0, FIRST_WORDS);
  const space = cut.lastIndexOf(" ");
  return `${space > 0 ? cut.slice(0, space) : cut}…`;
}

/**
 * The goals a typed line is looking for.
 *
 * Every word typed has to be somewhere in the id or in the intent, in any
 * order and in either place, because a human looking for a goal remembers a
 * word of what it is for at least as often as they remember its id. Case is
 * nothing: an id is lowercase and an intent is a sentence, and a human typing
 * either is typing what they remember.
 *
 * The goal being opened is never among them. A goal cannot unblock itself,
 * and a list that offers the thing being created is a list offering a record
 * that does not exist yet.
 */
export function matching(
  goals: readonly PickableGoal[],
  typed: string,
  exclude = "",
): readonly PickableGoal[] {
  const words = typed
    .toLowerCase()
    .split(/\s+/)
    .filter((word) => word !== "");
  return goals.filter((goal) => {
    if (goal.id === exclude || goal.id === "") {
      return false;
    }
    const searched = `${goal.id} ${goal.intent}`.toLowerCase();
    return words.every((word) => searched.includes(word));
  });
}

/** What a goal that has already ended answers, in the two ways it can end. */
const CONCLUDED_REFUSAL: Record<PickableGoal["concluded"], string> = {
  "": "",
  done: "already done, nothing to unblock",
  abandoned: "abandoned, nothing to unblock",
};

/** What a line that named no goal says, said with what was typed. */
export function noSuchGoal(typed: string): string {
  return `no goal named ${typed.trim()}`;
}

/**
 * The line under the field, or "" when the field is answered.
 *
 * Three things are not an answer and each says its own why: a goal that has
 * already ended, a line that matches nothing at all, and a line that matches
 * something a human has not chosen yet. The third is the sheet's own addition
 * rather than the design's: a half-typed line with the act pressed would
 * otherwise be sent as no blocker at all, which is a typed intent silently
 * dropped, and this field exists precisely so that nothing about a blocker is
 * silent. An empty field is not one of the three — no blocker is the common
 * case and always was.
 */
export function pickRefusal(
  chosen: string,
  typed: string,
  goals: readonly PickableGoal[],
  exclude = "",
): string {
  if (chosen !== "") {
    const goal = goals.find((candidate) => candidate.id === chosen);
    return goal === undefined ? "" : CONCLUDED_REFUSAL[goal.concluded];
  }
  const wanted = typed.trim();
  if (wanted === "") {
    return "";
  }
  return matching(goals, wanted, exclude).length === 0
    ? noSuchGoal(wanted)
    : "Choose one of the goals listed, or clear the field.";
}

/** One option's own id, which is what aria-activedescendant names. */
function optionId(field: string, at: number): string {
  return `${field}-option-${String(at)}`;
}

/** What one option wears: what it is, and whether the arrows are on it. */
function optionClass(goal: PickableGoal, active: boolean): string {
  return [
    "ms-pick-option",
    goal.concluded === "" ? "" : "ms-pick-option--closed",
    active ? "ms-pick-option--active" : "",
  ]
    .filter((name) => name !== "")
    .join(" ");
}

export function GoalPicker({
  id,
  goals,
  chosen,
  exclude = "",
  placeholder = "",
  onChoose,
}: {
  /** The field's id, which the label points at and the list is named from. */
  id: string;
  goals: readonly PickableGoal[];
  /** The goal that is chosen, as its id, or "" while none is. */
  chosen: string;
  /** A goal that cannot be among the options: the one being opened. */
  exclude?: string;
  placeholder?: string;
  /**
   * The choice and what the field refuses, together, because the form that
   * holds this field has to know both to know whether it can be sent.
   */
  onChoose: (id: string, refusal: string) => void;
}) {
  const [typed, setTyped] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const field = useRef<HTMLInputElement | null>(null);
  const clear = useRef<HTMLButtonElement | null>(null);
  const list = useRef<HTMLUListElement | null>(null);
  /** Whether the last change was a human's, which is when focus should move. */
  const moved = useRef(false);

  const options = matching(goals, typed, exclude);
  const at = Math.min(active, Math.max(options.length - 1, 0));
  const listed = open && options.length > 0;
  const listId = `${id}-list`;

  // Choosing replaces the input with a chip and clearing puts it back, so the
  // caret has to follow: to the × that clears, and back to the field that
  // types. It follows a human's own change and never a render, which is what
  // the flag is for — a sheet that opened on a goal already chosen would
  // otherwise take the caret off the first field a human has to fill in.
  useEffect(() => {
    if (!moved.current) {
      return;
    }
    moved.current = false;
    if (chosen === "") {
      field.current?.focus();
    } else {
      clear.current?.focus();
    }
  }, [chosen]);

  // A list that opens near the foot of a sheet whose body scrolls is a list
  // below the fold, so the list brings itself into view; and the option the
  // arrows are on brings itself into view inside the list. Nearest is what
  // makes both minimal, and a no-op once what they name is on screen.
  useEffect(() => {
    if (!listed) {
      return;
    }
    list.current?.scrollIntoView({ block: "nearest" });
    const option = list.current?.children.item(at);
    if (option instanceof HTMLElement) {
      option.scrollIntoView({ block: "nearest" });
    }
  }, [listed, at, options.length]);

  const say = (picked: string, line: string) => {
    onChoose(picked, pickRefusal(picked, line, goals, exclude));
  };

  const choose = (goal: PickableGoal) => {
    moved.current = true;
    setTyped("");
    setOpen(false);
    setActive(0);
    say(goal.id, "");
  };

  const clearChoice = () => {
    moved.current = true;
    setTyped("");
    setOpen(false);
    setActive(0);
    say("", "");
  };

  const keys = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      // Escape closes the list it opened, and the sheet only once there is no
      // list left to close.
      if (!listed) {
        return;
      }
      event.stopPropagation();
      event.preventDefault();
      setOpen(false);
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      setOpen(true);
      if (options.length === 0) {
        return;
      }
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((at + step + options.length) % options.length);
      return;
    }
    if (event.key === "Enter" && listed) {
      event.preventDefault();
      choose(options[at]);
    }
  };

  if (chosen !== "") {
    const goal = goals.find((candidate) => candidate.id === chosen);
    return (
      <div className="ms-pick">
        <span className="ms-pick-chosen">
          <span className="ms-mono">{chosen}</span>
          <span className="ms-pick-dot" aria-hidden="true">
            ·
          </span>
          <span className="ms-pick-words">{firstWords(goal?.intent ?? "")}</span>
          <button
            id={id}
            type="button"
            className="ms-pick-clear"
            aria-label={`Clear ${chosen}`}
            ref={clear}
            onClick={clearChoice}
            onKeyDown={(event) => {
              if (event.key !== "Backspace") {
                return;
              }
              event.preventDefault();
              clearChoice();
            }}
          >
            <X size={12} strokeWidth={2} aria-hidden="true" />
          </button>
        </span>
      </div>
    );
  }

  return (
    <div className="ms-pick">
      <input
        id={id}
        type="text"
        role="combobox"
        value={typed}
        placeholder={placeholder}
        autoComplete="off"
        aria-expanded={listed}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={listed ? optionId(id, at) : undefined}
        aria-describedby={`${id}-hint`}
        ref={field}
        onFocus={() => {
          setOpen(true);
        }}
        onBlur={() => {
          setOpen(false);
        }}
        onChange={(event) => {
          setTyped(event.target.value);
          setOpen(true);
          setActive(0);
          say("", event.target.value);
        }}
        onKeyDown={keys}
      />
      {listed && (
        <ul
          className="ms-pick-list"
          id={listId}
          role="listbox"
          aria-label="Goals on this board"
          ref={list}
          // The press that chooses must not be a press that blurs the field
          // first, or the list is gone before the click lands on it.
          onMouseDown={(event) => {
            event.preventDefault();
          }}
        >
          {options.map((goal, index) => (
            <li
              key={goal.id}
              id={optionId(id, index)}
              role="option"
              className={optionClass(goal, index === at)}
              aria-selected={index === at}
              aria-disabled={goal.concluded === "" ? undefined : true}
              onClick={() => {
                choose(goal);
              }}
            >
              <span className="ms-pick-option-id ms-mono">{goal.id}</span>
              <span className="ms-pick-option-words">{firstWords(goal.intent)}</span>
              <Chip>{goal.lane}</Chip>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
