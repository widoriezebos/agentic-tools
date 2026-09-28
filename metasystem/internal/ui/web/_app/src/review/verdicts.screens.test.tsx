import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { BRIEF_SAID, NO_FIX } from "./room";
import { SendBackBrief } from "./ReviewRoom";
import { CardVerdict } from "./Verdict";
import type { Row } from "../backlog/api";
import { FIX, LEFT_OPEN, type Entry } from "../partner/sitting";

/**
 * The Send back step and the card's verdict line (g1-s69 D2, §8), read from
 * the markup.
 */

function rendered(node: React.ReactNode): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>{node}</TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function finding(text: string, clause: string, answer: string, mark: string): Entry {
  return { when: "2026-09-29", who: "Wido", text, clause, section: "Findings", mark, answer };
}

const noop = () => undefined;

describe("Send back", () => {
  const entries = [
    finding("the owner reads the wrong tree", "internal/owner.go:60", FIX, "deposit:t1#0"),
    finding("the log is noisy", "internal/log.go:3", LEFT_OPEN, "deposit:t1#1"),
  ];

  it("lists the findings answered fix, and only those, and shows the brief to read and edit", () => {
    const markup = rendered(<SendBackBrief entries={entries} brief={"# Correction brief\n"} busy={false} onEdit={noop} onSend={noop} onBack={noop} />);
    expect(markup).toContain(BRIEF_SAID);
    expect(markup).toContain("the owner reads the wrong tree");
    expect(markup).not.toContain("the log is noisy");
    expect(markup).toContain("<textarea");
    expect(markup).toContain("# Correction brief");
    expect(markup).toMatch(/<button[^>]*>Send back</u);
  });

  it("is refused in words where no finding is answered fix", () => {
    const markup = rendered(<SendBackBrief entries={[entries[1]]} brief="" busy={false} onEdit={noop} onSend={noop} onBack={noop} />);
    expect(markup).toContain(NO_FIX);
    expect(markup).not.toMatch(/>Send back</u);
  });

  it("waits while the room's closing turn is unsettled", () => {
    const markup = rendered(<SendBackBrief entries={entries} brief={"# Correction brief\n"} busy onEdit={noop} onSend={noop} onBack={noop} />);
    expect(markup).toMatch(/<button[^>]*disabled=""[^>]*>Send back</u);
  });
});

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

  it("says a clear to land by whom, and nothing where no verdict stands", () => {
    expect(rendered(<CardVerdict row={row({ ...sent, verdict: "clear-to-land", answered: false })} />)).toContain("reviewed by Wido · clear to land");
    expect(rendered(<CardVerdict row={row(undefined)} />)).toBe("");
  });
});
