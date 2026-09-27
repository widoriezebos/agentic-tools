import { useEffect, useId, useRef, type ReactNode } from "react";
import { NavLink } from "react-router";

import { bringUp } from "./scrolling";
import {
  APPLY,
  applyLabel,
  argumentsOf,
  ASK_THE_PARTNER,
  busyAnswering,
  cardHead,
  cardIn,
  CONTINUE,
  dismissableIn,
  DISMISS,
  footLine,
  foldedLine,
  lineState,
  NOT_OFFERED,
  offersContinue,
  offersTryAgain,
  OPEN_THE_GOAL,
  SELECT_ALL,
  selectedLine,
  SIGN_IN_TO_APPLY,
  ticked,
  TRY_AGAIN,
  verbWord,
  waiting,
  waitingIn,
  type Card,
  type Line,
} from "./proposing";
import { usePartner } from "./store";
import "./proposal.css";
import { Help } from "../help/Help";
import { goalPath } from "../routes";
import { Button } from "../shell/controls";
import { useSession } from "../shell/identity";

/**
 * The acts the Partner proposed, as a card under its answer — and the one press
 * that applies them.
 *
 * The card IS the confirmation control. It carries the whole reviewed basis —
 * every argument the act will carry, the approval's intent and next step and the
 * budget with where it came from — so no sheet repeats it: a second dialog
 * listing what the card already lists would be a press that adds nothing to read.
 *
 * Nothing happens unless the human presses. The Partner cannot act; the press is
 * theirs, under their own sign-in, and the ledger names them as the hand. Until
 * the answer's terminal beat the checkboxes and buttons are asleep, because until
 * then there is no message for an outcome to be recorded on (Astra S58-04).
 *
 * Every line says where it stands, and the words are the engine's own wherever
 * the engine said them. A refused line offers Try again and Ask the Partner; a
 * line whose answer said the act landed and its proof did not offers neither,
 * because sending it again would make a second act rather than repair the first
 * (Astra S58-05).
 *
 * Where a card that waits BELONGS is the Decisions inbox, in step 2. Until then
 * an older card with a waiting line folds to one line, so a stale Apply is never
 * the first thing in view and a conversation does not bury a decision (D8).
 */
export function ProposalCard({ id }: { id: string }) {
  const { proposals, reopenProposals, showing } = usePartner();
  const card = cardIn(proposals, id);
  const box = useRef<HTMLDivElement | null>(null);
  // The bar's count opened the drawer at this card, so the column comes to it.
  useEffect(() => {
    if (showing === id) {
      bringUp(box.current);
    }
  }, [showing, id]);
  if (card === undefined) {
    return null;
  }

  if (card.folded) {
    return (
      <button
        type="button"
        className="ms-proposal-folded"
        title="Show what the Partner proposed"
        onClick={() => {
          reopenProposals(id);
        }}
      >
        {foldedLine(card)}
      </button>
    );
  }

  return (
    <div ref={box} className="ms-proposal" data-proposal={id}>
      <p className="ms-proposal-head">
        <span>{cardHead(card)}</span>
        <Help id="proposed-action" />
      </p>
      {card.lines.map((line) => (
        <ProposalLine key={line.id} card={card} line={line} />
      ))}
      <Foot card={card} />
    </div>
  );
}

/**
 * One line: the word the page's own button uses, the subject with its id, every
 * argument the act will carry, the Partner's why, and where it stands.
 *
 * The checkbox is on a card of more than one action. A card of one has no
 * checkbox and one button: there is nothing to choose between.
 */
function ProposalLine({ card, line }: { card: Card; line: Line }) {
  const { tickProposal, tryProposal, askAboutProposal, runningProposals } = usePartner();
  const tick = useId();
  const many = card.lines.filter((one) => one.offered).length > 1;
  const running = runningProposals !== "";
  // In flight while THIS page is running it. A line found at `applying` by a page
  // that has just loaded is one a page went away in the middle of, and says so.
  const said = lineState(line, running);

  if (!line.offered) {
    return (
      <div className="ms-proposal-line ms-proposal-line--refused" data-line={line.id}>
        <p className="ms-proposal-verb">
          <span className="ms-proposal-not-offered">{NOT_OFFERED}</span>
          <span>{verbWord(line.verb)}</span>
          <span className="ms-proposal-id ms-mono">{line.goal}</span>
        </p>
        <p className="ms-proposal-said" role="status">
          {line.reason}
        </p>
      </div>
    );
  }

  return (
    <div className="ms-proposal-line" data-line={line.id} data-state={line.state}>
      <ProposalSubstance
        line={line}
        tick={
          many && (
            <>
              <label className="ms-visually-hidden" htmlFor={tick}>
                {`Apply ${verbWord(line.verb)} on ${line.title}`}
              </label>
              <input
                id={tick}
                className="ms-proposal-tick"
                type="checkbox"
                checked={line.mark.ticked}
                disabled={!waiting(line) || running}
                onChange={(event) => {
                  tickProposal(line.id, event.target.checked);
                }}
              />
            </>
          )
        }
      />
      {said !== "" && (
        <p className="ms-proposal-said" data-said={line.state} role="status">
          {said}
        </p>
      )}
      {offersTryAgain(line, running) && !running && (
        <div className="ms-proposal-recovery">
          <button
            type="button"
            className="ms-act-link"
            onClick={() => {
              tryProposal(line.id);
            }}
          >
            {TRY_AGAIN}
          </button>
          <button
            type="button"
            className="ms-act-link"
            onClick={() => {
              askAboutProposal(line.id);
            }}
          >
            {ASK_THE_PARTNER}
          </button>
          <NavLink className="ms-act-link" to={goalPath(line.goal)}>
            {OPEN_THE_GOAL}
          </NavLink>
        </div>
      )}
    </div>
  );
}

