import { describe, expect, it } from "vitest";

import { readNonce } from "./nonce";

/** A stand-in for the document: Vitest runs in the node environment. */
function rootWith(meta: { nonce?: string } | null): ParentNode {
  return { querySelector: () => meta } as unknown as ParentNode;
}

describe("readNonce", () => {
  it("returns the meta element's nonce property", () => {
    expect(readNonce(rootWith({ nonce: "cnVubmluZ0Egbm9uY2U" }))).toBe("cnVubmluZ0Egbm9uY2U");
  });

  it("returns the empty string when the page carries no nonce meta", () => {
    expect(readNonce(rootWith(null))).toBe("");
  });

  it("returns the empty string when the property is absent", () => {
    expect(readNonce(rootWith({}))).toBe("");
  });
});
