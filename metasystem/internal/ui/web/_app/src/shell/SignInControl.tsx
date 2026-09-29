import { useState } from "react";

import { Button, Skeleton } from "./controls";
import { useSession } from "./identity";
import { controlFor, signOut } from "./session";
import { Trouble } from "./Trouble";

/**
 * Who the server is acting as, in the header, beside who the workspace is.
 *
 * Signed in, it says the handle and when the session stops, because a session
 * that will expire is one a human should be able to see expiring. A server
 * the terminal proved says so and offers no sign-out: there is nothing to
 * sign out of, and the way to end it is to stop the server.
 *
 * And it carries the one thing a sign-in can answer besides who is acting: what
 * went wrong alongside it. The first sign-in on a seat that had named nobody
 * moves that seat's conversation to the human, and a move that failed leaves the
 * session standing and the transcript behind. The sheet cannot say so — it
 * closes the moment the sign-in lands — so the words stand here, beside the
 * handle they are about, for as long as the session that answered them
 * (Astra A-03).
 */
export function SignInControl() {
  const { session, askToSignIn, settled } = useSession();
  const [leaving, setLeaving] = useState(false);
  const control = controlFor(session);
  const trouble = session.state === "known" ? (session.session.trouble ?? "") : "";

  if (control.kind === "loading") {
    return (
      <span className="ms-signin">
        <Skeleton />
      </span>
    );
  }

  if (control.kind === "sign-in") {
    return (
      <span className="ms-signin">
        <Button
          onClick={() => {
            askToSignIn();
          }}
        >
          Sign in
        </Button>
      </span>
    );
  }

  return (
    <span className="ms-signin">
      {trouble !== "" && (
        <Trouble text={trouble} role="alert" as="span" variant="small" />
      )}
      <span className="ms-signin-who">{control.who}</span>
      <span className="ms-signin-until">· {control.qualifier}</span>
      {control.canSignOut && (
        <Button
          disabled={leaving}
          onClick={() => {
            setLeaving(true);
            signOut()
              .then((state) => {
                setLeaving(false);
                settled(state);
              })
              .catch(() => {
                // A sign-out the server did not answer leaves the page saying
                // what it last knew, which is the truthful thing to say.
                setLeaving(false);
              });
          }}
        >
          Sign out
        </Button>
      )}
    </span>
  );
}
