import { describe, expect, it } from "vitest";

import { actingAs, SIGN_IN, UNPROVEN } from "./acting";
import type { Authority } from "./api";
import type { SessionStatus } from "../shell/session";

/**
 * Whether the board may act, and as whom. Two hands can answer, and the page
 * asks both before it says no.
 */

const agentStarted: Authority = {
  proven: false,
  human: "",
  reason: "the interface was started by an agent process (claude-code)",
};

const terminalStarted: Authority = { proven: true, human: "Wido", reason: "" };

function known(human: string, signedIn: boolean): SessionStatus {
  return { state: "known", session: { human, signedIn, until: "", source: signedIn ? "code" : "none" } };
}

describe("who the board acts as", () => {
  it("is the signed-in browser, even on a server nothing else proves", () => {
    expect(actingAs(agentStarted, known("Wido", true))).toEqual({ proven: true, human: "Wido", reason: "" });
  });

  it("says both reasons, and the remedy that needs no terminal, when nobody is acting", () => {
    const acting = actingAs(agentStarted, known("", false));
    expect(acting.proven).toBe(false);
    expect(acting.reason).toBe(`${agentStarted.reason}; ${SIGN_IN}`);
  });

  it("supplies a reason of its own where the server gave none", () => {
    const acting = actingAs({ proven: false, human: "", reason: "" }, known("", false));
    expect(acting.reason).toBe(`${UNPROVEN}; ${SIGN_IN}`);
  });

  it("keeps the boot proof while the session is still being read", () => {
    expect(actingAs(terminalStarted, { state: "loading" })).toEqual(terminalStarted);
    expect(actingAs(agentStarted, { state: "loading" })).toEqual(agentStarted);
    expect(actingAs(terminalStarted, { state: "failed", message: "unreachable" })).toEqual(terminalStarted);
  });

  it("refuses a terminal-proven server once the session says nobody is acting", () => {
    // The session route answers "terminal" on a server the terminal proved,
    // so a not-signed-in answer beside a proven boot proof cannot happen —
    // and if it did, what the session says now is what is true now.
    expect(actingAs(terminalStarted, known("Wido", false)).proven).toBe(false);
  });
});
