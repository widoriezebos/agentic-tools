/**
 * The tab title. A human with six workspaces open reads the title bar before
 * anything else, so the subject and the mode are in it, and a workspace whose
 * identity is not known says so rather than guessing.
 */

export const FALLBACK_TITLE = "MetaSystem interface";

export type Identity =
  | { state: "loading" }
  | { state: "unknown" }
  | { state: "known"; subject: string; mode: "self-hosted" | "adopted"; conflict: boolean };

/**
 * What is waiting, in front of everything else.
 *
 * A tab that is not the one in front is a strip of text a few characters wide,
 * and the count has to survive being cut off there — so it leads, the way a
 * mail client's does, and it is the whole of what the title says about
 * notifications. Nothing unread adds nothing: a title that always carried a
 * bracket would teach a human to stop reading the front of it.
 */
export function unreadPrefix(unread: number): string {
  return unread > 0 ? `(${String(unread)}) ` : "";
}

export function titleFor(sectionTitle: string, identity: Identity, unread = 0): string {
  return unreadPrefix(unread) + subjectTitle(sectionTitle, identity);
}

function subjectTitle(sectionTitle: string, identity: Identity): string {
  if (identity.state === "loading") {
    return FALLBACK_TITLE;
  }
  if (identity.state === "unknown") {
    return `${sectionTitle} · ${FALLBACK_TITLE}`;
  }
  if (identity.conflict) {
    return `Identity conflict · ${FALLBACK_TITLE}`;
  }
  if (identity.mode === "self-hosted") {
    return `${sectionTitle} · ${identity.subject} · self-hosted`;
  }
  return `${sectionTitle} · ${identity.subject} · built with MetaSystem`;
}
