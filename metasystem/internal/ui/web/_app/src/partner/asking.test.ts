import { describe, expect, it } from "vitest";

import { keyFor, mintKey } from "./asking";

/**
 * The key one send is asked under. What a question carries is capture.test.ts's.
 */

describe("the turn key", () => {
  it("is minted once and kept while nothing has been accepted", () => {
    let minted = 0;
    const mint = () => {
      minted += 1;
      return `k-${String(minted)}`;
    };
    const first = keyFor("", mint);
    expect(first).toBe("k-1");
    // The send was refused, so the page still holds the key: pressing Send
    // again is this turn again rather than a second one.
    expect(keyFor(first, mint)).toBe("k-1");
    expect(minted).toBe(1);
    // Accepted, so the page lets it go and the next question mints its own.
    expect(keyFor("", mint)).toBe("k-2");
  });

  it("is different every time it is minted", () => {
    const keys = new Set([mintKey(), mintKey(), mintKey()]);
    expect(keys.size).toBe(3);
    for (const key of keys) {
      expect(key.length).toBeGreaterThan(8);
    }
  });
});
