import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { drawWith } from "./drawings";
import { Drawing } from "./Drawing";
import { Markdown } from "../project/Markdown";
import { askedBefore } from "../partner/Transcript";

/**
 * A drawing on screen (g1-s71 D2): a mermaid fence renders through one lazily
 * loaded chunk, in the conversation, on the desk and on the board; the source
 * stays a press away, and is shown instead, in words, when the chunk fails to
 * load or the parse fails.
 */

const FENCE = "sequenceDiagram\n  Owner->>Holder: take the lock";

describe("drawing one fence", () => {
  it("loads the chunk only when a drawing is drawn, and draws with it", async () => {
    let loads = 0;
    const drawn: string[] = [];
    const load = async () => {
      loads += 1;
      return { renderDrawing: async (source: string) => { drawn.push(source); } };
    };
    expect(loads).toBe(0);
    expect(await drawWith(load, FENCE, {} as Element, "d-1")).toEqual({ state: "drawn" });
    expect(loads).toBe(1);
    expect(drawn).toEqual([FENCE]);
  });

  it("says in words that the chunk did not load, and shows the source", async () => {
    const failed = await drawWith(async () => { throw new Error("Failed to fetch dynamically imported module"); }, FENCE, {} as Element, "d-2");
    expect(failed).toEqual({
      state: "refused",
      said: "The drawing could not be loaded (Failed to fetch dynamically imported module), so its source is shown instead.",
    });
  });

  it("says in words that the parse failed, and shows the source", async () => {
    const failed = await drawWith(
      async () => ({ renderDrawing: async () => { throw new Error("Parse error on line 2"); } }),
      FENCE, {} as Element, "d-3",
    );
    expect(failed).toEqual({ state: "refused", said: "The drawing could not be drawn: Parse error on line 2. Its source is shown instead." });
  });
});

describe("a drawing's first frame", () => {
  it("is the drawing arriving, with its source a press away", () => {
    const markup = renderToStaticMarkup(<Drawing source={FENCE} />);
    expect(markup).toContain("Drawing…");
    expect(markup).toContain("Show the source");
    expect(markup).not.toContain("take the lock");
  });

  it("is what a mermaid fence in any rendered Markdown becomes, and no other fence", () => {
    const markup = renderToStaticMarkup(
      <MemoryRouter>
        <Markdown
          from=""
          blocks={[
            { type: "code", lang: "mermaid", text: FENCE },
            { type: "code", lang: "go", text: "package owner" },
          ]}
        />
      </MemoryRouter>,
    );
    expect(markup).toContain("ms-drawing");
    expect(markup).toContain("package owner");
    expect(markup).not.toContain("take the lock");
  });
});

describe("the question a drawing is kept with (D3)", () => {
  const at = "2026-09-29T09:00:00Z";
  const messages = [
    { id: "1", role: "human", text: "How does the handoff work?", at, turn: "t1" },
    { id: "2", role: "partner", text: "```mermaid\nflowchart LR\n  a --> b\n```", at, turn: "t1" },
    { id: "3", role: "partner", text: "More.", at, turn: "t2" },
  ] as unknown as Parameters<typeof askedBefore>[0];

  it("is the nearest human question before the answer", () => {
    expect(askedBefore(messages, 1)).toBe("How does the handoff work?");
    expect(askedBefore(messages, 2)).toBe("How does the handoff work?");
    expect(askedBefore(messages, 0)).toBe("");
  });
});
