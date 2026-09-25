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
  "go-build: refused: the gate fence holds this checkout; run scripts/agents/go-gate.sh --fast and land the red it names before building here";

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
    reviewBy: "2026-10-02",
    created: { destination: true, nickname: false, evidenceRoot: true },
    steps: [
      { step: "clone", outcome: "done", at: "2026-09-25T10:57:10Z", words: "" },
      { step: "tracking", outcome: "done", at: "2026-09-25T10:57:40Z", words: "" },
      { step: "engine", outcome: "pending", at: "2026-09-25T10:57:41Z", words: "" },
    ],
    orientation: "",
    next: {
      session: "cd /w/agentic-tools-m1f && claude",
      stop: "metasystem stop --repo /w/agentic-tools-m1f/metasystem",
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
        onDismiss={() => {
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
    expect(markup).toContain("metasystem stop --repo /w/agentic-tools-m1f/metasystem");
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
    expect(markup).toContain("go-gate.sh --fast");
  });

  it("offers Retry, and asks for the authorization again", () => {
    const markup = rendered(stopped);
    expect(markup).toContain("Retry");
    expect(markup).toContain("Your authorization, again");
    expect(markup).toContain("The record never held it");
  });

  it("removes nothing, and says what the directory is", () => {
    const markup = rendered(stopped);
    expect(markup).not.toContain(">Remove<");
    expect(markup).toContain("is a directory you delete");
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
    expect(markup).toContain("metasystem steward health --repo /w/agentic-tools-m1f/metasystem");
    expect(markup).not.toContain("Retry");
  });
});

describe("a machine that joined", () => {
  it("folds to one line once its row is in the table", () => {
    const markup = rendered(launch({ outcome: "done", endedAt: "2026-09-25T10:58:00Z" }), true);
    expect(markup).toContain("m1f joined 2 min ago");
    expect(markup).toContain("temporary enrollment, review due");
    // The steps are the table row's business now, not the card's.
    expect(markup).not.toContain("ms-launch-steps");
  });
});
