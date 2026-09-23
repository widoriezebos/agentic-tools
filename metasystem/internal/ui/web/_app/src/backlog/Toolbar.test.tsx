import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { noFilters } from "./filters";
import { syncOf } from "./sync";
import { BoardToolbar } from "./Toolbar";
import type { FetchClause, Ledger } from "./api";

/**
 * What the one row above the lanes is made of, and in what order.
 *
 * The order is the point of the change: the switch, then what the board is
 * narrowed to, then — at the far end — whether the page is current, the ask
 * that reads it again, and the one act. It is asserted from the markup the
 * row renders rather than from a list it was given, because a list would be
 * this test agreeing with itself.
 */

const OBSERVED = "2026-09-23T10:30:03Z";

const clause: FetchClause = {
  outcome: "current",
  startedAt: "2026-09-23T10:29:40Z",
  finishedAt: "2026-09-23T10:29:41Z",
  tip: "",
  detail: "already at the canonical tip",
  message: "",
  failures: 0,
  cadence: "5m",
  nextAt: "2026-09-23T10:34:41Z",
};

const ledger: Ledger = {
  state: "read",
  tip: "2ef5d8d9c1b4a70f3e2d1c0b9a8f7e6d5c4b3a29",
  committedAt: "2026-09-23T10:12:00Z",
  stale: false,
  staleAfterSeconds: 3600,
  syncMode: "",
  stateRoot: "",
  message: "",
  problems: [],
  fetch: clause,
};

function markupOf(node: ReactNode): string {
  return renderToStaticMarkup(<TooltipPrimitive.Provider>{node}</TooltipPrimitive.Provider>);
}

function board(): string {
  return markupOf(
    <BoardToolbar
      reading={{ view: "board", onView: () => undefined, onNew: () => undefined }}
      narrowing={{ filters: noFilters, onFilters: () => undefined, seats: ["fable"], arcs: ["ui"] }}
      sync={syncOf(ledger, OBSERVED)}
      observedAt={OBSERVED}
      onRefresh={() => undefined}
    />,
  );
}

/** Where each part starts in the markup, or -1 for a part that is not there. */
function at(markup: string, needle: string): number {
  return markup.indexOf(needle);
}

describe("the toolbar", () => {
  const markup = board();

  it("is one row, and holds every part of what used to be five", () => {
    expect(markup).toContain('class="ms-board-toolbar"');
    expect(markup).toContain(">Board<");
    expect(markup).toContain(">List<");
    expect(markup).toContain('aria-label="Narrow the board"');
    expect(markup).toContain('class="ms-sync ms-sync--rest"');
    expect(markup).toContain('aria-label="Refresh"');
    expect(markup).toContain(">New goal<");
  });

  it("reads switch, filters, chip, refresh, New goal, in that order", () => {
    const places = [
      at(markup, 'aria-label="How the backlog is read"'),
      at(markup, 'aria-label="Narrow the board"'),
      at(markup, 'class="ms-sync'),
      at(markup, 'aria-label="Refresh"'),
      at(markup, ">New goal<"),
    ];
    expect(places).toEqual([...places].sort((first, second) => first - second));
    expect(places.every((place) => place >= 0)).toBe(true);
  });

  it("puts the chip, the refresh and the act at the far end, in one group", () => {
    const end = markup.indexOf('class="ms-board-toolbar-end"');
    expect(end).toBeGreaterThan(at(markup, 'aria-label="Narrow the board"'));
    expect(at(markup, 'class="ms-sync')).toBeGreaterThan(end);
  });

  // The five filters narrow the board, so they are offered where there is a
  // board: the list is not narrowed by them and would be offering a control
  // that did nothing.
  it("offers the filters where there is a board, and not otherwise", () => {
    const list = markupOf(
      <BoardToolbar
        reading={{ view: "list", onView: () => undefined, onNew: () => undefined }}
        narrowing={null}
        sync={syncOf(ledger, OBSERVED)}
        observedAt={OBSERVED}
        onRefresh={() => undefined}
      />,
    );
    expect(list).not.toContain('aria-label="Narrow the board"');
    expect(list).toContain(">New goal<");
  });

  // Intake is never disabled for want of proof: the act asks, and a server
  // that finds no human behind it answers with the sign-in this page opens.
  it("disables nothing, proof or otherwise", () => {
    expect(markup).not.toContain("disabled");
  });

  it("offers the ask and the chip even where no ledger was read", () => {
    const unread = markupOf(
      <BoardToolbar reading={null} narrowing={null} sync={null} observedAt="" onRefresh={() => undefined} />,
    );
    expect(unread).toContain('aria-label="Refresh"');
    expect(unread).toContain("ms-skeleton");
    expect(unread).not.toContain(">New goal<");
    expect(unread).not.toContain(">Board<");
  });
});

describe("the chip", () => {
  it("is muted at rest, wears the marker when old, and the danger colour when wrong", () => {
    expect(board()).toContain("ms-sync--rest");
    const old = markupOf(
      <BoardToolbar
        reading={null}
        narrowing={null}
        sync={syncOf({ ...ledger, stale: true, committedAt: "2026-09-23T08:12:00Z" }, OBSERVED)}
        observedAt={OBSERVED}
        onRefresh={() => undefined}
      />,
    );
    expect(old).toContain("ms-sync--stale");
    expect(old).toContain("2 h behind");
    const broken: Ledger = { ...ledger, fetch: { ...clause, outcome: "failed", message: "host unreachable" } };
    const wrong = markupOf(
      <BoardToolbar
        reading={null}
        narrowing={null}
        sync={syncOf(broken, OBSERVED)}
        observedAt={OBSERVED}
        onRefresh={() => undefined}
      />,
    );
    expect(wrong).toContain("ms-sync--wrong");
  });

  // The report is the tooltip's, and a tooltip nothing but a mouse can reach
  // is a fact behind a gesture. The chip takes a tab stop of its own.
  it("can be reached without a pointer", () => {
    expect(board()).toContain('tabindex="0"');
  });
});
