import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Source } from "./api";
import { Board } from "./Board";
import { EvidenceImageShown, EvidenceListingShown, EvidenceTextShown, SourceShown } from "./Desk";
import { remarkAbout, sectionRemark } from "./remarks";
import { deskItemOf, Room } from "./ReviewRoom";
import { firstDesk } from "./room";
import { keptDrawing } from "../drawing/drawings";
import { emptyStore } from "../partner/conversation";
import { countsIn, entriesIn } from "../partner/sitting";
import { PartnerAs } from "../partner/store";
import type { About, Notepad } from "../stickies/api";

type Held = Parameters<typeof PartnerAs>[0]["held"];
import { StickiesContext } from "../stickies/store";

/**
 * The whiteboard on screen (g1-s71): a remark on its lines at its commit and
 * nowhere else, the board's Remarks naming where each was made, kept drawings
 * under Drawings, the evidence on the desk, and a remark being written.
 */

const RECORD = "plans/reviews/review-of-landing.md";
const TIP = "9c1f0a2e".repeat(5);
const NEW_TIP = "7d3b1e40".repeat(5);

const REVIEW = (tip: string) =>
  [
    "# Review of landing",
    "",
    "- Kind: review",
    "- Goals: landing",
    `- Reviewed: ${tip} (the tip of goal/landing)`,
    "",
    "## Findings",
    "",
    "## Drawings",
    "",
    "## Outcome",
    "",
  ].join("\n");

function read(over: Partial<Source> = {}): Source {
  return {
    path: "internal/owner.go", commit: TIP, from: 40, to: 43, total: 90,
    lines: [
      { number: 40, text: "func held() {" },
      { number: 41, text: "\tmu.Lock()" },
      { number: 42, text: "\tmu.Lock()" },
      { number: 43, text: "}" },
    ],
    ...over,
  };
}

function notepadOf(abouts: About[][]): Notepad {
  return {
    schemaVersion: 1, human: "wido",
    stickies: abouts.map((about, at) => ({
      id: `s${String(at)}`, text: at === 0 ? "this lock is taken twice" : `remark ${String(at)}`, about,
      createdAt: "2026-09-29T09:00:00Z", updatedAt: "2026-09-29T09:00:00Z", doneAt: "",
    })),
    counts: { open: abouts.length, done: 0 },
  };
}

function screen(node: ReactNode, held: Held, notepad: Notepad): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <StickiesContext.Provider
          value={{
            notepad, loaded: true, problem: "", panelIsOpen: false, doneIsOpen: false, showDone: () => {},
            openPanel: () => {}, closePanel: () => {}, opening: null,
            jot: async () => "", change: async () => "", remove: async () => "",
          }}
        >
          <PartnerAs held={held}>{node}</PartnerAs>
        </StickiesContext.Provider>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

function sittingOf(purpose: string, source: string): Held {
  return {
    sitting: { subject: { kind: "record", id: RECORD, title: "Review of landing" }, purpose, startedAt: "2026-09-29T09:00:00Z" },
    table: { counts: countsIn(source), entries: entriesIn(source), revision: "r1", source },
  };
}

const remarked = remarkAbout(RECORD, read(), 41, 42);

describe("a remark in a review room", () => {
  it("is a marker on its lines while the desk reads its commit, with both presses", () => {
    const markup = screen(<SourceShown source={read()} put={() => {}} record={RECORD} />, sittingOf("review", REVIEW(TIP)), notepadOf([[remarked]]));
    expect(markup.match(/ms-desk-line--remarked/gu)?.length).toBe(2);
    expect(markup).toContain("this lock is taken twice");
    expect(markup).toContain("Record as a fact");
    expect(markup).toContain("Make a finding");
  });

  it("leaves the lines after Review the new tip, and stands beside them naming its commit", () => {
    const held = sittingOf("review", REVIEW(NEW_TIP));
    const markup = screen(<SourceShown source={read({ commit: NEW_TIP })} put={() => {}} record={RECORD} />, held, notepadOf([[remarked]]));
    expect(markup).not.toContain("ms-desk-line--remarked");
    expect(markup).toContain("on lines 41-42 at 9c1f0a2");
    const board = screen(<Board />, held, notepadOf([[remarked]]));
    expect(board).toContain("Remarks");
    expect(board).toContain("this lock is taken twice");
    expect(board).toContain("on lines 41-42 at 9c1f0a2");
  });

  it("on a section stands on the board with a Remark press on every pile", () => {
    const board = screen(<Board />, sittingOf("review", REVIEW(TIP)), notepadOf([[sectionRemark(RECORD, "Findings")]]));
    expect(board).toContain("on § Findings");
  });
});

