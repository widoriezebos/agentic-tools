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

export function titleFor(sectionTitle: string, identity: Identity): string {
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
