import { useEffect, useState } from "react";

import { usePartner } from "./store";
import {
  ASK_THE_PARTNER,
  askLine,
  EDITED_SINCE,
  FEWER,
  moreLabel,
  PROPOSED_EARLIER,
  PROPOSES,
  proposalsFor,
  refusedSaveLine,
  SAVING,
  unresolvedSaveLine,
  USE_AND_SAVE,
  USED_AND_SAVED,
  type Offered,
} from "./suggesting";
import { Help } from "../help/Help";
import { Button } from "../shell/controls";

/**
 * The Partner's words where the human is writing: under the field they are for,
 * inside the sheet, with Use this beside them — and, on the field's own label,
 * the link that asks for them.
 *
 * This is the whole of what g1-s52 changes about where things stand. The words
 * used to arrive as a card in the drawer, and the drawer's default height hides a
 * card behind the composer: a human asked the Partner to rewrite an intent, was
 * told a card had been put on the sheet, and saw nothing. So the proposal renders
 * under its field. The card in the transcript stays as the record, and both read
 * the same store state, so the two cannot disagree about one proposal.
 *
 * Nothing here writes by itself, and nothing here knows how to save. Use this
 * puts the words in the field and stops; Save is the human's, at the foot of the
 * same sheet. Beside it, where the sheet owns a submission path that takes the
 * next draft explicitly and reports what the ledger did with it, stands the one
 * press that does both: Use and save (g1-s56 D2). What it says afterwards is what
 * actually happened — used and saved, or used with the save refused in the
 * engine's own words, or used with the save unconfirmed — because the sheet
 * answers with its outcome rather than with the fact that it tried (Astra F1, F2
 * on g1-s52, answered by the path this block now calls).
 */

/**
 * The link on a writable field's label: "Ask the Partner".
 *
 * It is the entry, and it is on the field because that is where the human is
 * when they want it. Pressing it puts a request for this field in the composer
 * and takes the caret there; it sends nothing, because the request is still
 * theirs to finish. It also says the caret was here, so a request that names no
 * field is about this one (D4, D6).
 */
export function AskThePartner({
  opening,
  field,
  value,
}: {
  /** The opening this field belongs to, as the sheet minted it. */
  opening: string;
  /** The field, as its own label says it. */
  field: string;
  /** What the field holds right now, which decides how the request is worded. */
  value: string;
}) {
  const { fillComposer, noteWriting } = usePartner();
  return (
    <button
      type="button"
      className="ms-field-ask"
      title={`Put a request for ${field} in your Project Partner's composer. Nothing is sent.`}
      onClick={() => {
        noteWriting(opening, field);
        fillComposer(askLine(field, value));
      }}
    >
      {ASK_THE_PARTNER}
    </button>
  );
}

/**
 * What the Partner has proposed for this field, under it, inside the sheet.
 *
 * The newest proposal stands whole, because words a human is deciding about have
 * to be readable without opening anything. Older ones are behind "n more", which
 * opens the drawer at the card that is the record of them. Nothing shows here
 * that the human folded away, and nothing shows that was never offered — a
 * refusal belongs where the answer is read, not under a field it was refused for.
 *
 * It is also how a proposal learns that the human has typed since: the field says
 * what it now holds, and Undo stops being offered once that is no longer the
 * words the Partner wrote. That report is here rather than on the label because
 * this is the part of the field that is on screen whenever a proposal is.
 */
