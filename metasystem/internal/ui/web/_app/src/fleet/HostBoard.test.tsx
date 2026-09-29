import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { BoardPayload } from "./api";
import { BoardBlock } from "./HostBoard";

/**
 * The host board's block on the Fleet page (batch-lane design D14-r2, U10d):
 * one line per seat of this host, in the server's own words, and whether the
 * bridge is live. The block judges nothing; the server classified the cards.
 * It is rendered through its pure half, because the component's first act is
 * to read the resource and this file reaches no network at all.
 */

function payload(over: Partial<BoardPayload> = {}): BoardPayload {
  return {
    readable: true,
    bridge: "absent",
    seats: [],
    lines: [
      { machine: "m1b", text: "goal-x, review round 2 of 3 since 10:12" },
      { machine: "m1e", text: "goal-z unknown: writer dead (pid 77) since 09:40" },
    ],
    ...over,
  };
}

describe("the host board block", () => {
  it("draws one line per seat with the bridge's state", () => {
    const markup = renderToStaticMarkup(<BoardBlock board={payload()} />);
    expect(markup).toContain("This host");
    expect(markup).toContain("bridge absent");
    expect(markup).toContain("m1b");
    expect(markup).toContain("goal-x, review round 2 of 3 since 10:12");
    expect(markup).toContain("goal-z unknown: writer dead (pid 77) since 09:40");
    expect(markup.match(/ms-fleet-board-row/g)?.length).toBe(2);
  });

  it("says an unreadable board is unreadable, never empty", () => {
    const markup = renderToStaticMarkup(
      <BoardBlock board={payload({ readable: false, reason: "registry: permission denied", lines: [] })} />,
    );
    expect(markup).toContain("The board could not be read: registry: permission denied");
  });

  it("says a host with no armed seat has none", () => {
    const markup = renderToStaticMarkup(<BoardBlock board={payload({ lines: [], bridge: "live" })} />);
    expect(markup).toContain("No seat is armed on this host.");
    expect(markup).toContain("bridge live");
  });
});
