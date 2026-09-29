import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { Block } from "./api";
import { InterfaceLinks, Markdown } from "./Markdown";

/**
 * A recovery that is a press in this interface is given as a link, never as a
 * card (g1-s68 D3): in a Partner's answer a link to one of this interface's own
 * addresses is followed; in a document it is not.
 */

const ROOM = "/review/plans/reviews/review-of-g1-s22.md";
const blocks: Block[] = [
  {
    type: "paragraph",
    inlines: [
      { type: "text", text: "Open " },
      { type: "link", href: ROOM, target: "unresolved", inlines: [{ type: "text", text: "the review room on g1-s22" }] },
      { type: "text", text: " and " },
      { type: "link", href: "/api/backlog", target: "unresolved", inlines: [{ type: "text", text: "a route" }] },
    ],
  },
];

function rendered(inAnswer: boolean): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <InterfaceLinks.Provider value={inAnswer}>
        <Markdown blocks={blocks} from="" />
      </InterfaceLinks.Provider>
    </MemoryRouter>,
  );
}

describe("a place in this interface, linked from an answer", () => {
  it("is a link the page follows", () => {
    const markup = rendered(true);
    expect(markup).toContain(`href="${ROOM}"`);
    expect(markup).toContain("the review room on g1-s22</a>");
    expect(markup).not.toContain('href="/api/backlog"');
  });

  it("and stays words in a document", () => {
    expect(rendered(false)).not.toContain(`href="${ROOM}"`);
  });
});
