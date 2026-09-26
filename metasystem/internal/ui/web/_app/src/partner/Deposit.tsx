import { useId } from "react";

import { usePartner } from "./store";
import {
  cardHead,
  cardIn,
  clauseOf,
  DISMISSED,
  editable,
  elsewhereLine,
  missing,
  NOT_OFFERED,
  notOfferedLine,
  RECORD_IT,
  RECORDING,
  recordedLine,
  sectionOf,
} from "./sitting";
import { Copy } from "./Suggestion";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";
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
        title={`Show what the Partner offered for ${sectionOf(card.kind)} again`}
        onClick={() => {
          reopenDeposit(id);
        }}
      >
        {DISMISSED}
      </button>
    );
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
        <Help id="deposit" />
        <span className="ms-deposit-where">{sectionOf(card.kind)}</span>
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
            rows={3}
            value={card.mark.text}
            readOnly={frozen}
            onChange={(event) => {
              editDeposit(id, event.target.value);
            }}
          />
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
              {card.mark.recording ? RECORDING : RECORD_IT}
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
