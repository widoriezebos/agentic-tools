import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Sitting } from "./api";
import { DepositCard } from "./Deposit";
import {
  cardsIn,
  DECIDE,
  END,
  END_SAID,
  END_WITHOUT,
  ENDED_WITHOUT,
  LEAVE_OPEN,
  RECORD_IT,
  RECORD_OUTCOME,
  recordedIn,
} from "./sitting";
import { SittingControl } from "./StartSitting";
import { PartnerAs } from "./store";

/**
 * The two cards step 2 adds and the two ways a sitting ends, read from the
 * markup — because that is where the claim is: a case card offers two presses
 * and NOT Record it, an outcome card offers no clause field, and the way out
 * that writes nothing says so where it is pressed.
 */

const SUBJECT = "plans/designs/sessions.md";

const standing: Sitting = {
  subject: { kind: "record", id: SUBJECT, title: "Session limits" },
  purpose: "shape a design",
  startedAt: "2026-09-26T09:00:00Z",
};

function rendered(node: React.ReactNode): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>{node}</TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

/** One card over one sitting and one reading of its record. */
function card(kind: string, over: Record<string, unknown> = {}, source = ""): string {
  const deposit = {
    kind,
    text: kind === "outcome" ? "The limit counts from last activity." : "a laptop sleeps with the page open.",
    subject: { kind: "record", id: SUBJECT, title: "Session limits" },
    offered: true,
    ...over,
  };
  const cards = cardsIn([{ turn: "t1", deposits: [deposit] }], {}, SUBJECT, recordedIn(source));
  return rendered(
    <PartnerAs held={{ deposits: cards }}>
      <DepositCard id="deposit:t1#0" />
    </PartnerAs>,
  );
}

describe("a case card", () => {
  it("offers Decide and Leave open, and not Record it", () => {
    const markup = card("case", {
      clause: "sleeping does not keep a session alive",
      consequence: "a woken laptop signs out",
    });
    expect(markup).toContain("ms-deposit--case");
    expect(markup).toContain("A case at the edge");
    expect(markup).toContain(`>${DECIDE}<`);
    expect(markup).toContain(`>${LEAVE_OPEN}<`);
    // Not Record it: a case is not an entry, and there is nothing here to record
    // until the human has said which of the two it is.
    expect(markup).not.toContain(`>${RECORD_IT}<`);
    // And no fields: the case is the Partner's words, and the Decide sheet is
    // where a human writes.
    expect(markup).not.toContain("ms-deposit-field");
    expect(markup).toContain("a laptop sleeps with the page open.");
    expect(markup).toContain("a woken laptop signs out");
  });

  it("says where the record put it once one of its presses has landed", () => {
    const source =
      "# Session limits\n\n## Decisions\n\n" +
      "- 2026-09-26 · Wido · sleeping does not keep a session alive [d:deposit:t1#0]\n" +
      "  - Reason: a sleeping page is not a session in use\n";
    const markup = card("case", { clause: "sleeping does not keep a session alive" }, source);
    expect(markup).toContain("Recorded in Decisions");
    expect(markup).not.toContain(`>${DECIDE}<`);
    expect(markup).not.toContain(`>${LEAVE_OPEN}<`);
    expect(markup).toContain("a sleeping page is not a session in use");
  });
});

describe("the outcome card", () => {
  it("offers the press that ends the sitting, and no clause to fill in", () => {
    const markup = card("outcome");
    expect(markup).toContain("What this sitting came to");
    expect(markup).toContain(`>${RECORD_OUTCOME}<`);
    expect(markup).toContain("ms-deposit-field");
    // No clause field and no clause label: an outcome is a whole section.
    expect(markup).not.toContain("ms-deposit-clause-field");
    expect(markup).toContain('rows="12"');
  });

  it("shows what the record holds once the Outcome is written", () => {
    const source =
      "# Session limits\n\n## Outcome\n\nThe limit counts from last activity.\n\n" +
      "- Recorded from the sitting · 2026-09-26 · Wido [d:deposit:t1#0]\n";
    const markup = card("outcome", {}, source);
    expect(markup).toContain("Recorded in Outcome");
    expect(markup).not.toContain(`>${RECORD_OUTCOME}<`);
    expect(markup).not.toContain("ms-deposit-field");
  });
});

describe("ending a sitting", () => {
  it("is one press in the bar while a sitting stands", () => {
    const markup = rendered(
      <PartnerAs held={{ sitting: standing }}>
        <SittingControl onOpenTable={() => undefined} />
      </PartnerAs>,
    );
    expect(markup).toContain(`>${END}<`);
    expect(markup).toContain("Sitting: Session limits");
  });

  // The sheet itself renders through a portal, which a static render does not
  // reach, so what is asserted here is what it says; that both presses are on it
  // is what the screenshots at 1280 and 400 show.
  it("says what the press that drafts will do, and offers the way out that writes nothing", () => {
    expect(END_SAID).toContain("becomes the record's Outcome section");
    expect(END_SAID).toContain("press Record it");
    expect(END_WITHOUT).toBe("End without recording");
  });

  it("says what ending without recording left behind, where the press was made", () => {
    const markup = rendered(
      <PartnerAs held={{ sitting: null, sittingEnded: ENDED_WITHOUT }}>
        <SittingControl onOpenTable={() => undefined} />
      </PartnerAs>,
    );
    expect(markup).toContain("Start a sitting");
    expect(markup).toContain("no outcome was written into the record");
  });

  it("says nothing about an outcome where no sitting has been ended", () => {
    const markup = rendered(
      <PartnerAs held={{ sitting: null }}>
        <SittingControl onOpenTable={() => undefined} />
      </PartnerAs>,
    );
    expect(markup).not.toContain("no outcome was written");
  });
});
