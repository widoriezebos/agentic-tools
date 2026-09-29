import { mkdtempSync, readFileSync, rmSync, existsSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, type Browser } from "playwright";
import { build } from "vite";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

/**
 * D2's proof, first (g1-s71): a mermaid render under the policy header this
 * server really sends carries the page's nonce on every style element it
 * writes, and writes no style attribute the policy would refuse — or it is not
 * built at all.
 *
 * The policy is read out of internal/ui/httpd/httpd.go, the constants the
 * server composes every page's header from, so a change there is a change
 * here. The drawing chunk is built by Vite from the interface's own render
 * module, served from this origin, and drawn in Chromium; the browser's own
 * securitypolicyviolation reports are the witness, because a refused style is
 * silent on the page and loud only there. A drawing that is refused nothing but
 * is unstyled would pass that alone, so the styles are also read back as the
 * browser applied them.
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const APP = path.resolve(HERE, "..", "..");
const HTTPD = path.resolve(APP, "..", "..", "httpd", "httpd.go");

/** The policy's two halves, as the Go constants spell them. */
export function policyOf(go: string): { head: string; tail: string } {
  const head = /policyHead\s*=\s*"([^"]*)"/u.exec(go)?.[1];
  const tail = /policyTail\s*=\s*"([^"]*)"/u.exec(go)?.[1];
  if (head === undefined || tail === undefined) {
    throw new Error(`${HTTPD} no longer spells policyHead and policyTail`);
  }
  return { head, tail };
}

const NONCE = "ZHJhd2luZy1wcm9vZi1ub25jZQ";

const SEQUENCE = [
  "sequenceDiagram",
  "  participant O as Owner",
  "  participant H as Holder",
  "  O->>H: take the lock",
  "  Note over O,H: taken twice",
  "  H-->>O: done",
].join("\n");

const FLOW = ["flowchart LR", "  a[Start] --> b{Held?}", "  b -->|yes| c[Release]", "  style c fill:#f9f"].join("\n");

let dir = "";
let server: Server | null = null;
let browser: Browser | null = null;
let address = "";

beforeAll(async () => {
  const { head, tail } = policyOf(readFileSync(HTTPD, "utf8"));
  dir = mkdtempSync(path.join(tmpdir(), "drawing-csp-"));
  const out = path.join(dir, "dist");
  await build({
    configFile: false,
    logLevel: "silent",
    root: APP,
    build: {
      outDir: out,
      emptyOutDir: true,
      assetsInlineLimit: 0,
      modulePreload: false,
      rolldownOptions: {
        input: path.join(HERE, "render.ts"),
        preserveEntrySignatures: "exports-only",
        output: { entryFileNames: "render.js", chunkFileNames: "chunks/[name]-[hash].js", format: "es" },
      },
    },
  });
  const page = [
    "<!doctype html><html><head><meta charset=\"utf-8\">",
    `<meta property="csp-nonce" nonce="${NONCE}"></head>`,
    "<body><div id=\"drawings\"></div><script type=\"module\" src=\"/harness.js\"></script></body></html>",
  ].join("");
  // The page's one module: it draws each source into a holder of its own,
  // through the chunk exactly as the interface loads it.
  const harness = [
    "import { renderDrawing } from \"./render.js\";",
    "globalThis.drawAll = async (sources, into) => {",
    "  for (const [at, source] of sources.entries()) {",
    "    const holder = globalThis.document.createElement(\"div\");",
    "    into.append(holder);",
    "    await renderDrawing(source, holder, `proof-${String(at)}`);",
    "  }",
    "};",
  ].join("\n");
  server = createServer((request, response) => {
    const asked = new URL(request.url ?? "/", "http://drawing.test").pathname;
    if (asked === "/") {
      response.setHeader("Content-Security-Policy", `${head} 'nonce-${NONCE}'${tail}`);
      response.setHeader("Content-Type", "text/html; charset=utf-8");
      response.end(page);
      return;
    }
    if (asked === "/harness.js") {
      response.setHeader("Content-Type", "text/javascript");
      response.end(harness);
      return;
    }
    const file = path.join(out, path.normalize(asked));
    if (!file.startsWith(out) || !existsSync(file)) {
      response.statusCode = 404;
      response.end();
      return;
    }
    response.setHeader("Content-Type", "text/javascript");
    response.end(readFileSync(file));
  });
  await new Promise<void>((done) => {
    server?.listen(0, "127.0.0.1", () => {
      done();
    });
  });
  const bound = server.address();
  address = typeof bound === "object" && bound !== null ? `http://127.0.0.1:${String(bound.port)}/` : "";
  browser = await chromium.launch();
}, 180_000);

