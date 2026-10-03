import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { CardVerdict } from "./Verdict";
import type { Row } from "../backlog/api";

/**
 * The card's verdict line (g1-s69 D2, §8), read from the markup. Send it back
 * is the guided review's own now (guided.screens.test.tsx).
 */

function rendered(node: React.ReactNode): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>{node}</TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function row(verdict: Row["verdict"]): Row {
  return {
    ref: { kind: "goal", id: "g", revision: 4 }, where: "live", lane: "in-progress", phase: "sent back", state: "claimed",
    intent: "", nextStep: "", concluded: "", origin: "main", priority: 1, sequence: 1, tier: 1, labels: [], arc: "", pinned: "",
    blockedBy: [], openBlockers: [], holds: [], sliced: false, decomposed: false, openedAt: "", doneAt: "", lastChangeAt: "",
    lastVerb: "review", gaps: [], verdict,
  } as Row;
}

describe("the card's verdict", () => {
  const sent = { verdict: "send-back", by: "Wido", at: "", tip: "9c1f0a2b", record: "metasystem/plans/reviews/review-of-g.md", answered: true };

  it("says the holder needs to know which work, with a press per name", () => {
    const markup = rendered(<CardVerdict row={row({ ...sent, candidates: ["discovery", "writer"] })} />);
    expect(markup).toContain("the holder needs to know which work: discovery or writer");
    expect(markup).toMatch(/<button[^>]*>discovery</u);
    expect(markup).toMatch(/<button[^>]*>writer</u);
  });

  it("says who takes a send-back and with what, since nothing takes it unprompted yet", () => {
    const markup = rendered(<CardVerdict row={row({ ...sent, answered: false })} />);
    expect(markup).toContain("sent back by Wido · the seat that holds g revises on its next turn");
    expect(markup).not.toContain("awaiting the holder");
  });

  it("says a clear to land by whom, and nothing where no verdict stands", () => {
    expect(rendered(<CardVerdict row={row({ ...sent, verdict: "clear-to-land", answered: false })} />)).toContain("reviewed by Wido · clear to land");
    expect(rendered(<CardVerdict row={row(undefined)} />)).toBe("");
  });
});
