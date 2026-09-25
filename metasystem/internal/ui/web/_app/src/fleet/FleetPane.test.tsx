import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Held, Machine, Page, ThisSeat } from "./api";
import { captureOfFleet } from "./capture";
import { copyLine, NEEDS_YOU_REMEDY, NO_PRESENCE } from "./fleet";
import { minuteTime } from "../backlog/format";
import { Blocks } from "./FleetPane";

/**
 * What the Fleet page puts on the screen, from the payload shapes the server
 * actually sends.
 *
 * The page judges nothing, so nothing here asserts a standing: the server
 * composed it. What is asserted is that every shape reaches the screen as
 * something a human can read — a fleet with a silent holder in it, one with
 * none, a checkout that is not armed, and a fleet nobody has published to.
 *
 * The blocks are rendered through the pane's own internals rather than
 * through FleetPane itself, because FleetPane's first act is to read the
 * resource and this file reaches no network at all.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");

const now = new Date("2026-09-25T14:37:00Z");

function seat(over: Partial<ThisSeat> = {}): ThisSeat {
  return {
    machine: "m1u",
    noNickname: false,
    armed: "armed",
    health: {
      state: "healthy",
      observedAt: "2026-09-25T14:25:00Z",
      problem: "",
      roles: [
        { role: "steward-runner", status: "alive", reason: "runner alive" },
        { role: "seat-presence", status: "dead", reason: "presence not published since 09:10" },
      ],
    },
    publication: {
      lastAttemptAt: "2026-09-25T14:35:00Z",
      lastSuccessAt: "2026-09-25T14:35:00Z",
      lastOutcome: "published",
      rung: 1,
      detail: "",
    },
    publicationProblem: "",
    running: { job: "j2", role: "implementer", round: 2, goal: "g1-s42", startedAt: "2026-09-25T13:00:00Z" },
    runningProblem: "",
    ...over,
  };
}

function held(over: Partial<Held> = {}): Held {
  return {
    goal: "tests-parallel-and-deterministic",
    title: "Run the suite in parallel.",
    lane: "in-progress",
    machine: "m1c",
    standing: "unreachable",
    since: "2026-09-25T09:40:00Z",
    // The server's words carry no clock: the instant is the field above, and
    // every surface renders it in the viewer's own zone.
    flag: "held by m1c, unreachable",
    ...over,
  };
}

function machine(over: Partial<Machine> = {}): Machine {
  return {
    machine: "m1c",
    standing: "unreachable",
    reason: "no presence for 6 h, past 30 min",
    ageSeconds: 21600,
    seen: "2026-09-25T08:37:00Z",
    since: "2026-09-25T09:40:00Z",
    running: null,
    engine: "3f9c1e2abcdef",
    generation: 4,
    holds: [held()],
    this: false,
    working: [],
    workingProblem: "",
    ...over,
  };
}

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: "2026-09-25T14:37:00Z",
    copy: {
      source: "the interface",
      attemptedAt: "2026-09-25T14:36:20Z",
      succeededAt: "2026-09-25T14:36:20Z",
      failedAt: "",
      problem: "",
      metadataProblem: "",
    },
    claims: { tip: "5b9d958c1a2b3c4d", unavailable: "" },
    this: seat(),
    needsYou: [held()],
    machines: [
      machine({ machine: "m1u", standing: "reachable", seen: "2026-09-25T14:36:30Z", holds: [], this: true }),
      machine(),
    ],
    launches: [],
    launching: { parent: "/w", repository: "agentic-tools" },
    ...over,
  };
}

/** The pane's blocks, rendered as markup, with no request anywhere near it. */
function rendered(payload: Page): string {
  // Blocks is what FleetPane renders once its read has answered; the read
  // itself is the pane's own effect, which never runs under static markup.
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Blocks page={payload} />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the fleet page", () => {
  it("shows a needs-you line for every silent holder, with the remedies named", () => {
    const markup = rendered(page());

    expect(markup).toContain("Needs you");
    expect(markup).toContain("tests-parallel-and-deterministic");
    expect(markup).toContain(`held by m1c, unreachable since ${minuteTime(held().since)}`);
    expect(markup).toContain(NEEDS_YOU_REMEDY);
    expect(markup).toContain("goal steal");
    expect(markup).toContain("goal resume");
  });

  it("leaves the block out entirely when nothing needs a human", () => {
    const calm = page({
      needsYou: [],
      machines: [machine({ machine: "m1u", standing: "reachable", holds: [], this: true })],
    });

    const markup = rendered(calm);

    expect(markup).not.toContain("Needs you");
    expect(markup).not.toContain(NEEDS_YOU_REMEDY);
    // A calm fleet is not an empty one: the table is still there.
    expect(markup).toContain("This seat");
    expect(markup).toContain("The fleet");
  });

  it("says a checkout with no nickname publishes no presence", () => {
    const markup = rendered(
      page({
        this: seat({
          machine: "",
          noNickname: true,
          armed: "not armed",
          health: null,
          publication: null,
          running: null,
        }),
      }),
    );

    expect(markup).toContain("This checkout has no machine nickname and publishes no presence.");
    expect(markup).toContain("supervision is not armed here");
    expect(markup).toContain("no presence has been published from this checkout");
    expect(markup).toContain("no health verdict has been recorded on this checkout");
  });

  it("names a stale verdict and an unreadable one apart from an unarmed seat", () => {
    const stale = rendered(page({ this: seat({ armed: "stale" }) }));
    const torn = rendered(
      page({
        this: seat({
          armed: "unreadable",
          health: { state: "", observedAt: "", problem: "the health record here is malformed", roles: [] },
        }),
      }),
    );

    expect(stale).toContain("the last health verdict is older than the window this seat judges by");
    expect(torn).toContain("the health record here is malformed");
  });

  it("collapses the alive roles behind a disclosure and shows the rest", () => {
    const markup = rendered(page());

    expect(markup).toContain("presence not published since 09:10");
    expect(markup).toContain("1 roles alive");
    expect(markup).toContain("<details");
  });

  it("says what an empty fleet is, rather than showing an empty table", () => {
    const markup = rendered(page({ needsYou: [], machines: [] }));

    expect(markup).toContain(NO_PRESENCE);
    expect(markup).not.toContain("<table");
  });

  it("shows every row's standing, what it was seen at, and the goals it holds", () => {
    const markup = rendered(page());

    expect(markup).toContain("unreachable");
    expect(markup).toContain("reachable");
    expect(markup).toContain(`since ${minuteTime(machine().since)}`);
    expect(markup).toContain("3f9c1e2");
    expect(markup).toContain("generation 4");
    expect(markup).toContain("this seat");
  });

  it("ends with where the presence copy and the claims came from", () => {
    const markup = rendered(page());

    expect(markup).toContain(copyLine(page(), now).split(";")[1].trim());
    expect(markup).toContain("5b9d958");
  });

  it("says the copy's own trouble where the last fetch failed", () => {
    const markup = rendered(
      page({
        copy: {
          source: "the interface",
          attemptedAt: "2026-09-25T14:36:20Z",
          succeededAt: "2026-09-25T14:20:00Z",
          failedAt: "2026-09-25T14:36:20Z",
          problem: "presence fetch: the remote refused",
          metadataProblem: "",
        },
      }),
    );

    expect(markup).toContain("presence fetch: the remote refused");
  });
});

