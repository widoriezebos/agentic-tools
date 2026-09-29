import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { CardGate, DecideBody } from "./Gate";
import { LAND_WITHOUT_SITTING_SAID, REASON_NEEDED } from "./gating";
import { InboxRow, type Acts } from "../decisions/InboxRow";
import type { Need } from "../decisions/api";
import type { Gate, Row } from "../backlog/api";
import { LandingGateFacts } from "../panes/Settings";
import { IdentityAs } from "../shell/identity";

/**
 * The landing gate on the screens (g1-s70 D5, §8): the card's four wordings
 * from the board's payload with the press on the one that waits for you, the
 * Decide sheet's required reason (the route and its body are the server's
 * test, internal/ui/httpd/landgate_test.go), the inbox row's two
 * answers, and the Settings page's two facts with their sources.
 */

const now = new Date("2026-09-28T14:50:00Z");
const noop = () => undefined;

function rendered(node: React.ReactNode): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <IdentityAs held={{ session: { state: "known", session: { human: "Wido", signedIn: true, until: "", source: "session" } } as never }}>
        <TooltipPrimitive.Provider>{node}</TooltipPrimitive.Provider>
      </IdentityAs>
    </MemoryRouter>,
  );
}

function row(gate: Gate): Row {
  return {
    ref: { kind: "goal", id: "backlog-ordered-by-priority", revision: 4 }, where: "live", lane: "review", phase: "", state: "claimed",
    intent: "", nextStep: "", concluded: "", origin: "main", priority: 1, sequence: 1, tier: gate.tier, labels: [], arc: "", pinned: "",
    blockedBy: [], openBlockers: [], holds: [], sliced: false, decomposed: false, openedAt: "", doneAt: "", lastChangeAt: "",
    lastVerb: "land-ready", gaps: [], gate,
  } as Row;
}

const below: Gate = {
  tier: 1, humanFromTier: 2, autoAfter: "4h", waitsForHuman: false,
  clockFrom: "2026-09-28T14:02:00Z", autoLandsAt: "2026-09-28T18:02:00Z", eligible: false, landed: false,
};
const above: Gate = { tier: 2, humanFromTier: 2, autoAfter: "4h", waitsForHuman: true, eligible: false, landed: false };

describe("the card's gate", () => {
  it("says the four wordings, and offers Land without a sitting only where the goal waits for you", () => {
    const clock = rendered(<CardGate row={row(below)} now={now} />);
    expect(clock).toContain("eligible to land in 3h 12m");
    expect(clock).not.toContain("Land without a sitting");

    expect(rendered(<CardGate row={row({ ...below, eligible: true })} now={now} />)).toContain("eligible to land, waiting for the holder");

    const held = rendered(<CardGate row={row({ ...above, heldBy: [{ by: "Wido", record: "plans/reviews/review-of-b.md", since: "" }] })} now={now} />);
    expect(held).toContain("held by your sitting");
    expect(held).not.toContain("Land without a sitting");

    const waits = rendered(<CardGate row={row(above)} now={now} />);
    expect(waits).toContain("waits for your review");
    expect(waits).toMatch(/<button[^>]*>Land without a sitting</u);
  });

  it("asks for the word again once the branch moved past it, with Land without a sitting", () => {
    const word = { kind: "clear-to-land", by: "Wido", tip: "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b" };
    const moved = rendered(
      <CardGate row={row({ ...above, reviewed: { ...word, moved: true, branchTip: "a1b2c3d4e5f60718293a4b5c6d7e8f9011223344" } })} now={now} />,
    );
    expect(moved).toContain("cleared by Wido at 9c1f0a2, but the branch moved to a1b2c3d: needs the word again");
    expect(moved).toMatch(/<button[^>]*>Land without a sitting</u);
    const standing = rendered(<CardGate row={row({ ...above, reviewed: word })} now={now} />);
    expect(standing).toContain("cleared to land by Wido at 9c1f0a2, waiting for the holder");
    expect(standing).not.toContain("Land without a sitting");
  });
});

describe("the Decide sheet", () => {
  it("asks for the reason and waits for one before its press", () => {
    const markup = rendered(<DecideBody reason="  " onReason={noop} sending={false} refusal="" onPress={noop} />);
    expect(markup).toContain(LAND_WITHOUT_SITTING_SAID);
    expect(markup).toContain(REASON_NEEDED);
    expect(markup).toContain("<textarea");
    expect(markup).toMatch(/<button[^>]*disabled=""[^>]*>Land without a sitting</u);
    const ready = rendered(<DecideBody reason="read the diff" onReason={noop} sending={false} refusal="" onPress={noop} />);
    expect(ready).not.toContain(REASON_NEEDED);
    expect(ready).not.toMatch(/<button[^>]*disabled=""[^>]*>Land without a sitting</u);
  });
});

describe("the inbox's landing row", () => {
  const acts: Acts = {
    signedIn: true, onSignIn: noop, onApprove: noop, onPark: noop, onEdit: noop, onReturn: noop, onLanded: noop, onWrite: noop,
    writing: "", refusedAt: "", refusal: "",
    proposals: { lineOf: () => null, onApply: noop, onDismiss: noop, onAsk: noop, running: false } as never,
  };
  const need: Need = {
    kind: "landing", id: "backlog-ordered-by-priority", title: "Order the backlog by priority",
    asked: "backlog-ordered-by-priority waits for your review", by: "m2a+coordinator", since: "2026-09-28T14:02:00Z",
    deadline: "", silence: "it stays in Review; nothing lands until a sitting ends clear to land or you land it without one",
    recommend: "", where: { kind: "goal", id: "backlog-ordered-by-priority" }, act: "land-without-sitting", command: "",
    row: row(above), new: false, words: "", context: "", owner: "", class: "", due: "", path: "", goals: [], proposal: null,
  };

  it("carries its two answers", () => {
    const markup = rendered(<ul><InboxRow need={need} now={now} open onOpen={noop} acts={acts} /></ul>);
    expect(markup).toContain("backlog-ordered-by-priority waits for your review");
    expect(markup).toMatch(/<button[^>]*>Review it</u);
    expect(markup).toMatch(/<button[^>]*>Land without a sitting</u);
  });
});

describe("the Settings page's landing gate", () => {
  it("says the two settings with where each came from", () => {
    const markup = renderToStaticMarkup(
      <LandingGateFacts gate={{ facts: [
        { key: "landing.review.human-from-tier", value: "1", source: "conf-local" },
        { key: "landing.review.auto-after", value: "4h", source: "default" },
      ] }} />,
    );
    expect(markup).toContain("Waits for a person from tier");
    expect(markup).toContain("from metasystem.conf.local");
    expect(markup).toContain("Lands by itself after");
    expect(markup).toContain("4h");
    expect(markup).toContain("the default");
    expect(markup).toContain("landing.review.auto-after");
  });
});
