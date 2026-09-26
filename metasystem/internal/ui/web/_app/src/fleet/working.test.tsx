import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Box, Machine, Page, ThisSeat, Working } from "./api";
import { captureOfFleet } from "./capture";
import {
  attemptsLeftWords,
  attemptWords,
  barShare,
  capWords,
  jobWords,
  minutesWords,
  NO_BOX,
  phaseWords,
  reservedWords,
  RESERVED_MEANING,
  workingSource,
  workingWords,
} from "./fleet";
import { Blocks } from "./FleetPane";
import { minuteTime } from "../backlog/format";
import { FLEET_OPEN_KEY, readFleetOpen, writeFleetOpen, type Store } from "../storage";

/**
 * What a seat is doing, as this page says it.
 *
 * The page judges none of it: the server composed the phase, the box and the
 * chain, and what is asserted here is that every shape reaches the screen as
 * something a human can read — a build with no round denominator, a critic
 * with one, a reservation that has not begun, a goal with no box, a
 * projection nobody could make, and a row a viewer left open.
 *
 * Nothing here reaches the network. The blocks are rendered through the
 * pane's own internals, as every other test of this page is, because
 * FleetPane's first act is to read the resource.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");

const now = new Date("2026-09-25T14:37:00Z");

/** The instants below are written against that clock and never the wall. */
function at(minutes: number): string {
  return new Date(now.getTime() + minutes * 60000).toISOString().replace(/\.\d{3}Z$/, "Z");
}

function box(over: Partial<Box> = {}): Box {
  return {
    attempts: 3,
    attemptLimit: 10,
    reservedMinutes: 610,
    reservedMinutesLimit: 720,
    problem: "",
    ...over,
  };
}

function working(over: Partial<Working> = {}): Working {
  return {
    goal: "g1-s15",
    phase: { role: "implementer", round: 2, roundLimit: null },
    job: {
      id: "j-17",
      role: "implementer",
      status: "running",
      startedAt: at(-41),
      capMinutes: 120,
      capEndsAt: at(79),
    },
    box: box(),
    chain: [
      {
        job: "j-17",
        role: "implementer",
        round: 2,
        status: "running",
        startedAt: at(-41),
        endedAt: null,
        capMinutes: 120,
      },
      {
        job: "j-12",
        role: "code-critic",
        round: 1,
        status: "completed",
        startedAt: at(-180),
        endedAt: at(-120),
        capMinutes: 60,
      },
    ],
    ...over,
  };
}

function machine(over: Partial<Machine> = {}): Machine {
  return {
    machine: "m1e",
    standing: "reachable",
    reason: "published 2 min ago",
    ageSeconds: 120,
    seen: at(-2),
    since: "",
    running: null,
    engine: "3f9c1e2abcdef",
    generation: 4,
    holds: [],
    this: false,
    working: [working()],
    workingProblem: "",
    ...over,
  };
}

function seat(): ThisSeat {
  return {
    machine: "m1u",
    noNickname: false,
    armed: "armed",
    health: null,
    publication: null,
    publicationProblem: "",
    running: null,
    runningProblem: "",
  };
}

