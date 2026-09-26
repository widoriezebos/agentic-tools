import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { Drawer } from "./Drawer";
import { attachedDraft, type Attachment } from "../partner/attachments";
import { draftOf } from "../partner/drafting";
import { PartnerAs } from "../partner/store";

/**
 * The closed drawer's bar, and the hand-over it used to hide.
 *
 * Opening a sheet hands its draft over (g1-s52 D1), and a human who never opened
 * the drawer saw nothing of it: the bar held the field and the Seeing line and no
 * chip, so the one attachment that arrives without a press was invisible and its
 * × was unreachable. That is the silent sharing the master's rule forbids, so the
 * chip stands wherever the composer is shown — and the bar is the composer while
 * the drawer is closed.
 *
 * It is read from the markup because that is where the claim is: the chip is
 * either rendered beside that field or it is not.
 */

const EDIT = [
  { name: "Goal", value: "g1-s12" },
  { name: "Intent", value: "The board reads the ledger." },
];

const HANDED: readonly Attachment[] = [
  attachedDraft(draftOf("opening-1", "Edit goal", EDIT, ["Intent", "Next step"], "Intent")),
];

function drawer(attachments: readonly Attachment[], open = false): string {
  return renderToStaticMarkup(
    <MemoryRouter>
      <TooltipPrimitive.Provider>
        <PartnerAs held={{ attachments }}>
          <Drawer
            open={open}
            caret="none"
            onCompose={() => undefined}
            onToggle={() => undefined}
            onEscape={() => undefined}
          />
        </PartnerAs>
      </TooltipPrimitive.Provider>
    </MemoryRouter>,
  );
}

describe("the closed drawer's bar", () => {
  it("shows the handed-over draft as a chip beside the field, with its clear", () => {
    const markup = drawer(HANDED);
    expect(markup).toContain("ms-partner-draft-chip");
    expect(markup).toContain("Draft: Edit goal");
    expect(markup).toContain("writing in Intent");
    // The × is the whole point: a hand-over a human cannot take back is not an
    // offer. It says what it will do in the words the act itself would use.
    expect(markup).toContain('aria-label="Stop offering the Edit goal sheet"');
    // And it stands beside the composer that is on screen, not instead of it.
    expect(markup).toContain('id="drawer-composer"');
  });

  it("shows no chip where nothing has been handed over", () => {
    const markup = drawer([]);
    expect(markup).not.toContain("ms-partner-subject-chip");
    expect(markup).not.toContain("Stop offering");
    expect(markup).toContain('id="drawer-composer"');
  });

  /**
   * One chip, not two. Open, the bar has no composer of its own and the panel's
   * composer card carries the chip, which is where it always stood.
   */
  it("leaves the chip to the composer card once the drawer is open", () => {
    const markup = drawer(HANDED, true);
    expect(markup).not.toContain('id="drawer-composer"');
    expect(markup.split("ms-partner-draft-chip").length - 1).toBe(1);
  });
});
