import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Notepad, Sticky } from "./api";
import { StickiesBlock } from "./Block";
import { StickiesButton } from "./Button";
import {
  aboutThePage,
  about,
  captured,
  chipPath,
  chipWords,
  doneLabel,
  doneStickies,
  emptyNotepad,
  openStickies,
  savesOn,
  SIGN_IN_LINE,
} from "./stickies";
import { StickiesContext } from "./store";
import type { Subject } from "../shell/about";

/**
 * The notepad, as the three surfaces read it.
 *
 * What is asserted here is what each surface is made of and what belongs on
 * it: the order and the counts are the server's, and a test that recomputed
 * them would be this file agreeing with itself.
 */

function sticky(over: Partial<Sticky> = {}): Sticky {
  return {
    id: "STICKY1",
    text: "ask Sol about the retry",
    about: [],
    createdAt: "2026-09-25T11:00:00Z",
    updatedAt: "2026-09-25T11:00:00Z",
    doneAt: "",
    ...over,
  };
}

function notepad(stickies: Sticky[], human = "Wido"): Notepad {
  return {
    schemaVersion: 1,
    human,
    stickies,
    counts: {
      open: stickies.filter((one) => one.doneAt === "").length,
      done: stickies.filter((one) => one.doneAt !== "").length,
    },
  };
}

/** One surface, rendered over a notepad that is not a live server's. */
function markupOf(held: Notepad, node: ReactNode, panelIsOpen = false): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <StickiesContext.Provider
          value={{
            notepad: held,
            loaded: true,
            problem: "",
            panelIsOpen,
            openPanel: () => undefined,
            closePanel: () => undefined,
            opening: null,
            jot: async () => "",
            change: async () => "",
            remove: async () => "",
          }}
        >
          {node}
        </StickiesContext.Provider>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the header's button", () => {
  it("names itself, opens the panel, and counts the open ones", () => {
    const markup = markupOf(notepad([sticky(), sticky({ id: "STICKY2" })]), <StickiesButton />);

    expect(markup).toContain('aria-label="Stickies"');
    expect(markup).toContain('aria-controls="stickies-panel"');
    expect(markup).toContain('aria-label="2 open"');
    expect(markup).toContain(">2<");
  });

  it("counts what is open and never what is done", () => {
    const markup = markupOf(
      notepad([sticky(), sticky({ id: "STICKY2", doneAt: "2026-09-25T12:00:00Z" })]),
      <StickiesButton />,
    );

    expect(markup).toContain('aria-label="1 open"');
  });

  it("shows no badge at all when nothing is open", () => {
    expect(markupOf(emptyNotepad, <StickiesButton />)).not.toContain("ms-stickies-count");
  });
});

describe("the block a page carries", () => {
  const named = { kind: "goal", id: "g1-s45" } as const;

  it("shows the open stickies about this page, and offers to write one", () => {
    const markup = markupOf(
      notepad([
        sticky({ id: "STICKY1", text: "check g1-s45 tomorrow", about: [named] }),
        sticky({ id: "STICKY2", text: "about something else", about: [{ kind: "goal", id: "g1-s13" }] }),
      ]),
      <StickiesBlock named={named} />,
    );

    expect(markup).toContain("check g1-s45 tomorrow");
    expect(markup).not.toContain("about something else");
    expect(markup).toContain("Add a sticky");
  });

  it("never shows one that has been struck off", () => {
    const markup = markupOf(
      notepad([sticky({ text: "already dealt with", about: [named], doneAt: "2026-09-25T12:00:00Z" })]),
      <StickiesBlock named={named} />,
    );

    expect(markup).not.toContain("already dealt with");
    expect(markup).toContain("Add a sticky");
  });

  it("names the OTHER things a sticky is about, and not the page it is on", () => {
    const markup = markupOf(
      notepad([
        sticky({
          text: "two things at once",
          about: [named, { kind: "record", id: "plans/designs/user-interface/launch.md" }],
        }),
      ]),
      <StickiesBlock named={named} />,
    );

    expect(markup).toContain("launch.md");
    expect(markup).not.toContain(">g1-s45<");
  });
});

describe("what the page on screen is, as a sticky says it", () => {
  it("is the goal on a goal page and the document in the reader", () => {
    expect(aboutThePage({ kind: "goal", subject: "g1-s45" })).toEqual({ kind: "goal", id: "g1-s45" });
    expect(aboutThePage({ kind: "document", subject: "plans/designs/a.md" })).toEqual({
      kind: "record",
      id: "plans/designs/a.md",
    });
  });

  it("is nothing anywhere else, so the composer offers no chip there", () => {
    expect(aboutThePage({})).toBeNull();
    expect(aboutThePage({ kind: "goal" })).toBeNull();
    expect(aboutThePage({ tab: "Designs", subject: "" })).toBeNull();
  });
});

