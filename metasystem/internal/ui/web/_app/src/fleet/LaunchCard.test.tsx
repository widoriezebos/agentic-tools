import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Launch } from "./api";
import { LaunchCard } from "./LaunchCard";

/**
 * The card in each of its states, as markup.
 *
 * What is asserted is what only this component can be wrong about: that the
 * steps are drawn in the verb's own order with the verb's own outcomes, that
 * a failed step's sentence reaches the screen VERBATIM rather than translated
 * into something this page made up, that an armed launch reads as armed and
 * not as failed, and that a machine that joined folds to one line instead of
 * repeating the row below it.
 */

const now = new Date("2026-09-25T11:00:00Z");

const fence =
  "go-build: refused: the gate fence holds this checkout; run go run ./cmd/devgate static and land the red it names before building here";

function launch(over: Partial<Launch> = {}): Launch {
  return {
    schemaVersion: 1,
    launch: "01M3BQAVYXE2AT6F0JG9YB64PG",
    machine: "m1f",
    destination: "/w/agentic-tools-m1f",
    clonedCommit: "b50abb9",
    builtStamp: "b50abb9",
    process: { pid: 40912, startedAt: 1764000000 },
    startedAt: "2026-09-25T10:57:00Z",
    endedAt: null,
    outcome: "running",
    created: { destination: true, nickname: false, evidenceRoot: true },
    steps: [
      { step: "clone", outcome: "done", at: "2026-09-25T10:57:10Z", words: "" },
      { step: "tracking", outcome: "done", at: "2026-09-25T10:57:40Z", words: "" },
      { step: "engine", outcome: "pending", at: "2026-09-25T10:57:41Z", words: "" },
    ],
    orientation: "",
    discardedAt: null,
    destinationPresent: false,
    next: {
      session: "cd /w/agentic-tools-m1f && claude",
      stop: "metasystem system stop --repo /w/agentic-tools-m1f/metasystem",
    },
    ...over,
  };
}

function rendered(record: Launch, joined = false): string {
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <LaunchCard
        record={record}
        joined={joined}
        now={now}
        onStarted={() => {
          // nothing: this render never acts
        }}
        onDiscarded={() => {
          // nothing: this render never acts
        }}
      />
    </TooltipPrimitive.Provider>,
  );
}

describe("a launch in flight", () => {
  it("lists the verb's steps in the verb's order, with its outcomes", () => {
    const markup = rendered(launch());
    expect(markup).toContain("Launching m1f");
    expect(markup).toContain("/w/agentic-tools-m1f");
    expect(markup.indexOf("clone")).toBeLessThan(markup.indexOf("tracking"));
    expect(markup.indexOf("tracking")).toBeLessThan(markup.indexOf("engine"));
    expect(markup).toContain("ms-launch-dot--done");
    expect(markup).toContain("ms-launch-dot--pending");
  });

  it("names the idle alerts a sessionless machine raises, beside the two commands", () => {
    const markup = rendered(launch());
    expect(markup).toContain("raises an idle alert");
    expect(markup).toContain("cd /w/agentic-tools-m1f &amp;&amp; claude");
    expect(markup).toContain("metasystem system stop --repo /w/agentic-tools-m1f/metasystem");
  });
});