describe("a remark in a shaping room", () => {
  const atA = read({ commit: "", checkout: true, head: "a".repeat(40) });
  const shaping = remarkAbout(RECORD, atA, 41, 42);
  const edited = read({
    commit: "", checkout: true, head: "a".repeat(40),
    lines: [
      { number: 40, text: "func held() {" },
      { number: 41, text: "\tmu.Lock()" },
      { number: 42, text: "\tdefer mu.Unlock()" },
      { number: 43, text: "}" },
    ],
  });

  it("shaping_remark_after_uncommitted_edit: no marker on edited lines, and the board says an earlier reading", () => {
    const held = { ...sittingOf("shape a design", REVIEW(TIP)), deskRead: edited };
    const desk = screen(<SourceShown source={edited} put={() => {}} record={RECORD} />, held, notepadOf([[shaping]]));
    expect(desk).not.toContain("ms-desk-line--remarked");
    const board = screen(<Board />, held, notepadOf([[shaping]]));
    expect(board).toContain("this lock is taken twice");
    expect(board).toContain("on lines 41-42 at an earlier reading");
    expect(board).not.toContain("Make a finding");
    const restored = screen(<SourceShown source={atA} put={() => {}} record={RECORD} />, { ...held, deskRead: atA }, notepadOf([[shaping]]));
    expect(restored.match(/ms-desk-line--remarked/gu)?.length).toBe(2);
  });

  // The return landing on the Board face (SOL-S71-02): the saved face is the
  // board, the lines were edited while away, and the reload made no read of
  // them, so the board claims no currency until the desk reads them again.
  const returned = (deskRead: Source | null) => ({
    ...sittingOf("shape a design", REVIEW(TIP)),
    conversation: RECORD,
    store: { ...emptyStore, conversation: RECORD, state: "ready" as const },
    room: { desk: firstDesk(RECORD, REVIEW(TIP)), face: "board" as const, drafts: {} },
    deskRead,
  });

  it("shaping_remark_after_uncommitted_edit: a return to the Board face with no read says an earlier reading", () => {
    const board = screen(<Room record={RECORD} />, returned(null), notepadOf([[shaping]]));
    expect(board).toContain('data-face="board"');
    expect(board).toContain("this lock is taken twice");
    expect(board).toContain("on lines 41-42 at an earlier reading");
    expect(board).not.toContain("on lines 41-42 of owner.go");
    expect(board).toContain(">Open<");
  });

  it("shaping_remark_after_uncommitted_edit: a later desk read with the same bytes brings the current wording back", () => {
    const matching = screen(<Room record={RECORD} />, returned(atA), notepadOf([[shaping]]));
    expect(matching).toContain("on lines 41-42 of owner.go");
    expect(matching).not.toContain("at an earlier reading");
    const other = screen(<Room record={RECORD} />, returned(read({ commit: "", checkout: true, head: "a".repeat(40), path: "internal/other.go" })), notepadOf([[shaping]]));
    expect(other).toContain("on lines 41-42 at an earlier reading");
  });

  it("shaping_remark_after_uncommitted_edit: a review remark's board wording needs no read", () => {
    const review = { ...sittingOf("review", REVIEW(TIP)), room: returned(null).room };
    for (const deskRead of [null, read()]) {
      const board = screen(<Board />, { ...review, deskRead }, notepadOf([[remarked]]));
      expect(board).toContain("on lines 41-42 of owner.go");
      expect(board).not.toContain("at an earlier reading");
    }
  });
});

