import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";

import {
  askQuestion,
  createRecord,
  failureMessage,
  setQuestionStatus,
  setRecordStatus,
  type Asked,
  type Written,
} from "./api";
import {
  ACTIONS,
  asked as askedOf,
  ASK,
  incomplete,
  noteFor,
  pageFor,
  scopeGoals,
  startDraft,
  statusNote,
  STATUSES,
  type Draft,
  type Kind,
  type Scope,
} from "./writing";
import { AskAboutSheet } from "../partner/AskSheet";
import type { Field } from "../partner/drafting";
import { useOpenSheet } from "../partner/store";
import { Button } from "../shell/controls";
import { DEFAULT_MODALITY, useOpener, useWorkModal, type Modality } from "../shell/workmodal";

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
 * dialog role, focus is returned to whatever opened it when it closes, and
 * Escape closes it. Nothing here navigates and nothing here closes on a
 * refusal — a refusal is the server's own sentence, shown in the sheet, with
 * the fields still filled in.
 *
 * How modal it is, is workmodal.tsx's: over the work area by default, so the
 * Project Partner beneath stays live and Tab reaches it, and over the whole
 * window where there is no work area. Its head carries "Ask about this",
 * which hands the fields as they stand to the Partner as a draft.
 */

/**
 * What the sheet was opened to do. The two acts that write something new carry
 * the scope of the page they were opened from, which is what they will be
 * about; it is read to the human and never offered as a choice.
 */
export type Request =
  | { mode: "record"; kind: Kind; scope: Scope }
  | { mode: "status"; id: string; title: string; path: string; status: string }
  | {
      mode: "question";
      scope: Scope;
      /**
       * The question as it arrives, where something composed it: an open
       * question of a sitting's table, asked with its consequence and the record
       * it came out of (g1-s55 D2). It is a prefill and not a fact — the field is
       * the human's, and Ask is still their press.
       */
      question?: string;
      /**
       * The ledger goals this question is about, where the caller knows them
       * better than the page's scope does. A sitting's question is about what
       * its record is about, and a record names goals of the ledger, which is
       * what this route scopes by.
       */
      goals?: string[];
    }
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

/** What the capture calls each of these sheets while it is open. */
const SHEET_NAMES: Record<Request["mode"], string> = {
  record: "New record",
  status: "Change status",
  question: "New question",
  answer: "Answer",
  discard: "Discard changes",
};

