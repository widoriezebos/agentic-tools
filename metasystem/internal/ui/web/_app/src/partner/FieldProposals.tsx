import { useEffect } from "react";

import { usePartner } from "./store";
import {
  ASK_THE_PARTNER,
  askLine,
  EDITED_SINCE,
  moreLabel,
  PROPOSES,
  proposalsFor,
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
 * Nothing here saves and nothing here writes by itself. Use this puts the words
 * in the field and stops; Save is the human's, one press away, at the foot of the
 * same sheet. There is deliberately no button that does both: the sheet's save
 * reads its draft from the render and guards busy on its own button, so a second
 * caller would save the previous words or report a save it cannot confirm (Astra
 * F1, F2). That press waits for a submission path that can tell the truth.
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
}: {
  opening: string;
  field: string;
  /** What the field holds right now. */
  value: string;
}) {
  const { offered, noteField } = usePartner();
  // Every render, because every render is a keystroke or a press: the value
  // above is what the field holds now, and nothing else tells the store.
  useEffect(() => {
    noteField(opening, field, value);
  });
  const standing = proposalsFor(offered, opening, field);
  const newest = standing.at(0);
  if (newest === undefined) {
    return null;
  }
  return (
    <div className="ms-proposal" data-proposal={newest.id}>
      <p className="ms-proposal-head">
        <span>{PROPOSES}</span>
        <Help id="proposal" />
      </p>
      <p className="ms-proposal-text">{newest.text}</p>
      <div className="ms-proposal-foot">
        <Foot id={newest.id} standing={newest.standing} />
        {standing.length > 1 && <Older id={standing[1].id} count={standing.length - 1} field={field} />}
      </div>
    </div>
  );
}

/**
 * The three things a proposal under a field can be doing: waiting to be used,
 * used and undoable, or used and typed over.
 *
 * Undo is offered only while the field still holds the Partner's words. Once the
 * human has typed over them, putting back what was there would throw away their
 * words in the name of undoing ours, so the proposal says what happened and stops
 * offering (g1-s51 F2).
 */
function Foot({ id, standing }: { id: string; standing: string }) {
  const { use, undo } = usePartner();
  if (standing === "waiting") {
    return (
      <>
        <Button
          primary
          onClick={() => {
            use(id);
          }}
        >
          Use this
        </Button>
        <Dismiss id={id} />
      </>
    );
  }
  if (standing === "used") {
    return (
      <>
        <span className="ms-proposal-said">Used</span>
        <Button
          onClick={() => {
            undo(id);
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
 * The older proposals for this field, behind one line.
 *
 * They are the record rather than the offer, so the line opens the drawer at the
 * card that holds the next one, which is where a conversation's own history is
 * read. What the field shows whole is the newest, because that is the one a human
 * is deciding about.
 */
function Older({ id, count, field }: { id: string; count: number; field: string }) {
  const { show } = usePartner();
  return (
    <button
      type="button"
      className="ms-proposal-more"
      title={`Open your Project Partner at the earlier proposals for ${field}`}
      onClick={() => {
        show(id);
      }}
    >
      {moreLabel(count)}
    </button>
  );
}
