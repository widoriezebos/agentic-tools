import { describe, expect, it } from "vitest";

import { blockedForSignIn, controlFor, isCode, needsAHandle, type SessionStatus } from "./session";

/**
 * The identity control, and the one rule the sign-in sheet has of its own.
 *
 * Both are read from the server's answer and from nothing else: there is no
 * state in this browser about who is signed in, so there is nothing here that
 * could disagree with the server.
 */

function known(
  human: string,
  signedIn: boolean,
  source: "code" | "terminal" | "none",
  until = "",
): SessionStatus {
  return { state: "known", session: { human, signedIn, until, source } };
}

describe("the identity control", () => {
  it("shows nothing but a shape while the session is being read", () => {
    expect(controlFor({ state: "loading" })).toEqual({ kind: "loading" });
  });

  it("offers a sign-in when nobody is acting", () => {
    expect(controlFor(known("", false, "none"))).toEqual({ kind: "sign-in" });
    expect(controlFor(known("Wido", false, "none"))).toEqual({ kind: "sign-in" });
  });

  it("offers a sign-in when the session could not be read at all", () => {
    expect(controlFor({ state: "failed", message: "/api/session answered 500" })).toEqual({ kind: "sign-in" });
  });

  it("names the handle and when the browser session stops, with a way out of it", () => {
    const at = new Date(2026, 8, 22, 9, 12, 0);
    const control = controlFor(known("Wido", true, "code", at.toISOString()));
    expect(control).toEqual({ kind: "signed-in", who: "Wido", qualifier: "until 09:12", canSignOut: true });
  });

  it("says an unreadable expiry is unknown rather than showing an epoch", () => {
    expect(controlFor(known("Wido", true, "code", "not an instant"))).toEqual({
      kind: "signed-in",
      who: "Wido",
      qualifier: "until unknown",
      canSignOut: true,
    });
  });

  it("names the terminal that started the server, and offers no sign-out of it", () => {
    expect(controlFor(known("Wido", true, "terminal"))).toEqual({
      kind: "signed-in",
      who: "Wido",
      qualifier: "terminal",
      canSignOut: false,
    });
  });
});

describe("the sheet's name field", () => {
  it("is asked for only where the server named nobody", () => {
    expect(needsAHandle(known("", false, "none"))).toBe(true);
    expect(needsAHandle(known("Wido", false, "none"))).toBe(false);
    expect(needsAHandle(known("Wido", true, "terminal"))).toBe(false);
  });

  it("is not asked for while nothing has been read", () => {
    expect(needsAHandle({ state: "loading" })).toBe(false);
    expect(needsAHandle({ state: "failed", message: "unreachable" })).toBe(false);
  });

  it("holds the act until the name is one word and the code is six digits", () => {
    expect(blockedForSignIn("123456", "", false)).toBe("");
    expect(blockedForSignIn("12345", "", false)).toBe("A one-time code is six digits.");
    expect(blockedForSignIn("12345a", "", false)).toBe("A one-time code is six digits.");
    expect(blockedForSignIn("123456", "", true)).toContain("does not know who you are");
    expect(blockedForSignIn("123456", "Wido Riezebos", true)).toContain("one word");
    expect(blockedForSignIn("123456", "Wido", true)).toBe("");
  });
});

describe("what counts as a code", () => {
  it("is six digits, trimmed, and nothing else", () => {
    expect(isCode(" 123456 ")).toBe(true);
    expect(isCode("1234567")).toBe(false);
    expect(isCode("")).toBe(false);
    expect(isCode("12 456")).toBe(false);
  });
});
