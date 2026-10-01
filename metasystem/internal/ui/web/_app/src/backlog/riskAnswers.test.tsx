import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Backlog } from "./api";
import { OpenSheet } from "./OpenSheet";
import { derivedTier, emptyRisk, riskForm, RISK_ANSWERS, RISK_MEANINGS_FIELD, riskMeanings, tierLine, type Risk } from "./opening";
import type { Suggestion } from "../partner/api";
import { FieldProposals } from "../partner/FieldProposals";
import { answersIn, NOT_THE_RISK_FORM, withAnswers } from "../partner/proposing";
import { PartnerAs } from "../partner/store";
import { SuggestionCard } from "../partner/Suggestion";
import { holding, idOf, offeredIn, refusalOf, undoable, used, type Registered } from "../partner/suggesting";

/**
 * The Partner answers the four risk questions (g1-s78).
 *
 * The sheet hands the four answers over as one more field, "Risk answers", in
 * the command's own form, and a suggestion for that field — when used — sets the
 * four pills, from which the tier is derived exactly as before. A suggestion that
 * is not that form, or carries an answer outside 1–3, is shown as not usable and
 * is never half-applied.
 */

const OPENING = "opening-1";
const BASIS = "severity 1: reversible on one machine";

describe("the four answers in the command's own form", () => {
  it("are written the way the command and the open proposal write them", () => {
    const risk: Risk = { severity: "2", novelty: "3", exposure: "1", accumulation: "2", basis: "" };
    expect(riskForm(risk)).toBe("severity=2,novelty=3,exposure=1,accumulation=2");
  });

  it("are read back exactly, the whole value around trimmed", () => {
    expect(answersIn("  severity=2,novelty=3,exposure=1,accumulation=2\n")).toEqual({
      severity: "2",
      novelty: "3",
      exposure: "1",
      accumulation: "2",
    });
  });

  it.each([
    ["", "empty"],
    ["severity=2,novelty=3,exposure=1", "one answer missing"],
    ["severity=2,novelty=3,exposure=1,accumulation=2,tier=3", "one answer too many"],
    ["novelty=3,severity=2,exposure=1,accumulation=2", "out of the command's order"],
    ["severity=4,novelty=3,exposure=1,accumulation=2", "an answer above 3"],
    ["severity=0,novelty=3,exposure=1,accumulation=2", "an answer below 1"],
    ["severity=2, novelty=3, exposure=1, accumulation=2", "spaces the command refuses"],
    ["severity=two,novelty=3,exposure=1,accumulation=2", "a word for a number"],
    ["severity=2;novelty=3;exposure=1;accumulation=2", "another separator"],
    ["Tier 3: severity=2,novelty=3,exposure=1,accumulation=2", "a sentence around it"],
  ])("refuse %j (%s)", (text) => {
    expect(answersIn(text)).toBeNull();
  });
});

describe("using a suggestion for the Risk answers", () => {
  it("sets the four answers and keeps the basis, and the tier is derived from them", () => {
    const before: Risk = { ...emptyRisk, basis: BASIS };
    const after = withAnswers(before, "severity=2,novelty=3,exposure=1,accumulation=2");
    expect(after).toEqual({ severity: "2", novelty: "3", exposure: "1", accumulation: "2", basis: BASIS });
    expect(derivedTier(after ?? before)).toBe(3);
    expect(tierLine(after ?? before)).toContain("3");
  });

  it("changes nothing when the suggestion is not the form", () => {
    const before: Risk = { severity: "2", novelty: "1", exposure: "2", accumulation: "1", basis: BASIS };
    expect(withAnswers(before, "severity=3,novelty=9,exposure=1,accumulation=2")).toBeNull();
    expect(withAnswers(before, "severity=3,novelty=2")).toBeNull();
  });

  it("leaves every answer the person's to change afterwards", () => {
    const used = withAnswers(emptyRisk, "severity=3,novelty=3,exposure=3,accumulation=3");
    expect(used).not.toBeNull();
    const changed: Risk = { ...(used ?? emptyRisk), novelty: "1", severity: "1" };
    expect(derivedTier(changed)).toBe(1);
    expect(riskForm(changed)).toBe("severity=1,novelty=1,exposure=3,accumulation=3");
  });
});

describe("what the sheet hands over", () => {
  it("names the meanings the sheet shows, every stop of every score", () => {
    const said = riskMeanings();
    expect(said).toContain("severity 1: visible and reversible on one machine");
    expect(said).toContain("novelty 3: a new law, verb, schema, seam or role");
    expect(said).toContain("accumulation 2: several landings since");
    expect(said.split("\n")).toHaveLength(4);
    expect(RISK_MEANINGS_FIELD).not.toBe(RISK_ANSWERS);
  });

  it("puts an Ask the Partner link on the risk section itself", () => {
    const markup = renderToStaticMarkup(
      <TooltipPrimitive.Provider>
        <OpenSheet backlog={backlogOf()} onClose={() => undefined} onDone={() => undefined} />
      </TooltipPrimitive.Provider>,
    );
    // Id, Intent, Next step, the risk section, Why these answers, Labels.
    expect(markup.split('class="ms-field-ask"').length - 1).toBe(6);
    expect(markup).toContain("Put a request for Risk answers and Why these answers");
  });
});

