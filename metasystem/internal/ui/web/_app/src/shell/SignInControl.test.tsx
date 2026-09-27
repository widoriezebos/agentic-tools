import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { IdentityAs } from "./identity";
import type { SessionState } from "./session";
import { SignInControl } from "./SignInControl";

/**
 * What the header says about a sign-in that succeeded and took something with
 * it (Astra A-03).
 *
 * The first sign-in on a seat that had named nobody moves that seat's
 * conversation to the human. The sign-in stands whether or not the move does, and
 * the move can fail: the session is real, the acts are this human's, and the
 * transcript is still on the seat. The server says so in its own words beside the
 * session it answered, and they are shown — the sheet cannot show them, because
 * it closes the moment the sign-in lands, so they would be lost there.
 *
 * It is read from the markup because that is where the claim is: the line is
 * either rendered beside the handle or it is not.
 */

const MOVED = "signed in, but the conversation on this seat could not be moved to you: no such file";

function acting(over: Partial<SessionState> = {}): string {
  return renderToStaticMarkup(
    <IdentityAs
      held={{
        session: {
          state: "known",
          session: { human: "Wido", signedIn: true, until: "2026-09-27T18:00:00Z", source: "code", ...over },
        },
      }}
    >
      <SignInControl />
    </IdentityAs>,
  );
}

describe("the identity control's own notice", () => {
  it("shows what went wrong alongside the sign-in, in the server's words", () => {
    const markup = acting({ trouble: MOVED });

    expect(markup).toContain(MOVED);
    // In the class this control already says a refusal in, and announced.
    expect(markup).toContain('class="ms-signin-refusal"');
    expect(markup).toContain('role="alert"');
    // And the handle is still there: the sign-in succeeded.
    expect(markup).toContain("Wido");
  });

  it("shows no notice where the sign-in answered none", () => {
    const markup = acting();

    expect(markup).not.toContain("ms-signin-refusal");
    expect(markup).not.toContain('role="alert"');
    expect(markup).toContain("Wido");
  });
});
