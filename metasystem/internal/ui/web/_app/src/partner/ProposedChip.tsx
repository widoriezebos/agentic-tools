import { chipName, chipWords, type Proposed } from "./proposed";
import { usePartner, useProposedFor } from "./store";
import "./proposal.css";

/**
 * One chip on a goal's own row: the Partner has an opinion on this goal, and
 * nobody has answered it.
 *
 * It is here because a proposal that waits has to be where the human looks, and
 * the human looks at the board. The conversation is where a proposal is made and
 * answered, the Decisions inbox is where one that waits lives, and neither is
 * where a morning of triage happens: a goal the Partner said "put this away"
 * about last night looked exactly like a goal nobody had ever mentioned.
 *
 * One chip per row, whatever the row is — the board's card, the queue's row, the
 * goal page's own header — because a row is read at a glance and four verbs on
 * one is a row nobody reads. What it says is the act where there is one of them
 * and a count where there are several, in the words the button that offers that
 * act uses, and a refused or an unresolved line puts it in the danger colour,
 * since those wait on the human too and not for a yes (g1-s61 D2).
 *
 * It is a button, and it does one thing: it opens the conversation at the card
 * where that line is decided (D3). So it is never icon-only and never a bare
 * count — its accessible name is the whole sentence, with the goal in it, because
 * a chip heard out of its row names nothing.
 */

/**
 * The chip as markup, over lines somebody else read out of the store.
 *
 * It is separated from the reading so that what it says can be rendered and
 * asserted without a conversation, a provider or a press: the three rows below
 * hand it the hook's answer, and the test hands it a list.
 */
export function ProposedChipView({
  goal,
  lines,
  onPress,
  onMouseDown,
}: {
  goal: string;
  lines: readonly Proposed[];
  onPress: () => void;
  /**
   * Said as the press begins, where the row it is on needs to know. The board's
   * card is a drag handle, and a pointer that goes down on this chip and then
   * moves is a human reaching for the chip rather than for the card: the card
   * stops being draggable while it is down, the way it does for the slice
   * disclosure.
   */
  onMouseDown?: () => void;
}) {
  const said = chipWords(lines);
  // No line waiting is no chip. Not a chip that says nothing, and not an empty
  // box: a row with nothing proposed on it is the ordinary row, unchanged.
  if (said === null) {
    return null;
  }
  return (
    <button
      type="button"
      className={said.danger ? "ms-chip ms-chip-proposed ms-chip-proposed--wrong" : "ms-chip ms-chip-proposed"}
      aria-label={chipName(goal, lines)}
      onMouseDown={onMouseDown}
      onClick={onPress}
    >
      {said.words}
    </button>
  );
}

/**
 * The chip on one goal's row, reading the conversation the shell already holds.
 *
 * This is what the three rows render. It asks the store for that goal's waiting
 * lines and hands the press back to the store: nothing is fetched, and a row
 * rendered outside the conversation's provider reads an empty conversation and
 * shows nothing.
 */
export function ProposedChip({ goal, onMouseDown }: { goal: string; onMouseDown?: () => void }) {
  const lines = useProposedFor(goal);
  const { showProposedFor } = usePartner();
  return (
    <ProposedChipView
      goal={goal}
      lines={lines}
      onMouseDown={onMouseDown}
      onPress={() => {
        showProposedFor(goal);
      }}
    />
  );
}
