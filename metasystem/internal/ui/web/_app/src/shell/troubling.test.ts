import { describe, expect, it } from "vitest";

import {
  ASK_WHAT_HAPPENED,
  boundaryText,
  chipLine,
  held,
  MAX_TROUBLE_TEXT,
  pendingIn,
  released,
  scrub,
  sendChoice,
  TROUBLE_REQUEST,
  troubleOf,
  waitingLine,
  WITHHELD,
  type Pending,
  type Trouble,
} from "./troubling";

/**
 * Asking what happened (g1-s68): what a trouble carries, what it never
 * carries, and whose a pending one is.
 */

const LAND: Trouble = {
  text: "work land is refused: goal/backlog-ordered-by-priority has moved past the tip the review named (REVIEW_STALE)",
  code: "REVIEW_STALE",
  where: { section: "Backlog", path: "/backlog", subject: "backlog-ordered-by-priority", kind: "goal" },
  act: { verb: "Land", object: "goal", target: "backlog-ordered-by-priority" },
  at: "2026-09-28T19:41:00.000Z",
};

function pending(id: string, conversation: string, trouble: Trouble = LAND): Pending {
  return { id, origin: `line-${id}`, key: `key-${id}`, conversation, trouble, label: waitingLine(conversation, "") };
}

describe("a trouble", () => {
  it("asks one fixed sentence, the server's own", () => {
    expect(TROUBLE_REQUEST).toBe("What just happened here, why, and how do I recover?");
    expect(ASK_WHAT_HAPPENED).toBe("Ask what happened");
  });

  it("carries the sentence, the code, where, the act as words, when, the tip and the sign-in remedy", () => {
    const made = troubleOf(
      { text: LAND.text, code: "REVIEW_STALE", where: LAND.where, act: LAND.act, at: LAND.at, tip: "7ed3baf" },
      [],
    );
    expect(made).toEqual({ ...LAND, tip: "7ed3baf" });
    expect(Object.keys(made).sort()).toEqual(["act", "at", "code", "text", "tip", "where"]);
  });

  it("is scrubbed of every secret the page holds before it is kept or sent", () => {
    const code = "482913";
    const refused = troubleOf(
      { text: `the code ${code} was not accepted; ${code} is spent`, where: { section: "Backlog", path: "/backlog" }, at: LAND.at, signIn: true },
      [code, "", "  "],
    );
    expect(refused.text).toBe(`the code ${WITHHELD} was not accepted; ${WITHHELD} is spent`);
    expect(JSON.stringify(refused)).not.toContain(code);
    expect(refused.signIn).toBe(true);
    expect(refused.code).toBeUndefined();
    // The longer secret goes first, so one inside another is not half-kept.
    expect(scrub("token abc123 and abc", ["abc", "abc123"])).toBe(`token ${WITHHELD} and ${WITHHELD}`);
  });

  it("is bounded to the sentence the server keeps", () => {
    const long = troubleOf({ text: "é".repeat(MAX_TROUBLE_TEXT + 50), where: LAND.where, at: LAND.at }, []);
    expect([...long.text].length).toBe(MAX_TROUBLE_TEXT);
    expect(long.text.endsWith("…")).toBe(true);
  });

  it("says the act or the pane, the code and the time on its chip", () => {
    const at = new Date(LAND.at);
    const clock = `${String(at.getHours()).padStart(2, "0")}:${String(at.getMinutes()).padStart(2, "0")}`;
    expect(chipLine(LAND)).toBe(`Land · backlog-ordered-by-priority · REVIEW_STALE · ${clock}`);
    expect(chipLine({ text: "The backlog could not be read", where: { section: "Backlog", path: "/backlog" }, at: LAND.at }))
      .toBe(`Backlog · ${clock}`);
  });

  it("from a pane that threw carries the error's name", () => {
    expect(boundaryText(new TypeError("cannot read properties of undefined (reading 'lane')")))
      .toBe("This pane could not be rendered: TypeError: cannot read properties of undefined (reading 'lane')");
  });
});

describe("a pending trouble", () => {
  it("says which conversation it waits for", () => {
    expect(waitingLine("plans/reviews/review-of-backlog-ordered-by-priority.md", "backlog-ordered-by-priority"))
      .toBe("waiting for the room on backlog-ordered-by-priority");
    expect(waitingLine("plans/reviews/review-of-x.md", "")).toBe("waiting for the room on x");
    expect(waitingLine("plans/designs/sessions.md", "")).toBe("waiting for the room on sessions");
    expect(waitingLine("", "")).toBe("waiting for your conversation");
  });

  // pending_trouble_stays_in_origin_room: Ask while room A is busy, switch to
  // room B, B cannot send A's pending trouble, back in A it sends with the
  // draft and attachments intact.
  it("pending_trouble_stays_in_origin_room", () => {
    const roomA = "plans/reviews/review-of-a.md";
    const roomB = "plans/reviews/review-of-b.md";
    let list: readonly Pending[] = [];
    // A press while A is busy is held, never refused.
    expect(sendChoice(list, roomA, "half a question", { busy: true, sending: false })).toEqual({ kind: "none" });
    list = held(list, pending("1", roomA));
    expect(pendingIn(list, roomA)?.id).toBe("1");
    // In B, the pending trouble is not B's: its Send sends B's draft, and A's stays.
    expect(pendingIn(list, roomB)).toBeNull();
    expect(sendChoice(list, roomB, "a question in B", { busy: false, sending: false })).toEqual({ kind: "draft" });
    expect(sendChoice(list, roomB, "", { busy: false, sending: false })).toEqual({ kind: "none" });
    expect(list.map((entry) => entry.id)).toEqual(["1"]);
    // Back in A, idle: the next Send sends the trouble first, and the draft is not what is sent.
    const choice = sendChoice(list, roomA, "half a question", { busy: false, sending: false });
    expect(choice).toEqual({ kind: "trouble", pending: list[0] });
    // Accepted, it is released, and the draft is then what the next Send sends.
    list = released(list, "1");
    expect(sendChoice(list, roomA, "half a question", { busy: false, sending: false })).toEqual({ kind: "draft" });
  });

  it("is held once per press, and a press again replaces itself", () => {
    let list = held([], pending("1", ""));
    list = held(list, { ...pending("1", ""), label: "again" });
    expect(list).toHaveLength(1);
    expect(list[0].label).toBe("again");
    list = held(list, pending("2", ""));
    expect(pendingIn(list, "")?.id).toBe("1");
  });
});