describe("a chip", () => {
  it("says a goal by its id and a document by its file name", () => {
    expect(chipWords({ kind: "goal", id: "g1-s45" })).toBe("g1-s45");
    expect(chipWords({ kind: "record", id: "plans/designs/user-interface/launch.md" })).toBe("launch.md");
    expect(chipWords({ kind: "record", id: "README.md" })).toBe("README.md");
  });

  it("leads to the goal's own page and to the document in the reader", () => {
    expect(chipPath({ kind: "goal", id: "g1-s45" })).toBe("/backlog/goal/g1-s45");
    expect(chipPath({ kind: "record", id: "plans/a.md" })).toBe("/project/doc/plans/a.md");
  });
});

describe("the composer's one key", () => {
  it("saves on Enter and breaks a line on Shift+Enter", () => {
    expect(savesOn("Enter", false)).toBe(true);
    expect(savesOn("Enter", true)).toBe(false);
    expect(savesOn("Escape", false)).toBe(false);
    expect(savesOn("a", false)).toBe(false);
  });
});

describe("the disclosure over the struck-off ones", () => {
  it("says how many are behind it", () => {
    expect(doneLabel(0)).toBe("Done (0)");
    expect(doneLabel(12)).toBe("Done (12)");
  });
});

describe("the two halves of the list", () => {
  it("are told apart by whether one was struck off, in the order the server gave", () => {
    const held = notepad([
      sticky({ id: "A", text: "open one" }),
      sticky({ id: "B", text: "open two" }),
      sticky({ id: "C", text: "done one", doneAt: "2026-09-25T12:00:00Z" }),
    ]);

    expect(openStickies(held).map((one) => one.text)).toEqual(["open one", "open two"]);
    expect(doneStickies(held).map((one) => one.text)).toEqual(["done one"]);
  });
});

describe("the stickies about one thing", () => {
  it("are the open ones that name it, and nothing when nothing is named", () => {
    const held = notepad([
      sticky({ id: "A", text: "about the goal", about: [{ kind: "goal", id: "g1-s45" }] }),
      sticky({ id: "B", text: "about the document", about: [{ kind: "record", id: "plans/a.md" }] }),
      sticky({ id: "C", text: "struck off", about: [{ kind: "goal", id: "g1-s45" }], doneAt: "2026-09-25T12:00:00Z" }),
    ]);

    expect(about(held, { kind: "goal", id: "g1-s45" }).map((one) => one.text)).toEqual(["about the goal"]);
    expect(about(held, { kind: "record", id: "plans/a.md" }).map((one) => one.text)).toEqual(["about the document"]);
    expect(about(held, null)).toEqual([]);
  });
});

describe("what the Partner is given", () => {
  const held = notepad([
    sticky({ id: "A", text: "about this page", about: [{ kind: "goal", id: "g1-s45" }] }),
    sticky({ id: "B", text: "the Fleet page feels cramped" }),
    sticky({ id: "C", text: "struck off", doneAt: "2026-09-25T12:00:00Z" }),
  ]);
  const goalPage: Subject = { kind: "goal", subject: "g1-s45" };

  // Astra's F2: a note about no subject reaches the Partner only through the
  // panel, and "what did I want to remember about the Fleet page" is asked by
  // opening the panel.
  it("is everything the panel shows while the panel is open", () => {
    const sent = captured(held, true, { });

    expect(sent.stickies.map((one) => one.text)).toEqual([
      "about this page",
      "the Fleet page feels cramped",
      "struck off",
    ]);
    expect(sent.stickies[2].done).toBe(true);
    expect(sent.open).toBe(2);
  });

  it("is what the page shows while the panel is closed", () => {
    const sent = captured(held, false, goalPage);

    expect(sent.stickies).toEqual([{ text: "about this page", about: ["goal g1-s45"] }]);
    expect(sent.open).toBe(2);
  });

  it("carries the open count even where no sticky is about the page", () => {
    const sent = captured(held, false, { kind: "goal", subject: "g1-s13" });

    expect(sent.stickies).toEqual([]);
    expect(sent.open).toBe(2);
  });

  it("names a document by its own path, which is how the server looks one up", () => {
    const reader = notepad([sticky({ text: "the wording is off", about: [{ kind: "record", id: "plans/a.md" }] })]);

    expect(captured(reader, false, { kind: "document", subject: "plans/a.md" }).stickies).toEqual([
      { text: "the wording is off", about: ["plans/a.md"] },
    ]);
  });
});

describe("a seat that knows nobody", () => {
  it("has a line to say so, and it asks rather than refuses", () => {
    expect(SIGN_IN_LINE).toContain("Sign in");
    expect(emptyNotepad.human).toBe("");
  });
});
