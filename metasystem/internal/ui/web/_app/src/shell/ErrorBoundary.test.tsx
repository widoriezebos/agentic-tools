import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Caught, holdsAt } from "./ErrorBoundary";
import { TroublesAs } from "./troubles";
import { BROKEN_DRAWER, BROKEN_ROOM } from "./troubling";

/**
 * A pane the boundary caught is a trouble line too (g1-s68 D1, D2): it says
 * the error by name, and it offers the Ask only where the answer can be read.
 */

const THROWN = new TypeError("cannot read properties of undefined (reading 'lane')");

function caught(conversation?: "room" | "drawer"): string {
  return renderToStaticMarkup(
    <TroublesAs held={{ ask: () => {} }}>
      <Caught error={THROWN} conversation={conversation} />
    </TroublesAs>,
  );
}

describe("the boundary's trouble", () => {
  it("carries the error's name and offers the Ask over a work-area pane", () => {
    const markup = caught();
    expect(markup).toContain("This pane could not be rendered");
    expect(markup).toContain("TypeError: cannot read properties of undefined (reading &#x27;lane&#x27;)");
    expect(markup).toContain("Ask what happened");
  });

  it("offers the reload and no Ask where the room itself broke", () => {
    const markup = caught("room");
    expect(markup).toContain(BROKEN_ROOM);
    expect(markup).toContain("Reload page");
    expect(markup).not.toContain("Ask what happened");
  });

  it("and where the drawer's conversation broke", () => {
    const markup = caught("drawer");
    expect(markup).toContain(BROKEN_DRAWER);
    expect(markup).not.toContain("Ask what happened");
  });

  // Sol SOL-S68-02: the focused view at /brain is the conversation in full and
  // shows no drawer, so its work-area boundary takes the conversation down too.
  it("offers the reload and no Ask where the focused Partner view broke", () => {
    const markup = caught(holdsAt("/brain"));
    expect(markup).toContain(BROKEN_DRAWER);
    expect(markup).toContain("Reload page");
    expect(markup).not.toContain("Ask what happened");
  });

  it("and still offers the Ask over any other work-area pane", () => {
    expect(holdsAt("/backlog")).toBeUndefined();
    expect(caught(holdsAt("/backlog"))).toContain("Ask what happened");
  });
});
