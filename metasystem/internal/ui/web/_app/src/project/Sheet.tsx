import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import {
  askQuestion,
  createRecord,
  failureMessage,
  setQuestionStatus,
  setRecordStatus,
  type Asked,
  type Goal,
  type Written,
} from "./api";
import {
  ACTIONS,
  asked as askedOf,
  emptyDraft,
  incomplete,
  noteFor,
  pageFor,
  statusNote,
  STATUSES,
  type Draft,
  type Kind,
} from "./writing";
import { Button } from "../shell/controls";

/**
 * The sheet: every contribution, as a proposal, before it is made.
 *
 * One panel serves all four — a new record, a change of status, a question, an
 * answer — because they are one act with four shapes: a human says what they
 * want, reads exactly what will be written and where, and confirms. A new
 * record shows the whole page it will become, composed from the same grammar
 * the server writes with, so there is nothing between what is read here and
 * what lands on disk.
 *
 * It is a panel and not a browser dialog: the element is a section with the
 * dialog role, focus is held inside it while it is open and returned to
 * whatever opened it when it closes, and Escape closes it. Nothing here
 * navigates and nothing here closes on a refusal — a refusal is the server's
 * own sentence, shown in the sheet, with the fields still filled in.
 */

/** What the sheet was opened to do. */
export type Request =
  | { mode: "record"; kind: Kind; goal: string | null }
  | { mode: "status"; id: string; title: string; path: string; status: string }
  | { mode: "question"; goal: string | null }
  | { mode: "answer"; id: string; question: string }
  | { mode: "discard"; path: string };

/** What it did, for the caller to act on. */
export type Done =
  | { mode: "record"; written: Written }
  | { mode: "status"; written: Written }
  | { mode: "question"; asked: Asked }
  | { mode: "answer"; asked: Asked }
  | { mode: "discard" };

