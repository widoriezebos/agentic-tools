import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Two rules of the drawer's own Dismiss, asserted over the store's source.
 *
 * They live inside the provider's callbacks, and a press is the only thing that
 * reaches one: this suite mounts nothing and runs no effect, so the rules are read
 * out of the file the way `shell/refresh.test.ts` reads what each pane offers as
 * its re-read. What a guard like this proves is narrower than a driven run — it
 * reads the handler rather than watching it answer — so each assertion names the
 * fact it stands for, and a handler that stopped answering one of the three
 * things the route can return fails here with that fact named.
 *
 * The rules themselves are the ones Astra's C-06 and C-07 were about. A dismissal
 * writes `dismissed` onto every line that can still be moved out of where it is,
 * and those writes can be refused. A write the conversation could not make means
 * what the human pressed did NOT happen, so the card comes back with the failure's
 * words on it rather than folding over a proposal that is still waiting; a write
 * the server refused with the entry is somebody else having moved that line first,
 * which the card shows exactly as the inbox does. And when the writes have
 * answered, the page behind the drawer is asked to read again, once, so a mounted
 * Decisions page drops the row it no longer has.
 */

const SOURCE = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "store.tsx"), "utf8");

/**
 * The body of `const <name> = …` in the store, from its first `{` to the brace
 * that closes it — the declaration's own text and nothing after it, so a later
 * callback is never read as this one's.
 */
function bodyOf(name: string): string {
  const at = SOURCE.indexOf(`const ${name} = `);
  if (at < 0) {
    return "";
  }
  const opens = SOURCE.indexOf("{", at);
  if (opens < 0) {
    return "";
  }
  let depth = 0;
  for (let index = opens; index < SOURCE.length; index += 1) {
    if (SOURCE[index] === "{") {
      depth += 1;
    } else if (SOURCE[index] === "}") {
      depth -= 1;
      if (depth === 0) {
        return SOURCE.slice(opens, index + 1);
      }
    }
  }
  return SOURCE.slice(opens);
}

/** Every occurrence of one string in a body, for a rule about how many. */
function times(body: string, said: string): number {
  return body.split(said).length - 1;
}

describe("what the drawer's dismissal does with what its writes answer", () => {
  it("shows the entry a conflict returned, as the inbox does", () => {
    const body = bodyOf("writeState");
    expect(body).not.toBe("");
    // Written, or refused with the entry somebody else left: both are the line as
    // the route now holds it, and the card shows it either way. Only a failure —
    // no entry at all — is left to the caller.
    expect(body).toContain('answered.kind !== "failed"');
    expect(body).toContain("proposalMoved");
    // And the answer is handed back, because the press has something to do with it.
    expect(body).toContain("return answered");
  });

  it("unfolds the card with the failure's own words where a write failed", () => {
    const body = bodyOf("dismissProposals");
    expect(body).not.toBe("");
    expect(body).toContain('answered.kind === "failed"');
    // The words go on the line, as the inbox's own dismissal puts them there.
    expect(body).toContain("refusedUnsent: answered.words");
    // And the card is open again: folding it was this human's act, and it did not
    // happen.
    expect(times(body, "setDismissedCards")).toBe(2);
  });

  it("asks for the offered re-read once, after the writes have answered", () => {
    const body = bodyOf("dismissProposals");
    expect(body).toContain("await Promise.all");
    expect(times(body, "askTheReread")).toBe(1);
  });
});

/**
 * The answer a page holds for want of a record is held in the STORE, and the
 * store is what clears it (Astra C-04).
 *
 * The drawer and the inbox read one mark, because they are two readings of one
 * record. A surface that has just recorded or reconciled a newer outcome must
 * therefore be able to drop the old answer where both of them read it; the inbox
 * cleared only its own copy, and went on importing the drawer's obsolete refusal
 * over an entry the record had applied. This is read out of the source for the
 * reason the guards above are: a press is the only thing that reaches the
 * callback.
 */
describe("the answer the store holds for want of a record", () => {
  it("can be cleared by the surface that recorded a newer outcome", () => {
    const body = bodyOf("clearProposalMark");
    expect(body).not.toBe("");
    // The shared mark, through the same writer every other mark goes through.
    expect(body).toContain("mark(id,");
    expect(body).toContain("unrecorded: null");
  });
});
