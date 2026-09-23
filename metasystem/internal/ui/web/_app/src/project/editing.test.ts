import { describe, expect, it } from "vitest";

import type { Block, Problem } from "./api";
import {
  CHANGED_ON_DISK,
  dirty,
  next,
  opening,
  refusal,
  REFUSED_BY_THE_PROJECT,
  savedAt,
  statusOf,
  type Editor,
} from "./editing";

const SOURCE = "# The ledger\n\nAs it was read.\n";
const REVISION = "blob:0000000000000000000000000000000000000001";

const heading: Block = { type: "heading", level: 1, text: "The ledger" };

function typed(editor: Editor, source: string): Editor {
  return next(editor, { kind: "typed", source });
}

describe("an editor over one document", () => {
  it("opens over the bytes that were read, showing the source, holding nothing", () => {
    const editor = opening(SOURCE, REVISION);

    expect(editor).toEqual({
      source: SOURCE,
      opened: SOURCE,
      revision: REVISION,
      mode: "source",
      pending: false,
      blocks: [],
      message: "",
      problems: [],
    });
    expect(dirty(editor)).toBe(false);
    expect(statusOf(editor)).toBe("");
  });

  it("is dirty exactly when the box holds something the file does not", () => {
    const editor = typed(opening(SOURCE, REVISION), `${SOURCE}One more line.\n`);

    expect(dirty(editor)).toBe(true);
    expect(statusOf(editor)).toBe("Unsaved changes");
    expect(dirty(typed(editor, SOURCE))).toBe(false);
  });

  it("keeps the revision it opened at, so a save is against the read", () => {
    const editor = typed(opening(SOURCE, REVISION), "# Something else\n");

    expect(editor.revision).toBe(REVISION);
  });
});

describe("what the editor does with an outcome", () => {
  it("renders the text into the preview, and leaves the text alone", () => {
    const written = typed(opening(SOURCE, REVISION), "# A new title\n");
    const sending = next(written, { kind: "sending", saying: "Rendering…" });

    expect(sending.pending).toBe(true);
    expect(statusOf(sending)).toBe("Rendering…");

    const rendered = next(sending, { kind: "rendered", blocks: [heading] });

    expect(rendered.mode).toBe("preview");
    expect(rendered.pending).toBe(false);
    expect(rendered.blocks).toEqual([heading]);
    expect(rendered.source).toBe("# A new title\n");
    expect(statusOf(rendered)).toBe("Unsaved changes");
  });

  it("goes back to the text with the text intact", () => {
    const rendered = next(next(typed(opening(SOURCE, REVISION), "# A new title\n"), { kind: "sending", saying: "Rendering…" }), {
      kind: "rendered",
      blocks: [heading],
    });

    const back = next(rendered, { kind: "source" });

    expect(back.mode).toBe("source");
    expect(back.source).toBe("# A new title\n");
  });

  it("takes no typing while the preview is showing, so a preview is never of other text", () => {
    const rendered = next(next(opening(SOURCE, REVISION), { kind: "sending", saying: "Rendering…" }), {
      kind: "rendered",
      blocks: [heading],
    });

    expect(typed(rendered, "# Typed into the preview\n").source).toBe(SOURCE);
  });

  it("starts nothing while something is in flight", () => {
    const sending = next(opening(SOURCE, REVISION), { kind: "sending", saying: "Saving…" });

    expect(next(sending, { kind: "sending", saying: "Rendering…" })).toBe(sending);
    expect(next(sending, { kind: "source" })).toBe(sending);
    expect(typed(sending, "# Typed while saving\n")).toBe(sending);
  });

  it("answers a refusal with the sentence and the problems, and stays open", () => {
    const problems: Problem[] = [
      { path: "plans/designs/ledger.md", line: 3, message: "the goal logistics is not in the ledger" },
    ];
    const sending = next(typed(opening(SOURCE, REVISION), "# Changed\n"), { kind: "sending", saying: "Saving…" });

    const kept = next(sending, {
      kind: "refused",
      status: 422,
      said: "the record this would write is one the project refuses",
      problems,
    });

    expect(kept.pending).toBe(false);
    expect(kept.source).toBe("# Changed\n");
    expect(kept.problems).toEqual(problems);
    expect(statusOf(kept)).toBe(REFUSED_BY_THE_PROJECT);
  });

  it("clears the last refusal as soon as the text moves on", () => {
    const refusedOnce = next(next(opening(SOURCE, REVISION), { kind: "sending", saying: "Saving…" }), {
      kind: "refused",
      status: 422,
      said: "refused",
      problems: [{ path: "a.md", line: 3, message: "a problem" }],
    });

    const moved = typed(refusedOnce, "# Fixed\n");

    expect(moved.problems).toEqual([]);
    expect(statusOf(moved)).toBe("Unsaved changes");
  });
});

describe("what a refusal says", () => {
  it("tells a human what to do about a file that changed underneath", () => {
    expect(refusal(409, "the file changed since you opened it")).toBe(CHANGED_ON_DISK);
  });

  it("says the project refused the record, and leaves the problems to follow", () => {
    expect(refusal(422, "the record this would write is one the project refuses")).toBe(REFUSED_BY_THE_PROJECT);
  });

  it("says everything else in the server's own words", () => {
    expect(refusal(413, "the text is larger than 1 MiB")).toBe("the text is larger than 1 MiB");
    expect(refusal(0, "Failed to fetch")).toBe("Failed to fetch");
  });
});

describe("what the page says after a save", () => {
  it("is that it landed, and at which minute", () => {
    expect(savedAt(new Date(2026, 8, 23, 10, 52, 13))).toBe("Saved 10:52");
    expect(savedAt(new Date(2026, 8, 23, 9, 5, 0))).toBe("Saved 09:05");
  });
});
