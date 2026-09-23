import type { Page } from "./api";
import { activeSection } from "../routes";
import type { Subject } from "../shell/about";

/**
 * What a question carries with it, and the key it is asked under.
 *
 * Both are here rather than in the provider because both are decisions rather
 * than wiring: where the human was is captured at the moment of asking, so
 * navigating afterwards cannot retarget a question already sent; and the key
 * survives a refusal, so pressing Send again is the same turn rather than a
 * second one.
 */

/** Where the human is, as the server needs it: the page, and its subject. */
export function pageOf(pathname: string, subject: Subject): Page {
  const section = activeSection(pathname);
  const page: Page = { section: section?.title ?? "", path: pathname };
  if (subject.tab !== undefined && subject.tab !== "") {
    page.tab = subject.tab;
  }
  if (subject.kind !== undefined && subject.kind !== "") {
    page.kind = subject.kind;
  }
  if (subject.subject !== undefined && subject.subject !== "") {
    page.subject = subject.subject;
  }
  if (subject.title !== undefined && subject.title !== "") {
    page.title = subject.title;
  }
  if (subject.revision !== undefined && subject.revision !== "") {
    page.revision = subject.revision;
  }
  if (subject.filters !== undefined && subject.filters.length > 0) {
    page.filters = subject.filters;
  }
  return page;
}

/**
 * The key one send is asked under: the one already held where a previous send
 * was refused, and a fresh one otherwise.
 *
 * It is the page's, not the server's, because its whole purpose is to survive
 * an answer this page never received: a send whose response was lost is
 * retried with the same key, and the server answers the same turn rather than
 * asking the Partner twice.
 */
export function keyFor(held: string, mint: () => string = mintKey): string {
  return held === "" ? mint() : held;
}

/** One key. The browser's own generator where there is one. */
export function mintKey(): string {
  const random = globalThis.crypto as { randomUUID?: () => string } | undefined;
  if (random?.randomUUID !== undefined) {
    return random.randomUUID();
  }
  return `k-${String(Date.now())}-${Math.random().toString(36).slice(2)}`;
}