describe("a suggestion that is not the form", () => {
  function registered(): Registered {
    return {
      sheet: "New goal",
      read: () => ({ opening: OPENING, sheet: "New goal", fields: [], writable: [RISK_ANSWERS], writing: "" }),
      raw: () => riskForm(emptyRisk),
      set: () => "",
      refuses: (field, text) => (field === RISK_ANSWERS && answersIn(text) === null ? NOT_THE_RISK_FORM : ""),
    };
  }

  function offer(text: string): Suggestion {
    return { opening: OPENING, editor: "New goal", field: RISK_ANSWERS, text, offered: true };
  }

  it("is refused by the sheet that owns the field, and a good one is not", () => {
    const [bad, good] = offeredIn(
      [{ turn: "t1", suggestions: [offer("severity=5,novelty=1,exposure=1,accumulation=1"), offer("severity=2,novelty=1,exposure=1,accumulation=1")] }],
      {},
      [OPENING],
    );
    expect(refusalOf(registered(), bad)).toBe(NOT_THE_RISK_FORM);
    expect(refusalOf(registered(), good)).toBe("");
    expect(refusalOf(null, bad)).toBe("");
  });

  it("is shown under the field as not usable, with no Use this", () => {
    const cards = offeredIn([{ turn: "t1", suggestions: [offer("tier 3")] }], {}, [OPENING]);
    const markup = renderToStaticMarkup(
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ offered: cards, refusal: () => NOT_THE_RISK_FORM }}>
          <FieldProposals opening={OPENING} field={RISK_ANSWERS} value={riskForm(emptyRisk)} />
        </PartnerAs>
      </TooltipPrimitive.Provider>,
    );
    expect(markup).toContain("tier 3");
    expect(markup).toContain(NOT_THE_RISK_FORM);
    expect(markup).not.toContain(">Use this<");
    expect(markup).toContain(">Dismiss<");
  });

  it("is shown in the drawer as not usable too", () => {
    const cards = offeredIn([{ turn: "t1", suggestions: [offer("tier 3")] }], {}, [OPENING]);
    const markup = renderToStaticMarkup(
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ offered: cards, refusal: () => NOT_THE_RISK_FORM }}>
          <SuggestionCard id={idOf("t1", 0)} />
        </PartnerAs>
      </TooltipPrimitive.Provider>,
    );
    expect(markup).toContain(NOT_THE_RISK_FORM);
    expect(markup).not.toContain(">Use this<");
  });
});

/**
 * A valid suggestion the transport left padded (Sol S78-01): it is usable, and
 * after Use the field holds it — so Undo stays — and Undo puts the person's
 * previous answers back. The card's text is the form without the padding,
 * because the form is what the field holds once it is used.
 */
describe("a valid suggestion with whitespace around it", () => {
  const padded = "severity=2,novelty=3,exposure=1,accumulation=2 \n";

  it("still holds after Use, and Undo restores the previous answers", () => {
    const before: Risk = { severity: "1", novelty: "2", exposure: "3", accumulation: "1", basis: BASIS };
    const offer: Suggestion = { opening: OPENING, editor: "New goal", field: RISK_ANSWERS, text: padded, offered: true };
    const id = idOf("t1", 0);
    const [card] = offeredIn([{ turn: "t1", suggestions: [offer] }], {}, [OPENING]);
    expect(answersIn(card.text)).not.toBeNull();
    // Use: the sheet sets the four answers and answers what it held.
    const applied = withAnswers(before, card.text);
    expect(applied).not.toBeNull();
    const now = applied ?? before;
    let marks = used({}, id, riskForm(before));
    // The field reports what it now holds, as FieldProposals does on render.
    let [after] = offeredIn([{ turn: "t1", suggestions: [offer] }], marks, [OPENING]);
    marks = holding(marks, [after], OPENING, RISK_ANSWERS, riskForm(now));
    [after] = offeredIn([{ turn: "t1", suggestions: [offer] }], marks, [OPENING]);
    expect(after.mark.holds).toBe(true);
    expect(undoable(after, [OPENING])).toBe(true);
    // Undo compares the field's raw value with the card's words, then puts back
    // what the field held.
    expect(riskForm(now)).toBe(after.text);
    expect(withAnswers(now, after.mark.previous ?? "")).toEqual(before);
  });
});

function backlogOf(): Backlog {
  return {
    schemaVersion: 1,
    observedAt: "2026-10-01T08:00:00Z",
    ledger: {
      state: "read",
      tip: "abc1234",
      committedAt: "2026-10-01T07:00:00Z",
      freshness: { state: "current", since: "2026-10-01T07:59:00Z", detail: "" },
      stale: false,
      staleAfterSeconds: 900,
      syncMode: "",
      stateRoot: "",
      message: "",
      problems: [],
      fetch: {
        outcome: "current",
        startedAt: "",
        finishedAt: "",
        tip: "abc1234",
        detail: "",
        message: "",
        failures: 0,
        cadence: "",
        nextAt: "",
        succeededAt: "",
        succeededTip: "abc1234",
      },
    },
    admission: { answered: true, message: "" },
    workingTree: { liveFiles: 0, archivedFiles: 0 },
    authority: { proven: true, human: "Wido", reason: "" },
    budgetDefaults: {},
    counts: {},
    draft: { statement: "" },
    rows: [],
    closed: [],
  };
}
