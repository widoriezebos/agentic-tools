import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { clockOf, LiveLine, tallyOf } from "./LiveLine";
import type { Look, PartnerEvent, Snapshot } from "../partner/api";
import { asked, emptyStore, loaded, nothingRunning, received, type Live, type Store } from "../partner/conversation";
import { PartnerAs } from "../partner/store";
import { Transcript } from "../partner/Transcript";

/**
 * The live line (g1-s74 D1): the dot, the words, the tally and the clock,
 * rendered by one component wherever a turn is running.
 *
 * It is read from the markup, because the claims are about what a human and
 * a screen reader are given: which words, which count, and which parts are
 * spoken at all.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "../..");

const PAGE: Look = { what: "the Backlog page", outcome: "read", page: true };
const READ: Look = { what: "Read plans/goals/backlog.md", outcome: "read" };
const PART: Look = { what: "Read plans/designs/a.md", outcome: "partial" };
const FAILED: Look = { what: "Read plans/missing.md", outcome: "failed" };

function live(over: Partial<Live> = {}): Live {
  return { ...nothingRunning, turn: "t1", seq: 1, ...over };
}

function line(running: Live): string {
  return renderToStaticMarkup(<LiveLine live={running} />);
}

/** The words span's own text, which is what a screen reader hears. */
function spoken(markup: string): string {
  const unclocked = markup.replace(/<span class="ms-live-clock"[^>]*>.*?<\/span>/, "");
  const match = /<span class="ms-live-words" role="status" aria-live="polite">(.*)<\/span><\/span>$/.exec(unclocked);
  return (match?.[1] ?? "").replace(/<[^>]+>/g, "");
}

describe("the live line's words", () => {
  it("says Thinking with the clock when the turn is doing nothing named", () => {
    const markup = line(live({ startedAt: new Date().toISOString() }));
    expect(spoken(markup)).toBe("Thinking");
    expect(markup).not.toContain("Thinking…");
    expect(markup).toMatch(/class="ms-live-clock"[^>]*> · 0:0\d</);
  });

  it("says the doing line as the stream sent it, and how many things it has read", () => {
    const markup = line(live({ doing: "Read plans/designs/a.md", looked: [PAGE, READ, PART] }));
    expect(spoken(markup)).toBe("Read plans/designs/a.md · 2 read");
  });

  it("names a failed read in the marker class and never counts it", () => {
    const markup = line(live({ looked: [PAGE, READ, FAILED] }));
    expect(spoken(markup)).toBe("Thinking · 1 read, 1 failed");
    expect(markup).toContain('<span class="ms-live-failed">, 1 failed</span>');
  });

  it("does not count the page the human was looking at", () => {
    expect(spoken(line(live({ looked: [PAGE] })))).toBe("Thinking");
    expect(tallyOf([PAGE])).toBeNull();
    expect(tallyOf([PAGE, READ, PART, FAILED])).toEqual({ read: 2, failed: 1 });
  });

  it("says Thinking after a completed call when nothing else is in flight", () => {
    // The host retires a finished call's line with a doing beat of its own.
    let store: Store = asked(loaded(emptyStore, SNAPSHOT), "t1", "k1", "q", { section: "Backlog", path: "/backlog" }, "");
    store = received(store, beat(1, "doing", "Read plans/goals/backlog.md"));
    store = received(store, beat(2, "doing", ""));
    store = received(store, { ...beat(3, "look"), look: READ });
    expect(spoken(line(store.live))).toBe("Thinking · 1 read");
  });

  it("speaks the words and the tally, and hides the dot and the clock", () => {
    const markup = line(live({ startedAt: new Date().toISOString() }));
    expect(markup).toContain('<span class="ms-live-dot" aria-hidden="true"></span>');
    expect(markup).toContain('role="status" aria-live="polite"');
    expect(markup).toMatch(/<span class="ms-live-clock" aria-hidden="true">/);
  });

  it("shows no clock before the server's start is known", () => {
    expect(line(live())).not.toContain("ms-live-clock");
  });
});

describe("the clock", () => {
  const start = "2026-09-29T10:00:00Z";
  const after = (seconds: number) => Date.parse(start) + seconds * 1000;

  it("counts minutes and seconds since the server admitted the turn", () => {
    expect(clockOf(start, after(3))).toBe("0:03");
    expect(clockOf(start, after(65))).toBe("1:05");
    expect(clockOf(start, after(59 * 60 + 59))).toBe("59:59");
  });

  it("counts hours past an hour", () => {
    expect(clockOf(start, after(3600))).toBe("1:00:00");
    expect(clockOf(start, after(3600 + 61))).toBe("1:01:01");
  });

  it("never runs backwards on a browser clock behind the server's", () => {
    expect(clockOf(start, after(-5))).toBe("0:00");
  });

  it("says nothing without a start", () => {
    expect(clockOf("", after(3))).toBe("");
    expect(clockOf("not a time", after(3))).toBe("");
  });
});

/** An idle snapshot, as the conversation tests write it. */
const SNAPSHOT: Snapshot = {
  runtime: "claude",
  model: "claude-opus-5-5",
  human: "Wido",
  busy: false,
  turn: "",
  partial: "",
  partialSeq: 0,
  activity: null,
  doing: "",
  looked: null,
  suggestions: null,
  deposits: null,
  proposals: null,
  sitting: null,
  index: null,
  readOnly: "",
  messages: null,
};

