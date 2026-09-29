/**
 * The workspace resource, and the one place this build talks to the server.
 *
 * It is read once on load and again on Retry, and nowhere else: no stream, no
 * socket, no timer, and no window event refetches it. src/cuts.test.ts holds
 * that by counting call sites, so a second request written under any other
 * name fails the guard rather than the review.
 */

export type WorkspaceMode = "self-hosted" | "adopted";

/**
 * One workspace's share of the private store, as the server measured it: the
 * directory's own key, what it holds in words, and whether it is the one in
 * front of the human.
 */
export type StoreWorkspace = {
  name: string;
  size: string;
  current?: boolean;
};

/**
 * The private store the interface keeps outside every checkout: where it is,
 * what it holds per workspace, and the bounds it is kept to, in sentences.
 * The server composes the words, because the numbers are its configuration and
 * the sizes are its own walk of a directory this page cannot see.
 */
export type Store = {
  path: string;
  workspaces?: StoreWorkspace[];
  bounds?: string[];
  problem?: string;
};

export type Workspace = {
  schemaVersion: number;
  subject: string;
  mode: WorkspaceMode;
  conflict: boolean;
  checkout: string;
  installation: string;
  stateRoot: string;
  engineBuild: string;
  startedAt: string;
  executableDigest: string;
  sourceHead: string;
  adoptedFrom: string;
  adoptionRecord: string;
  store?: Store;
  /** The landing gate's two settings, each with its source (g1-s70 D1). */
  landingGate?: LandingGate;
};

/** One setting as the layered resolution answered it. */
export type LandingGateFact = { key: string; value: string; source: string };

/** The landing gate's settings, or why they could not be read. */
export type LandingGate = { facts?: LandingGateFact[]; problem?: string };

export async function loadWorkspace(signal?: AbortSignal): Promise<Workspace> {
  const response = await fetch("/api/workspace", { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    throw new Error(`/api/workspace answered ${String(response.status)}`);
  }
  return (await response.json()) as Workspace;
}

/** What went wrong, in one line a human can act on. */
export function failureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/**
 * The code a refusal was made under, where the error carries one, or "". It is
 * what a trouble line carries beside the sentence (g1-s68 D1), so the Partner
 * can read the register's row for it.
 */
export function failureCode(error: unknown): string {
  if (typeof error === "object" && error !== null && "code" in error) {
    const code = (error as { code: unknown }).code;
    return typeof code === "string" ? code : "";
  }
  return "";
}
