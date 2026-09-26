import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Proposal } from "./api";
import { ProposalCard } from "./Proposal";
import { cardsIn, lineID, NEEDS_ITS_BUDGET, type Card, type Displayeds, type Marks } from "./proposing";
import { PartnerAs } from "./store";
import type { Budget } from "../backlog/api";

/**
 * What the card puts on the screen.
 *
 * The card is the confirmation control, so what is asserted here is that every
 * argument the act will carry is ON it: a human who presses Apply has read the
 * whole of what will happen, and nothing is behind a second dialog. The words come
 * from the validated action and never from the Partner's prose, which is why the
 * Partner's own sentence is the one thing marked as its words.
 *
 * The presses themselves are proved in proposing.test.ts, over the run's own
 * rules; what this file holds is that the reader can see what they are pressing.
 */

const BOX: Budget = {
  elapsedLimit: "4h",
  attemptLimit: 6,
  reservedJobMinutesLimit: 720,
  activeJobLimit: 1,
  reviewRoundLimit: 2,
};

function proposal(over: Partial<Proposal> = {}): Proposal {
  return {
    index: 0,
    verb: "park-goal",
    goal: "fleet-presence",
    title: "Fleet presence is read from the census",
    fields: { because: "superseded by the seat inventory (g1-s42)" },
    read: null,
    why: "the five name the fleet inventory g1-s42 built",
    offered: true,
    reason: "",
    state: "waiting",
    words: "",
    at: "2026-09-26T12:00:00Z",
    version: 1,
    ...over,
  };
}