describe("a launch that stopped", () => {
  const stopped = launch({
    outcome: "failed",
    endedAt: "2026-09-25T10:58:00Z",
    steps: [
      { step: "clone", outcome: "done", at: "2026-09-25T10:57:10Z", words: "" },
      { step: "engine", outcome: "failed", at: "2026-09-25T10:58:00Z", words: fence },
    ],
  });

  it("shows the failed step's own words, whole", () => {
    const markup = rendered(stopped);
    expect(markup).toContain("Launching m1f stopped");
    expect(markup).toContain("go-build: refused: the gate fence holds this checkout");
    expect(markup).toContain("go run ./cmd/devgate static");
  });

  // g1-s72: signed in is enough, so Retry is one press with nothing to fill in.
  it("offers Retry as one press, with nothing to fill in", () => {
    const markup = rendered(stopped);
    const retry = /<button[^>]*>Retry<\/button>/u.exec(markup);
    expect(retry).not.toBe(null);
    expect(retry?.[0]).not.toContain("disabled");
    const form = /<div class="ms-launch-retry">(.*)$/su.exec(markup)?.[1] ?? "";
    expect(form).not.toBe("");
    expect(form).not.toContain("<textarea");
    expect(form).not.toContain("<input");
    expect(markup).not.toContain("authorization");
    expect(markup).not.toContain("Review by");
  });

  it("removes nothing, and names the clone while it is still on disk", () => {
    const markup = rendered({ ...stopped, destinationPresent: true });
    expect(markup).not.toContain(">Remove<");
    expect(markup).toContain("The clone at /w/agentic-tools-m1f stays on disk; delete it yourself.");
    // No verb is named that this engine does not have.
    expect(markup).not.toContain("machine remove");
    expect(markup).not.toContain("is a directory you delete");
  });

  it("says nothing about a clone that is gone", () => {
    const markup = rendered({ ...stopped, destinationPresent: false });
    expect(markup).not.toContain("stays on disk");
    expect(markup).not.toContain("is a directory you delete");
  });

  it("offers Discard launch beside Retry, always enabled", () => {
    const markup = rendered(stopped);
    const discard = /<button[^>]*>Discard launch<\/button>/u.exec(markup);
    expect(discard).not.toBe(null);
    expect(discard?.[0]).not.toContain("disabled");
    // Beside Retry: in the same row of actions, after it.
    const actions = /<div class="ms-launch-actions">(.*?)<\/div>/su.exec(markup)?.[1] ?? "";
    expect(actions.indexOf("Retry")).toBeGreaterThan(-1);
    expect(actions.indexOf("Discard launch")).toBeGreaterThan(actions.indexOf("Retry"));
  });
});

describe("a launch that armed", () => {
  it("reads as armed rather than failed, with the health command", () => {
    const markup = rendered(
      launch({
        outcome: "armed",
        endedAt: "2026-09-25T10:59:00Z",
        steps: [
          { step: "supervision", outcome: "done", at: "2026-09-25T10:58:30Z", words: "" },
          {
            step: "presence",
            outcome: "armed",
            at: "2026-09-25T10:59:00Z",
            words: "armed; presence not confirmed within three ticks: read /w/agentic-tools-m1f's health",
          },
        ],
      }),
    );
    expect(markup).toContain("m1f armed; presence not yet seen");
    expect(markup).toContain("metasystem system check --repo /w/agentic-tools-m1f/metasystem");
    expect(markup).not.toContain("Retry");
  });
});

describe("a machine that joined", () => {
  it("folds to one line once its row is in the table", () => {
    const markup = rendered(launch({ outcome: "done", endedAt: "2026-09-25T10:58:00Z" }), true);
    expect(markup).toContain("m1f joined 2 min ago");
    // g1-s72: no review clause, and no claim about how it was enrolled.
    expect(markup).toContain(">m1f joined 2 min ago</p>");
    expect(markup).not.toContain("review due");
    expect(markup).not.toContain("enroll");
    // The steps are the table row's business now, not the card's.
    expect(markup).not.toContain("ms-launch-steps");
    expect(markup).toContain(">Dismiss</button>");
  });

  // SOL-DL-02: a machine can join while its launch is still writing its last
  // steps. Its card stays whole until the record has stopped changing, so no
  // Dismiss is offered that the server would refuse.
  it("keeps a still-running launch whole, with no Dismiss", () => {
    const markup = rendered(launch({ outcome: "running" }), true);
    expect(markup).toContain("ms-launch-steps");
    expect(markup).not.toContain(">Dismiss</button>");
  });
});

// g1-s72 D5: no card, in any state, names a review or a temporary enrollment,
// and none claims a session enrollment either: the card does not read one.
describe("no card names a review", () => {
  for (const outcome of ["starting", "running", "failed", "armed", "done"]) {
    for (const joined of [false, true]) {
      it(`${outcome}${joined ? ", joined" : ""}`, () => {
        const markup = rendered(launch({ outcome, endedAt: "2026-09-25T10:58:00Z" }), joined);
        expect(markup).not.toMatch(/review/iu);
        expect(markup).not.toMatch(/temporary/iu);
        expect(markup).not.toMatch(/signed-in browser session/iu);
      });
    }
  }
});
