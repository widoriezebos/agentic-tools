/**
 * The workspace resource, and the one place this build talks to the server.
 *
 * It is read once on load and again on Retry, and nowhere else: no stream, no
 * socket, no timer, and no window event refetches it. src/cuts.test.ts holds
 * that by counting call sites, so a second request written under any other
 * name fails the guard rather than the review.
 */

export type WorkspaceMode = "self-hosted" | "adopted";

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
};

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
