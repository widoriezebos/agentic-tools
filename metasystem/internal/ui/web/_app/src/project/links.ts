/**
 * The belt.
 *
 * The engine already classified every link, and the policy already refuses an
 * inline script and a javascript: navigation. This is the third wall, and it
 * is here because it is the cheapest of the three: a link becomes an anchor
 * only when this file agrees, and everything else is rendered as text.
 *
 * The rules are the reader's own, written again: an external link is only the
 * two spellings a browser may open, and a document link resolves against the
 * directory of the document it was written in, stays beneath the checkout, is
 * Markdown, and names no directory the route refuses.
 */

/** The directories the route never serves, whatever the letter case. */
const REFUSED = [".git", "node_modules", "artifacts", "bin"];

/** The href a new tab may be opened with, or null for anything else. */
export function externalHref(href: string): string | null {
  return href.startsWith("http://") || href.startsWith("https://") ? href : null;
}

/** The heading an in-page link points at, or null. */
export function fragmentOf(href: string): string | null {
  return href.startsWith("#") && href.length > 1 ? href.slice(1) : null;
}

/**
 * The id of the document a relative href names, or null when this build would
 * not serve it.
 */
export function documentIdFor(from: string, href: string): string | null {
  const target = href.split("#")[0].split("?")[0];
  if (target === "" || target.startsWith("/") || hasScheme(target)) {
    return null;
  }
  const resolved = resolve(directoryOf(from), decode(target));
  return resolved !== null && admissible(resolved) ? resolved : null;
}

function decode(target: string): string {
  try {
    return decodeURIComponent(target);
  } catch {
    return target;
  }
}

function directoryOf(id: string): string[] {
  const segments = id.split("/");
  segments.pop();
  return segments;
}

/** Joins a relative path onto a directory, or null when it climbs out. */
function resolve(directory: string[], target: string): string | null {
  const segments = [...directory];
  for (const segment of target.split("/")) {
    if (segment === "" || segment === ".") {
      continue;
    }
    if (segment === "..") {
      if (segments.length === 0) {
        return null;
      }
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return segments.length === 0 ? null : segments.join("/");
}

function admissible(id: string): boolean {
  if (!id.endsWith(".md")) {
    return false;
  }
  return id.split("/").every((segment) => !REFUSED.includes(segment.toLowerCase()));
}

/** A colon before the first slash is a scheme, which a relative path has not. */
function hasScheme(href: string): boolean {
  for (const [index, character] of [...href].entries()) {
    if (character === ":") {
      return index > 0;
    }
    if (!/[A-Za-z0-9+\-.]/.test(character) || (index === 0 && !/[A-Za-z]/.test(character))) {
      return false;
    }
  }
  return false;
}
