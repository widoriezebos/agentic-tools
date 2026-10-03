import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { AboutFacts, LandingGateFacts, StoreFacts } from "./Settings";
import { dateAndTime } from "../backlog/format";
import type { Store, Workspace } from "../shell/workspace";

/**
 * What Settings tells a human about the private store (g1-s54 D3).
 *
 * A human asked "what about automatic cleanup? I do not want this to grow and
 * grow until it fills up the file system." The answer they get is on this card:
 * where the store is, what each workspace in it costs, and what it is kept to —
 * in the server's own sentences, because the numbers are its configuration and
 * the sizes are its walk of a directory this page cannot see.
 */
describe("the private store's lines", () => {
  const store: Store = {
    path: "/Users/one/.metasystem/ui",
    workspaces: [
      { name: "agentic-tools-0a1b2c", size: "1.2 MB", current: true },
      { name: "another-checkout-d4e5f6", size: "49 KB" },
    ],
    bounds: [
      "The Partner's wire journal is rotated at 8 MB, keeping one previous.",
      "A conversation is trimmed from its oldest end when it passes 2 MB or its oldest message is more than 90 days old, and the last 200 messages are always kept.",
      "Nothing else here is ever removed, and no record of your project is: the records are the memory that survives.",
    ],
  };

  it("says where the store is", () => {
    const markup = renderToStaticMarkup(<StoreFacts store={store} />);
    expect(markup).toContain("Kept at");
    expect(markup).toContain("/Users/one/.metasystem/ui");
  });

  it("says the size of every workspace, and which one is this one", () => {
    const markup = renderToStaticMarkup(<StoreFacts store={store} />);
    expect(markup).toContain("This workspace");
    expect(markup).toContain("agentic-tools-0a1b2c");
    expect(markup).toContain("1.2 MB");
    expect(markup).toContain("Another workspace");
    expect(markup).toContain("another-checkout-d4e5f6");
    expect(markup).toContain("49 KB");
  });

  it("says the bounds in words", () => {
    const markup = renderToStaticMarkup(<StoreFacts store={store} />);
    expect(markup).toContain("rotated at 8 MB, keeping one previous");
    expect(markup).toContain("passes 2 MB or its oldest message is more than 90 days old");
    expect(markup).toContain("the last 200 messages are always kept");
    expect(markup).toContain("the records are the memory that survives");
  });

  it("says why there is nothing to report instead of showing an empty card", () => {
    const markup = renderToStaticMarkup(
      <StoreFacts
        store={{
          path: "",
          problem: "this account has no registry home the interface can read, so this seat keeps no private store",
        }}
      />,
    );
    expect(markup).toContain("this seat keeps no private store");
    expect(markup).not.toContain("Another workspace");
  });
});

/**
 * What a person reads on the card, and what stands behind Details for whoever
 * maintains the workspace: a whole build hash and a setting's key.
 */
describe("the facts a person reads", () => {
  const workspace: Workspace = {
    schemaVersion: 1,
    subject: "agentic-tools",
    mode: "self-hosted",
    conflict: false,
    checkout: "/Users/one/agentic-tools",
    installation: "/Users/one/agentic-tools/metasystem",
    stateRoot: "/Users/one/agentic-tools/.metasystem",
    engineBuild: "a0159fd3c1e24b7788aa90c1d2e3f4a5b6c7d8e9",
    startedAt: "2026-10-02T21:25:07Z",
    executableDigest: "",
    sourceHead: "",
    adoptedFrom: "",
    adoptionRecord: "absent",
  };
  /** The markup with every Details disclosure taken out: what is read without opening one. */
  const read = (markup: string) => markup.replace(/<details[\s\S]*?<\/details>/g, "");

  it("shows the engine build short, and whole under Details", () => {
    const markup = renderToStaticMarkup(<AboutFacts workspace={workspace} />);
    expect(read(markup)).toContain("a0159fd");
    expect(read(markup)).not.toContain(workspace.engineBuild);
    expect(markup).toContain("<summary>Details</summary>");
    expect(markup).toContain(workspace.engineBuild);
  });

  it("shows a build that is not a commit hash whole, with no Details", () => {
    for (const engineBuild of ["walkthrough", "v1.4.2-rc1"]) {
      const markup = renderToStaticMarkup(<AboutFacts workspace={{ ...workspace, engineBuild }} />);
      expect(read(markup)).toContain(`>${engineBuild}<`);
      expect(markup).not.toContain("<summary>Details</summary>");
    }
  });

  it("shows when the server started in the reader's own time, and no row that states no fact", () => {
    const markup = renderToStaticMarkup(<AboutFacts workspace={workspace} />);
    expect(markup).toContain(dateAndTime(workspace.startedAt));
    expect(markup).not.toContain(workspace.startedAt);
    expect(markup).not.toContain("Source at HEAD");
  });

  it("names a landing setting by its plain label, with its key under Details", () => {
    const markup = renderToStaticMarkup(
      <LandingGateFacts
        gate={{
          facts: [
            { key: "landing.review.human-from-tier", value: "2", source: "conf" },
            { key: "landing.review.later", value: "on", source: "default" },
          ],
        }}
      />,
    );
    expect(read(markup)).toContain("Waits for a person from tier");
    expect(read(markup)).toContain("Another landing setting");
    expect(read(markup)).not.toContain("landing.review");
    expect(markup).toContain("landing.review.human-from-tier");
    expect(markup).toContain("landing.review.later");
  });
});