/**
 * What one proposed action IS, whichever surface is showing it: the word the
 * page's own button uses, the subject with its id, every argument the act will
 * carry, and the Partner's own words about it.
 *
 * It is one component because there are two surfaces. The card in the transcript
 * is the answer to what the human just asked; the row in the Decisions inbox is
 * where the same action waits when they did not answer it (g1-s60 D3). A second
 * rendering would be a second vocabulary, and the two would drift on the day a
 * verb takes another argument.
 *
 * It is pure and takes no store: what is passed in is the line, and the checkbox
 * where the surface showing it has one. Where the line stands, and what may be
 * pressed on it, belong to the surface — a card counts a run and a row does not.
 */
export function ProposalSubstance({ line, tick }: { line: Line; tick?: ReactNode }) {
  return (
    <>
      <p className="ms-proposal-verb">
        {tick}
        <span className="ms-proposal-word">{verbWord(line.verb)}</span>
        <span className="ms-proposal-title">{line.title}</span>
        <NavLink className="ms-proposal-id ms-mono" to={goalPath(line.goal)} title={OPEN_THE_GOAL}>
          {line.goal}
        </NavLink>
      </p>
      {argumentsOf(line).map((argument) => (
        <p className="ms-proposal-argument" key={argument.label}>
          <span className="ms-proposal-label">{argument.label}</span>
          {argument.value}
        </p>
      ))}
      {(line.why ?? "") !== "" && <p className="ms-proposal-why">{`The Partner: ${line.why ?? ""}`}</p>}
    </>
  );
}

/**
 * The foot: how many are selected while the human is still choosing, what
 * happened once anything has, and the presses.
 *
 * Apply says how many it would send. With no proven session it says "Sign in to
 * apply" and opens the sign-in sheet, and the run starts when the code is
 * accepted — the same path every act from this browser takes (D7).
 */
function Foot({ card }: { card: Card }) {
  const {
    applyProposals,
    continueProposals,
    dismissProposals,
    selectProposals,
    runningProposals,
    store,
  } = usePartner();
  const { session, askToSignIn } = useSession();
  const signedIn = session.state === "known" && session.session.signedIn;
  const running = runningProposals === card.id;
  const wake = !busyAnswering(card, store.live.turn) && runningProposals === "";
  const waitingLines = waitingIn(card.lines);
  // What can still be put away: the waiting lines, and a line a page went away
  // in the middle of. Nothing else will ever settle that one — the run that
  // wrote `applying` is gone — so a card without this offer keeps it for good
  // (Sol S58-C-06). The route admits `applying` to `dismissed` for exactly this.
  const openLines = dismissableIn(card.lines);
  const foot = footLine(card);
  const many = card.lines.filter((line) => line.offered).length > 1;

  const apply = () => {
    if (signedIn) {
      applyProposals(card.id);
      return;
    }
    // The run never waits on a sheet: the sheet's success starts it, and a sheet
    // closed without signing in leaves the card exactly as it was.
    askToSignIn(() => {
      applyProposals(card.id);
    });
  };

  return (
    <div className="ms-proposal-foot">
      {/* How many are selected, where there is a choice to make. A card of one
          action has no checkbox and one button, so a count beside it would be
          counting the only thing there is (D3). */}
      {many && waitingLines > 0 && <span className="ms-proposal-count">{selectedLine(card)}</span>}
      {foot !== "" && <span className="ms-proposal-tally">{foot}</span>}
      {waitingLines > 0 && (
        <>
          <Button
            primary
            disabled={!wake || ticked(card) === 0}
            onClick={apply}
          >
            {running ? `${APPLY}…` : signedIn ? applyLabel(card) : SIGN_IN_TO_APPLY}
          </Button>
          {many && (
            <Button
              disabled={!wake}
              onClick={() => {
                selectProposals(card.id);
              }}
            >
              {SELECT_ALL}
            </Button>
          )}
        </>
      )}
      {offersContinue(card) && (
        <Button
          disabled={!wake}
          onClick={() => {
            continueProposals(card.id);
          }}
        >
          {CONTINUE}
        </Button>
      )}
      {openLines > 0 && (
        <Button
          disabled={!wake}
          onClick={() => {
            dismissProposals(card.id);
          }}
        >
          {DISMISS}
        </Button>
      )}
    </div>
  );
}
