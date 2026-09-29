import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Notification } from "./notifications";
import { Toast } from "./Toasts";
import { TroublesAs } from "../shell/troubles";
import { ASK_WHAT_HAPPENED } from "../shell/troubling";

/**
 * A failure toast is a trouble line too (Sol SOL-S68-04): the message a human
 * sees flash up carries its own Ask beside it, and the toast still opens the
 * panel at that message.
 */

const ASK = `<button type="button" class="ms-trouble-ask">${ASK_WHAT_HAPPENED}</button>`;

function notification(over: Partial<Notification> = {}): Notification {
  return {
    id: "n1", at: "2026-09-29T03:12:00Z", message: "HEALTH unhealthy: the sync seat stopped", source: "alert",
    ref: "episode-7", delivered: true, error: "", ...over,
  };
}

function toast(shown: Notification): string {
  return renderToStaticMarkup(
    <TooltipPrimitive.Provider>
      <TroublesAs held={{ ask: () => {} }}>
        <Toast notification={shown} onOpen={() => undefined} onClose={() => undefined} />
      </TroublesAs>
    </TooltipPrimitive.Provider>,
  );
}

describe("a failure toast", () => {
  it("offers Ask what happened beside an alert's message, and still opens the panel", () => {
    const markup = toast(notification());
    expect(markup).toContain("HEALTH unhealthy: the sync seat stopped");
    expect(markup).toContain(ASK);
    expect(markup).toMatch(/<button type="button" class="ms-toast-body"[^>]*>/u);
    // The control is not inside the panel-opening button: a button in a button is none.
    expect(markup).not.toMatch(/class="ms-toast-body"(?:(?!<\/button>).)*ms-trouble-ask/u);
  });

  it("offers it beside a delivery that failed", () => {
    const markup = toast(notification({ source: "steward", message: "the steward ran", delivered: false, error: "osascript refused" }));
    expect(markup).toContain("not delivered to macOS: osascript refused");
    expect(markup).toContain(ASK);
  });

  it("offers none on the steward's narration", () => {
    const markup = toast(notification({ source: "steward", message: "the steward ran" }));
    expect(markup).not.toContain(ASK);
  });
});
