import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TroubleProvider, useTroubles } from "./troubles";
import { chipLine, pendingIn, sendChoice, waitingLine, type Pending, type Trouble } from "./troubling";
import { PENDING_TROUBLES_KEY, readPendingTroubles, writePendingTroubles, type Store } from "../storage";

/**
 * A waiting question survives a reload (goal ask-what-happened-follow-ups):
 * a press made while its conversation answered is kept where the reloaded
 * page finds it, and is there again, in its own conversation, ready to send.
 */

/** A store that answers from a map, the way a browser's would, and survives the "reload". */
function store(values: Record<string, string> = {}): Store & { raw: Map<string, string> } {
  const raw = new Map(Object.entries(values));
  return {
    raw,
    getItem: (key) => raw.get(key) ?? null,
    setItem: (key, value) => {
      raw.set(key, value);
    },
  };
}

const ROOM = "plans/reviews/review-of-backlog-ordered-by-priority.md";

const LAND: Trouble = {
  text: "work land is refused: the review is stale (REVIEW_STALE)",
  code: "REVIEW_STALE",
  where: { section: "Review", path: "/review/x", subject: "backlog-ordered-by-priority", kind: "goal" },
  act: { verb: "Land", object: "goal", target: "backlog-ordered-by-priority" },
  at: "2026-09-28T19:41:00.000Z",
};

const WAITING: Pending = {
  id: "«r7»",
  origin: "«r7»",
  key: "0b8c3f1e-turn-key",
  conversation: ROOM,
  trouble: LAND,
  label: waitingLine(ROOM, ""),
};

/** What the page reads of the context once it has loaded: the waiting question and what Send would do. */
function Probe({ conversation }: { conversation: string }) {
  const { pending } = useTroubles();
  const waiting = pendingIn(pending, conversation);
  const choice = sendChoice(pending, conversation, "half a question", { busy: false, sending: false });
  return (
    <p>
      {waiting === null ? "nothing waits" : `waits: ${chipLine(waiting.trouble)}`} | send: {choice.kind}
    </p>
  );
}

describe("a waiting question", () => {
  it("is still there after a reload, in its own conversation, ready to send", () => {
    const browser = store();
    writePendingTroubles([WAITING], browser);
    // The reload: a fresh provider over the same browser store.
    const markup = renderToStaticMarkup(
      <TroubleProvider store={browser}>
        <Probe conversation={ROOM} />
      </TroubleProvider>,
    );
    expect(markup).toContain(`waits: ${chipLine(LAND)}`);
    expect(markup).toContain("send: trouble");
    // Another conversation is not given it.
    const elsewhere = renderToStaticMarkup(
      <TroubleProvider store={browser}>
        <Probe conversation="" />
      </TroubleProvider>,
    );
    expect(elsewhere).toContain("nothing waits");
  });

  it("keeps its turn key, so a send after the reload is the same turn, and lets go of the line it was pressed on", () => {
    const browser = store();
    writePendingTroubles([WAITING], browser);
    const [back] = readPendingTroubles(browser);
    expect(back.key).toBe(WAITING.key);
    expect(back.conversation).toBe(ROOM);
    expect(back.trouble).toEqual(LAND);
    expect(back.label).toBe(WAITING.label);
    // The line's id was this page's; a line on the reloaded page may be given
    // the same one and must not take this question for its own.
    expect(back.origin).toBe("");
    expect(back.id).not.toBe(WAITING.id);
  });

  it("is let go of when it is sent or taken back", () => {
    const browser = store();
    writePendingTroubles([WAITING], browser);
    writePendingTroubles([], browser);
    expect(readPendingTroubles(browser)).toEqual([]);
    expect(browser.raw.get(PENDING_TROUBLES_KEY)).toBe("");
  });

  it("reads as none from a store that holds something else or throws", () => {
    expect(readPendingTroubles(store({ [PENDING_TROUBLES_KEY]: "not json" }))).toEqual([]);
    expect(readPendingTroubles(store({ [PENDING_TROUBLES_KEY]: JSON.stringify([{ id: 1 }, "x", null]) }))).toEqual([]);
    const throwing: Store = {
      getItem: () => {
        throw new Error("site data is blocked");
      },
      setItem: () => {
        throw new Error("site data is blocked");
      },
    };
    expect(readPendingTroubles(throwing)).toEqual([]);
    expect(() => {
      writePendingTroubles([WAITING], throwing);
    }).not.toThrow();
  });
});

// SOL-AF-01: one tab's withdrawal is not undone by another tab. The waiting
// questions are the tab's own: two tabs share the browser's lasting store but
// each has a store of its own, and only that one may hold them.
describe("a waiting question in two tabs", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("withdrawn in tab A, is not offered again by A's reload after tab B adds another", () => {
    const shared = store();
    const tabA = store();
    const tabB = store();
    const inTab = (tab: Store) => {
      vi.stubGlobal("localStorage", shared);
      vi.stubGlobal("sessionStorage", tab);
    };
    const P = WAITING;
    const Q: Pending = { ...WAITING, id: "«r9»", origin: "«r9»", key: "q-turn-key", trouble: { ...LAND, text: "the backlog could not be read" } };

    inTab(tabA);
    writePendingTroubles([P]);
    inTab(tabB);
    writePendingTroubles([P]);
    // A takes P back; B, still holding P, then adds Q.
    inTab(tabA);
    writePendingTroubles([]);
    inTab(tabB);
    writePendingTroubles([P, Q]);
    // A reloads.
    inTab(tabA);
    expect(readPendingTroubles().map((entry) => entry.key)).toEqual([]);
    const markup = renderToStaticMarkup(
      <TroubleProvider>
        <Probe conversation={ROOM} />
      </TroubleProvider>,
    );
    expect(markup).toContain("nothing waits");
    // B's reload still finds both of its own.
    inTab(tabB);
    expect(readPendingTroubles().map((entry) => entry.key)).toEqual([P.key, Q.key]);
  });
});
