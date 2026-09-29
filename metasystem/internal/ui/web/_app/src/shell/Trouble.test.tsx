import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Trouble } from "./Trouble";
import { TroublesAs } from "./troubles";
import { BROKEN_ROOM, type Pending } from "./troubling";

/**
 * The trouble line (g1-s68 D1, D2): the sentence as it was, the role as it
 * was, and one control — drawn only where a press can reach a colleague.
 */

const SAID = "work land is refused: the review is stale (REVIEW_STALE)";

describe("the trouble line", () => {
  it("shows no control before the Partner has registered its ask", () => {
    const markup = renderToStaticMarkup(<Trouble text={SAID} role="alert" />);
    expect(markup).toContain(SAID);
    expect(markup).toContain('role="alert"');
    expect(markup).not.toContain("Ask what happened");
  });

  it("offers Ask what happened once the Partner has", () => {
    const markup = renderToStaticMarkup(
      <TroublesAs held={{ ask: () => {} }}>
        <Trouble text={SAID} role="status" />
      </TroublesAs>,
    );
    expect(markup).toContain(`<span class="ms-trouble-text">${SAID}</span>`);
    expect(markup).toContain('role="status"');
    expect(markup).toContain('<button type="button" class="ms-trouble-ask">Ask what happened</button>');
  });

  it("offers the reload and no control where the conversation itself cannot be shown", () => {
    const markup = renderToStaticMarkup(
      <TroublesAs held={{ ask: () => {} }}>
        <Trouble text="This pane could not be rendered: TypeError: x" broken={BROKEN_ROOM} />
      </TroublesAs>,
    );
    expect(markup).toContain(BROKEN_ROOM);
    expect(markup).toContain("Reload page");
    expect(markup).not.toContain("Ask what happened");
  });

  it("draws the sentence the site draws, and keeps its own words as what travels", () => {
    const markup = renderToStaticMarkup(
      <Trouble text="The backlog could not be read: no tip" variant="pane" as="div">
        <strong>The backlog could not be read</strong>
      </Trouble>,
    );
    expect(markup).toContain('class="ms-trouble ms-trouble--pane"');
    expect(markup).toContain("<strong>The backlog could not be read</strong>");
  });

  it("says it is waiting, and for whom, while a press waits for an answer", () => {
    // The line's own id is React's useId, and a static render's first one is
    // "_R_0_" in the React this build pins.
    const waiting: Pending = {
      id: "1", origin: "_R_0_", key: "k", conversation: "plans/reviews/review-of-a.md",
      trouble: { text: SAID, where: { section: "Review", path: "/review/x" }, at: "2026-09-28T19:41:00Z" },
      label: "waiting for the room on a",
    };
    const markup = renderToStaticMarkup(
      <TroublesAs held={{ ask: () => {}, pending: [waiting] }}>
        <Trouble text={SAID} />
      </TroublesAs>,
    );
    expect(markup).toContain('<span class="ms-trouble-waiting">waiting for the room on a</span>');
  });
});
