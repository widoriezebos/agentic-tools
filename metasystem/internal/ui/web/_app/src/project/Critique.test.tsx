import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { DocumentPayload } from "./api";
import { CritiqueBlock, FindingCard, RoundCards, SectionCard, SendFields } from "./Critique";
import { cardsOf, goalSheetAsked, OPEN_FROM, type DesignLoop, type DesignRound } from "./critiquing";
import { FileActions } from "./DocumentPane";
import { foldConflicted, foldOpened, foldUse } from "./folding";
import { nextStepFor, outcomeParagraph } from "./sections";
import { PartnerAs } from "../partner/store";
import { EndShapingWays, OutcomeRecordedWays } from "../review/ReviewRoom";
import { recordedOutcome } from "../review/room";

/**
 * The loop from the room on the design's page (g1-s66 §3, §8), read from the
 * markup: the Send to critique sheet with the goal and the budget, a round's
 * cards with three presses, a decided card with the row a reload found, Answer
 * the round only when every card has its row, the engine's words as said, the
 * section card with old and new, and the way to a goal from the design.
 */

function rendered(node: React.ReactNode): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{}}>{node}</PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

const noop = () => undefined;

const round: DesignRound = {
  round: 1,
  findings: [
    { id: "S66-01", severity: "high", material: true, claim: "the act as written cannot start a review", evidence: "metasystem/cmd/metasystem/intent_delivery.go:771, §4 D1" },
    { id: "S66-09", severity: "low", material: false, claim: "a word is loose in §6", evidence: "§6" },
  ],
  decisions: [],
  answerable: false,
};

function loop(over: Partial<DesignLoop> = {}): DesignLoop {
  return { design: "plans/designs/g1-s66.md", toolCalls: 30, chain: "rev1", goal: "g", state: "deciding", status: "completed",
    round: 1, limit: 2, rounds: [round], ...over };
}

function block(of: DesignLoop, words: string[] = []): string {
  return rendered(
    <CritiqueBlock
      loop={of}
      words={words}
      busy={false}
      onAnswer={noop}
      renderRound={(one, live) => (
        <RoundCards key={one.round} round={one} live={live} headings={[]} pressing={null} busy={false} asked={[]}
          onOpen={noop} onChange={noop} onCancel={noop} onWrite={noop} onFold={noop} />
      )}
    />,
  );
}

describe("Send to critique", () => {
  it("is a sheet with the funding goal preselected and the budget prefilled, both editable", () => {
    const markup = rendered(<SendFields goals={["g"]} states={{ g: "approved" }} goal="g" budget="30" refusal="" busy={false}
      onGoal={noop} onBudget={noop} onSend={noop} />);
    expect(markup).toMatch(/<select id="ms-critique-goal"/u);
    expect(markup).toMatch(/<option value="g" selected="">g · approved<\/option>/u);
    expect(markup).toMatch(/<input id="ms-critique-budget" type="number" min="1" value="30"/u);
    expect(markup).toMatch(/<button[^>]*>Send to critique</u);
  });

  it("asks for the goal when several are approved, and says so when none is", () => {
    const several = rendered(<SendFields goals={["a", "b"]} states={{ a: "approved", b: "claimed" }} goal="" budget="30" refusal=""
      busy={false} onGoal={noop} onBudget={noop} onSend={noop} />);
    expect(several).toContain("Choose the goal");
    const none = rendered(<SendFields goals={["a"]} states={{ a: "queued" }} goal="" budget="30" refusal="" busy={false}
      onGoal={noop} onBudget={noop} onSend={noop} />);
    expect(none).toContain("None of the goals this design names is approved; the engine will say so.");
  });

  it("shows the engine's refusal as it was said", () => {
    const markup = rendered(<SendFields goals={["g"]} states={{ g: "approved" }} goal="g" budget="" busy={false}
      refusal="a review brief states the reader's tool-call budget, and none is configured" onGoal={noop} onBudget={noop} onSend={noop} />);
    expect(markup).toContain("a review brief states the reader&#x27;s tool-call budget, and none is configured");
  });
});

