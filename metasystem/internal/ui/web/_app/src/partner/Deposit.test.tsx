import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Deposit } from "./api";
import { copyable, DepositCard } from "./Deposit";
import { appended, cardsIn, ELSEWHERE, marked, RECORD_IT, recordedIn, type Entry } from "./sitting";
import { PartnerAs } from "./store";

/**
 * The deposit card, in the three states Sol's read added to it.
 *
 * It is read from the markup because that is where the claim is: the press is
 * either offered on this card or it is not, and the fields are either the
 * human's to change or they are not. What the card is made of is asserted here;
 * which state it is in, given a record and a sitting, is asserted in
 * sitting.test.ts over the rules themselves.
 */

const SUBJECT = "plans/designs/sessions.md";

function offered(over: Partial<Deposit> = {}): Deposit {
  return {
    kind: "fact",
    text: "the limit is twelve hours",
    anchor: "plans/intent/sessions.md",
    subject: { kind: "record", id: SUBJECT, title: "Session limits" },
    offered: true,
    ...over,
  };
}

const RECORDED: Entry = {
  when: "2026-09-26",
  who: "Wido",
  text: "the limit is twelve hours",
  clause: "plans/intent/sessions.md",
  section: "Facts",
  mark: "deposit:t1#0",
};

/** The card, rendered over one sitting and one reading of its record. */
function card(sitting: string, source = "", marks = {}): string {
  const cards = cardsIn([{ turn: "t1", deposits: [offered()] }], marks, sitting, recordedIn(source));
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ deposits: cards }}>
          <DepositCard id="deposit:t1#0" />
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("a card offered to another record", () => {
  it("offers Copy instead of the press, and says what happened to it", () => {
    const markup = card("plans/designs/other.md");

    expect(markup).toContain("ms-deposit--elsewhere");
    expect(markup).toContain(ELSEWHERE);
    expect(markup).toContain(SUBJECT);
    expect(markup).toContain(">Copy<");
    // The one thing that must not be there: the press that wrote the words into
    // whichever record the sitting happened to be about.
    expect(markup).not.toContain(RECORD_IT);
    // And no fields, because there is nothing here to edit.
    expect(markup).not.toContain("ms-deposit-field");
  });

  it("copies the words with the clause they were offered with", () => {
    expect(copyable("the limit is twelve hours", "sessions.md:14", "Anchor")).toBe(
      "the limit is twelve hours\nAnchor: sessions.md:14",
    );
    expect(copyable("the limit is twelve hours", "  ", "Anchor")).toBe("the limit is twelve hours");
  });
});

describe("a card the record already carries", () => {
  it("shows the record's entry and offers no second press", () => {
    const markup = card(SUBJECT, appended("# Session limits\n", RECORDED, "fact"));

    expect(markup).toContain("Recorded in Facts");
    expect(markup).not.toContain(RECORD_IT);
    expect(markup).not.toContain("ms-deposit-field");
    expect(markup).toContain("the limit is twelve hours");
  });
});

describe("a card whose press is in flight", () => {
  it("freezes both fields and says the press is running", () => {
    const markup = card(SUBJECT, "", {
      "deposit:t1#0": { ...marked(offered()), recording: true },
    });

    expect(markup).toContain("ms-deposit-field");
    // Both fields are read-only: the entry the press composed is the entry the
    // record takes, so what is on the card is what was submitted.
    expect(markup.split('readOnly=""').length - 1).toBe(2);
    expect(markup).toContain("Recording…");
    expect(markup).not.toContain(`>${RECORD_IT}<`);
  });

  it("leaves both fields the human's while nothing is in flight", () => {
    const markup = card(SUBJECT);

    expect(markup).toContain(`>${RECORD_IT}<`);
    expect(markup).not.toContain("readOnly");
  });
});