function rendered(
  proposals: readonly Proposal[],
  marks: Marks = {},
  displayed: Displayeds = {},
  dismissed: readonly string[] = [],
): string {
  const cards: readonly Card[] = cardsIn([{ turn: "t1", proposals }], marks, displayed, dismissed);
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ proposals: cards }}>
          <ProposalCard id="t1" />
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the card the Partner's proposed acts stand on", () => {
  it("heads itself with how many actions the answer proposed", () => {
    expect(rendered([proposal()])).toContain("The Partner proposes");
    expect(rendered([proposal({ index: 0 }), proposal({ index: 1, goal: "refunds" })]))
      .toContain("The Partner proposes 2 actions");
  });

  /**
   * The word is the page's own button word, the subject is the title as the pages
   * say it with its id beside it, and every argument is there to read.
   */
  it("says the act in the page's own word, with the subject and every argument", () => {
    const markup = rendered([proposal()]);
    expect(markup).toContain("Not now");
    expect(markup).toContain("Fleet presence is read from the census");
    expect(markup).toContain("fleet-presence");
    expect(markup).toContain("Reason");
    expect(markup).toContain("superseded by the seat inventory (g1-s42)");
    // And the Partner's own words are marked as its words.
    expect(markup).toContain("The Partner: the five name the fleet inventory g1-s42 built");
  });

  /** A card of one action has no checkbox: there is nothing to choose between. */
  it("offers no checkbox on a card of one action", () => {
    expect(rendered([proposal()])).not.toContain('type="checkbox"');
    expect(rendered([proposal({ index: 0 }), proposal({ index: 1, goal: "refunds" })]))
      .toContain('type="checkbox"');
  });

  /**
   * An approval carries the whole reviewed basis: the intent and the next step as
   * the Partner read them, and the budget with where it came from.
   */
  it("shows an approval's reviewed intent and its budget", () => {
    const markup = rendered(
      [
        proposal({
          verb: "approve-goal",
          fields: {},
          read: { intent: "Refunds land within a day.", nextStep: "Read the retry loop.", tier: 3, labels: [] },
        }),
      ],
      {},
      { [lineID("t1", 0)]: { budget: BOX, source: "project" } },
    );
    expect(markup).toContain("Approve");
    expect(markup).toContain("Refunds land within a day.");
    expect(markup).toContain("Read the retry loop.");
    expect(markup).toContain("4h elapsed");
    expect(markup).toContain("from the project&#x27;s budget law for this goal&#x27;s tier");
  });

  it("names the approval it cannot prefill a budget for", () => {
    const markup = rendered(
      [proposal({ verb: "approve-goal", fields: {} })],
      {},
      { [lineID("t1", 0)]: { budget: null, source: "none" } },
    );
    expect(markup).toContain(NEEDS_ITS_BUDGET);
  });

  /** An open shows the tier it derived, with the four answers named. */
  it("shows an open's derived tier and the answers it came from", () => {
    const markup = rendered([
      proposal({
        verb: "open-goal",
        goal: "refund-worker",
        title: "Every refund lands within a day",
        fields: {
          id: "refund-worker",
          intent: "Every refund lands within a day, with nobody touching the queue.",
          nextStep: "Read the refund worker's retry loop.",
          severity: "2", novelty: "1", exposure: "2", accumulation: "1",
          basis: "payments, one team, one month of history",
          labels: "payments, robustness",
          blockedBy: "bank-sandbox",
        },
      }),
    ]);
    expect(markup).toContain("Open goal");
    expect(markup).toContain("2, from severity 2 · novelty 1 · exposure 2 · accumulation 1");
    expect(markup).toContain("payments, one team, one month of history");
    expect(markup).toContain("bank-sandbox");
  });

  /** An action the service did not offer says why, and offers nothing. */
  it("says why an action was not offered, and offers nothing for it", () => {
    const markup = rendered([
      proposal({ offered: false, goal: "no-such-goal", reason: "the accepted tip carries no goal no-such-goal" }),
    ]);
    expect(markup).toContain("Not offered");
    expect(markup).toContain("the accepted tip carries no goal no-such-goal");
    expect(markup).not.toContain("Try again");
  });

  /** A refused line offers the three ways on; a landed one offers none. */
  it("offers Try again on a refused line and none on an applied one", () => {
    const refusedMarkup = rendered([proposal({ state: "refused", words: "goal is claimed by m1e" })]);
    expect(refusedMarkup).toContain("refused: goal is claimed by m1e");
    expect(refusedMarkup).toContain("Try again");
    expect(refusedMarkup).toContain("Ask the Partner");
    expect(refusedMarkup).toContain("Open the goal");

    const applied = rendered([proposal({ state: "applied" })]);
    expect(applied).toContain("applied");
    expect(applied).not.toContain("Try again");
  });

  /**
   * A line the page went away in the middle of says it was being applied, never
   * says fresh, and can still be put away.
   *
   * The second half is what a card without it costs: nothing will ever settle
   * that line — the run that wrote `applying` is gone — so the card would keep
   * it for good. The route admits `applying` to `dismissed` for exactly this
   * (Sol S58-C-06).
   */
  it("offers Try again and Dismiss for a line left in flight", () => {
    const markup = rendered([proposal({ state: "applying" })]);
    expect(markup).toContain("was being applied when the page left; check the goal before applying again");
    expect(markup).toContain("Try again");
    expect(markup).toContain("Dismiss");
    // And no Apply: there is nothing waiting on this card to apply.
    expect(markup).not.toContain(">Apply<");
  });

  /** A card whose every line is settled offers no Dismiss: there is nothing to put away. */
  it("offers no Dismiss where every line is settled", () => {
    const markup = rendered([proposal({ state: "applied" })]);
    expect(markup).not.toContain("Dismiss");
  });

  /** The foot counts what happened, and offers Continue where a run stopped short. */
  it("counts the run in its foot and offers Continue where lines were not run", () => {
    const markup = rendered(
      [
        proposal({ index: 0, state: "applied" }),
        proposal({ index: 1, goal: "refunds", state: "refused", words: "claimed" }),
        proposal({ index: 2, goal: "bank-sandbox" }),
      ],
      { [lineID("t1", 2)]: { ticked: true, notRun: true, refusedUnsent: "", unrecorded: null } },
    );
    expect(markup).toContain("1 applied · 1 refused · 1 not run");
    expect(markup).toContain("Continue with the rest");
  });

  /** An older card with a waiting line folds to one line that reopens it. */
  it("folds to one line where the card is folded", () => {
    const markup = rendered([proposal()], {}, {}, ["t1"]);
    expect(markup).toContain("The Partner proposed earlier · 1 waiting");
    expect(markup).not.toContain("Reason");
  });
});
