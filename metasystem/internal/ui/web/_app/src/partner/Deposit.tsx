import { useId, useState } from "react";

import { usePartner } from "./store";
import {
  cardHead,
  cardIn,
  clausedKind,
  clauseHeard,
  clauseOf,
  DECIDE,
  DECIDE_CLAUSE,
  DECIDE_REASON,
  DECIDE_SAID,
  DECIDE_TITLE,
  DISMISSED,
  editable,
  elsewhereLine,
  LEAVE_OPEN,
  leftOpenLine,
  missing,
  NOT_OFFERED,
  notOfferedLine,
  RECORD_IT,
  RECORD_OUTCOME,
  RECORDING,
  recordedLine,
  sectionOf,
  type Card,
} from "./sitting";
import { Copy } from "./Suggestion";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
import { Sheet } from "../shell/Sheet";
import "./sitting.css";

/**
 * What the Partner offered the record, as a card under its answer — and the one
 * press that writes it.
 *
 * The paper's rule for a sitting is that the room proposes and only records
 * rule, so this card never records by itself: **Record it** is the human's
 * press, and the card says "Recorded" only after the record has taken it. Until
 * then the words are theirs to change — the entry and its one clause are fields
 * on the card, not text to read — because the Partner may have heard the reason
 * roughly and may not have found the anchor at all.
 *
 * Two of the six states are the ones a human would notice if they were wrong. A
 * decision with no reason and a fact with no anchor cannot be recorded: the card
 * says which is missing and Record it waits, because an entry that says a choice
 * was made and cannot say why is exactly the entry a sitting exists to prevent.
 * And a press the record refused because it had moved keeps every word the human
 * typed and offers Record it again against the record as it now stands — nothing
 * is written, and nothing of theirs is lost.
 *
 * Two of the states came from Sol's read. A card offered to another record — the
 * sitting it belonged to ended, or moved on — offers no press at all, because
 * the press it used to offer wrote its words into whichever record the sitting
 * was now about; what is left to do with it is copy the words. And the two
 * fields are frozen while the press is in flight, because the entry the press
 * composed is the entry the record takes, and a field that kept accepting
 * keystrokes through the write let the card say "recorded" over words the record
 * never carried.
 */
/**
 * The words to copy from a card nobody can record here: the entry, and its one
 * clause where it has one, labelled as the card labelled it.
 *
 * Both, because the clause is half of what the card was: a decision without its
 * reason and a fact without its anchor are exactly the entries a sitting refuses
 * to record, so handing a human only the words would hand them the half that
 * cannot be recorded again.
 */
export function copyable(text: string, clause: string, label: string): string {
  const said = clause.trim();
  return said === "" ? text : `${text}\n${label}: ${said}`;
}

