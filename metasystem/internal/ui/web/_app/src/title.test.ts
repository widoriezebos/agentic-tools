import { describe, expect, it } from "vitest";

import { FALLBACK_TITLE, titleFor } from "./title";

describe("titleFor", () => {
  it("names the workspace and its mode when it is self-hosted", () => {
    expect(titleFor("Backlog", { state: "known", subject: "MetaSystem", mode: "self-hosted", conflict: false })).toBe(
      "Backlog · MetaSystem · self-hosted",
    );
  });

  it("names an adopted workspace by what built it", () => {
    expect(titleFor("Fleet", { state: "known", subject: "Ledger", mode: "adopted", conflict: false })).toBe(
      "Fleet · Ledger · built with MetaSystem",
    );
  });

  it("says nothing else while an identity conflict stands", () => {
    expect(titleFor("Overview", { state: "known", subject: "MetaSystem", mode: "self-hosted", conflict: true })).toBe(
      "Identity conflict · MetaSystem interface",
    );
  });

  it("falls back while the workspace is being read", () => {
    expect(titleFor("Overview", { state: "loading" })).toBe(FALLBACK_TITLE);
  });

  it("keeps the section when the workspace cannot be read", () => {
    expect(titleFor("Decisions", { state: "unknown" })).toBe("Decisions · MetaSystem interface");
  });

  it("carries the not-found pane's own name", () => {
    expect(titleFor("Not found", { state: "known", subject: "Ledger", mode: "adopted", conflict: false })).toBe(
      "Not found · Ledger · built with MetaSystem",
    );
  });

  it("leads with the dot while a turn runs, the unread count kept behind it", () => {
    const known = { state: "known", subject: "MetaSystem", mode: "self-hosted", conflict: false } as const;
    expect(titleFor("Backlog", known, 0, true)).toBe("● Backlog · MetaSystem · self-hosted");
    expect(titleFor("Backlog", known, 3, true)).toBe("● (3) Backlog · MetaSystem · self-hosted");
    expect(titleFor("Backlog", known, 3, false)).toBe("(3) Backlog · MetaSystem · self-hosted");
    expect(titleFor("Backlog", known)).toBe("Backlog · MetaSystem · self-hosted");
  });
});