function page(rows: Machine[]): Page {
  return {
    schemaVersion: 1,
    readAt: at(0),
    copy: {
      source: "the interface",
      attemptedAt: at(-1),
      succeededAt: at(-1),
      failedAt: "",
      problem: "",
      metadataProblem: "",
    },
    claims: { tip: "5b9d958c0d1e", unavailable: "" },
    this: seat(),
    needsYou: [],
    machines: rows,
    launches: [],
    launching: { parent: "", repository: "" },
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

describe("the phase sentence", () => {
  it("says the round, the goal, how long the job has run and its cap", () => {
    expect(phaseWords(working(), now)).toBe("implementer round 2 on g1-s15 · running 41 min, cap 2 h");
  });

  it("shows a build's round with no denominator, because a build has no limit of its own", () => {
    expect(phaseWords(working(), now)).not.toContain(" of ");
  });

  it("counts a critic's round against the limit its chain froze", () => {
    const critic = working({ phase: { role: "code-critic", round: 1, roundLimit: 2 }, goal: "g1-s19" });

    expect(phaseWords(critic, now)).toContain("code-critic round 1 of 2 on g1-s19");
  });

  it("says a reservation that has not begun rather than reading as work in flight", () => {
    const held = working({
      job: { id: "j-22", role: "code-critic", status: "pending", startedAt: null, capMinutes: 60, capEndsAt: null },
    });

    expect(phaseWords(held, now)).toBe("implementer round 2 on g1-s15 · pending, cap 60 min");
  });

  /**
   * The engine creates a build record pending with a start stamped at the
   * same instant, so a minute count taken from that stamp said a job had been
   * running for forty-one minutes while the line beside it said pending.
   */
  it("never counts a pending job in minutes, however it is stamped", () => {
    const stamped = working({
      job: { ...working().job, status: "pending" },
    });

    expect(jobWords(stamped.job, now)).toBe("pending, cap 2 h");
    // The stamp and the bound are untouched by the wording.
    expect(stamped.job.startedAt).toBe(at(-41));
    expect(capWords(stamped.job, now)).toBe("its cap ends in 79 min");
  });

  it("says how a job that has ended, ended", () => {
    for (const status of ["completed", "failed", "cancelled", "timeout"]) {
      expect(jobWords({ ...working().job, status }, now)).toBe(status);
    }
  });

  it("says a running job with no start is running rather than counting from nothing", () => {
    expect(jobWords({ ...working().job, status: "running", startedAt: null }, now)).toBe("running, cap 2 h");
  });

  it("is idle where a machine carries nothing", () => {
    expect(workingWords(machine({ working: [] }), now)).toBe("idle");
  });

  it("falls back to the chain for a machine that publishes no phase", () => {
    // A fleet is not one build: a machine still publishing the chain alone
    // is answered from the chain rather than reported idle.
    const older = machine({
      working: [],
      running: { job: "j-9", role: "critic", round: 1, goal: "g1-s19", startedAt: at(-30) },
    });

    expect(workingWords(older, now)).toBe("critic round 1 on g1-s19");
  });

  it("names the newest of several in flight and counts the rest", () => {
    const busy = machine({ this: true, working: [working(), working({ goal: "g1-s26" })] });

    expect(workingWords(busy, now)).toContain("(and 1 more in flight)");
  });

  it("carries the jobs reader's own problem rather than showing idle", () => {
    const torn = machine({ this: true, working: [], workingProblem: "chain unread: torn.json" });

    expect(workingWords(torn, now)).toBe("chain unread: torn.json");
  });
});

describe("the only forward-looking words", () => {
  it("name the cap and round up", () => {
    expect(capWords(working().job, now)).toBe("its cap ends in 79 min");
  });

  it("say a cap that has passed in the past tense", () => {
    const over = working({
      job: { ...working().job, capEndsAt: at(-5) },
    });

    expect(capWords(over.job, now)).toBe("its cap ended 5 min ago");
  });

  it("are empty where no deadline can be named", () => {
    const held = working({ job: { ...working().job, capEndsAt: null } });

    expect(capWords(held.job, now)).toBe("");
  });

  it("is minutes, then hours from two hours, and never days", () => {
    // A budget's day is eight hours in this kit, so a duration printed in days
    // would read as two different lengths depending on who read it.
    const read: Record<number, string> = {
      0: "0 min",
      1: "1 min",
      59: "59 min",
      60: "60 min",
      119: "119 min",
      120: "2 h",
      121: "2 h 1 min",
      481: "8 h 1 min",
      720: "12 h",
      2880: "48 h",
    };
    for (const [minutes, words] of Object.entries(read)) {
      expect(minutesWords(Number(minutes))).toBe(words);
    }
    for (const minutes of [1, 59, 60, 481, 2880]) {
      expect(minutesWords(minutes)).not.toContain("d");
    }
  });
});

describe("the box", () => {
  it("says what is spent, against what, and what is left", () => {
    expect(attemptWords(box())).toBe("attempt 3 of 10");
    expect(attemptsLeftWords(box())).toBe("7 attempts left");
    expect(reservedWords(box())).toBe("10 h 10 min of 12 h reserved");
  });

  it("says nothing about attempts where the projection carries no numbers", () => {
    const unknown = box({ attempts: null, attemptLimit: null, reservedMinutes: null, reservedMinutesLimit: null });

    expect(attemptWords(unknown)).toBe("");
    expect(reservedWords(unknown)).toBe("");
  });

  it("never draws a bar past its track, and never below nothing", () => {
    expect(barShare(3, 10)).toBeCloseTo(0.3);
    expect(barShare(610, 720)).toBeCloseTo(0.847, 3);
    expect(barShare(12, 10)).toBe(1);
    expect(barShare(5, 0)).toBe(0);
  });

  it("draws the share in twentieths, which is what the stylesheet defines", () => {
    const markup = rendered(page([machine()]));

    // 3 of 10 is six twentieths; 610 of 720 is seventeen.
    expect(markup).toContain("ms-fleet-bar-fill--6");
    expect(markup).toContain("ms-fleet-bar-fill--17");
  });

  it("defines every twentieth the page can ask for", () => {
    const css = readFileSync(path.join(SRC, "fleet.css"), "utf8");

    for (let index = 0; index <= 20; index += 1) {
      expect(css).toContain(`.ms-fleet-bar-fill--${String(index)} {`);
    }
  });
});

describe("what the row opens to", () => {
  it("is hidden until a viewer opens it, and is in the markup either way", () => {
    const markup = rendered(page([machine()]));

    expect(markup).toContain('id="ms-fleet-work-m1e"');
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).toContain("hidden");
  });

  it("carries the goal, the job, the box and the chain", () => {
    const markup = rendered(page([machine()]));

    expect(markup).toContain("Goal");
    expect(markup).toContain("This job");
    expect(markup).toContain("attempt 3 of 10 · 7 attempts left");
    expect(markup).toContain(`started ${minuteTime(at(-41))}`);
    expect(markup).toContain("10 h 10 min of 12 h reserved");
    expect(markup).toContain(RESERVED_MEANING);
    expect(markup).toContain("j-12");
  });

  /**
   * The bound is rendered by the browser's own clock, and this page renders
   * against the real one. So the cap is put far enough ahead that the
   * sentence reads the same whenever the suite runs; how many minutes it
   * names is asserted above, against a fixed clock.
   */
  it("says when the cap ends, and names it as the cap", () => {
    const ahead = working({ job: { ...working().job, capEndsAt: "2099-01-01T00:00:00Z" } });
    const markup = rendered(page([machine({ working: [ahead] })]));

    expect(markup).toContain("its cap ends in");
    expect(markup).not.toContain("finishes in");
  });

  it("says a goal with no box in words rather than drawing a bar at nothing", () => {
    const markup = rendered(page([machine({ working: [working({ box: null })] })]));

    expect(markup).toContain(NO_BOX);
    expect(markup).not.toContain("ms-fleet-bar-fill");
  });

  it("carries the projection's own problem where it could not be made", () => {
    const reason = "the claim episode timestamp is malformed";
    const markup = rendered(
      page([
        machine({
          working: [
            working({
              box: box({
                attempts: null,
                attemptLimit: null,
                reservedMinutes: null,
                reservedMinutesLimit: null,
                problem: reason,
              }),
            }),
          ],
        }),
      ]),
    );

    expect(markup).toContain(reason);
    expect(markup).not.toContain("ms-fleet-bar-fill");
  });

  /**
   * Two jobs on one goal are one group with the goal named once. It is what
   * this seat's row most often carries — a round and the critique of it — and
   * listed flat it repeated the goal and its title per job, which reads as two
   * goals running at once.
   */
  it("groups this seat's jobs by goal, the goal named once, in the payload's order", () => {
    const critique = working({
      goal: "g1-s15",
      job: { ...working().job, id: "j-18", startedAt: at(-12) },
      phase: { role: "code-critic", round: 1, roundLimit: null },
    });
    const other = working({ goal: "g1-s26", job: { ...working().job, id: "j-20" } });
    const markup = rendered(page([machine({ working: [critique, working(), other] })]));

    // One goal line per goal, and the count where a goal has more than one.
    expect(markup.match(/ms-fleet-goal/g)).toHaveLength(2);
    expect(markup).toContain("2 jobs in flight");
    // The groups stand where their newest job stood: g1-s15 opened the list.
    expect(markup.indexOf("g1-s15")).toBeLessThan(markup.indexOf("g1-s26"));
    // And every job is still there, each with its own phase and chain.
    expect(markup).toContain("code-critic");
    expect(markup).toContain("implementer");
  });

  it("carries the goal's title beside its chip where this page has one", () => {
    const held = {
      goal: "g1-s15",
      title: "Say what a seat is doing.",
      lane: "in-progress" as const,
      machine: "m1e",
      standing: "reachable" as const,
      since: "",
      flag: "",
    };
    const markup = rendered(page([machine({ holds: [held] })]));

    expect(markup).toContain("Say what a seat is doing.");
  });

  it("says where it came from: this seat's records, or when it was published", () => {
    expect(workingSource(machine({ this: true }), now)).toContain("this host's own job records");
    expect(workingSource(machine(), now)).toBe("as published 2 min ago");
  });
});

describe("the fleet at phone width", () => {
  const css = readFileSync(path.join(SRC, "fleet.css"), "utf8");
  const phone = css.slice(css.indexOf("@media (max-width: 599px)"));

  it("joins an opened row to the card above it rather than making a second card", () => {
    expect(phone).toContain(".ms-fleet-row--open");
    expect(phone).toContain("border-radius: 8px 8px 0 0;");
    expect(phone).toContain(".ms-fleet-opened:not([hidden])");
    expect(phone).toContain("border-radius: 0 0 8px 8px;");
  });

  it("gives the bars the card's full width", () => {
    const bar = phone.slice(phone.indexOf("  .ms-fleet-bar {"));

    expect(bar).toContain("width: 100%;");
  });
});

describe("which rows a viewer left open", () => {
  function store(values: Record<string, string>): Store {
    const written = new Map(Object.entries(values));
    return {
      getItem: (key) => written.get(key) ?? null,
      setItem: (key, value) => {
        written.set(key, value);
      },
    };
  }

  it("is read from the one key, and written back sorted", () => {
    const written = store({ [FLEET_OPEN_KEY]: "m2a m1e" });

    expect([...readFleetOpen(written)].sort()).toEqual(["m1e", "m2a"]);
    writeFleetOpen(new Set(["m2a", "m1e", "m1u"]), written);
    expect(written.getItem(FLEET_OPEN_KEY)).toBe("m1e m1u m2a");
  });

  it("reads a stored value that is not a list of nicknames as nothing open", () => {
    expect(readFleetOpen(store({ [FLEET_OPEN_KEY]: '{"m1e":true}' })).size).toBe(0);
  });

  it("is nothing at all in a browser with site data blocked", () => {
    const blocked: Store = {
      getItem: () => {
        throw new Error("site data is blocked");
      },
      setItem: () => {
        throw new Error("site data is blocked");
      },
    };

    expect(readFleetOpen(blocked).size).toBe(0);
    expect(() => {
      writeFleetOpen(new Set(["m1e"]), blocked);
    }).not.toThrow();
  });
});

describe("what a question asked from this page carries", () => {
  it("is the phase sentence per row and which rows were open", () => {
    const captured = captureOfFleet(page([machine(), machine({ machine: "m2a" })]), now, new Set(["m2a"]));

    expect(captured.machines?.[0].phase).toBe("implementer round 2 on g1-s15 · running 41 min, cap 2 h");
    expect(captured.machines?.[0].open).toBe(false);
    expect(captured.machines?.[1].open).toBe(true);
  });
});