function beat(seq: number, kind: PartnerEvent["kind"], text = ""): PartnerEvent {
  return { turn: "t1", seq, kind, text, at: "2026-09-29T10:00:00Z" };
}

describe("the turn's start", () => {
  it("is set by a 202 that arrives after the turn's first beats, which it keeps", () => {
    let store = loaded(emptyStore, SNAPSHOT);
    store = received(store, beat(1, "doing", "Read plans/goals/backlog.md"));
    store = received(store, { ...beat(2, "look"), look: READ });
    store = asked(store, "t1", "k1", "q", { section: "Backlog", path: "/backlog" }, "", {}, "2026-09-29T10:00:00Z");
    expect(store.live.startedAt).toBe("2026-09-29T10:00:00Z");
    expect(store.live.doing).toBe("Read plans/goals/backlog.md");
    expect(store.live.looked).toEqual([READ]);
    expect(store.live.seq).toBe(2);
  });

  it("is filled from a snapshot behind the beats, and nothing else is replaced", () => {
    let store = loaded(emptyStore, SNAPSHOT);
    store = received(store, beat(1, "doing", "Read plans/goals/backlog.md"));
    store = received(store, { ...beat(2, "look"), look: READ });
    store = received(store, beat(3, "text", "Two goals"));
    const behind: Snapshot = {
      ...SNAPSHOT, busy: true, turn: "t1", partial: "", partialSeq: 1, doing: "Read plans/goals/backlog.md",
      startedAt: "2026-09-29T10:00:00Z",
    };
    const after = loaded(store, behind);
    expect(after.live).toEqual({ ...store.live, startedAt: "2026-09-29T10:00:00Z" });
  });

  it("is not moved by a snapshot behind the beats once the page has it", () => {
    let store = loaded(emptyStore, SNAPSHOT);
    store = asked(store, "t1", "k1", "q", { section: "Backlog", path: "/backlog" }, "", {}, "2026-09-29T10:00:00Z");
    store = received(store, beat(1, "text", "Two"));
    const behind: Snapshot = { ...SNAPSHOT, busy: true, turn: "t1", partialSeq: 0, startedAt: "2026-09-29T10:00:09Z" };
    expect(loaded(store, behind).live.startedAt).toBe("2026-09-29T10:00:00Z");
  });

  it("is the snapshot's for a turn the page learns of from a snapshot alone", () => {
    const store = loaded(emptyStore, {
      ...SNAPSHOT, busy: true, turn: "t9", partialSeq: 4, startedAt: "2026-09-29T09:58:00Z",
    });
    expect(store.live.startedAt).toBe("2026-09-29T09:58:00Z");
  });
});

describe("the answer's place", () => {
  function transcript(store: Store): string {
    return renderToStaticMarkup(
      <MemoryRouter>
        <TooltipPrimitive.Provider>
          <PartnerAs held={{ store, busy: store.live.turn !== "" }}>
            <Transcript />
          </PartnerAs>
        </TooltipPrimitive.Provider>
      </MemoryRouter>,
    );
  }

  it("holds the live line while nothing has been said", () => {
    const store: Store = { ...loaded(emptyStore, SNAPSHOT), live: live({ doing: "Read plans/goals/backlog.md" }) };
    const markup = transcript(store);
    expect(markup).toContain("ms-live-dot");
    expect(markup).toContain("Read plans/goals/backlog.md");
    expect(markup).not.toContain("ms-partner-working");
  });

  it("gives the whole place to the first words of the answer", () => {
    const store: Store = { ...loaded(emptyStore, SNAPSHOT), live: live({ text: "Two goals are ready.", looked: [PAGE, READ] }) };
    const markup = transcript(store);
    expect(markup).toContain("Two goals are ready.");
    expect(markup).not.toContain("ms-live");
  });
});

describe("the dot's motion", () => {
  const css = readFileSync(path.join(SRC, "shell", "shell.css"), "utf8");

  /** The body of every rule for one selector, inside a reduced-motion block. */
  function stillRules(): string {
    const found: string[] = [];
    for (const match of css.matchAll(/@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n\}/g)) {
      found.push(match[1]);
    }
    return found.join("\n");
  }

  it("breathes in one 2400 ms ease-out cycle with a halo in its own token", () => {
    expect(css).toMatch(/\.ms-live-dot \{[^}]*animation: ms-live-breath 2400ms ease-out infinite;/);
    expect(css).toMatch(/@keyframes ms-live-breath \{[\s\S]*?var\(--ms-ok-halo\)/);
    expect(css).toMatch(/\.ms-live-dot \{[^}]*background: var\(--ms-ok\);/);
  });

  it("stands still with no halo under reduced motion", () => {
    expect(stillRules()).toMatch(/\.ms-live-dot \{\s*animation: none;\s*box-shadow: none;\s*transform: none;\s*\}/);
  });

  it("leaves the words unpulsed: the old pulse is gone", () => {
    const partner = readFileSync(path.join(SRC, "partner", "partner.css"), "utf8");
    expect(partner).not.toContain("ms-partner-pulse");
    expect(partner).not.toContain("ms-partner-working");
  });
});
