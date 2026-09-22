import type { Authority } from "./api";
import type { SessionStatus } from "../shell/session";

/**
 * Whether an act would reach the ledger, and as whom.
 *
 * There are two hands now. The server proves one when it starts, from the
 * ancestry of the process that started it; the other is a browser a human
 * signed into with this seat's one-time code. The board asks the same
 * question of both — is there a human behind this act — so every place that
 * used to read the boot proof reads this instead.
 *
 * While the session is still being read the boot proof answers alone, which
 * is what this page did before sessions existed: a server the terminal proved
 * keeps working from its first paint.
 */
export function actingAs(boot: Authority, status: SessionStatus): Authority {
  if (status.state !== "known") {
    return boot;
  }
  if (status.session.signedIn) {
    return { proven: true, human: status.session.human, reason: "" };
  }
  const found = boot.reason === "" ? UNPROVEN : boot.reason;
  return { proven: false, human: status.session.human, reason: `${found}; ${SIGN_IN}` };
}

/** What a server says when it found no reason of its own to give. */
export const UNPROVEN = "This interface cannot act as a human";

/** The remedy that needs no terminal, which is the one this page offers. */
export const SIGN_IN = "sign in with this seat's one-time code to act as yourself";