describe("a remark being written", () => {
  it("stands on the desk and on the board from the room's drafts, with what it is about", () => {
    const held = {
      ...sittingOf("review", REVIEW(TIP)),
      remarking: { "remark-1": { text: "this lock is ta", clause: "", kind: "remark", about: remarked } },
    };
    const board = screen(<Board />, held, notepadOf([]));
    expect(board).toContain("Remark on owner.go:41-42");
    expect(board).toContain("this lock is ta");
  });
});

describe("kept drawings on the board", () => {
  it("are listed under Drawings with the question each was drawn for", () => {
    const kept = keptDrawing(REVIEW(TIP), { id: "0a1b2c3d", caption: "How does the handoff work?", date: "2026-09-29", source: "flowchart LR\n  a --> b" }) ?? "";
    const board = screen(<Board />, sittingOf("review", kept), notepadOf([]));
    expect(board).toContain("Drawings");
    expect(board).toContain("How does the handoff work?");
    expect(board).toContain("kept 2026-09-29");
  });
});

describe("the evidence on the desk (D4)", () => {
  it("a present of kind evidence puts the file on the desk, by the room's record", () => {
    expect(deskItemOf({ kind: "evidence", path: "room-1280-light.png" }, RECORD)).toEqual({
      kind: "evidence", record: RECORD, path: "room-1280-light.png",
    });
    expect(deskItemOf({ kind: "evidence" }, RECORD)).toBeNull();
  });

  it("a listing says the whole it walked, or, cut at its bound, that it is a part", () => {
    const entries = [{ path: "a.png", kind: "image" as const, size: 1 }];
    const whole = screen(<EvidenceListingShown listing={{ root: "~/e/", entries, supplied: 1, total: 503 }} record={RECORD} put={() => {}} />, {}, notepadOf([]));
    expect(whole).toContain("1 of 503 files are listed.");
    const cut = screen(<EvidenceListingShown listing={{ root: "~/e/", entries, supplied: 1, total: 700, cut: true }} record={RECORD} put={() => {}} />, {}, notepadOf([]));
    expect(cut).toContain("The first 1 files found are listed; the listing stopped at its bound, so the evidence may hold more.");
    expect(cut).not.toContain("of 700");
    expect(cut.indexOf("the listing stopped at its bound")).toBeLessThan(cut.indexOf("a.png"));
    const none = screen(<EvidenceListingShown listing={{ root: "~/e/", entries: [], supplied: 0, total: 0 }} record={RECORD} put={() => {}} />, {}, notepadOf([]));
    expect(none).toContain("The evidence holds no image and no text.");
  });

  it("a listing cut before it found a file says so first, and never that the evidence holds nothing", () => {
    // The deep-only and wide-only walks: a screenshot below the depth bound, or
    // after the visited bound's other entries, answers an empty listing, cut.
    for (const root of ["~/deep-only/", "~/wide-only/"]) {
      const cut = screen(<EvidenceListingShown listing={{ root, entries: [], supplied: 0, total: 0, cut: true }} record={RECORD} put={() => {}} />, {}, notepadOf([]));
      expect(cut).toContain(
        "The listing stopped at its bound before it found an image or text: nothing was found within the listing&#x27;s bounds, and the evidence may hold more.",
      );
      expect(cut).not.toContain("holds no image");
      expect(cut).not.toContain("The first 0 files");
    }
  });

  it("an image is shown large from the evidence read's address", () => {
    const markup = screen(<EvidenceImageShown record={RECORD} path="room-1280-light.png" />, {}, notepadOf([]));
    expect(markup).toContain(`src="/api/review/plans/reviews/review-of-landing.md/evidence?path=room-1280-light.png"`);
  });

  it("a headingless text is a report, drawn by the section renderer over the read's own blocks", () => {
    const markup = screen(
      <EvidenceTextShown
        text={{
          path: "notes/walk.txt", kind: "text", text: "no headings here\n", supplied: 1, total: 1,
          blocks: [{ type: "paragraph", inlines: [{ type: "text", text: "no headings here" }] }],
        }}
      />,
      {}, notepadOf([]),
    );
    expect(markup).toContain("no headings here");
    expect(markup).toContain("notes/walk.txt");
  });
});
