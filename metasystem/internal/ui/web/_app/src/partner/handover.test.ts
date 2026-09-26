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
    expect(ASK_SHEET).toContain("}, [handOver, id, sheet, writes]);");
  });

  it("is not what closes it: the hand-over runs once, on the way in", () => {
    expect(ASK_SHEET).toContain("}, [handOver, id, sheet, writes]);");
  });

  /**
   * The draft goes when THIS opening goes, and by nothing else.
   *
   * It is its own effect, on every sheet that registers an opening rather than
   * only on the ones with writable fields, because a draft attached by "Ask about
   * this" on a sheet with nothing writable has to be retired too — and it is
   * retired by the opening's id, so that the other of two sheets of one name
   * keeps its own (Astra F2 on g1-s56). The store's sheet stack does not retire
   * it any more: a name knows nothing about which of two openings closed.
   */
  it("drops the draft of its own opening when the sheet goes, and no other", () => {
    expect(ASK_SHEET).toContain("dropDraft(id);");
    expect(STORE).toContain("retireOnOpeningClosed(held, opening)");
    expect(STORE).not.toContain("retireOnSheetClosed");
    const list = attach([], attachedDraft(edited()));
    expect(draftIn(list)).not.toBeNull();
    expect(draftIn(remove(list, idFor("draft", OPENING)))).toBeNull();
    // Another opening of a sheet of the same name is not this one.
    expect(draftIn(remove(list, idFor("draft", "opening-2")))).not.toBeNull();
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
    const taken = remove(attach([], attachedDraft(edited())), idFor("draft", OPENING));
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
 * The one press that does both, and the two places it is allowed to exist.
 *
 * Astra's two material findings on g1-s52 were both in this press, and both about
 * telling the truth: a caller that set the words and then read the draft back
 * from the render would send the previous delta, and a save that reported no
 * outcome could say "Used and saved" of a request that was refused. Both are
 * answered by the sheet owning one submission path that takes the next draft
 * explicitly and returns what the ledger did (g1-s56 D1), so the words are in the
 * build now — in the block that offers the press, in the register that explains
 * it, and nowhere that sends anything without it.
 */
describe("the press that uses and saves at once", () => {
  it("is offered where the sheet can send, and explained where it is offered", () => {
    const shipped = sources();
    // The scan asserts its own reach before it asserts anything else.
    expect(shipped).toContain("backlog/EditSheet.tsx");
    expect(shipped).toContain("partner/FieldProposals.tsx");
    const saying: string[] = [];
    for (const file of shipped) {
      const contents = readFileSync(path.join(SRC, file), "utf8");
      if (/use and save|used and saved/i.test(contents)) {
        saying.push(file);
      }
    }
    // The card in the transcript and the store say it through the one constant
    // the words are written in, which is why they are not on this list.
    expect(saying).toEqual([
      "backlog/EditSheet.tsx",
      "help/terms.ts",
      "partner/FieldProposals.tsx",
      "partner/suggesting.ts",
    ]);
  });

  /**
   * And it goes through the one path, with the draft as its argument. Read where
   * it is written, because no static render can press it.
   */
  it("sends the draft it is putting the words into, not the one on screen", () => {
    const sheet = readFileSync(path.join(SRC, "backlog", "EditSheet.tsx"), "utf8");
    expect(sheet).toContain("const submit = async (next: EditDraft): Promise<Outcome> => {");
    expect(sheet).toContain("const asked = changedIn(opened, next);");
    expect(sheet).toContain("const next = { ...draft, [at]: text };");
    expect(sheet).toContain("return submit(next);");
    // Save is the same path, called with what the fields hold.
    expect(sheet).toContain("void submit(draft);");
    // The guard a second press in the same render can see.
    expect(sheet).toContain("if (inFlight.current) {");
    expect(sheet).toContain("return refusedSave(IN_FLIGHT);");
  });

  /**
   * Undo after a save would put the field back and leave the ledger where the
   * save left it, so the store does not offer it — and what a press reaches is
   * the sheet's own save, never a second way to the route.
   */
  it("reports what the save answered, and offers no undo of an act", () => {
    expect(STORE).toContain("const outcome = await registered.save(card.field, card.text);");
    expect(STORE).toContain("setMarks((held) => answered(held, id, outcome));");
    expect(STORE).toContain("setMarks((held) => saving(held, id, previous));");
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