export function DepositCard({ id }: { id: string }) {
  const { deposits, editDeposit, editClause, recordDeposit, dismissDeposit, reopenDeposit } = usePartner();
  const card = cardIn(deposits, id);
  const entryField = useId();
  const clauseField = useId();
  if (card === undefined) {
    return null;
  }

  // Nothing was offered: there was no sitting to offer it to. There is nothing
  // to press and nothing to edit; what the card owes the human is the reason and
  // the words the Partner would have offered.
  if (card.standing === "refused") {
    return (
      <div className="ms-deposit ms-deposit--refused" data-deposit={id}>
        <p className="ms-deposit-head">
          <span>{NOT_OFFERED}</span>
          <Help id="not-offered" />
        </p>
        <p className="ms-deposit-reason" role="status">
          {notOfferedLine(card)}
        </p>
        <p className="ms-deposit-text">{card.text}</p>
      </div>
    );
  }

  // Offered to another record: the sitting it belonged to ended, or the human is
  // sitting on something else now. There is nothing here to press — its words
  // are not this record's — so the card says which record it was for and offers
  // the words to copy.
  if (card.standing === "elsewhere") {
    return (
      <div className="ms-deposit ms-deposit--elsewhere" data-deposit={id} data-kind={card.kind}>
        <p className="ms-deposit-head">
          <span>{cardHead(card.kind)}</span>
          <Help id="deposit" />
        </p>
        <p className="ms-deposit-reason" role="status">
          {elsewhereLine(card)}
        </p>
        <p className="ms-deposit-text">{card.mark.text}</p>
        <div className="ms-deposit-foot">
          <Copy text={copyable(card.mark.text, card.mark.clause, clauseOf(card.kind))} />
        </div>
      </div>
    );
  }

  if (card.standing === "dismissed") {
    return (
      <button
        type="button"
        className="ms-deposit-folded"
        title={`Show what the Partner offered again`}
        onClick={() => {
          reopenDeposit(id);
        }}
      >
        {DISMISSED}
      </button>
    );
  }

  // A case is not an entry and is not offered as one: it lands on no pile by
  // itself, and the human settles it or leaves it open (g1-s55 D1). So the card
  // for one has no fields and no Record it — two presses, and what each of them
  // records is a different entry. Once one of them has landed, it is what the
  // record says it is, and the card below says so like any other.
  if (card.kind === "case" && card.standing !== "recorded") {
    return <CaseCard card={card} />;
  }

  const recorded = card.standing === "recorded";
  const clause = clauseOf(card.kind);
  const needs = missing(card.kind, card.mark);
  // Frozen for the press in flight, and read-only rather than disabled: the
  // words are still what the human wrote and still there to be read, they are
  // just no longer theirs to change until the record has answered.
  const frozen = !editable(card.standing);
  return (
    <div className="ms-deposit" data-deposit={id} data-kind={card.kind}>
      <p className="ms-deposit-head">
        <span>{cardHead(card.kind)}</span>
        <Help id={card.kind === "outcome" ? "the-outcome" : "deposit"} />
        {/* Where it would land, or — once the record has taken it — where the
            record put it. The two are not always the same thing: a case lands on
            no pile by itself, and what lands is the decision or the open
            question the human made of it (g1-s55 D1). */}
        <span className="ms-deposit-where">{recorded ? card.mark.recorded : sectionOf(card.kind)}</span>
      </p>
      {/* The entry and its clause are the human's to change until the record has
          taken them. Once it has, they are what the record says, so the card
          shows them and stops offering to edit something that is written. */}
      {recorded ? (
        <>
          <p className="ms-deposit-text">{card.mark.text}</p>
          {card.mark.clause.trim() !== "" && (
            <p className="ms-deposit-clause">
              <span className="ms-deposit-label">{clause}</span>
              {card.mark.clause}
            </p>
          )}
        </>
      ) : (
        <>
          <label className="ms-visually-hidden" htmlFor={entryField}>
            {`The ${card.kind} to record`}
          </label>
          <textarea
            id={entryField}
            className="ms-deposit-field"
            /* An outcome is a whole section — what was decided, the constraints,
               what was left open and what the table holds — so the box it is
               read and edited in is the size of the thing rather than of an
               entry. */
            rows={card.kind === "outcome" ? 12 : 3}
            value={card.mark.text}
            readOnly={frozen}
            onChange={(event) => {
              editDeposit(id, event.target.value);
            }}
          />
          {clausedKind(card.kind) && (
            <>
              <label className="ms-deposit-label" htmlFor={clauseField}>
                {clause}
              </label>
              <input
                id={clauseField}
                className="ms-deposit-clause-field"
                type="text"
                value={card.mark.clause}
                placeholder={clause === "Anchor" ? "where it can be checked" : ""}
                readOnly={frozen}
                onChange={(event) => {
                  editClause(id, event.target.value);
                }}
              />
            </>
          )}
        </>
      )}
      {needs !== "" && (
        <p className="ms-deposit-needs" role="status">
          {needs}
        </p>
      )}
      {card.mark.refusal !== "" && (
        <p className="ms-deposit-refusal" role="status">
          {card.mark.refusal}
        </p>
      )}
      <div className="ms-deposit-foot">
        {recorded ? (
          <span className="ms-deposit-said">{recordedLine(card.mark.recorded)}</span>
        ) : (
          <>
            <Button
              primary
              disabled={needs !== "" || card.mark.recording}
              onClick={() => {
                recordDeposit(id);
              }}
            >
              {card.mark.recording ? RECORDING : card.kind === "outcome" ? RECORD_OUTCOME : RECORD_IT}
            </Button>
            <Button
              disabled={card.mark.recording}
              onClick={() => {
                dismissDeposit(id);
              }}
            >
              Dismiss
            </Button>
          </>
        )}
      </div>
    </div>
  );
}

/**
 * A case at the edge, with the two presses that settle it (g1-s55 D1).
 *
 * The Partner's job in a sitting is to produce the cases at the edge — "a person
 * reads a long page without touching anything; a laptop sleeps with the page
 * open" — and the human's is to say what the rule is for each of them, or to say
 * that it stays open. So this card is not an entry waiting for a press: it is a
 * question with two answers, and each answer records something different.
 *
 * Decide opens a small sheet with the clause as the Partner heard it and a line
 * for the reason, and records a decision. Leave open records an open question in
 * the Partner's own words, with the consequence it attached. Neither is the case:
 * a case lands on no pile by itself, and a case dismissed leaves nothing behind.
 *
 * Which is the third press, and it is here because a case the human has no answer
 * to and no wish to leave open has to be able to go: a card offering only the two
 * answers would keep asking for one of them. Dismissed, it folds to the line every
 * dismissed card folds to and writes nothing — there is nothing to undo.
 *
 * Both write through the one serialized recorder, against the one reading of the
 * record, exactly as every other press does — and both mark the entry with this
 * card, so the record says which deposit it came from and neither press can be
 * made twice.
 */
