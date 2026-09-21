import { describe, expect, it } from "vitest";

import { documentIdFor, externalHref, fragmentOf } from "./links";

/**
 * The belt, tested on its own.
 *
 * These are the rules the reader already applied, written again in the
 * browser, so a link becomes an anchor only when both walls agree. Each case
 * below is one the engine also answers; a disagreement would mean text where
 * there could have been a link, which is the safe direction.
 */

describe("externalHref", () => {
  it("opens the two spellings a browser may open, and nothing else", () => {
    expect(externalHref("https://example.invalid/page")).toBe("https://example.invalid/page");
    expect(externalHref("http://example.invalid/page")).toBe("http://example.invalid/page");
  });

  it("refuses every other scheme, including the ones that look like one", () => {
    for (const href of [
      "javascript:alert(1)",
      "data:text/html,<script></script>",
      "HTTPS://example.invalid",
      "//example.invalid/page",
      "file:///etc/passwd",
      "mailto:someone@example.invalid",
      "",
    ]) {
      expect({ href, opened: externalHref(href) }).toEqual({ href, opened: null });
    }
  });
});

describe("fragmentOf", () => {
  it("names the heading an in-page link points at", () => {
    expect(fragmentOf("#a-heading")).toBe("a-heading");
    expect(fragmentOf("#")).toBeNull();
    expect(fragmentOf("b.md#a-heading")).toBeNull();
  });
});

describe("documentIdFor", () => {
  it("resolves against the directory of the document it was written in", () => {
    expect(documentIdFor("docs/a.md", "b.md")).toBe("docs/b.md");
    expect(documentIdFor("docs/a.md", "./b.md")).toBe("docs/b.md");
    expect(documentIdFor("docs/a.md", "design/c.md")).toBe("docs/design/c.md");
    expect(documentIdFor("docs/design/c.md", "../a.md")).toBe("docs/a.md");
    expect(documentIdFor("a.md", "b.md")).toBe("b.md");
    expect(documentIdFor("docs/a.md", "b.md#a-heading")).toBe("docs/b.md");
    expect(documentIdFor("docs/a.md", "a%20name.md")).toBe("docs/a name.md");
  });

  it("answers an id for a document that is not there, which opens the not-found card", () => {
    expect(documentIdFor("docs/a.md", "gone.md")).toBe("docs/gone.md");
  });

  it("refuses what this build would not serve", () => {
    for (const href of [
      "/etc/passwd.md",
      "../../elsewhere.md",
      "picture.png",
      ".git/config.md",
      ".GIT/config.md",
      "node_modules/x.md",
      "javascript:alert(1)",
      "data:text/plain,hello",
      "https://example.invalid/page",
      "#a-heading",
      "",
    ]) {
      expect({ href, id: documentIdFor("docs/a.md", href) }).toEqual({ href, id: null });
    }
  });
});
