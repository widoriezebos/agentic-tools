import "./stickies.css";
import { X } from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { NavLink } from "react-router";

import type { About, Sticky } from "./api";
import { useStickies } from "./store";
import {
  aboutThePage,
  chipPath,
  chipWords,
  doneLabel,
  doneStickies,
  openStickies,
  sameAbout,
  savesOn,
  SIGN_IN_LINE,
} from "./stickies";
import { ageBetween } from "../backlog/format";
import { Help } from "../help/Help";
import { loadPane } from "../project/api";
import { Button } from "../shell/controls";
import { GoalPicker, type PickableGoal } from "../shell/GoalPicker";
import { Sheet } from "../shell/Sheet";
import { useSubject } from "../shell/about";

/**
 * The notepad, as a sheet from the right.
 *
 * It is the notifications panel's chrome, deliberately: the two are the same
 * gesture — a control in the header opens a list over the work area, Escape
 * closes it, the drawer beneath stays live — and a second kind of side panel
 * would be a second thing to learn for no gain.
 *
 * What is different is the composer at the top. A sticky is written in two
 * seconds or it is not written at all, so the field is the first thing in the
 * panel and the first thing focused, Enter saves, and Shift+Enter breaks a
 * line, exactly as the Partner's composer does.
 *
 * A card is drawn in the marker tokens the sign-in bar uses, so it reads as a
 * note rather than as another row of the application: the ochre is the one
 * colour in this interface that already means "a human wrote this".
 */

/** What the panel says when a human has never written one. */
const NOTHING_YET = "Nothing yet. Write what you want to remember; it stays on this seat and no agent reads it.";

export function StickiesPanel() {
  const { notepad, loaded, problem, panelIsOpen, closePanel } = useStickies();
  const open = openStickies(notepad);
  const done = doneStickies(notepad);

  return (
    <Sheet
      open={panelIsOpen}
      onOpenChange={(next) => {
        if (!next) {
          closePanel();
        }
      }}
      id="stickies-panel"
      side="right"
      label="Stickies"
      title="Stickies"
      closeLabel="Close the stickies"
      bodyClassName="ms-sheet-body--stickies"
      actions={<Help id="stickies" />}
    >
      <Composer />
      {notepad.human === "" && loaded && <p className="ms-stickies-seat">{SIGN_IN_LINE}</p>}
      {problem !== "" && (
        <p className="ms-stickies-problem" role="alert">
          {problem}
        </p>
      )}
      {loaded && problem === "" && notepad.stickies.length === 0 && (
        <p className="ms-stickies-empty">{NOTHING_YET}</p>
      )}
      {open.length > 0 && (
        <ul className="ms-stickies-rows">
          {open.map((sticky) => (
            <Card key={sticky.id} sticky={sticky} />
          ))}
        </ul>
      )}
      {done.length > 0 && (
        <details className="ms-stickies-done">
          <summary className="ms-stickies-done-head">{doneLabel(done.length)}</summary>
          <ul className="ms-stickies-rows">
            {done.map((sticky) => (
              <Card key={sticky.id} sticky={sticky} />
            ))}
          </ul>
        </details>
      )}
    </Sheet>
  );
}

/**
 * The composer: one field, one key, done.
 *
 * What it is about starts as the thing on the screen — the goal on a goal
 * page, the document in the reader, nothing elsewhere — or as whatever "Add a
 * sticky" opened it with. A human can take that chip off and add a goal by
 * name; a sticky about nothing at all is the common case and is never in the
 * way.
 */
