import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { Notification } from "./notifications";
import { Row } from "./Panel";

/**
 * A bell row's trouble is the notification's, not the page's (Sol SOL-S68-05):
 * it happened when the notification says, and it is about the notification's
 * own reference. The line is read through what the row hands `Trouble`, since
 * that is where the time and the subject travel from.
 */

vi.mock("../shell/Trouble", () => ({
  Trouble: ({ text, at, subject }: { text: string; at?: string; subject?: { id: string; kind: string } }) => (
    <p data-text={text} data-at={at ?? ""} data-subject={subject?.id ?? ""} data-kind={subject?.kind ?? ""} />
  ),
}));

function notification(over: Partial<Notification> = {}): Notification {
  return {
    id: "n1", at: "2026-09-28T22:04:00Z", message: "HEALTH unhealthy: the sync seat stopped", source: "alert",
    ref: "episode-7", delivered: true, error: "", ...over,
  };
}

describe("a bell row's trouble", () => {
  it("carries the notification's time and reference", () => {
    const markup = renderToStaticMarkup(<Row notification={notification()} focused={false} onAsked={() => undefined} />);
    expect(markup).toContain('data-text="HEALTH unhealthy: the sync seat stopped"');
    expect(markup).toContain('data-at="2026-09-28T22:04:00Z"');
    expect(markup).toContain('data-subject="episode-7"');
  });

  it("and so does its failed delivery", () => {
    const markup = renderToStaticMarkup(
      <Row notification={notification({ source: "steward", delivered: false, error: "osascript refused" })} focused={false} onAsked={() => undefined} />,
    );
    expect(markup).toContain('data-text="not delivered to macOS: osascript refused"');
    expect(markup).toContain('data-at="2026-09-28T22:04:00Z"');
    expect(markup).toContain('data-subject="episode-7"');
  });
});
