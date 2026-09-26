import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { ASK_IT, countsIn, entriesIn, entryPath, sittingChip } from "./sitting";
import { PartnerAs } from "./store";
import { SittingTable } from "./Table";

/**
 * The table's entries, and where pressing one goes (g1-s53 D7).
 *
 * It is read from the markup because the claim is about the markup: an entry is
 * either a link into the record or it is a paragraph a human cannot press. What
 * the target is spelled as is asserted here too, against the reader's own
 * heading ids — the record's four headings are what the anchors have to match,
 * and a table linking to an anchor the reader never mints would scroll nowhere
 * and say nothing about it.
 */

const RECORD = "plans/designs/sessions.md";

/** A record two entries have been recorded into, in two different piles. */
const SOURCE = [
  "# Session limits",
  "",
  "## Facts",
  "",
  "- 2026-09-26 · Wido · the limit is twelve hours",
  "  - Anchor: internal/session/session.go:212",
  "",
  "## Open questions",
  "",
  "- 2026-09-26 · Wido · what the current limit protects",
  "",
].join("\n");

/** The table over one sitting and one reading of its record. */
function table(source: string): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs
          held={{
            sitting: {
              subject: { kind: "record", id: RECORD, title: "Session limits" },
              purpose: "shape a design",
              startedAt: "2026-09-26T09:00:00Z",
            },
            table: { counts: countsIn(source), entries: entriesIn(source), revision: "r1", source },
          }}
        >
          <SittingTable />
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("an entry of the table", () => {
  it("is a link into the sitting's record, anchored at its own pile", () => {
    const markup = table(SOURCE);

    expect(markup).toContain(`href="/project/doc/plans/designs/sessions.md#facts"`);
    expect(markup).toContain(`href="/project/doc/plans/designs/sessions.md#open-questions"`);
    // One link per entry, and no link for a pile that holds nothing.
    expect(markup.split("ms-table-entry-link").length - 1).toBe(2);
    expect(markup).not.toContain("#proposals");
    expect(markup).not.toContain("#decisions");
    // The words, the clause and the attribution are inside the press, so the
    // whole entry is what a human aims at.
    expect(markup).toContain("the limit is twelve hours");
    expect(markup).toContain("internal/session/session.go:212");
  });

  it("names the anchors the document reader itself mints for the four piles", () => {
    expect(entryPath(RECORD, "Facts")).toBe("/project/doc/plans/designs/sessions.md#facts");
    expect(entryPath(RECORD, "Proposals")).toBe("/project/doc/plans/designs/sessions.md#proposals");
    expect(entryPath(RECORD, "Decisions")).toBe("/project/doc/plans/designs/sessions.md#decisions");
    expect(entryPath(RECORD, "Open questions")).toBe(
      "/project/doc/plans/designs/sessions.md#open-questions",
    );
  });

  it("is not there at all on a record nothing has been recorded into", () => {
    const markup = table("# Session limits\n");

    expect(markup).not.toContain("ms-table-entry-link");
    expect(markup).toContain("Nothing has been recorded in this sitting yet.");
  });

  /**
   * An open question goes to the register with one press, and nothing else does
   * (g1-s55 D2).
   *
   * It is beside the entry rather than inside its link, because a press inside a
   * link is a press that navigates — and what this one does is open a sheet over
   * the page the human is on.
   */
  it("offers Ask it on an open question, and on no other pile", () => {
    const markup = table(SOURCE);

    expect(markup.split("ms-table-ask").length - 1).toBe(1);
    expect(markup).toContain(`>${ASK_IT}<`);
    // The fact is not a question, so it carries no press of its own.
    const upToTheFact = markup.slice(0, markup.indexOf("Open questions"));
    expect(upToTheFact).not.toContain("ms-table-ask");
  });

  /**
   * Resume, from the browser's side: a conversation whose sitting mark stands
   * comes back with its chip and its table on load (g1-s55 D3).
   *
   * There is nothing to submit here and nothing to remember: the sitting is the
   * server's answer and the table is a reading of the record, so a page that has
   * just loaded shows both from the snapshot alone. The turn that says what is on
   * the table is the server's own, decided where it reads the conversation.
   */
  it("stands with its subject and its piles on a page that has just loaded", () => {
    const markup = table(SOURCE);

    expect(markup).toContain("Session limits");
    expect(markup).toContain("the limit is twelve hours");
    expect(markup).toContain("what the current limit protects");
    expect(sittingChip({
      subject: { kind: "record", id: RECORD, title: "Session limits" },
      purpose: "shape a design",
      startedAt: "2026-09-26T09:00:00Z",
    })).toBe("Sitting: Session limits");
  });
});
