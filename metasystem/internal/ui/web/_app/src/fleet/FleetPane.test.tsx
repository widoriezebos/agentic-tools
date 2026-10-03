import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { BoardPayload, Held, Lane, Machine, Page, ThisSeat } from "./api";
import { captureOfFleet } from "./capture";
import { NO_PRESENCE } from "./fleet";
import { minuteTime } from "../backlog/format";
import { Blocks, Panel } from "./FleetPane";
import { type BoardReading, type FleetReading } from "./panel";

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

// The panel reads the clock once as it renders; every age on it is measured
// against the fixture's own instant, never the wall.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(now);
});

afterEach(() => {
  vi.useRealTimers();
});

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

/** The markup's words as a person reads them: no tags, no attributes. */
function text(markup: string): string {
  return markup.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();
}

/** One row's default cells, without the row it opens to. */
function row(markup: string, name: string): string {
  const cells = markup.slice(markup.indexOf(`<span class="ms-mono">${name}</span>`));
  return cells.slice(0, cells.indexOf("</tr>"));
}

/** Needs you, up to the fleet's own heading. */
function needsOf(markup: string): string {
  return markup.slice(markup.indexOf("Needs you"), markup.indexOf("The fleet<"));
}

describe("the verdict strip", () => {
  it("stands on top and says All good on this computer when every read succeeded and nothing needs you", () => {
    const markup = rendered(calm(), board());

    expect(markup.indexOf("All good on this computer")).toBeGreaterThanOrEqual(0);
    expect(markup.indexOf("All good on this computer")).toBeLessThan(markup.indexOf("The fleet<"));
    expect(markup).toContain("ms-fleet-verdict--ok");
    expect(text(markup)).toContain("0 seats working · 0 waiting to land · updated");
  });

  it("keeps what it checked beside the counts, and moves the long sentence into its help", () => {
    const markup = rendered(calm(), board());
    const strip = markup.slice(markup.indexOf('aria-label="Verdict"'), markup.indexOf("</section>"));

    expect(text(strip)).toMatch(/updated \d\d:\d\d · questions: this checkout only$/);
    expect(strip).toContain('aria-label="What is Verdict?"');
    expect(markup).not.toContain("Questions asked on other computers are not checked here.");
  });

  it("makes where the presence copy and the claims came from the title of updated, and draws no line for it", () => {
    const markup = rendered(page());

    expect(markup).toMatch(/<span title="presence fetched less than a minute ago by the interface; claims from the accepted ledger · The accepted ledger at 5b9d958">updated \d\d:\d\d<\/span>/);
    expect(markup).not.toContain("ms-fleet-provenance");
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

  it("still says, in the fleet's own section, every read of this seat's own it could not make", () => {
    const claims = rendered(calm({ claims: { tip: "", unavailable: "the accepted ledger could not be read" } }), board());
    const published = rendered(calm({ this: seat({ publicationProblem: "the publication state is malformed" }) }), board());
    const running = rendered(calm({ this: seat({ runningProblem: "chain unread: a.json" }) }), board());

    expect(claims).toContain("Can&#x27;t read the fleet");
    expect(claims).toContain("The goals each machine holds could not be read: the accepted ledger could not be read");
    expect(published).toContain("Can&#x27;t read the fleet");
    expect(published).toContain("What this seat last published could not be read: the publication state is malformed");
    expect(running).toContain("Can&#x27;t read the fleet");
    expect(running).toContain("What this seat is running could not be read: chain unread: a.json");
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
  it("draws no This seat section: its name is on its row", () => {
    const markup = rendered(page(), board());

    expect(markup).not.toMatch(/<h2[^>]*>This seat/);
    expect(markup).not.toContain("roles alive");
    expect(markup).not.toContain("supervision is armed here");
    expect(row(markup, "m1u")).toContain("this seat");
  });

  it("shows a needs-you line for a silent holder, by its title, with the take-over command for its goal", () => {
    const markup = needsOf(rendered(page()));

    expect(markup).toContain("“Run the suite in parallel.” is held by m1c, unreachable since");
    expect(markup).toContain(
      '<code class="ms-mono ms-fleet-needs-command">metasystem goal claim tests-parallel-and-deterministic --take-over --reason &quot;&lt;why&gt;&quot;</code>',
    );
    expect(markup).not.toContain("goal steal");
    expect(markup).not.toContain("Open goal");
  });

  it("leaves the block out entirely when nothing needs a human", () => {
    const markup = rendered(calm(), board());

    expect(markup).not.toContain("Needs you");
    // A calm fleet is not an empty one: the table is still there.
    expect(markup).toContain("The fleet");
    expect(markup).toContain("<table");
  });

  it("draws each item as its line with its one act at the end, or its one command in code", () => {
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
          { goal: "plain-lane", branch: "goal/plain-lane", sha: "abc", seat: "m1g", at: "2026-09-25T12:00:00Z", state: "returned", reason: "the full test run is red. Rebase and hand in again.", returned_at: "2026-09-25T14:00:00Z" },
        ],
      }),
      questions: [{ id: "q-1", goal: "plain-lane", machine: "m1f", question: "Land slice 2 now?", openedAt: "2026-09-25T14:10:00Z" }],
    });

    const markup = rendered(unhealthy, read);
    const needs = needsOf(markup);
    const items = needs.split('<li class="ms-fleet-needs-row">').slice(1);

    expect(items).toHaveLength(4);
    for (const item of items) {
      const acts = (item.match(/class="[^"]*ms-fleet-needs-act/g) ?? []).length;
      const commands = (item.match(/ms-fleet-needs-command/g) ?? []).length;
      expect({ item: text(item), offers: acts + commands }).toEqual({ item: text(item), offers: 1 });
    }
    expect(needs).toContain("The landing lane is paused by m1e since");
    expect(needs).toContain(">metasystem landing start</code>");
    expect(needs).toContain("“Plain lane landing.” came back.");
    expect(needs).toContain('<span class="ms-fleet-needs-note">the full test run is red.</span>');
    expect(needs).not.toContain("Rebase and hand in again.");
    expect(needs).toMatch(/<a [^>]*href="\/backlog\/goal\/plain-lane"[^>]*>Open goal<\/a>/);
    expect(needs).toContain("m1f asks about “Plain lane landing.”: Land slice 2 now?");
    expect(needs).toMatch(/href="\/decisions"[^>]*>Answer<\/a>/);
    expect(needs).toContain("This computer&#x27;s steward is not running.");
    expect(needs).toContain(">metasystem system start</code>");
    expect(markup).toContain("4 things need you");
    // A paused lane is said once in Needs you; the lane says its one word.
    expect(markup).toContain(">Paused<");
    expect(markup).not.toContain("Pause<");
  });

  it("puts a stuck seat in Needs you with the stop of its machine, says stalled in the marker colour with no live dot, and counts no seat working", () => {
    const holding = calm({
      machines: [
        machine({ machine: "m1u", standing: "reachable", seen: "2026-09-25T14:36:30Z", holds: [], this: true }),
        machine({
          machine: "m1f",
          standing: "reachable",
          seen: "2026-09-25T14:36:00Z",
          holds: [held({ goal: "plain-lane", title: "Plain lane landing.", machine: "m1f", standing: "reachable", flag: "", since: "" })],
        }),
      ],
    });
    const read = board({
      seats: [
        {
          machine: "m1f",
          installation: "/w/m1f/metasystem",
          goals: [{ goal: "plain-lane", stage: "build", since: "2026-09-25T13:30:00Z", lastProgressAt: "2026-09-25T14:02:00Z", unknown: "stalled" }],
        },
      ],
    });

    const markup = rendered(holding, read);
    const needs = needsOf(markup);

    expect(needs).toContain(`“Plain lane landing.” on m1f has not moved since ${minuteTime("2026-09-25T14:02:00Z")} (building).`);
    expect(needs).toContain(">metasystem machine stop m1f</code>");
    // What an override does is said whole, never cut to one line (R-143-m1e).
    expect(needs).toContain('<span class="ms-fleet-needs-impact">Stopping m1f ends its seat and every job on it; its steward will not start it again.</span>');
    expect(markup).toContain("1 thing needs you");
    expect(markup).toContain("0 seats working");
    expect(markup).toMatch(/ms-fleet-doing--stalled"><span>stalled · building · [^<]*min<\/span>/);
    expect(markup).not.toContain("ms-live-dot");
  });

  it("gives a red proof in Needs you an Open log link that opens the log in a tab of its own", () => {
    const read = board({
      lane: lane({
        last_proof: { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l/p.log", at: "2026-09-25T14:20:00Z", attempt: "a-9", reason: "the proving command exited 1" },
      }),
    });

    const markup = rendered(calm(), read);
    const needs = needsOf(markup);

    expect(needs).toContain("The landing lane&#x27;s last proof is red: the proving command exited 1.");
    expect(needs).toMatch(/<a [^>]*href="\/api\/fleet\/proof-logs\/a-9"[^>]*target="_blank"[^>]*>Open log<\/a>/);
    expect(needs).toContain('rel="noopener noreferrer"');
  });

  it("puts a failed health check's reasons behind Details, one line each, with no role name and none of the alive roles", () => {
    const unhealthy = calm({
      this: seat({
        health: {
          state: "unhealthy",
          observedAt: "2026-09-25T14:30:00Z",
          problem: "",
          roles: [
            { role: "steward-runner", status: "alive", reason: "runner alive" },
            { role: "narrator-freshness", status: "dead", reason: "lastSuccess is stale" },
            { role: "session-main", status: "unknown", reason: "no session main is announced" },
          ],
        },
      }),
    });

    const needs = needsOf(rendered(unhealthy, board()));

    expect(needs).toContain("This computer&#x27;s health check failed on 2 checks.");
    expect(needs).toContain(">metasystem system check</code>");
    expect(needs).toMatch(/<details class="ms-fleet-details"><summary class="ms-fleet-details-summary">Details<\/summary><ul class="ms-fleet-needs-reasons"><li>lastSuccess is stale<\/li><li>no session main is announced<\/li><\/ul><\/details>/);
    expect(needs).not.toMatch(/steward-runner|narrator-freshness|session-main|runner alive/);
  });

  it("names a stale health record and an unreadable one in Needs you, each with its command", () => {
    const stale = needsOf(rendered(page({ needsYou: [], this: seat({ armed: "stale" }) }), board()));
    const torn = needsOf(
      rendered(
        page({ needsYou: [], this: seat({ armed: "unreadable", health: { state: "", observedAt: "", problem: "the health record here is malformed", roles: [] } }) }),
        board(),
      ),
    );

    expect(stale).toContain("This computer&#x27;s steward has not recorded its health since");
    expect(stale).toContain(">metasystem system start</code>");
    expect(torn).toContain("This computer&#x27;s health record could not be read.");
    expect(torn).toContain("<li>the health record here is malformed</li>");
    expect(torn).toContain(">metasystem system check</code>");
  });

  it("says in Needs you when this checkout's questions could not be read", () => {
    const markup = rendered(calm(), board({ questionsProblem: "channel folder unreadable" }));

    expect(markup).toContain("Needs you");
    expect(markup).toContain("This checkout&#x27;s questions could not be read: channel folder unreadable");
    expect(markup).toContain("Can&#x27;t read questions");
  });

  it("says a checkout with no nickname publishes no presence, and nothing else of a This seat", () => {
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
    expect(markup).not.toContain("supervision is not armed here");
    expect(markup).not.toContain("no health verdict has been recorded on this checkout");
  });

  it("says what an empty fleet is, rather than showing an empty table", () => {
    const markup = rendered(page({ needsYou: [], machines: [] }));

    expect(markup).toContain(NO_PRESENCE);
    expect(markup).not.toContain("<table");
  });

  it("draws four columns: Machine, Doing, Holds and Seen", () => {
    const markup = rendered(page());
    const head = markup.slice(markup.indexOf("<thead"), markup.indexOf("</thead>"));

    expect(text(head)).toBe("Machine Doing Holds Seen");
  });

  it("says no engine and no standing word in a row's default cells, and Seen says unreachable with the reason as its title", () => {
    const markup = rendered(page());
    const silent = row(markup, "m1c");
    const here = row(markup, "m1u");

    for (const cells of [silent, here]) {
      expect(cells).not.toContain("3f9c1e2");
      expect(text(cells)).not.toMatch(/\breachable\b/);
      expect(cells).not.toContain("ms-fleet-pill");
    }
    expect(silent).toContain('<td class="ms-fleet-cell ms-fleet-cell--seen" title="no presence for 6 h, past 30 min');
    expect(text(silent)).toContain("unreachable · 6 h");
    expect(silent).toContain("ms-fleet-seen--bad");
  });

  it("shows the engine and the generation in the opened row", () => {
    const markup = rendered(page());
    const opened = markup.slice(markup.indexOf('id="ms-fleet-work-m1c"'));

    expect(opened).toContain("3f9c1e2abcdef · generation 4");
  });

  it("says in this seat's opened row when it last published, without the rung", () => {
    const markup = rendered(page());
    const opened = markup.slice(markup.indexOf('id="ms-fleet-work-m1u"'));
    const own = opened.slice(0, opened.indexOf("</tr>"));

    expect(text(own)).toContain("Presence published 2 min ago");
    expect(own).not.toContain("rung");
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
    expect(capture.machines?.[1].seen).toBe("unreachable · 6 h");
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
 * The table stacks into one card per machine at phone width: the machine and
 * what it is doing on the first line, then the titles it holds, then when it
 * was seen. A card carries no labels: what each line is reads from its words.
 *
 * It is a stylesheet rule and not a branch in the component: one table, one
 * DOM. So this reads the stylesheet, which is where the rule lives, and the
 * 390 pixel screenshot is what proves it renders.
 */
describe("what an override does", () => {
  it("wraps at every width and is never cut, unlike a reason (R-143-m1e)", () => {
    const css = readFileSync(path.join(SRC, "fleet.css"), "utf8");
    const at = css.indexOf(".ms-fleet-needs-impact {");
    const rule = css.slice(at, css.indexOf("}", at));

    expect(at).toBeGreaterThan(-1);
    expect(rule).toContain("white-space: normal;");
    expect(rule).not.toContain("ellipsis");
    expect(rule).not.toContain("overflow: hidden");
  });
});

describe("the fleet at phone width", () => {
  const css = readFileSync(path.join(SRC, "fleet.css"), "utf8");
  const phone = css.slice(css.indexOf("@media (max-width: 599px)"));

  it("stacks the table into cards", () => {
    expect(phone).not.toBe("");
    expect(phone).toContain(".ms-fleet-table thead");
    expect(phone).toContain("display: none;");
    expect(phone).toContain(".ms-fleet-row");
  });

  it("puts the machine and what it is doing on the card's first line, the holds and the sighting under them", () => {
    const card = phone.slice(phone.indexOf(".ms-fleet-row {"));
    const rule = (selector: string) => card.slice(card.indexOf(selector), card.indexOf("}", card.indexOf(selector)));

    expect(rule(".ms-fleet-row {")).toContain("flex-wrap: wrap;");
    expect(rule(".ms-fleet-cell--holds,")).toContain("flex-basis: 100%;");
  });

  it("renders no cell label on a card", () => {
    const markup = rendered(page());

    expect(markup).not.toContain("ms-fleet-label");
    expect(text(row(markup, "m1c"))).toMatch(/^m1c — Run the suite in parallel\. unreachable · 6 h$/);
  });

  it("stacks the verdict, Needs you and the lane's lists into one column", () => {
    expect(phone).toContain(".ms-fleet-verdict-line");
    expect(phone).toContain(".ms-fleet-needs-row");
    expect(phone).toContain(".ms-fleet-lane-item");
    expect(phone.match(/flex-direction: column;/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });
});