describe("the round's cards", () => {
  it("stand under the round's heading with the five fields and three presses", () => {
    const markup = block(loop());
    expect(markup).toContain("Critique · round 1 of 2 · 2 findings wait for your decisions");
    expect(markup).toContain("Round 1");
    expect(markup).toContain("S66-01");
    expect(markup).toContain(">material<");
    expect(markup).toContain(">not material<");
    expect(markup).toContain("the act as written cannot start a review");
    expect(markup).toMatch(/<a[^>]*href="\/project\/doc\/metasystem\/cmd\/metasystem\/intent_delivery.go"[^>]*>metasystem\/cmd\/metasystem\/intent_delivery.go:771</u);
    expect(markup.match(/>Fold</gu)?.length).toBe(2);
    expect(markup.match(/>Refute</gu)?.length).toBe(2);
    expect(markup.match(/>Defer</gu)?.length).toBe(2);
    expect(markup).not.toContain("Answer the round");
  });

  it("shows the row a reload found in the file, and no presses on a decided card", () => {
    const decided = { ...round, decisions: [{ finding: "S66-01", disposition: "refuted", reasoning: "line 771 reads the flag", amendment: "" }] };
    const markup = block(loop({ rounds: [decided] }));
    expect(markup).toContain("Refuted · line 771 reads the flag");
    expect(markup.match(/>Refute</gu)?.length).toBe(1);
  });

  it("offers Answer the round only when every card has its row", () => {
    const all = { ...round, answerable: true, decisions: [
      { finding: "S66-01", disposition: "refuted", reasoning: "r", amendment: "" },
      { finding: "S66-09", disposition: "noted", reasoning: "", amendment: "" },
    ] };
    const markup = block(loop({ state: "answered", rounds: [all] }));
    expect(markup).toMatch(/<button[^>]*>Answer the round</u);
    expect(markup).toContain("every finding is decided");
  });

  it("shows the engine's words after an answer, as said", () => {
    const markup = block(loop({ state: "closed" }), [
      "design 01M3KCV1EDQPB9X3PFZBE0R77R's critique is complete: every finding is decided and chain rev1 is closed",
    ]);
    expect(markup).toContain("Critique · closed at round 1");
    expect(markup).toContain("critique is complete: every finding is decided and chain rev1 is closed");
    expect(markup).not.toMatch(/>Fold</u);
  });

  it("refuses a refutation without its reason and a material deferral without its evidence, in words", () => {
    const [material] = cardsOf(round);
    const refute = rendered(<FindingCard card={material} live headings={[]} busy={false} folding={false}
      pressing={{ key: "1:S66-01", press: "refute", draft: { reasoning: "", amendment: "" }, heading: "",
        refusal: "A refutation carries your reason: the check you made and what it showed." }}
      onOpen={noop} onChange={noop} onCancel={noop} onWrite={noop} onFold={noop} />);
    expect(refute).toContain("Your reason: the check you made and what it showed");
    expect(refute).toContain("A refutation carries your reason");
    const defer = rendered(<FindingCard card={material} live headings={[]} busy={false} folding={false}
      pressing={{ key: "1:S66-01", press: "defer", draft: { reasoning: "", amendment: "" }, heading: "", refusal: "" }}
      onOpen={noop} onChange={noop} onCancel={noop} onWrite={noop} onFold={noop} />);
    expect(defer).toContain("The evidence that it is outside the brief&#x27;s scope");
    expect(defer).toMatch(/>Write out of scope</u);
  });

  it("opens Fold on the section the finding names, with the amendment", () => {
    const [material] = cardsOf(round);
    const markup = rendered(<FindingCard card={material} live headings={["3. The room", "4. Decisions"]} busy={false} folding={false}
      pressing={{ key: "1:S66-01", press: "fold", draft: { reasoning: "", amendment: "" }, heading: "4. Decisions", refusal: "" }}
      onOpen={noop} onChange={noop} onCancel={noop} onWrite={noop} onFold={noop} />);
    expect(markup).toMatch(/<option value="4. Decisions" selected="">4. Decisions<\/option>/u);
    expect(markup).toContain("The amendment, in one line");
    expect(markup).toMatch(/>Ask the Partner for this section</u);
  });
});