export function FieldProposals({
  opening,
  field,
  value,
  saves = false,
}: {
  opening: string;
  field: string;
  /** What the field holds right now. */
  value: string;
  /**
   * Whether this sheet can send what it holds, which is what makes Use and save
   * possible. A sheet without a submission path offers Use this alone.
   */
  saves?: boolean;
}) {
  const { offered, noteField } = usePartner();
  // Every render, because every render is a keystroke or a press: the value
  // above is what the field holds now, and nothing else tells the store.
  useEffect(() => {
    noteField(opening, field, value);
  });
  // Whether the earlier proposals for this field are unfolded. It is this
  // block's own state and nothing else's: unfolding is a way of looking, not
  // something that happened to a proposal.
  const [unfolded, setUnfolded] = useState(false);
  const standing = proposalsFor(offered, opening, field);
  const newest = standing.at(0);
  if (newest === undefined) {
    return null;
  }
  const older = standing.slice(1);
  return (
    <div className="ms-proposals">
      <div className="ms-proposal" data-proposal={newest.id}>
        <p className="ms-proposal-head">
          <span>{PROPOSES}</span>
          <Help id="proposal" />
        </p>
        <p className="ms-proposal-text">{newest.text}</p>
        <div className="ms-proposal-foot">
          <Foot card={newest} saves={saves} />
          {older.length > 0 && (
            <Older
              count={older.length}
              field={field}
              unfolded={unfolded}
              onToggle={() => {
                setUnfolded((shown) => !shown);
              }}
            />
          )}
        </div>
      </div>
      {unfolded &&
        older.map((card) => (
          <div className="ms-proposal ms-proposal--older" key={card.id} data-proposal={card.id}>
            <p className="ms-proposal-head">
              <span>{PROPOSED_EARLIER}</span>
            </p>
            <p className="ms-proposal-text">{card.text}</p>
            <div className="ms-proposal-foot">
              <Foot card={card} saves={saves} />
            </div>
          </div>
        ))}
    </div>
  );
}

/**
 * What a proposal under a field is doing: waiting to be used, used and undoable,
 * used and typed over, or used by the press that also saved — which then says
 * what the saving did.
 *
 * Undo is offered only while the field still holds the Partner's words and only
 * where no save was asked for. Once the human has typed over them, putting back
 * what was there would throw away their words in the name of undoing ours
 * (g1-s51 F2); once a save has carried them past this page, undoing the field
 * alone would leave the sheet and the ledger disagreeing (g1-s56 D2).
 *
 * The three sentences a save can end in are said in the order they become true,
 * and each is only ever about this press: what the field holds is never taken
 * back by any of them, which is why every one of them begins "Used".
 */
function Foot({ card, saves }: { card: Offered; saves: boolean }) {
  const { use, useAndSave, undo } = usePartner();
  const { sent, words } = card.mark;
  if (card.standing === "waiting") {
    return (
      <>
        <Button
          primary
          onClick={() => {
            use(card.id);
          }}
        >
          Use this
        </Button>
        {saves && (
          <Button
            onClick={() => {
              void useAndSave(card.id);
            }}
          >
            {USE_AND_SAVE}
          </Button>
        )}
        <Dismiss id={card.id} />
      </>
    );
  }
  if (card.standing === "saved") {
    return <span className="ms-proposal-said">{USED_AND_SAVED}</span>;
  }
  if (sent === "saving") {
    return <span className="ms-proposal-said">{SAVING}</span>;
  }
  if (sent === "refused") {
    return (
      <span className="ms-proposal-said" role="status">
        {refusedSaveLine(words)}
      </span>
    );
  }
  if (sent === "unresolved") {
    return (
      <span className="ms-proposal-said" role="status">
        {unresolvedSaveLine(words)}
      </span>
    );
  }
  if (card.standing === "used") {
    return (
      <>
        <span className="ms-proposal-said">Used</span>
        <Button
          onClick={() => {
            undo(card.id);
          }}
        >
          Undo
        </Button>
      </>
    );
  }
  return <span className="ms-proposal-said">Used · {EDITED_SINCE}</span>;
}

/** Folded away, here and in the transcript: one act, one state, one store. */
function Dismiss({ id }: { id: string }) {
  const { dismiss } = usePartner();
  return (
    <Button
      onClick={() => {
        dismiss(id);
      }}
    >
      Dismiss
    </Button>
  );
}

/**
 * The older proposals for this field, behind one line — and unfolded in place by
 * pressing it.
 *
 * It used to open the drawer at the next-newest card, which sent a human away
 * from the field they were deciding about in order to read a proposal for it, and
 * into a column whose height is why the proposal came down here in the first
 * place (g1-s52 Built, deferred). So they unfold here, under the newest, each
 * with its own press. The newest is still the one that stands whole and unasked
 * for: what a human is deciding about is the words the Partner wrote last.
 */
function Older({
  count,
  field,
  unfolded,
  onToggle,
}: {
  count: number;
  field: string;
  unfolded: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      className="ms-proposal-more"
      aria-expanded={unfolded}
      title={
        unfolded
          ? `Fold the earlier proposals for ${field} away again`
          : `Show the earlier proposals for ${field}, here under this one`
      }
      onClick={onToggle}
    >
      {unfolded ? FEWER : moreLabel(count)}
    </button>
  );
}
