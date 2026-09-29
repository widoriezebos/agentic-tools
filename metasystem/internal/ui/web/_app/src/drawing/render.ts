import mermaid from "mermaid";

import { carried, CARRIED_STYLE } from "./drawings";
import { readNonce } from "../nonce";
import { THEME_ATTRIBUTE } from "../theme";

/**
 * The drawing chunk (g1-s71 D2): mermaid, loaded only when a drawing is on
 * screen, turning one fence's source into a picture under the page's policy.
 *
 * The policy allows a style element only with this response's nonce and a style
 * attribute not at all, and mermaid writes both, three ways: into the live page
 * while it measures, into inert documents when it sanitizes, and into the markup
 * it answers. Each is met where it happens, for the length of one render and no
 * longer:
 *
 * - a style element the library creates carries the nonce before it is placed;
 * - a style attribute it sets is set through the element's own style object,
 *   which the policy allows, and not as markup, which it refuses;
 * - markup it parses (its sanitizer's) is read with every style attribute
 *   carried under a data attribute and every style element carrying the nonce,
 *   so the parse is refused nothing and the sanitizer still sees every node.
 *
 * The answer is placed the same way, and each carried attribute is then given
 * back through the style object. src/drawing/csp.test.ts proves the whole under
 * the header the server sends; src/cuts.test.ts names this file as the one that
 * may place markup, and holds it to reaching nothing.
 *
 * Each is drawn in the theme the page is painted in at that moment; a drawing
 * already on screen keeps its own until it is drawn again.
 *
 * Renders run one at a time, because the three are the page's own methods,
 * changed for a render and put back after it.
 */

let queue: Promise<unknown> = Promise.resolve();

/** Draw one fence's source into the element given, or reject with the parse's words. */
export function renderDrawing(source: string, into: Element, id: string): Promise<void> {
  const next = queue.then(() => drawn(source, into, id));
  queue = next.catch(() => undefined);
  return next;
}

async function drawn(source: string, into: Element, id: string): Promise<void> {
  const nonce = readNonce(globalThis.document);
  // Drawn in the theme the page is painted in when it is drawn.
  const dark = globalThis.document.documentElement.getAttribute(THEME_ATTRIBUTE) === "dark";
  mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
  const { svg } = await guarded(nonce, () => mermaid.render(id, source));
  into.innerHTML = carried(svg, nonce);
  for (const styled of into.querySelectorAll(`[${CARRIED_STYLE}]`)) {
    (styled as HTMLElement | SVGElement).style.cssText = styled.getAttribute(CARRIED_STYLE) ?? "";
    styled.removeAttribute(CARRIED_STYLE);
  }
}

/** Run one render with the page's three methods meeting the policy, and put them back after. */
async function guarded<T>(nonce: string, run: () => Promise<T>): Promise<T> {
  const set = Element.prototype.setAttribute;
  const create = Document.prototype.createElement;
  const createNS = Document.prototype.createElementNS;
  const parse = DOMParser.prototype.parseFromString;
  const stamped = <E extends Element>(element: E): E => {
    if (element.localName === "style") {
      (element as unknown as HTMLStyleElement).nonce = nonce;
    }
    return element;
  };
  Element.prototype.setAttribute = function (this: Element, name: string, value: string) {
    if (name.toLowerCase() === "style" && "style" in this) {
      (this as HTMLElement).style.cssText = String(value);
      return;
    }
    set.call(this, name, value);
  };
  Document.prototype.createElement = function (this: Document, ...named: Parameters<Document["createElement"]>) {
    return stamped(create.apply(this, named));
  } as Document["createElement"];
  Document.prototype.createElementNS = function (this: Document, ...named: [string | null, string]) {
    return stamped(createNS.apply(this, named));
  } as Document["createElementNS"];
  DOMParser.prototype.parseFromString = function (this: DOMParser, markup: string, type: DOMParserSupportedType) {
    return parse.call(this, carried(String(markup), nonce), type);
  } as DOMParser["parseFromString"];
  try {
    return await run();
  } finally {
    Element.prototype.setAttribute = set;
    Document.prototype.createElement = create;
    Document.prototype.createElementNS = createNS;
    DOMParser.prototype.parseFromString = parse;
  }
}
