import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { attach, attachedDraft, draftIn, refreshDraft, remove, idFor } from "./attachments";
import { draftOf } from "./drafting";
import { NOWHERE, writingIn } from "./suggesting";

/**
 * Opening a sheet is the hand-over, and what the hand-over must not do.
 *
 * The press that used to be the gate was invisible. A human asked the Partner to
 * rewrite an intent with the edit sheet open and nothing handed over; the service
 * refused the suggestion, because nothing had been; the refusal went to an
 * activity line the drawer does not show; and the Partner said it had put a card
 * on the sheet. So the draft is offered when a sheet with writable fields opens,
 * and the chip above the composer — with its × — is what makes that explicit
 * rather than silent (g1-s52 D1).
 *
 * Three of the claims below cannot be read from rendered markup: effects do not
 * run in a static render, there is no drawer here to open and no caret here to
 * move. They are read where they are written, which is the same thing the sheets'
 * own tests do with their registrations. The rest are the rules themselves, which
 * are functions and are exercised as functions.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");
const ASK_SHEET = readFileSync(path.join(SRC, "partner", "AskSheet.tsx"), "utf8");
const STORE = readFileSync(path.join(SRC, "partner", "store.tsx"), "utf8");

const OPENING = "opening-1";

const EDIT = [
  { name: "Goal", value: "g1-s12" },
  { name: "Intent", value: "The board reads the ledger." },
];

const WRITABLE = ["Intent", "Next step", "Labels"];

function edited(writing = ""): ReturnType<typeof draftOf> {
  return draftOf(OPENING, "Edit goal", EDIT, WRITABLE, writing);
}

describe("the hand-over on open", () => {
  /**
   * The whole of the difference between this and "Ask about this". Ask opens the
   * drawer and takes the caret, by counting up two numbers the shell and the
   * composer watch; the hand-over touches neither, because a human opened a sheet
   * in order to write in it.
   */
  it("attaches the draft and asks for neither the drawer nor the caret", () => {
    const handOver = STORE.slice(STORE.indexOf("const handOver = useCallback"));
    const body = handOver.slice(0, handOver.indexOf("}, ["));
    expect(body).toContain("attach(held, attachedDraft(draft))");
    expect(body).not.toContain("wantComposer");
    // And the act that does want them still does: the two are different acts.
    const askAbout = STORE.slice(STORE.indexOf("const askAbout = useCallback"));
    expect(askAbout.slice(0, askAbout.indexOf("}, ["))).toContain("wantComposer()");
  });

  it("is the sheet's own mount, only for a sheet with writable fields", () => {
    expect(ASK_SHEET).toContain("const writes = writable.length > 0;");
    expect(ASK_SHEET).toContain("handOver(draftOf(id, sheet, fields, writable, \"\"));");
    expect(ASK_SHEET).toContain("if (!writes) {");
    // The fields are read at that moment and not watched: a keystroke must not
    // re-run the hand-over, so they are not among the dependencies.
    expect(ASK_SHEET).toContain("}, [handOver, dropDraft, id, sheet, writes]);");
  });

  it("drops the draft when the sheet goes", () => {
    expect(ASK_SHEET).toContain("dropDraft(sheet);");
    const list = attach([], attachedDraft(edited()));
    expect(draftIn(list)).not.toBeNull();
    expect(draftIn(remove(list, idFor("draft", "Edit goal")))).toBeNull();
  });

  /**
   * The ×, and why bringing the chip up to date cannot undo it.
   *
   * The chip has to say where the caret is, which means the attachment is written
   * again when the caret moves. If that could create one, the × would be undone
   * by moving the caret — so the refresh only ever replaces a draft that is still
   * being offered.
   */
  it("leaves a draft the human took back out of every later question", () => {
    const taken = remove(attach([], attachedDraft(edited())), idFor("draft", "Edit goal"));
    expect(draftIn(taken)).toBeNull();
    expect(refreshDraft(taken, edited("Intent"))).toBe(taken);
    expect(draftIn(refreshDraft(taken, edited("Intent")))).toBeNull();
    // "Ask about this" is the way back, and it attaches as it always did.
    expect(draftIn(attach(taken, attachedDraft(edited("Intent"))))?.writing).toBe("Intent");
  });

  it("says on the chip which field the caret is in, once one holds it", () => {
    const list = attach([], attachedDraft(edited()));
    expect(list[0].label).toContain("writing in nothing yet");
    const moved = refreshDraft(list, edited("Next step"));
    expect(moved[0].label).toContain("writing in Next step");
    expect(draftIn(moved)?.writing).toBe("Next step");
  });
});

describe("the field in hand", () => {
  it("belongs to one opening, and is nothing to another", () => {
    expect(writingIn({ opening: OPENING, field: "Intent" }, OPENING)).toBe("Intent");
    expect(writingIn({ opening: OPENING, field: "Intent" }, "opening-2")).toBe("");
    expect(writingIn(NOWHERE, OPENING)).toBe("");
  });

  /**
   * The last field stands. Blurring one does not put the human nowhere: it leaves
   * them where they were, and a request they then write in the composer is about
   * the field they came from.
   */
  it("is set by focus and left alone by everything else", () => {
    const note = STORE.slice(STORE.indexOf("const noteWriting = useCallback"));
    const body = note.slice(0, note.indexOf("}, ["));
    expect(body).toContain("{ opening, field }");
    // An answer that has not changed changes no state, so clicking about inside
    // one field re-renders nothing.
    expect(body).toContain("held.opening === opening && held.field === field ? held");
    expect(STORE).not.toContain("onBlur");
  });
});

/**
 * What this step does not build.
 *
 * Astra's two material findings were both in the combined press, and both are
 * about telling the truth: this sheet's save reads its draft and its "nothing
 * changed" guard from the render, and guards busy on its own button, so a second
 * caller that first set the words would save the previous delta, or refuse, or
 * report a save it cannot confirm. One press waits for a submission path that
 * takes the next draft explicitly and returns its real outcome.
 */
describe("no press that uses and saves at once", () => {
  it("is nowhere in what this build ships", () => {
    const shipped = sources();
    // The scan asserts its own reach before it asserts anything else.
    expect(shipped).toContain("backlog/EditSheet.tsx");
    expect(shipped).toContain("partner/FieldProposals.tsx");
    const offenders: string[] = [];
    for (const file of shipped) {
      const contents = readFileSync(path.join(SRC, file), "utf8");
      if (/use and save|used and saved/i.test(contents)) {
        offenders.push(file);
      }
    }
    expect(offenders).toEqual([]);
  });
});

/**
 * Everything this build ships, which is every source but the tests: a test file
 * reaches no browser, and the two that forbid these words have to write them
 * down in order to forbid them.
 */
function sources(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...sources(next));
      continue;
    }
    if (/\.(ts|tsx|css)$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
      found.push(next);
    }
  }
  return found.sort();
}
