import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Machine, Page, Working } from "./api";
import { workingLines } from "./fleet";
import { Blocks } from "./FleetPane";
import { Rail } from "../shell/Rail";

/**
 * The rail's Fleet dot (g1-s74 D5): a machine is working when its standing is
 * reachable and one of its jobs is running — the status says so, never a
 * start stamp — and the rail says one line per running job, in the phase
 * words of the entry that lit the dot.
 */

const now = new Date("2026-09-29T14:37:00Z");

function at(minutes: number): string {
  return new Date(now.getTime() + minutes * 60000).toISOString().replace(/\.\d{3}Z$/, "Z");
}

function job(id: string, status: string, over: Partial<Working> = {}): Working {
  return {
    goal: "verbs-match-intent",
    phase: { role: "implementer", round: 2, roundLimit: null },
    job: { id, role: "implementer", status, startedAt: at(-41), capMinutes: null, capEndsAt: null },
    box: null,
    chain: [],
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
    engine: "",
    generation: 0,
    holds: [],
    this: false,
    working: [job("j-17", "running")],
    workingProblem: "",
    ...over,
  };
}

function page(rows: Machine[]): Page {
  return {
    schemaVersion: 1,
    readAt: at(0),
    copy: { source: "", attemptedAt: "", succeededAt: "", failedAt: "", problem: "", metadataProblem: "" },
    claims: { tip: "", unavailable: "" },
    this: {
      machine: "m1u", noNickname: false, armed: "armed", health: null, publication: null,
      publicationProblem: "", running: null, runningProblem: "",
    },
    needsYou: [],
    machines: rows,
    launches: [],
    launching: { parent: "", repository: "" },
  };
}

function rail(working: readonly string[], expanded: boolean): string {
  return renderToStaticMarkup(
    <MemoryRouter initialEntries={["/backlog"]}>
      <TooltipPrimitive.Provider>
        <Rail
          expanded={expanded}
          toggleable
          onToggle={() => undefined}
          theme="system"
          onTheme={() => undefined}
          working={working}
        />
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

/** The Fleet row's own markup, from its link to the link's end. */
function fleetRow(markup: string): string {
  const start = markup.indexOf('href="/fleet"');
  return markup.slice(markup.lastIndexOf("<a", start), markup.indexOf("</a>", start) + 4);
}

describe("which machines are working", () => {
  it("counts a reachable machine whose job is running, in the phase words of that job", () => {
    expect(workingLines(page([machine()]), now)).toEqual([
      "m1e: implementer round 2 on verbs-match-intent · running 41 min",
    ]);
  });

  it("counts nothing from an unreachable machine", () => {
    expect(workingLines(page([machine({ standing: "unreachable" })]), now)).toEqual([]);
    expect(workingLines(page([machine({ standing: "unknown" })]), now)).toEqual([]);
  });

  it("counts nothing from a pending job that carries a start stamp", () => {
    expect(workingLines(page([machine({ working: [job("j-18", "pending")] })]), now)).toEqual([]);
  });

  it("counts nothing from a job with no status", () => {
    expect(workingLines(page([machine({ working: [job("j-19", "")] })]), now)).toEqual([]);
  });

  it("counts nothing from no page", () => {
    expect(workingLines(null, now)).toEqual([]);
  });

  it("rail_words_follow_running_job: this seat's running job beside a newer reservation counts, and is the one named", () => {
    const seat = machine({
      machine: "m1u",
      this: true,
      working: [
        job("j-21", "pending", { goal: "the-newer-reservation", phase: { role: "code-critic", round: 1, roundLimit: 3 } }),
        job("j-20", "running", { goal: "g1-s74" }),
      ],
    });
    expect(workingLines(page([seat]), now)).toEqual(["m1u: implementer round 2 on g1-s74 · running 41 min"]);
  });

  it("says one line per running job", () => {
    const busy = machine({ working: [job("j-17", "running"), job("j-16", "running", { goal: "g1-s73" })] });
    expect(workingLines(page([busy, machine({ machine: "m2a", standing: "unreachable" })]), now)).toEqual([
      "m1e: implementer round 2 on verbs-match-intent · running 41 min",
      "m1e: implementer round 2 on g1-s73 · running 41 min",
    ]);
  });
});

describe("the rail's Fleet row", () => {
  const LINES = ["m1e: implementer round 2 on verbs-match-intent · running 41 min"];

  it("carries the dot and the words, expanded and collapsed", () => {
    for (const expanded of [true, false]) {
      const row = fleetRow(rail(LINES, expanded));
      expect({ expanded, dot: row.includes('<span class="ms-live-dot ms-rail-badge" aria-hidden="true"></span>') }).toEqual({ expanded, dot: true });
      expect({ expanded, words: row.includes(LINES[0]) }).toEqual({ expanded, words: true });
    }
  });

  it("carries neither when nothing is working", () => {
    for (const expanded of [true, false]) {
      const markup = rail([], expanded);
      expect({ expanded, dot: markup.includes("ms-live-dot") }).toEqual({ expanded, dot: false });
    }
  });

  it("puts the dot on no other row", () => {
    const markup = rail(LINES, true);
    expect(markup.split("ms-live-dot").length - 1).toBe(1);
  });
});

describe("the Fleet table's Running column", () => {
  function table(rows: Machine[]): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <Blocks page={page(rows)} />
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  it("takes the same dot before its words for a working machine, and keeps its words", () => {
    const markup = table([machine()]);
    expect(markup).toMatch(/Running<\/span><span class="ms-live-dot" aria-hidden="true"><\/span><span>implementer round 2 on verbs-match-intent · running/);
  });

  it("has no dot for a machine that is not working", () => {
    expect(table([machine({ working: [job("j-18", "pending")] })])).not.toContain("ms-live-dot");
    expect(table([machine({ standing: "unreachable" })])).not.toContain("ms-live-dot");
  });
});
