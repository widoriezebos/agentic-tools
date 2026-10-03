import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { BoardPayload, Held, Lane, Machine, Page, ThisSeat } from "./api";
import { captureOfFleet } from "./capture";
import { copyLine, NEEDS_YOU_REMEDY, NO_PRESENCE } from "./fleet";
import { minuteTime } from "../backlog/format";
import { Blocks, Panel } from "./FleetPane";
import { SCOPE, type BoardReading, type FleetReading } from "./panel";

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
function rendered(payload: Page, board?: BoardReading): string {
  // Blocks is what FleetPane renders once its read has answered; the read
  // itself is the pane's own effect, which never runs under static markup.
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Blocks page={payload} board={board} />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

/** The whole panel, from both readings, whatever state each is in. */
function panel(fleet: FleetReading, board: BoardReading): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <Panel fleet={fleet} board={board} onRetry={() => undefined} />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function lane(over: Partial<Lane> = {}): Lane {
  return {
    root: "/w/landing",
    registered_by: "wido",
    registered_at: "2026-09-25T08:00:00Z",
    owner: { state: "idle", pid: null, since: null, last_exit: null, stopped_by: null, retry_hint: null },
    summary: "landing lane /w/landing: idle",
    paused: false,
    agent_alive: false,
    queue: [],
    running_proof: null,
    last_proof: null,
    last_push: null,
    problems: [],
    ...over,
  };
}

function board(over: Partial<BoardPayload> = {}): BoardReading {
  return {
    state: "read",
    board: {
      readable: true,
      bridge: "live",
      seats: [],
      lines: [{ machine: "m1g", text: "one-folder-deployed-and-evolved unknown: claim moved to m1f" }],
      lane: lane(),
      titles: {},
      questions: [],
      questionsProblem: "",
      ...over,
    },
  };
}

/** A fleet with nothing that needs anyone. */
function calm(over: Partial<Page> = {}): Page {
  return page({
    needsYou: [],
    this: seat({ health: { ...seat().health!, roles: [{ role: "steward-runner", status: "alive", reason: "runner alive" }] } }),
    machines: [machine({ machine: "m1u", standing: "reachable", seen: "2026-09-25T14:36:30Z", holds: [], this: true })],
    ...over,
  });
}

describe("the verdict strip", () => {
  it("stands on top and says All good on this computer when every read succeeded and nothing needs you", () => {
    const markup = rendered(calm(), board());

    expect(markup.indexOf("All good on this computer")).toBeGreaterThanOrEqual(0);
    expect(markup.indexOf("All good on this computer")).toBeLessThan(markup.indexOf("This seat"));
    expect(markup).toContain("ms-fleet-verdict--ok");
    expect(markup).toContain(SCOPE.replaceAll("'", "&#x27;"));
    expect(markup).toContain("0 seats working · 0 waiting to land · updated");
  });

  it("counts what needs you instead", () => {
    const markup = rendered(page(), board());

    expect(markup).toContain("1 thing needs you");
    expect(markup).toContain("ms-fleet-verdict--attention");
  });

  it("names the section it could not read, and that section says so", () => {
    const markup = rendered(calm(), { state: "failed", message: "board answered 500" });

    expect(markup).toContain("Can&#x27;t read this computer&#x27;s board, the landing lane and questions");
    expect(markup).toContain("The landing lane could not be read: board answered 500");
    expect(markup).toContain("This checkout&#x27;s questions could not be read: board answered 500");
    expect(markup).not.toContain("All good");
  });

  it("names a fleet it could not read again, beside the reading it keeps", () => {
    const markup = panel({ state: "read", page: calm(), problem: "fleet answered 502" }, board());

    expect(markup).toContain("Can&#x27;t read the fleet");
    expect(markup).toContain("what is on screen is the last reading: fleet answered 502");
    expect(markup).toContain("<table");
  });

  it("never says All good over a presence copy it could not read, and keeps Launch a machine, whose own flow checks itself", () => {
    const markup = rendered(
      calm({ copy: { ...page().copy, problem: "presence ref list: exit status 128" } }),
      board(),
    );

    expect(markup).toContain("Can&#x27;t read the fleet");
    expect(markup).not.toContain("All good");
    expect(markup).toContain("presence ref list: exit status 128");
    expect(markup).toMatch(/<button[^>]*>Launch a machine<\/button>/);
    expect(markup).not.toContain("waits until the fleet can be read");
  });

  it("says a first fleet read that failed in the fleet's own section, with Try again", () => {
    const markup = panel({ state: "failed", message: "fleet answered 500" }, board());

    expect(markup).toContain("Can&#x27;t read the fleet");
    expect(markup).toContain("The fleet could not be read: fleet answered 500");
    expect(markup).toContain(">Try again<");
    // The lane read on its own and is still drawn.
    expect(markup).toContain("Landing lane");
  });

  it("draws a skeleton, never an empty page, while the first reads are on their way", () => {
    const markup = panel({ state: "loading" }, { state: "loading" });

    expect(markup).toContain("Reading…");
    expect(markup).toContain("ms-skeleton");
    expect(markup).toContain('aria-busy="true"');
    expect(markup).not.toContain("All good");
  });
});

describe("the fleet page", () => {
  it("shows a needs-you line for every silent holder, by its title, with the remedies named", () => {
    const markup = rendered(page());

    expect(markup).toContain("Needs you");
    expect(markup).toContain("“Run the suite in parallel.” is held by m1c, unreachable since");
    expect(markup).toContain('href="/backlog/goal/tests-parallel-and-deterministic"');
    expect(markup).toContain(`held by m1c, unreachable since ${minuteTime(held().since)}`);
    expect(markup).toContain(NEEDS_YOU_REMEDY);
    expect(markup).toContain("goal steal");
    expect(markup).toContain("goal resume");
  });

  it("leaves the block out entirely when nothing needs a human", () => {
    const markup = rendered(calm(), board());

    expect(markup).not.toContain("Needs you");
    expect(markup).not.toContain(NEEDS_YOU_REMEDY);
    // A calm fleet is not an empty one: the table is still there.
    expect(markup).toContain("This seat");
    expect(markup).toContain("The fleet");
  });

  it("collects the lane, the returns, a red proof, the questions and this computer's health into Needs you", () => {
    const unhealthy = calm({
      this: seat({
        armed: "not armed",
        health: { state: "unhealthy", observedAt: "2026-09-25T14:30:00Z", problem: "", roles: [{ role: "steward-runner", status: "dead", reason: "runner gone" }] },
      }),
    });
    const read = board({
      titles: { "plain-lane": "Plain lane landing." },
      lane: lane({
        owner: { state: "stopped", pid: null, since: "2026-09-25T13:40:00Z", last_exit: null, stopped_by: "m1e", retry_hint: null },
        paused: true,
        queue: [
          { goal: "plain-lane", branch: "goal/plain-lane", sha: "abc", seat: "m1g", at: "2026-09-25T12:00:00Z", state: "returned", reason: "the full test run is red", returned_at: "2026-09-25T14:00:00Z" },
        ],
      }),
      questions: [{ id: "q-1", goal: "plain-lane", machine: "m1f", question: "Land slice 2 now?", openedAt: "2026-09-25T14:10:00Z" }],
    });

    const markup = rendered(unhealthy, read);
    const needs = markup.slice(markup.indexOf("Needs you"), markup.indexOf("This seat"));

    expect(needs).toContain("The landing lane is paused by m1e since");
    expect(needs).toContain("It lands nothing until someone resumes it with landing start at a terminal.");
    expect(needs).toContain("“Plain lane landing.” came back: the full test run is red.");
    expect(needs).toContain(">Open goal<");
    expect(needs).toContain("m1f asks about “Plain lane landing.”: Land slice 2 now?");
    expect(needs).toContain('href="/decisions"');
    expect(needs).toContain(">Answer<");
    expect(needs).toContain("This computer&#x27;s steward is not running.");
    expect(markup).toContain("4 things need you");
    // A paused lane is said once in Needs you; the lane says its one word.
    expect(markup).toContain(">Paused<");
    expect(markup).not.toContain("Pause<");
  });

  it("says in Needs you when this checkout's questions could not be read", () => {
    const markup = rendered(calm(), board({ questionsProblem: "channel folder unreadable" }));

    expect(markup).toContain("Needs you");
    expect(markup).toContain("This checkout&#x27;s questions could not be read: channel folder unreadable");
    expect(markup).toContain("Can&#x27;t read questions");
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
    expect(markup).toContain("this seat");
  });

  it("shows the engine short in the row and the generation in the opened row", () => {
    const markup = rendered(page());
    const engine = markup.slice(markup.indexOf("ms-fleet-cell--engine"));
    const cell = engine.slice(0, engine.indexOf("</td>"));
    const opened = markup.slice(markup.indexOf('id="ms-fleet-work-m1c"'));

    expect(cell).toContain("3f9c1e2");
    expect(cell).not.toContain("generation");
    expect(cell).not.toContain("3f9c1e2abcdef");
    expect(opened).toContain("3f9c1e2abcdef · generation 4");
  });

  it("shows a held goal by its title, with its id in the link", () => {
    const markup = rendered(page());
    const holds = markup.slice(markup.indexOf("ms-fleet-cell--holds", markup.indexOf("<tbody")));

    expect(holds).toContain('href="/backlog/goal/tests-parallel-and-deterministic"');
    expect(holds).toContain(">Run the suite in parallel.</a>");
  });

  it("says what each machine is doing in a Doing column, from this computer's board", () => {
    const read = board({
      seats: [
        {
          machine: "m1c",
          installation: "/w/m1c/metasystem",
          goals: [{ goal: "tests-parallel-and-deterministic", stage: "review", round: { n: 3, max: 20 }, since: "2026-09-25T14:28:00Z" }],
        },
      ],
    });

    const markup = rendered(page(), read);

    expect(markup).toContain("<th scope=\"col\">Doing");
    expect(markup).not.toContain("<th scope=\"col\">Running");
    expect(markup).toContain("reviewing · round 3 of 20 · ");
  });

  it("no longer draws the This host list, nor the board's stale cards anywhere", () => {
    const read = board({
      seats: [
        { machine: "m1g", installation: "/w/m1g/metasystem", goals: [{ goal: "one-folder-deployed-and-evolved", stage: "build", since: "2026-09-25T10:00:00Z", unknown: "claim moved to m1f" }] },
      ],
    });

    const markup = rendered(page({ machines: [...page().machines, machine({ machine: "m1g", standing: "reachable", holds: [] })] }), read);

    expect(markup).not.toContain("This host");
    expect(markup).not.toContain("claim moved");
    expect(markup).not.toContain("one-folder-deployed-and-evolved");
    expect(markup).not.toContain("bridge live");
  });

  it("says in This seat what its own row's Doing column says, never idle beside work", () => {
    const read = board({
      seats: [{ machine: "m1u", installation: "/w/m1u/metasystem", goals: [{ goal: "fleet-panel-ux", stage: "build", since: "2026-09-25T14:25:00Z" }] }],
    });
    const holding = page({
      this: seat({ running: null }),
      machines: [
        machine({
          machine: "m1u",
          standing: "reachable",
          this: true,
          holds: [held({ goal: "fleet-panel-ux", title: "The fleet panel.", machine: "m1u", standing: "reachable", flag: "", since: "" })],
        }),
      ],
    });

    const markup = rendered(holding, read);
    const seatBlock = markup.slice(markup.indexOf("This seat"), markup.indexOf("The fleet"));

    expect(seatBlock).toContain("building · ");
    expect(seatBlock).not.toContain(">idle<");
  });

  it("says under the table which parts of this computer's board it could not read", () => {
    const markup = rendered(calm(), board({ unreadable: ["/h/board/m1e: the nickname m1e names 2 armed checkouts; neither is read"] }));

    expect(markup).toContain("Can&#x27;t read this computer&#x27;s board");
    expect(markup).toContain(
      "Part of this computer&#x27;s board could not be read, so Doing says what those seats&#x27; records say: /h/board/m1e: the nickname m1e names 2 armed checkouts; neither is read",
    );
  });

  it("keeps the questions it could read beside the records it could not", () => {
    const markup = rendered(
      calm(),
      board({
        questions: [{ id: "q-1", goal: "", about: "lane", machine: "m1f", question: "Return the branch?", openedAt: "2026-09-25T14:10:00Z" }],
        questionsProblem: "1 of its records can't be read: /c/q-2.json: unexpected end of JSON input",
      }),
    );

    expect(markup).toContain("Can&#x27;t read questions · 1 thing needs you");
    expect(markup).toContain("m1f asks about the landing lane: Return the branch?");
    expect(markup).toContain("This checkout&#x27;s questions could not be read: 1 of its records can&#x27;t be read");
  });

  it("draws the landing lane after the table", () => {
    const markup = rendered(calm(), board());

    expect(markup.indexOf("The fleet")).toBeLessThan(markup.indexOf("Landing lane"));
  });

  it("ends with where the presence copy and the claims came from", () => {
    const markup = rendered(page());

    expect(markup).toContain(copyLine(page(), now).split(";")[1].trim());
    expect(copyLine(page(), now)).toContain("claims from the accepted ledger");
    expect(copyLine(page(), now)).not.toContain("5b9d958");
    expect(markup).toContain('title="The accepted ledger at 5b9d958"');
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
  it("carries what the page showed: every Needs you line, and each row's Doing words", () => {
    const capture = captureOfFleet(page(), now, new Set(), {
      needsYou: ["The landing lane is paused by m1e since 13:40."],
      doing: { m1u: "building · 42 min" },
    });

    expect(capture.needsYou).toEqual(["The landing lane is paused by m1e since 13:40."]);
    expect(capture.machines?.[0].phase).toBe("building · 42 min");
  });

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

    for (const label of ["Machine", "Standing", "Seen", "Doing", "Holds", "Engine"]) {
      expect(markup).toContain(`<span class="ms-fleet-label">${label}</span>`);
    }
  });

  it("stacks the verdict, Needs you and the lane's lists into one column", () => {
    const phone = css.slice(css.indexOf("@media (max-width: 599px)"));

    expect(phone).toContain(".ms-fleet-verdict-line");
    expect(phone).toContain(".ms-fleet-needs-row");
    expect(phone).toContain(".ms-fleet-lane-item");
    expect(phone.match(/flex-direction: column;/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });
});
