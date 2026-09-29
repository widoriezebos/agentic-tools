import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { LaunchSheet } from "./LaunchSheet";

// The shell's sheet draws through a portal, which server rendering does not
// reach; its body is drawn in place so the form itself can be read.
vi.mock("../shell/Sheet", () => ({
  Sheet: ({ children }: { children: ReactNode }) => <div className="ms-sheet">{children}</div>,
}));

/**
 * The Launch sheet as one screen (g1-s72 D5): signed in is enough.
 *
 * The form is the nickname, the path and what the machine gets. Nothing asks
 * for words or a review date, because the signed-in session is the human's
 * own enrollment; a browser nobody is signed into is answered by the route
 * with the sign-in sheet, not by a field here.
 */

function sheet(): string {
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <LaunchSheet
        machines={[]}
        launches={[]}
        thisSeat="m1u"
        where={{ parent: "/w", repository: "agentic-tools" }}
        onClose={() => undefined}
        onStarted={() => undefined}
      />
    </TooltipPrimitive.Provider>,
  );
}

describe("the Launch sheet", () => {
  it("asks for a nickname and a path, and proposes both", () => {
    const markup = sheet();
    expect(markup).toMatch(/<input[^>]*id="ms-launch-machine"[^>]*value="m1b"/u);
    expect(markup).toMatch(/<input[^>]*id="ms-launch-destination"[^>]*value="\/w\/agentic-tools-m1b"/u);
    expect(markup).toContain("What it gets");
  });

  it("has no authorization section, no words to type and no date to pick", () => {
    const markup = sheet();
    expect(markup).not.toContain("Your authorization");
    expect(markup).not.toContain("ms-launch-word");
    expect(markup).not.toContain("ms-launch-review");
    expect(markup).not.toContain("<textarea");
    expect(markup).not.toContain('type="date"');
    expect(markup).not.toMatch(/review/iu);
    expect(markup).not.toMatch(/temporary/iu);
  });

  it("offers Launch at once on its own proposals", () => {
    const button = /<button[^>]*>Launch<\/button>/u.exec(sheet());
    expect(button).not.toBe(null);
    expect(button?.[0]).not.toContain("disabled");
  });
});
