/**
 * The key one send is asked under.
 *
 * It is here rather than in the provider because it is a decision rather than
 * wiring: the key survives a refusal, so pressing Send again is the same turn
 * rather than a second one. What a question carries is capture.ts's, which is
 * the other half of what used to live here.
 */

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