function CaseCard({ card }: { card: Card }) {
  const { recordDeposit, dismissDeposit } = usePartner();
  const [deciding, setDeciding] = useState(false);
  const inFlight = card.mark.recording;
  return (
    <div className="ms-deposit ms-deposit--case" data-deposit={card.id} data-kind={card.kind}>
      <p className="ms-deposit-head">
        <span>{cardHead(card.kind)}</span>
        <Help id="the-case" />
      </p>
      <p className="ms-deposit-text">{card.mark.text}</p>
      {card.mark.clause.trim() !== "" && (
        <p className="ms-deposit-clause">
          <span className="ms-deposit-label">{clauseOf(card.kind)}</span>
          {card.mark.clause}
        </p>
      )}
      {card.mark.refusal !== "" && (
        <p className="ms-deposit-refusal" role="status">
          {card.mark.refusal}
        </p>
      )}
      <div className="ms-deposit-foot">
        {inFlight ? (
          <span className="ms-deposit-said">{RECORDING}</span>
        ) : (
          <>
            <Button
              primary
              onClick={() => {
                setDeciding(true);
              }}
            >
              {DECIDE}
            </Button>
            <Button
              title={leftOpenLine(card.mark.clause)}
              onClick={() => {
                recordDeposit(card.id, {
                  kind: "question",
                  text: card.mark.text,
                  clause: card.mark.clause,
                });
              }}
            >
              {LEAVE_OPEN}
            </Button>
            <Button
              onClick={() => {
                dismissDeposit(card.id);
              }}
            >
              Dismiss
            </Button>
          </>
        )}
      </div>
      {/* The sheet stands until the record has taken the decision, so a press the
          record refused keeps the words the human wrote in it rather than sending
          them back to an empty box. */}
      <DecideSheet
        card={card}
        open={deciding && card.standing !== "recorded"}
        onOpenChange={setDeciding}
      />
    </div>
  );
}

/**
 * The small sheet Decide opens: the clause as heard, and the reason.
 *
 * The reason is required, and that is the one rule in here worth arguing about:
 * a sitting exists so that a choice is recorded with the reason the human gave,
 * then and there, rather than reconstructed afterwards. So Record it waits until
 * there is one, and says so.
 */
function DecideSheet({
  card,
  open,
  onOpenChange,
}: {
  card: Card;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { recordDeposit } = usePartner();
  const [clause, setClause] = useState(() => clauseHeard(card));
  const [reason, setReason] = useState("");
  const clauseField = useId();
  const reasonField = useId();
  const needs = missing("decision", { ...card.mark, text: clause, clause: reason });
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      side="right"
      label={DECIDE_TITLE}
      title={DECIDE_TITLE}
      closeLabel="Close without deciding this case"
      bodyClassName="ms-sitting-sheet"
      sheetName={DECIDE_TITLE}
    >
      <p className="ms-sitting-said">{DECIDE_SAID}</p>
      <p className="ms-deposit-text">{card.mark.text}</p>
      <label className="ms-sitting-label" htmlFor={clauseField}>
        {DECIDE_CLAUSE}
      </label>
      <textarea
        id={clauseField}
        className="ms-deposit-field"
        rows={3}
        value={clause}
        onChange={(event) => {
          setClause(event.target.value);
        }}
      />
      <label className="ms-sitting-label" htmlFor={reasonField}>
        {DECIDE_REASON}
      </label>
      <textarea
        id={reasonField}
        className="ms-deposit-field"
        rows={2}
        value={reason}
        onChange={(event) => {
          setReason(event.target.value);
        }}
      />
      {needs !== "" && (
        <p className="ms-deposit-needs" role="status">
          {needs}
        </p>
      )}
      {card.mark.refusal !== "" && (
        <p className="ms-deposit-refusal" role="status">
          {card.mark.refusal}
        </p>
      )}
      <div className="ms-sitting-foot">
        <Button
          primary
          disabled={needs !== "" || card.mark.recording}
          onClick={() => {
            recordDeposit(card.id, { kind: "decision", text: clause, clause: reason });
          }}
        >
          {card.mark.recording ? RECORDING : RECORD_IT}
        </Button>
      </div>
    </Sheet>
  );
}
