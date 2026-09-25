import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Copy, Held, Machine, Page, ThisSeat } from "./api";
import { Blocks } from "./FleetPane";
import { NO_PRESENCE } from "./fleet";
import { minuteTime } from "../backlog/format";

/**
 * What this page renders that the server deliberately did not.
 *
 * Two facts about one moment in time used to be rendered by two clocks: the
 * server wrote "unreachable since 01:27" into the flag in UTC, and the
 * browser wrote "seen 03:27" beside it from the same instant. Now the server
 * composes words with no clock in them and carries the instant, and every
 * surface renders that instant once, here.
 */

const seat: ThisSeat = {
  machine: "m1u",
  noNickname: false,
  armed: "not armed",
  health: null,
  publication: null,
  publicationProblem: "",
  running: null,
  runningProblem: "",
};

const held: Held = {
  goal: "tests-parallel-and-deterministic",
  title: "Run the suite in parallel.",
  lane: "in-progress",
  machine: "m1c",
  standing: "unreachable",
  since: "2026-09-25T09:40:00Z",
  // The server's words. No clock: the instant is the field above.
  flag: "held by m1c, unreachable",
};

const machine: Machine = {
  machine: "m1c",
  standing: "unreachable",
  reason: "no presence for 6 h, past 30 min",
  ageSeconds: 21600,
  seen: "2026-09-25T08:37:00Z",
  since: "2026-09-25T09:40:00Z",
  running: null,
  engine: "3f9c1e2abcdef",
  generation: 4,
  holds: [held],
  this: false,
};

const read: Copy = {
  source: "the interface",
  attemptedAt: "2026-09-25T14:36:20Z",
  succeededAt: "2026-09-25T14:36:20Z",
  failedAt: "",
  problem: "",
  metadataProblem: "",
};

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: "2026-09-25T14:37:00Z",
    copy: read,
    claims: { tip: "5b9d958c1a2b3c4d", unavailable: "" },
    this: seat,
    needsYou: [held],
    machines: [machine],
    ...over,
  };
}

function rendered(payload: Page): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Blocks page={payload} />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("when a silence began", () => {
  it("is rendered by this browser, once, in the flag and beside the standing", () => {
    const markup = rendered(page());
    const when = minuteTime(held.since);

    expect(markup).toContain(`held by m1c, unreachable since ${when}`);
    expect(markup).toContain(`since ${when}`);
    // And the server's own words never carried a time of day to begin with.
    expect(held.flag).not.toMatch(/\d\d:\d\d/);
  });

  it("is left out where this seat has no frozen observation of it", () => {
    const markup = rendered(
      page({
        needsYou: [{ ...held, since: "" }],
        machines: [{ ...machine, since: "", holds: [{ ...held, since: "" }] }],
      }),
    );

    expect(markup).toContain("held by m1c, unreachable");
    expect(markup).not.toContain("unreachable since");
  });
});

describe("a presence copy that could not be read", () => {
  it("says so, rather than saying nobody has published", () => {
    const markup = rendered(
      page({
        copy: { ...read, problem: "presence ref list: exit status 128" },
        needsYou: [],
        machines: [],
      }),
    );

    expect(markup).toContain("The presence copy could not be read");
    expect(markup).toContain("presence ref list: exit status 128");
    expect(markup).not.toContain(NO_PRESENCE);
  });

  it("is told apart from an empty copy that was read", () => {
    const markup = rendered(page({ needsYou: [], machines: [] }));

    expect(markup).toContain(NO_PRESENCE);
    expect(markup).not.toContain("The presence copy could not be read");
  });
});

describe("the metadata the Partner's tool reads", () => {
  it("says when this server could not write it", () => {
    const markup = rendered(
      page({
        copy: {
          ...read,
          metadataProblem: "the Partner's presence metadata could not be written: permission denied",
        },
      }),
    );

    expect(markup).toContain("presence metadata could not be written");
  });
});
