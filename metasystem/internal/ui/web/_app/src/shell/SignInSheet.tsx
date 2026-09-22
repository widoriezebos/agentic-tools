import { useState } from "react";

import { Button } from "./controls";
import { blockedForSignIn, signIn, SIGN_IN_NOTE, type SessionState, type SessionStatus } from "./session";
import { Sheet } from "./Sheet";
import { failureMessage } from "./workspace";

/**
 * Signing in, with the code this seat already gives its human.
 *
 * Six digits, from the same one-time code the fleet channel asks for. The
 * name field is here only when the server said it knows nobody: a seat with a
 * human of its own never takes one from a browser, and a field offering a
 * choice the server would ignore is worse than no field.
 *
 * The sheet is the shell's, so the focus trap, the return of focus to whatever
 * opened it, Escape and the scrim are Radix's and are not written again here.
 */
export function SignInSheet({
  open,
  status,
  needsHandle,
  onOpenChange,
  onSignedIn,
}: {
  open: boolean;
  status: SessionStatus;
  /** Whether the server said it does not know who its human is. */
  needsHandle: boolean;
  onOpenChange: (open: boolean) => void;
  /** What the server answered, which is what the page shows afterwards. */
  onSignedIn: (state: SessionState) => void;
}) {
  const [code, setCode] = useState("");
  const [handle, setHandle] = useState("");
  const [refusal, setRefusal] = useState("");
  const [sending, setSending] = useState(false);

  const blocked = blockedForSignIn(code, handle, needsHandle);

  const close = (next: boolean) => {
    if (!next) {
      setCode("");
      setRefusal("");
    }
    onOpenChange(next);
  };

  const send = () => {
    if (blocked !== "") {
      return;
    }
    setSending(true);
    setRefusal("");
    signIn(code.trim(), needsHandle ? handle.trim() : "")
      .then((state) => {
        setSending(false);
        setCode("");
        onSignedIn(state);
      })
      .catch((error: unknown) => {
        setSending(false);
        setRefusal(failureMessage(error));
      });
  };

  return (
    <Sheet
      open={open}
      onOpenChange={close}
      side="right"
      label="Sign in"
      title="Sign in"
      closeLabel="Close sign in"
      bodyClassName="ms-sheet-body--signin"
    >
      <p className="ms-signin-note">{noteFor(status)}</p>
      {needsHandle && (
        <div className="ms-signin-field">
          <label htmlFor="ms-signin-human">Your name</label>
          <input
            id="ms-signin-human"
            type="text"
            autoComplete="username"
            value={handle}
            placeholder="one word, as the ledger will record it"
            onChange={(event) => {
              setHandle(event.target.value);
            }}
          />
        </div>
      )}
      <div className="ms-signin-field">
        <label htmlFor="ms-signin-code">One-time code</label>
        <input
          id="ms-signin-code"
          type="text"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          value={code}
          placeholder="123456"
          onChange={(event) => {
            setCode(event.target.value);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              send();
            }
          }}
        />
      </div>
      <div className="ms-signin-foot">
        <Button primary disabled={blocked !== "" || sending} onClick={send}>
          Sign in
        </Button>
        <p className="ms-signin-hint">{blocked === "" ? SIGN_IN_NOTE : blocked}</p>
      </div>
      {refusal !== "" && (
        <p className="ms-signin-refusal" role="alert">
          {refusal}
        </p>
      )}
    </Sheet>
  );
}

function noteFor(status: SessionStatus): string {
  if (status.state === "known" && status.session.source === "none" && status.session.human !== "") {
    return `Signing in as ${status.session.human}, with the same one-time code this seat gives the fleet channel.`;
  }
  return "Sign in with the same one-time code this seat gives the fleet channel.";
}
