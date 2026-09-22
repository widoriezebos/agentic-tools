import { useState } from "react";

import { Button, Skeleton } from "./controls";
import { useSession } from "./identity";
import { controlFor, signOut } from "./session";

/**
 * Who the server is acting as, in the header, beside who the workspace is.
 *
 * Signed in, it says the handle and when the session stops, because a session
 * that will expire is one a human should be able to see expiring. A server
 * the terminal proved says so and offers no sign-out: there is nothing to
 * sign out of, and the way to end it is to stop the server.
 */
export function SignInControl() {
  const { session, askToSignIn, settled } = useSession();
  const [leaving, setLeaving] = useState(false);
  const control = controlFor(session);

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