describe("the section card", () => {
  const SOURCE = "# D\n\n## 4. Decisions\n\n- D1. Old.\n\n## 5. Step 1\n";
  const DRAFT = "## 4. Decisions\n\n- D1. New.";

  it("shows old and new side by side with the changed lines marked, Use and Not this", () => {
    const markup = rendered(<SectionCard fold={foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1")} settled={false}
      asked={{ design: "plans/designs/g1-s66.md", chain: "rev1", round: 1, finding: "S66-01", heading: "4. Decisions", amendment: "§4 D1" }} onUse={noop} onRow={noop} onDismiss={noop} />);
    expect(markup).toContain("As it stands");
    expect(markup).toContain("As drafted");
    expect(markup).toMatch(/data-changed="yes">▍ - D1\. Old\./u);
    expect(markup).toMatch(/data-changed="yes">▍ - D1\. New\./u);
    expect(markup).toMatch(/<button[^>]*>Use</u);
    expect(markup).toMatch(/<button[^>]*>Not this</u);
    expect(markup).toContain("drafted anew for S66-01");
  });

  it("refuses a heading it cannot tell apart, in words, keeping the draft and offering no Use", () => {
    const refused = foldUse(foldOpened("4. Decisions", DRAFT, `${SOURCE}\n## 4. Decisions\n`, "blob:1")).fold;
    const markup = rendered(<SectionCard fold={refused} settled={false} asked={undefined} onUse={noop} onRow={noop} onDismiss={noop} />);
    expect(markup).toContain("occurs 2 times in this document");
    expect(markup).toContain("- D1. New.");
    expect(markup).toMatch(/<button[^>]*disabled=""[^>]*>Use</u);
  });

  it("says the design changed and compares against it as it is now", () => {
    const writing = foldUse(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1")).fold;
    const markup = rendered(<SectionCard fold={foldConflicted(writing, SOURCE.replace("Old", "Other"), "blob:2")} settled={false}
      asked={undefined} onUse={noop} onRow={noop} onDismiss={noop} />);
    expect(markup).toContain("The design changed since it was read");
    expect(markup).toContain("- D1. Other.");
  });

  it("offers the decision again when the section was written without it", () => {
    const owing = { ...foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1"), phase: "row" as const, said: "The section is written; its decision is not: gone. Write the decision again." };
    const markup = rendered(<SectionCard fold={owing} settled={false} asked={{ design: "plans/designs/g1-s66.md", chain: "rev1", round: 1, finding: "S66-01", heading: "4. Decisions", amendment: "a" }}
      onUse={noop} onRow={noop} onDismiss={noop} />);
    expect(markup).toMatch(/<button[^>]*>Write the decision</u);
    expect(markup).toContain("its decision is not");
  });
});

describe("a goal from this design (D5)", () => {
  const page = { id: "plans/designs/g1-s66.md", path: "plans/designs/g1-s66.md", title: "g1-s66", revision: "blob:1",
    record: { id: "01M3", kind: "design", status: "draft", goals: ["g"] } } as unknown as DocumentPayload;

  it("is offered on the design page under its own name", () => {
    const markup = rendered(<FileActions path={page.path} document={page} onEdit={null} onNewGoal={noop} newGoal={OPEN_FROM} busy="" />);
    expect(markup).toMatch(/<button[^>]*>Open a goal from this design</u);
  });

  it("is not reached from a design sitting's End sheet before Record it", () => {
    const markup = rendered(<EndShapingWays busy={false} onDraft={noop} onWithout={noop} design="plans/designs/g1-s66.md" />);
    expect(markup).not.toContain("open-goal");
    expect(markup).not.toMatch(/<a[^>]*>Open a goal from this design</u);
    const intent = rendered(<EndShapingWays busy={false} onDraft={noop} onWithout={noop} />);
    expect(intent).not.toContain(OPEN_FROM);
  });

  it("is offered once the Outcome is recorded, and the sheet opens prefilled from it", () => {
    const cards = [{ kind: "outcome", standing: "waiting" as const }];
    expect(recordedOutcome(cards)).toBe(false);
    expect(recordedOutcome([{ kind: "outcome", standing: "recorded" as const }])).toBe(true);
    const markup = rendered(<OutcomeRecordedWays design="plans/designs/g1-s66.md" onBack={noop} />);
    expect(markup).toMatch(/<a[^>]*href="\/project\/doc\/plans\/designs\/g1-s66.md\?open-goal=1"[^>]*>Open a goal from this design</u);
    const before = "# g1-s66\n\n## 5. Step 1\n\nD1 to D5.\n";
    const recorded = `${before}\n## Outcome\n\nBuild the loop from the room.\n`;
    expect(goalSheetAsked("?open-goal=1", before)).toBe(false);
    expect(goalSheetAsked("", recorded)).toBe(false);
    expect(goalSheetAsked("?open-goal=1", recorded)).toBe(true);
    expect(outcomeParagraph(recorded)).toBe("Build the loop from the room.");
    expect(nextStepFor(page.id, recorded)).toBe("Continue from the record plans/designs/g1-s66.md: build step 1 as its §5 says");
  });
});
