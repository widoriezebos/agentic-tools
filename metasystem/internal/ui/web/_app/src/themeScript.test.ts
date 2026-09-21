import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { DARK_QUERY, THEME_ATTRIBUTE, THEME_KEY } from "./theme";

const APP_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..");
const script = readFileSync(path.join(APP_DIR, "public", "theme.js"), "utf8");
const page = readFileSync(path.join(APP_DIR, "index.html"), "utf8");

/**
 * The no-flash script is a second implementation of one rule, so the two can
 * drift: this holds them to the same key, the same attribute, and the same
 * query.
 */
describe("public/theme.js", () => {
  it("reads the key and sets the attribute this module names", () => {
    expect(script).toContain(THEME_KEY);
    expect(script).toContain(THEME_ATTRIBUTE);
    expect(script).toContain(DARK_QUERY);
  });

  it("survives a browser that refuses site data", () => {
    expect(script).toContain("try");
    expect(script).toContain("catch");
  });
});

describe("index.html", () => {
  it("loads the script from this origin, in the head, before anything paints", () => {
    const head = page.slice(page.indexOf("<head>"), page.indexOf("</head>"));
    expect(head).toContain('<script src="/theme.js"></script>');
  });

  it("carries no inline script and no style at all", () => {
    for (const fragment of page.toLowerCase().split("<script").slice(1)) {
      expect(fragment.slice(0, fragment.indexOf(">"))).toContain("src=");
    }
    expect(page.toLowerCase()).not.toContain("<style");
    expect(page.toLowerCase()).not.toContain("style=");
  });
});
