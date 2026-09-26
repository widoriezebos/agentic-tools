import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Suggestion } from "./api";
import { FieldProposals } from "./FieldProposals";
import { PartnerAs } from "./store";
import {
  answered,
  idOf,
  moreLabel,
  offeredIn,
  refusedSave,
  SAVED,
  saving,
  unresolvedSave,
  USE_AND_SAVE,
  USED_AND_SAVED,
  type Marks,
} from "./suggesting";

/**
 * The block under a field, in the states one press can leave it in.
 *
 * It is read from the markup because that is where the claim is: the second
 * button is either offered on this sheet or it is not, and what the block says
 * after a save either names what happened or it does not. Which state a card is
 * in, given a press and an answer, is asserted in suggesting.test.ts over the
 * rules themselves.
 */

const OPENING = "opening-1";
const FIELD = "Intent";
const SAID = "Every refund lands within a day, with nobody touching the queue.";
const ID = idOf("t1", 0);

function suggestion(over: Partial<Suggestion> = {}): Suggestion {
  return { opening: OPENING, editor: "Edit goal", field: FIELD, text: SAID, offered: true, ...over };
}

/** The block, over one conversation's cards and one sheet's own answer to "can you save". */
function block(marks: Marks = {}, saves = true, ...offers: Suggestion[]): string {
  const cards = offeredIn(
    [{ turn: "t1", suggestions: offers.length > 0 ? offers : [suggestion()] }],
    marks,
    [OPENING],
  );
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <PartnerAs held={{ offered: cards }}>
        <FieldProposals opening={OPENING} field={FIELD} value="The board reads the ledger." saves={saves} />
      </PartnerAs>
    </TooltipPrimitive.Provider>,
  );
}

describe("a proposal waiting under a field", () => {
  it("offers the second press where the sheet can send what it holds", () => {
    const markup = block();
    expect(markup).toContain(SAID);
    expect(markup).toContain(">Use this<");
    expect(markup).toContain(`>${USE_AND_SAVE}<`);
    expect(markup).toContain(">Dismiss<");
  });

  /**
   * And nowhere else. Every other sheet in this build supplies no submission
   * path, so a press that claimed to save would be a press that could not.
   */
  it("offers Use this alone where the sheet supplies no submission path", () => {
    const markup = block({}, false);
    expect(markup).toContain(">Use this<");
    expect(markup).not.toContain(USE_AND_SAVE);
  });
});

describe("what the block says after that press", () => {
  it("says the words are in and the sheet is sending them", () => {
    const markup = block(saving({}, ID, "The board reads the ledger."));
    expect(markup).toContain("Used · saving…");
    expect(markup).not.toContain(">Undo<");
    expect(markup).not.toContain(USED_AND_SAVED);
  });

  it("says Used and saved on the one outcome that is a save", () => {
    const markup = block(answered(saving({}, ID, "was"), ID, SAVED));
    expect(markup).toContain(USED_AND_SAVED);
    expect(markup).not.toContain(">Undo<");
    expect(markup).not.toContain(">Use this<");
  });

  /**
   * The refusal is the engine's own sentence, beside words that stay in the
   * field: the human put them there, and the ledger said no to them. Save at the
   * foot of the sheet is still theirs, under its normal validation.
   */
  it("keeps the words and says why the save was refused", () => {
    const refusal = "goal ui-1 is approved: withdraw the approval, edit it, then approve it again";
    const markup = block(answered(saving({}, ID, "was"), ID, refusedSave(refusal)));
    expect(markup).toContain(SAID);
    expect(markup).toContain("Used; the save was refused:");
    expect(markup).toContain(refusal);
    expect(markup).not.toContain(">Undo<");
  });

  it("never calls a save nobody could confirm a refusal", () => {
    const said = "the act landed at tip 6984cde, but its authority proof did not; do not run it again";
    const markup = block(answered(saving({}, ID, "was"), ID, unresolvedSave(said)));
    expect(markup).toContain("Used; the save was not confirmed:");
    expect(markup).toContain(said);
    expect(markup).not.toContain("was refused");
  });
});

/**
 * The earlier proposals for this field.
 *
 * They stand behind one line that unfolds them HERE, under the newest, rather
 * than sending a human to the Partner's own column to read a proposal for the
 * field they are writing in (g1-s52 Built, deferred). Folded is what a static
 * render can show: the line, what it says, and that the older words are not on
 * screen until it is pressed.
 */
describe("more than one proposal for one field", () => {
  it("stands the newest whole, with the earlier ones behind one line", () => {
    const markup = block({}, true, suggestion(), suggestion({ text: "A second wording." }));
    expect(markup).toContain("A second wording.");
    expect(markup).not.toContain(SAID);
    expect(markup).toContain(moreLabel(1));
    expect(markup).toContain('aria-expanded="false"');
    // One proposal on screen, so one press to make: the older ones unfold.
    expect(markup.match(/ms-proposal-text/g)).toHaveLength(1);
  });
});
