import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { DocumentPayload } from "./api";
import { FileActions } from "./DocumentPane";
import { sittable, START } from "../partner/sitting";
import { PartnerAs } from "../partner/store";

/**
 * Which records a sitting may be started on, from the record's own act row
 * (g1-s53 D1).
 *
 * A sitting is about an intent or a design record, and the service refuses every
 * other subject in its own words. This is the other half: the page does not offer
 * a press it knows would be refused. It is read from the markup because that is
 * the claim — the button is either on the row or it is not.
 */

const PATH = "/checkout/plans/designs/sessions.md";

/** One record of the given kind, or a file that declares no head at all. */
function read(kind: string | null): DocumentPayload {
  return {
    kind: "document",
    id: "plans/designs/sessions.md",
    title: "Session limits",
    revision: "r1",
    source: "# Session limits\n",
    owner: "wido",
    path: PATH,
    bytes: 18,
    modifiedAt: "2026-09-26T09:00:00Z",
    readAt: "2026-09-26T09:00:00Z",
    state: "readable",
    reason: "",
    record:
      kind === null
        ? null
        : {
            kind,
            id: "d-1",
            status: "draft",
            goals: [],
            cites: [],
            affects: [],
            governs: [],
            supersedes: [],
            by: [],
          },
    referencedBy: [],
    supersededBy: [],
    headings: [],
    blocks: [],
  };
}

/** The record's act row, with no sitting standing anywhere. */
function acts(kind: string | null): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{}}>
          <FileActions path={PATH} document={read(kind)} onEdit={null} onNewGoal={null} busy="" />
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("Start a sitting on a record's own page", () => {
  it("is offered on an intent record and on a design record", () => {
    for (const kind of ["intent", "design"]) {
      expect(acts(kind)).toContain(START);
    }
  });

  it("is not offered on any other kind, nor on a file that is not a record", () => {
    for (const kind of ["doctrine", "decision", "question", ""]) {
      expect(acts(kind)).not.toContain(START);
    }
    expect(acts(null)).not.toContain(START);
  });

  it("still offers Ask on every one of them, which is the act every record has", () => {
    for (const kind of ["design", "doctrine", null]) {
      expect(acts(kind)).toContain(">Ask<");
    }
  });

  it("says which kinds a sitting is about, in one place", () => {
    expect(sittable("intent")).toBe(true);
    expect(sittable("design")).toBe(true);
    expect(sittable("doctrine")).toBe(false);
    expect(sittable("decision")).toBe(false);
    expect(sittable("")).toBe(false);
    expect(sittable(null)).toBe(false);
    expect(sittable(undefined)).toBe(false);
  });
});