export function Sheet({
  request,
  modal = DEFAULT_MODALITY,
  onClose,
  onDone,
}: {
  request: Request;
  /** How much of the window this is modal for. The work area, by default. */
  modal?: Modality;
  onClose: () => void;
  onDone: (done: Done) => void;
}) {
  // Declared first, so its cleanup is first: the work area is live again
  // before the caret goes back to whatever opened this.
  const host = useWorkModal(modal);
  useOpenSheet(SHEET_NAMES[request.mode]);
  const panel = useRef<HTMLElement | null>(null);
  // Read while it still has the caret: covering the work area takes the caret
  // off whatever had it, and an effect reads the document too late.
  const opener = useOpener();
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);

  // Focus enters the panel when it opens and goes back to whatever opened it
  // when it closes, so a human who cancels is where they were.
  useEffect(() => {
    const inside = panel.current?.querySelector<HTMLElement>(FOCUSABLE);
    (inside ?? panel.current)?.focus();
    return () => {
      opener.current?.focus();
    };
  }, [opener]);

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
    // Focus is held inside only while this is modal for the whole window.
    // Over the work area the Partner beneath is live, and Tab is how a human
    // reaches it.
    if (event.key !== "Tab" || panel.current === null || host !== null) {
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

  const sheet = (
    <>
      {/* The scrim dims the page and does not close the sheet: a contribution
          half typed is not something to lose to a stray click. Escape and
          Cancel are the two ways out, and both are deliberate. */}
      <div className="ms-writing-scrim" aria-hidden="true" />
      <section
        className="ms-writing-sheet"
        role="dialog"
        // Only a sheet that blocks the whole window is modal to a reader: one
        // that leaves the drawer live has to say so.
        aria-modal={host === null}
        aria-labelledby="ms-writing-title"
        tabIndex={-1}
        ref={panel}
        onKeyDown={keys}
      >
        {request.mode === "record" && (
          <RecordForm request={request} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "status" && (
          <StatusForm request={request} sending={sending} onClose={onClose} onSend={send} />
        )}
        {request.mode === "question" && (
          <QuestionForm request={request} sending={sending} onClose={onClose} onSend={send} />
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

  // Over the work area the sheet renders into the work area's own layer, so
  // the scrim is that box and the drawer beneath is outside it.
  return host === null ? sheet : createPortal(sheet, host);
}

type Send = (make: () => Promise<Done>) => void;

/**
 * The head of every sheet: what this is, what it is called, and the offer to
 * discuss it with the Project Partner before it is made.
 */
function Head({
  eyebrow,
  title,
  sheet,
  fields,
}: {
  eyebrow: string;
  title: string;
  /** What this sheet is called where the Partner is concerned. */
  sheet: string;
  /** What "Ask about this" hands over, in the order the sheet asks them. */
  fields: Field[];
}) {
  return (
    <div className="ms-writing-head">
      <div>
        <p className="ms-facts-eyebrow">{eyebrow}</p>
        <h2 className="ms-writing-title" id="ms-writing-title">
          {title}
        </h2>
      </div>
      <AskAboutSheet sheet={sheet} fields={fields} />
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
 * A new record: what it is about, its title, and the page as it will be
 * written. The page is read-only — it is what the fields make, not a second
 * place to type — and it changes as the fields change.
 */
function RecordForm({
  request,
  sending,
  onClose,
  onSend,
}: {
  request: { kind: Kind; scope: Scope };
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [draft, setDraft] = useState<Draft>(() => startDraft(request.kind, request.scope));
  const blocked = incomplete(draft);
  const action = ACTIONS[draft.kind];
  return (
    <>
      <Head
        eyebrow={action.eyebrow}
        title={action.offer}
        sheet="New record"
        fields={[
          { name: "Kind", value: ACTIONS[draft.kind].eyebrow },
          { name: "Title", value: draft.title },
          { name: "The page, as it will be written", value: pageFor(draft) },
        ]}
      />
      {/* What it will be about is the draft's own, not the page's: a chapter
          of a book carries no goal even where the page has one. */}
      <About scope={draft.goals.length === 0 ? null : request.scope} />
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
      <Head
        eyebrow="Change status"
        title={request.title}
        sheet="Change status"
        fields={[
          { name: "Record", value: request.path },
          { name: "From", value: request.status },
          { name: "To", value: status },
        ]}
      />
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
  sending,
  onClose,
  onSend,
}: {
  request: { scope: Scope; question?: string; goals?: string[] };
  sending: boolean;
  onClose: () => void;
  onSend: Send;
}) {
  const [question, setQuestion] = useState(request.question ?? "");
  const blocked = blockedQuestion(question);
  // What it is about: the goals the caller named where it named any, and the
  // page's own scope otherwise. A question out of a sitting is about what its
  // record is about, which is not the page the sheet was opened over.
  const goals = request.goals;
  return (
    <>
      <Head
        eyebrow="Ask a question"
        title={ASK}
        sheet="New question"
        fields={[{ name: "Question", value: question }]}
      />
      {goals === undefined ? <About scope={request.scope} /> : <AboutGoals goals={goals} />}
      <div className="ms-writing-field">
        <label htmlFor="ms-writing-question">Question</label>
        {/* A textarea and not a line: a question composed out of a sitting
            carries what follows from leaving it open and the record it came out
            of, and a human who cannot see the whole of what they are about to
            ask cannot edit it. It is still one row of the register — the write
            trims it to one question — so nothing here is a second field. */}
        <textarea
          id="ms-writing-question"
          rows={3}
          value={question}
          onChange={(event) => {
            setQuestion(event.target.value);
          }}
        />
      </div>
      <Foot
        confirm="Ask"
        note="Appends a row to memory/questions.md with a fresh id, today's date, and status open."
        blocked={blocked}
        sending={sending}
        onClose={onClose}
        onConfirm={() => {
          onSend(async () => ({
            mode: "question",
            asked: await askQuestion({
              question: question.trim(),
              goals: goals ?? scopeGoals(request.scope),
            }),
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
      <Head
        eyebrow="Answer"
        title={request.question}
        sheet="Answer"
        fields={[
          { name: "Question", value: request.question },
          { name: "Answered by", value: reference },
        ]}
      />
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
      <Head
        eyebrow="Unsaved changes"
        title="Discard your changes?"
        sheet="Discard changes"
        fields={[{ name: "Document", value: request.path }]}
      />
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
 * What a question out of a sitting is about: the goals its record names, read
 * rather than chosen, exactly as the scope line beside it is.
 *
 * A record that names none is about the project as a whole, and so is the
 * question — which is a scope and not a missing field.
 */
function AboutGoals({ goals }: { goals: readonly string[] }) {
  return (
    <p className="ms-writing-about">
      About:{" "}
      {goals.length === 0
        ? "this project"
        : goals.map((goal, at) => (
            <span key={goal}>
              {at > 0 && ", "}
              <span className="ms-mono">{goal}</span>
            </span>
          ))}
    </p>
  );
}

/**
 * What this contribution will be about, on the first line, read rather than
 * chosen.
 *
 * It was a multiple select over the whole ledger. A picker there asked a human
 * standing on one page to say again, in a list of six hundred goals, what the
 * address they were reading already said — and let them say something else,
 * which is a record filed under a goal nobody was looking at. At project level
 * it was worse than redundant: naming a goal from the page about the whole
 * project is the one thing that page cannot mean.
 */
function About({ scope }: { scope: Scope }) {
  return (
    <p className="ms-writing-about">
      About:{" "}
      {scope === null ? (
        "this project"
      ) : (
        <>
          {/* A ledger whose goal file has no heading of its own reads by its
              id, and "g1-s15 — g1-s15" says it twice. */}
          <span className="ms-mono">{scope.id}</span>
          {scope.title !== scope.id && <> — {scope.title}</>}
        </>
      )}
    </p>
  );
}