afterAll(async () => {
  await browser?.close();
  await new Promise<void>((done) => {
    if (server === null) {
      done();
      return;
    }
    server.close(() => {
      done();
    });
  });
  if (dir !== "") {
    rmSync(dir, { recursive: true, force: true });
  }
});

type Drawn = {
  violations: string[];
  svgs: number;
  styleNonces: string[];
  actorFill: string;
  releaseFill: string;
  maxWidth: string;
  failed: string;
};

async function drawn(): Promise<Drawn> {
  if (browser === null) {
    throw new Error("no browser was launched");
  }
  const page = await browser.newPage();
  try {
    await page.addInitScript(() => {
      const held: string[] = [];
      (globalThis as unknown as { violations: string[] }).violations = held;
      globalThis.addEventListener("securitypolicyviolation", (event) => {
        held.push(`${event.violatedDirective}: ${event.sample}`);
      });
    });
    await page.goto(address);
    await page.waitForFunction(() => "drawAll" in globalThis);
    return await page.evaluate(
      async ({ sources }) => {
        const drawings = globalThis.document.getElementById("drawings");
        let failed = "";
        try {
          await (globalThis as unknown as { drawAll: (sources: string[], into: Element | null) => Promise<void> }).drawAll(
            sources,
            drawings,
          );
        } catch (error) {
          failed = String(error);
        }
        const fillOf = (selector: string) => {
          const found = globalThis.document.querySelector(selector);
          return found === null ? "" : getComputedStyle(found).fill;
        };
        const svg = globalThis.document.querySelector("#drawings svg");
        return {
          violations: (globalThis as unknown as { violations: string[] }).violations,
          svgs: globalThis.document.querySelectorAll("#drawings svg").length,
          styleNonces: [...globalThis.document.querySelectorAll("#drawings style")].map((style) => (style as HTMLStyleElement).nonce ?? ""),
          actorFill: fillOf("#drawings rect.actor"),
          releaseFill: fillOf("#drawings g.node[id*='-c-'] rect"),
          maxWidth: svg === null ? "" : getComputedStyle(svg).maxWidth,
          failed,
        };
      },
      { sources: [SEQUENCE, FLOW] },
    );
  } finally {
    await page.close();
  }
}

describe("a drawing under the policy the server sends (g1-s71 D2)", () => {
  it("reads the policy from the server's own constants", () => {
    const { head, tail } = policyOf(readFileSync(HTTPD, "utf8"));
    expect(head).toBe("default-src 'none'; script-src 'self'; style-src 'self'");
    expect(tail).toContain("img-src 'self'");
  });

  it("draws with the page's nonce on every style it writes, and is refused nothing", async () => {
    const seen = await drawn();
    expect(seen.failed).toBe("");
    expect(seen.svgs).toBe(2);
    expect(seen.violations).toEqual([]);
    expect(seen.styleNonces.length).toBeGreaterThan(0);
    expect(seen.styleNonces.every((nonce) => nonce === NONCE)).toBe(true);
    // The theme's stylesheet applied: an actor is filled from it, not black.
    expect(seen.actorFill).not.toBe("");
    expect(seen.actorFill).not.toBe("rgb(0, 0, 0)");
    // The diagram's own style line applied, and the style the library sets on
    // the picture itself.
    expect(seen.releaseFill).toBe("rgb(255, 153, 255)");
    expect(seen.maxWidth).not.toBe("none");
  }, 120_000);
});