/** Everything that can hold focus inside the panel, in the order it is met. */
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function Sheet({
  request,
  goals,
  onClose,
  onDone,
}: {
  request: Request;
  /** The ledger's goals, live ones first, as the pane carries them. */
  goals: Goal[];
  onClose: () => void;
  onDone: (done: Done) => void;
}) {
  const panel = useRef<HTMLElement | null>(null);
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);

  // Focus enters the panel when it opens and goes back to whatever opened it
  // when it closes, so a human who cancels is where they were.
  useEffect(() => {
    const opener = globalThis.document.activeElement;
    const inside = panel.current?.querySelector<HTMLElement>(FOCUSABLE);
    (inside ?? panel.current)?.focus();
    return () => {
      if (opener instanceof HTMLElement) {
        opener.focus();
      }
    };
  }, []);

  const send = (make: () => Promise<Done>) => {
    setSending(true);
    setRefusal("");
    make()
      .then((done) => {
        setSending(false);
        onDone(done);
      })
      .catch((error: unknown) => {
        setSending(false);
        setRefusal(failureMessage(error));
      });
  };

  const keys = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== "Tab" || panel.current === null) {
      return;
    }
    const stops = [...panel.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
    const first = stops.at(0);
    const last = stops.at(-1);
    if (first === undefined || last === undefined) {
      return;
    }
    const at = globalThis.document.activeElement;
    if (event.shiftKey && at === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && at === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return (
    <>
      {/* The scrim dims the page and does not close the sheet: a contribution
          half typed is not something to lose to a stray click. Escape and
          Cancel are the two ways out, and both are deliberate. */}
      <div className="ms-writing-scrim" aria-hidden="true" />
      <section
        className="ms-writing-sheet"
        role="dialog"
        aria-modal="true"
        aria-labelledby="ms-writing-title"
        tabIndex={-1}
        ref={panel}
        onKeyDown={keys}
      >
        {request.mode === "record" && (
          <RecordForm request={request} goals={goals} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "status" && (
          <StatusForm request={request} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "question" && (
          <QuestionForm request={request} goals={goals} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "answer" && (
          <AnswerForm request={request} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "discard" && <DiscardForm request={request} onClose={onClose} onDone={onDone} />}
        {refusal !== "" && (
          <p className="ms-writing-refusal" role="alert">
            {refusal}
          </p>
        )}
      </section>
    </>
  );
}

type Send = (make: () => Promise<Done>) => void;

/** The head of every sheet: what this is, and what it is called. */
function Head({ eyebrow, title }: { eyebrow: string; title: string }) {
  return (
    <div>
      <p className="ms-facts-eyebrow">{eyebrow}</p>
      <h2 className="ms-writing-title" id="ms-writing-title">
        {title}
      </h2>
    </div>
  );
}

/** The foot of every sheet: the act, the way out, and the promise. */
function Foot({
  confirm,
  note,
  blocked,
  sending,
  onConfirm,
  onClose,
  cancel = "Cancel",
  secondary,
}: {
  confirm: string;
  note: string;
  blocked: string;
  sending: boolean;
  onConfirm: () => void;
  onClose: () => void;
  /** What the way out is called, where "Cancel" is not what it does. */
  cancel?: string;
  secondary?: ReactNode;
}) {
  return (
    <div className="ms-writing-foot">
      <div className="ms-writing-buttons">
        <Button primary disabled={blocked !== "" || sending} onClick={onConfirm}>
          {confirm}
        </Button>
        <Button onClick={onClose}>{cancel}</Button>
        {secondary}
      </div>
      <p className="ms-writing-note">{blocked === "" ? note : blocked}</p>
    </div>
  );
}

/**
 * A new record: the title, the goals it is about, and the page as it will be
 * written. The page is read-only — it is what the fields make, not a second
 * place to type — and it changes as the fields change.
 */
function RecordForm({
  request,
  goals,
  sending,
  onClose,
  onSend,
}: {
  request: { kind: Kind; goal: string | null };
  goals: Goal[];
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [draft, setDraft] = useState<Draft>(() => emptyDraft(request.kind, request.goal));
  const blocked = incomplete(draft);
  const action = ACTIONS[draft.kind];
  return (
    <>
      <Head eyebrow={action.eyebrow} title={action.offer} />
      <div className="ms-writing-field">
        <label htmlFor="ms-writing-record-title">Title</label>
        <input
          id="ms-writing-record-title"
          type="text"
          value={draft.title}
          onChange={(event) => {
            setDraft({ ...draft, title: event.target.value });
          }}
        />
      </div>
      <Goals
        name="record"
        goals={goals}
        chosen={draft.goals}
        onChange={(chosen) => {
          setDraft({ ...draft, goals: chosen });
        }}
      />
      <div className="ms-writing-field">
        <span className="ms-writing-label" id="ms-writing-page">
          The page, as it will be written
        </span>
        <pre className="ms-writing-page" aria-labelledby="ms-writing-page" tabIndex={0}>
          {pageFor(draft)}
        </pre>
      </div>
      <Foot
        confirm={action.confirm}
        note={noteFor(draft)}
        blocked={blocked}
        sending={sending}
        onClose={onClose}
        onConfirm={() => {
          onSend(async () => ({ mode: "record", written: await createRecord(askedOf(draft)) }));
        }}
      />
    </>
  );
}

/** A change of status: the one line that will be rewritten, and nothing else. */
function StatusForm({
  request,
  sending,
  onClose,
  onSend,
}: {
  request: { id: string; title: string; path: string; status: string };
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [status, setStatus] = useState(request.status);
  return (
    <>
      <Head eyebrow="Change status" title={request.title} />
      <div className="ms-writing-field">
        <label htmlFor="ms-writing-status">Status</label>
        <select
          id="ms-writing-status"
          value={status}
          onChange={(event) => {
            setStatus(event.target.value);
          }}
        >
          {STATUSES.map((candidate) => (
            <option key={candidate} value={candidate}>
              {candidate}
            </option>
          ))}
        </select>
      </div>
      <div className="ms-writing-field">
        <span className="ms-writing-label" id="ms-writing-line">
          The line, as it will be written
        </span>
        <pre className="ms-writing-page" aria-labelledby="ms-writing-line" tabIndex={0}>
          {`- Status: ${status}`}
        </pre>
      </div>
      <Foot
        confirm="Change status"
        note={statusNote(request.path, status)}
        blocked={status === request.status ? "This record already carries that status." : ""}
        sending={sending}
        onClose={onClose}
        onConfirm={() => {
          onSend(async () => ({ mode: "status", written: await setRecordStatus(request.id, status) }));
        }}
      />
    </>
  );
}

/** A question: one row of the register, open, dated by the server. */
function QuestionForm({
  request,
  goals,
  sending,
  onClose,
  onSend,
}: {
  request: { goal: string | null };
  goals: Goal[];
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [question, setQuestion] = useState("");
  const [chosen, setChosen] = useState<string[]>(request.goal === null ? [] : [request.goal]);
  const blocked = blockedQuestion(question);
  return (
    <>
      <Head eyebrow="Ask a question" title="Ask a question" />
      <div className="ms-writing-field">
        <label htmlFor="ms-writing-question">Question</label>
        <input
          id="ms-writing-question"
          type="text"
          value={question}
          onChange={(event) => {
            setQuestion(event.target.value);
          }}
        />
      </div>
      <Goals name="question" goals={goals} chosen={chosen} onChange={setChosen} />
      <Foot
        confirm="Ask"
        note="Appends a row to memory/questions.md with a fresh id, today's date, and status open."
        blocked={blocked}
        sending={sending}
        onClose={onClose}
        onConfirm={() => {
          onSend(async () => ({
            mode: "question",
            asked: await askQuestion({ question: question.trim(), goals: chosen }),
          }));
        }}
      />
    </>
  );
}

/** A pipe or a line break would make the row something other than a row. */
function blockedQuestion(question: string): string {
  if (question.trim() === "") {
    return "A question needs words.";
  }
  return question.includes("|") ? "A question carries no | : it is what divides one cell from the next." : "";
}

/** An answer: the reference that answered it, or the withdrawal of the row. */
function AnswerForm({
  request,
  sending,
  onClose,
  onSend,
}: {
  request: { id: string; question: string };
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [reference, setReference] = useState("");
  const answer = `answered: ${reference.trim()}`;
  const send = (status: string) => () => {
    onSend(async () => ({ mode: "answer", asked: await setQuestionStatus(request.id, status) }));
  };
  return (
    <>
      <Head eyebrow="Answer" title={request.question} />
      <div className="ms-writing-field">
        <label htmlFor="ms-writing-reference">Answered by</label>
        <input
          id="ms-writing-reference"
          type="text"
          value={reference}
          placeholder="the id of the record that answers it"
          onChange={(event) => {
            setReference(event.target.value);
          }}
        />
      </div>
      <Foot
        confirm="Answer"
        note={`Rewrites this row's status cell in memory/questions.md to "${answer}", and no other cell.`}
        blocked={reference.trim() === "" ? "An answer names what answered it." : ""}
        sending={sending}
        onClose={onClose}
        onConfirm={send(answer)}
        secondary={
          <Button disabled={sending} onClick={send("withdrawn")}>
            Withdraw
          </Button>
        }
      />
    </>
  );
}

/**
 * Leaving an editor with changes in it.
 *
 * It is the one sheet that asks rather than proposes: nothing is written
 * either way, and what is at stake is text that exists nowhere else. So it
 * writes nothing, reaches no network, and answers the caller directly — the
 * panel, the focus and the Escape are what it borrows.
 */
function DiscardForm({
  request,
  onClose,
  onDone,
}: {
  request: { path: string };
  onClose: () => void;
  onDone: (done: Done) => void;
}) {
  return (
    <>
      <Head eyebrow="Unsaved changes" title="Discard your changes?" />
      <Foot
        confirm="Discard"
        note={`The changes you made to ${request.path} are not written anywhere, and closing the editor loses them.`}
        blocked=""
        sending={false}
        cancel="Keep editing"
        onClose={onClose}
        onConfirm={() => {
          onDone({ mode: "discard" });
        }}
      />
    </>
  );
}

/**
 * The goals, as a multiple select over what the ledger carries, live ones
 * first, each labelled by its id and where it stands.
 *
 * It is a select and not a row of checkboxes because a real ledger carries
 * hundreds of goals: a list of that many checkboxes would be the sheet. It is
 * multiple because a record may be about more than one, and choosing none is a
 * record about the project as a whole, which is why nothing here is required.
 */
function Goals({
  name,
  goals,
  chosen,
  onChange,
}: {
  name: string;
  goals: Goal[];
  chosen: string[];
  onChange: (chosen: string[]) => void;
}) {
  const field = `ms-writing-${name}-goals`;
  return (
    <div className="ms-writing-field">
      <label className="ms-writing-label" htmlFor={field}>
        Goals — optional; none is the project as a whole
      </label>
      <select
        id={field}
        className="ms-writing-goals"
        multiple
        size={goals.length < 8 ? Math.max(goals.length, 2) : 8}
        value={chosen}
        onChange={(event) => {
          onChange([...event.target.selectedOptions].map((option) => option.value));
        }}
      >
        {goals.map((goal) => (
          <option key={goal.id} value={goal.id}>
            {goal.id} · {goal.state}
          </option>
        ))}
      </select>
    </div>
  );
}