describe("the capture a question from this page carries", () => {
  it("is the rows the page displayed, bounded, with the whole named", () => {
    const capture = captureOfFleet(page(), now);

    expect(capture.source).toBe("the interface");
    expect(capture.total).toBe(2);
    expect(capture.machines?.map((row) => row.machine)).toEqual(["m1u", "m1c"]);
    expect(capture.machines?.[1].flag).toBe(`held by m1c, unreachable since ${minuteTime(held().since)}`);
    expect(capture.machines?.[1].holds).toEqual(["tests-parallel-and-deterministic"]);
    expect(capture.needsYou).toEqual([
      `tests-parallel-and-deterministic is held by m1c, unreachable since ${minuteTime(held().since)}`,
    ]);
  });

  it("carries at most its own bound, and says what the whole was", () => {
    const many = page({
      machines: Array.from({ length: 60 }, (_, at) => machine({ machine: `m${String(at)}`, holds: [] })),
    });

    const capture = captureOfFleet(many, now);

    expect(capture.machines?.length).toBe(24);
    expect(capture.total).toBe(60);
  });
});

/**
 * The table stacks into one card per machine at phone width, standing first.
 *
 * It is a stylesheet rule and not a branch in the component: one table, one
 * DOM, and the labels each cell carries are in the markup at every width. So
 * this reads the stylesheet, which is where the rule lives, and the four
 * hundred pixel screenshot is what proves it renders.
 */
describe("the fleet at phone width", () => {
  const css = readFileSync(path.join(SRC, "fleet.css"), "utf8");

  it("stacks the table into cards", () => {
    const phone = css.slice(css.indexOf("@media (max-width: 599px)"));

    expect(phone).not.toBe("");
    expect(phone).toContain(".ms-fleet-table thead");
    expect(phone).toContain("display: none;");
    expect(phone).toContain(".ms-fleet-row");
    expect(phone).toContain("flex-direction: column;");
  });

  it("puts the standing first on the card", () => {
    const phone = css.slice(css.indexOf("@media (max-width: 599px)"));
    const standing = phone.slice(phone.indexOf(".ms-fleet-cell--standing"));

    expect(standing).toContain("order: -1;");
  });

  it("carries a label in every cell, so a card reads without the header row", () => {
    const markup = rendered(page());

    for (const label of ["Machine", "Standing", "Seen", "Running", "Holds", "Engine"]) {
      expect(markup).toContain(`<span class="ms-fleet-label">${label}</span>`);
    }
  });
});
