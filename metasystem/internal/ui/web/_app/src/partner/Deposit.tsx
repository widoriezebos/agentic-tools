import { useId } from "react";

import { usePartner } from "./store";
import {
  cardHead,
  cardIn,
  clauseOf,
  DISMISSED,
  missing,
  NOT_OFFERED,
  notOfferedLine,
  RECORD_IT,
  RECORDING,
  recordedLine,
  sectionOf,
} from "./sitting";
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
 */
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