function Composer() {
  const { opening, jot } = useStickies();
  const subject = useSubject();
  const [text, setText] = useState("");
  const [about, setAbout] = useState<About[]>(() => {
    const offered = opening ?? aboutThePage(subject);
    return offered === null ? [] : [offered];
  });
  const [picking, setPicking] = useState(false);
  const [goals, setGoals] = useState<PickableGoal[] | null>(null);
  const [refusal, setRefusal] = useState("");
  const [busy, setBusy] = useState(false);
  const field = useRef<HTMLTextAreaElement | null>(null);

  // The caret starts in the field, because the panel exists to be typed into.
  useEffect(() => {
    field.current?.focus();
  }, []);

  const save = () => {
    if (busy || text.trim() === "") {
      return;
    }
    setBusy(true);
    void jot(text, about).then((refused) => {
      setBusy(false);
      setRefusal(refused);
      if (refused === "") {
        setText("");
      }
    });
  };

  // Enter saves and Shift+Enter breaks a line, as the Partner's composer does.
  // Escape is the sheet's: it closes the panel, and this handler lets it pass.
  const keys = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (savesOn(event.key, event.shiftKey)) {
      event.preventDefault();
      save();
    }
  };

  /**
   * The goals the picker offers, read once, when a human asks for it.
   *
   * Every goal the project payload carries is offered and none is refused,
   * which is how the document reader adapts this picker for a record's own
   * Goals line: a sticky may be about a goal that has already shipped as
   * easily as one still open, and a list that silently lacked it would teach
   * a human nothing about where their note belongs.
   */
  const pick = () => {
    setPicking(true);
    if (goals !== null) {
      return;
    }
    loadPane()
      .then((pane) => {
        setGoals(pane.goals.map((one) => ({ id: one.id, intent: one.intent, lane: one.state, concluded: "" })));
      })
      .catch(() => {
        setGoals([]);
        setRefusal("The goals could not be read, so a sticky cannot be filed under one just now.");
      });
  };

  return (
    <div className="ms-stickies-composer">
      <label className="ms-stickies-label" htmlFor="ms-sticky-text">
        Write a sticky
      </label>
      <textarea
        id="ms-sticky-text"
        className="ms-stickies-field"
        rows={3}
        value={text}
        placeholder="ask Sol about the retry"
        ref={field}
        onChange={(event) => {
          setText(event.target.value);
        }}
        onKeyDown={keys}
      />
      <div className="ms-stickies-about">
        {about.map((named) => (
          <span className="ms-stickies-chosen" key={`${named.kind}:${named.id}`}>
            <span className="ms-mono">{chipWords(named)}</span>
            <button
              type="button"
              className="ms-stickies-clear"
              aria-label={`Not about ${chipWords(named)}`}
              onClick={() => {
                setAbout(about.filter((one) => !sameAbout(one, named)));
              }}
            >
              <X size={12} strokeWidth={2} aria-hidden="true" />
            </button>
          </span>
        ))}
        {!picking && (
          <button type="button" className="ms-act-link" onClick={pick}>
            Add a goal…
          </button>
        )}
      </div>
      {picking && (
        <GoalPicker
          id="ms-sticky-goal"
          goals={goals ?? []}
          chosen={about.filter((one) => one.kind === "goal").map((one) => one.id)}
          placeholder="e.g. g1-s45"
          onChoose={(picked) => {
            setAbout([
              ...about.filter((one) => one.kind !== "goal"),
              ...picked.map((id): About => ({ kind: "goal", id })),
            ]);
          }}
        />
      )}
      <div className="ms-stickies-actions">
        <Button primary disabled={busy || text.trim() === ""} onClick={save}>
          Save
        </Button>
        <span className="ms-stickies-hint">Enter saves · Shift+Enter breaks a line</span>
      </div>
      {refusal !== "" && (
        <p className="ms-stickies-problem" role="alert">
          {refusal}
        </p>
      )}
    </div>
  );
}

/**
 * One sticky: what it says, what it is about, how old it is, and the three
 * things a human does to it.
 *
 * Editing happens in the card rather than in a sheet, because a sticky is one
 * field and a sheet over one field is a ceremony. What it is about is not
 * edited here: a chip taken off would be a click away from an act nobody
 * confirmed, and rewriting the note is the act a human actually wants.
 */
function Card({ sticky }: { sticky: Sticky }) {
  const { change, remove } = useStickies();
  const [editing, setEditing] = useState<string | null>(null);
  const [refusal, setRefusal] = useState("");
  const [busy, setBusy] = useState(false);
  const done = sticky.doneAt !== "";
  const age = ageBetween(sticky.createdAt, new Date().toISOString());

  const run = (act: Promise<string>) => {
    setBusy(true);
    void act.then((refused) => {
      setBusy(false);
      setRefusal(refused);
      if (refused === "") {
        setEditing(null);
      }
    });
  };

  const keys = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (savesOn(event.key, event.shiftKey)) {
      event.preventDefault();
      if (editing !== null && editing.trim() !== "") {
        run(change(sticky.id, { text: editing }));
      }
    }
  };

  return (
    <li className={done ? "ms-sticky ms-sticky--done" : "ms-sticky"}>
      {editing === null ? (
        <p className="ms-sticky-text">{sticky.text}</p>
      ) : (
        <textarea
          className="ms-stickies-field"
          rows={3}
          value={editing}
          aria-label="What this sticky says"
          onChange={(event) => {
            setEditing(event.target.value);
          }}
          onKeyDown={keys}
        />
      )}
      <div className="ms-sticky-line">
        {sticky.about.map((named) => (
          <NavLink className="ms-sticky-chip ms-mono" key={`${named.kind}:${named.id}`} to={chipPath(named)}>
            {chipWords(named)}
          </NavLink>
        ))}
        <time className="ms-sticky-age" dateTime={sticky.createdAt}>
          {age}
        </time>
      </div>
      <div className="ms-sticky-acts">
        {editing === null ? (
          <>
            <button
              type="button"
              className="ms-act-link"
              disabled={busy}
              onClick={() => {
                run(change(sticky.id, { done: !done }));
              }}
            >
              {done ? "Open again" : "Done"}
            </button>
            <button
              type="button"
              className="ms-act-link"
              disabled={busy}
              onClick={() => {
                setEditing(sticky.text);
              }}
            >
              Edit
            </button>
            <button
              type="button"
              className="ms-act-link"
              disabled={busy}
              onClick={() => {
                run(remove(sticky.id));
              }}
            >
              Remove
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              className="ms-act-link"
              disabled={busy || editing.trim() === ""}
              onClick={() => {
                run(change(sticky.id, { text: editing }));
              }}
            >
              Save
            </button>
            <button
              type="button"
              className="ms-act-link"
              disabled={busy}
              onClick={() => {
                setEditing(null);
                setRefusal("");
              }}
            >
              Cancel
            </button>
          </>
        )}
      </div>
      {refusal !== "" && (
        <p className="ms-stickies-problem" role="alert">
          {refusal}
        </p>
      )}
    </li>
  );
}
