# g1-s23 Sign in with your code

- Kind: design
- Id: 01M35FP24C2R60PRGEPA9DD9Z4
- Status: accepted
- Goals: browser-interface
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-22, Claude on Fable, on Wido's ruling: "we should be able to enroll from the user interface, leveraging the TOTP functionality ... For now, we don't have a login mechanism, so it is fine to assume a user ... use the TOTP that we have in place." Built as the smallest thing that works: one secret, one human, the board's four acts. The master design's rule is "Human acts from the browser" in [user-interface-design.md](../user-interface-design.md); this page is what step 1 of it does and does not do.

## Outcome

A human signs into the interface with the seat's one-time code, the same TOTP the fleet channel verifies, and from then on the board's acts (approve, withdraw, priority, new goal) publish under that human's word. The ledger says so: the approval record reads `authority=session` and the History line carries `authorityOutcome=SIGNED_IN_SESSION` with the issuer, the handle and an opaque session reference, through the History keys that already existed. No terminal enrollment, no per-act code.

## Rollout

None. Wido, 2026-09-22: "there is no active seat anywhere, and before any of them becomes active, they will have rebuilt the binary and this will be in there ... there's nothing to migrate." The master design's two-step reader rollout was written for a fleet with live readers; there is none, so no flag guards the writer.

## The engine (commit 563ce1673)

- `humanauthority`: outcome `SIGNED_IN_SESSION`; `SignedInSessionProof(root, user, sessionRef, issuer, now)` mints an observed, in-process proof bound to its root; `SessionValidFor(root)` is its predicate. `Valid`/`ValidFor` are untouched: a session is not an enrolled terminal.
- `goal`: `ApprovalAuthoritySession = "session"` admitted at the shared approval gate (`approvalProofClass`) and at set-priority, so a session reaches everything that gate serves: approve, unapprove, open with a tier override, budget approval, the sweep, grants and revocations. That is the master design's rule, "full human standing ... no browser-specific table of verbs".
- Readers: the approval-record validator, the History parser and renderer, the attention projection and the refusal register all know the outcome. No new History key; a session line refuses `channelStep` and `channelContext`.

## The interface (commit 30f0e84e6)

- `POST /api/session/sign-in {code[, human]}` verifies the code against `channel.human.totp-secret` read through the config reader only (environment, then `metasystem.conf.local`; a committed value is refused). The channel's replay rule holds: a step is spent once, and the floor outlives a restart in `artifacts/agents/ui/sessions.json`, not the state record, which every clean shutdown removes.
- The cookie is HttpOnly, SameSite=Strict, Path=/; sessions are in memory and last `ui.session.hours` (default 12). `GET /api/session` answers `{human, signedIn, until, source: code|terminal|none}`; `POST /api/session/sign-out` ends one. Five wrong codes from one client pause it for a minute. `ui status` prints the live sessions.
- The human: the boot proof's name when an enrolled terminal started the server; else `ui.human` from `metasystem.conf`; else the sheet asks once and the seat keeps the answer. Handles are one word, because a History line is whitespace-separated tokens.
- Acts prefer the session, then the boot proof, else answer 403 with `signIn: true`; the interface opens the sheet and retries the act once. Session lineage is `browser-session`, never the terminal's.

## Sol's review and the dispositions

Sol reviewed both commits ([g1-s23-sign-in-sol-review.md](g1-s23-sign-in-sol-review.md)) and rejected. Accepted and fixed in commit c2f51b08a: finding 1 (the cookie's bearer was the session reference written to History; the proof now carries a separate public reference and the cookie alone holds the bearer), finding 4 (the replay floor failed open on a storage error; sign-in now refuses when the floor cannot be written or read), finding 5 (withdraw, priority, grant and revoke did not record the session source; they do, with existing keys), and the second half of finding 2 (`act.SignedIn` now checks its human and session against the proof). Kept as designed: a seat that knows no human takes the handle from the first valid-code sign-in and keeps it, which is the "assume a user" Wido asked for.

Ruled outside step 1, on R-121:

- Finding 3: `AuthorizesResume` and `AuthorizesSetObligation` still refuse a session. The interface offers neither act; the master design's session-stop presence binding and carry generation come with them.
- Finding 6: the one-word handle stays until the encoded identity the master design promises; a spaced name would publish a line older readers reject.

## Later, when it hurts

- Resume and set-obligation under a session, with the presence binding the master design describes.
- An encoded human identity separate from the display name; spaced names; two seats with one handle.
- More than one human per seat: a secret per human, and the sign-in system the master design defers to.
- `Secure` on the cookie when the interface leaves loopback; two servers on different state roots sharing one secret keep independent replay floors.
