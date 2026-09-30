# General power of attorney

- Kind: design
- Id: 01M3RKF89YAESX9K6728NZ3A6Z
- Status: accepted
- Goals: human-controls-goal

Revision 3. It folds Astra's round-1 critique of revision 2 (7 material, 3 deferred) under the coordinator's binding rulings (`agentic-tools-evidence/switch-on-20260930/astra-poa-r1.md`). The loop closes here by Wido's rule, because no finding still changes the build. Author Claude on Opus 5.5. Every cite was read at GitHub main `08fe48f27`. Two build decisions go beyond the rulings, and §11 names both.

## 1. Intent (verbatim)

Wido, 2026-09-30: he wants to grant the seat's agent "elevated privileges … so that you can do everything yourself, literally everything, so that you can test everything yourself", time-bounded, "really easy to use … grant for the next 24 hours or for the next week".

Rulings on revision 1, relayed by m1e:
- The grant lives in the goal ledger's grant store: one grant object, visible and revocable from any seat, audited.
- Compatibility with older engines is not a constraint.
- The log of acts stays local.
- `grant add --acts everything` requires the enrolled-terminal person proof only, never the helm yield.
- Arming and engine enrollment under the grant are allowed.

Rulings on revision 2:
- M1: the grant is bound to the exact checkout path and to the holder lineage recorded at add.
- M2: a plain trust-model paragraph.
- M3: authority-defining settings need direct person proof.
- M4: the grant is re-checked at the effect.
- M5: the injected clock is wired where the wall clock was read.
- M6: actor selection is grant-aware.
- M7: ambiguous or nonexistent local deadlines are refused, and d/w mean elapsed time.
- D3: `--include-delegates` is dropped.

## 2. How it feels

Wido, at his enrolled terminal in the m1e checkout:

    metasystem grant add --acts everything --for 24h
    granted 01M…-m1e: the main session of /Users/wido/…/agentic-tools-m1e acts for wido until 09:14 CEST (2026-10-01); metasystem grant revoke 01M…-m1e ends it

From then on, `metasystem status` in that checkout starts with this line:

    POWER OF ATTORNEY: m1e acts for wido until 09:14 CEST (2026-10-01) (grant 01M…-m1e) — metasystem grant revoke 01M…-m1e ends it

The main agent in that checkout runs `goal approve`, `goal done`, `disk clean`, `system stop`, `system start`, `steward arm` and the rest without `--by`, a relayed word, or the helm. Each admitted act prints one stderr line, `POWER OF ATTORNEY (wido, grant G): <verb> admitted for the seat's main session; recorded in …/power-of-attorney.log`. `grant list` on any seat shows the grant and the time left. After the end time, or after `grant revoke` from any seat, the next act is refused as it is today.

## 3. The verb (unit 1)

**`grant add --acts everything`** takes exactly one of the following:
- `--for N{h,d,w}`, where `h` is hours, `d` is 24 hours and `w` is 168 hours of elapsed time. Examples: `8h`, `24h`, `7d`, `1w`.
- `--until HH:MM`: today, local time. If that time has passed, or is now, it means tomorrow.
- `--until tomorrow`: the end of tomorrow, local time.
- `--until YYYY-MM-DD`: the end of that day, local time.

The end of a day is the next day's local 00:00. A local deadline that does not exist or is ambiguous is refused, never shifted, and the refusal says why ("02:30 does not exist on 2027-03-28 in Europe/Amsterdam"). This covers the spring-forward gap and the autumn repeat. The end is truncated to the minute and stored in UTC. It is at most 168 hours from now, so `--for 1w` is the longest; a longer request is refused, naming the latest local time allowed.

These are all refused with the five examples, and nothing is done:
- neither form, or both;
- `--tiers` given with `everything`;
- `everything` together with another act.

The scoped forms work exactly as today (`cmd/metasystem/intent_planning.go:1546-1591`, `internal/goal/verbs.go:1887-1960`).

**What it binds (M1).** The grant is bound to three things recorded at add time:
- the checkout it is added in: the canonical absolute path of the state root, with symlinks resolved;
- the lineage of the session that holds that checkout's lease at add (`lease.CurrentHolder(root).OwnerLineage`, `internal/lease/verbs.go:337-358`);
- this machine's name (`goalActorWith`, `cmd/metasystem/goalsync_verbs.go:35`).

The add is refused, and nothing is done, when:
- no session holds the lease ("start the seat's session first");
- the path contains whitespace.

Nothing matches by nickname, and no other clone inherits the grant.

**Who may add it.** Only a fresh proof for which `EnrolledTerminalFor(root) && proof.Helm == nil` holds (`internal/humanauthority/authority.go:227`). The owner, `goal.GrantGeneral`, refuses `Helm != nil`, so neither a helm proof nor a grant proof can add, extend or widen a grant.

**One grant object per checkout.** When a live general grant already exists for the checkout, the new one revokes it in the same transaction. A repeat with the same person, checkout, lineage and end while the grant is live is `unchanged` (R-129-ui).

**`grant revoke G`** is today's `goal.Revoke` (`verbs.go:1963-2002`), run by the person from any seat, or by the grantee under its own grant; a repeat is `unchanged`. Locally, the revoke takes the exclusive grant lock (§6) around its publish.

**`grant list`** shows `G  everything  by wido  for the main session of <checkout> on m1e  until 09:14 CEST (2026-10-01)  23h12m left`, and `closed: …` rows under `--all`. Times are shown in local time.

## 4. The ledger record

**Shape.** One entry in the root record's `PowerOfAttorney:` section (`internal/goal/root.go:282-308`, rendered at `:209-215`):

    - <opid> by=human:wido verbs=everything for=m1e checkout=<abs path> lineage=<lineage> since=<RFC3339 UTC> until=<RFC3339 UTC, whole minute> [revoked=… revokedBy=…]

**Parser.** `parseAttorneyEntry` gains the optional keys `for`, `checkout`, `lineage` and `until`:
- A general entry requires all four and forbids `tiers` and `expires`.
- A scoped entry forbids all four.

**Bounds.** `WithinBounds` and `LiveAt` gain a general branch:
- `since < until <= since+168h`, with `until` a whole minute;
- `checkout` is absolute, and `for` and `lineage` are non-empty;
- the entry is live when it is unrevoked and `since <= now < until`, and a zero `now` is never live.

A general entry is never used through `--under`, because `AttorneyVerbs` is unchanged.

**Older engines refuse the ledger.** The record grammar is closed: an unknown key is a parse error (`internal/goal/file.go:1608-1609`), and a problematic root refuses (`root.go:254-255`). **Rebuild all seats before adding a general grant.** The help for `grant add` says so.

## 5. What it admits

**Unit 1: the proof arm.** When the enrolled walk refuses, `proveEnrolled` asks `AtHelm` (`authority.go:912-941`). It now asks `AtAttorney func(root string, invokerPID int64, now time.Time) (HelmGrant, bool)` after the helm. That seam is nil in the library. When it grants, `Prove` returns `HelmProof(root, grant, now)`. `HelmGrant` gains `Grant` and `Until`. A proof shaped like the helm's needs no edit at the sites that already read `proof.Helm`:
- `Valid`;
- actor naming (`goalsync_mutations.go:1427`, `internal/goal/verbs.go:1220`);
- the take's grade (`intent_helm.go:180`);
- force-done (`helm_force.go:18`);
- the interface's boot proof (`internal/ui/act/act.go:128`), which refuses because the interface outlives the grant.

`RecordProof` writes `helm: {by, class, grant, until}`.

**The admitter** is `cmd/metasystem/attorney_admits.go`, with owner seams, wired beside `wireHelmAdmission`. It admits only when all of the following hold; any read, parse or clock error means not admitted:
- `now` is non-zero.
- The root is not in fixture mode.
- The accepted root record parses. That read is one `rev-parse` of the accepted ref and one `cat-file` of the root record, memoized per tip within the process. There is deliberately no on-disk cache: a cached grant would be authority held in a plain file.
- Exactly one live general entry has `for` equal to this machine and `checkout` equal to this root's canonical path.
- The caller classifies `MAIN`, its main id is `CurrentHolder(root).MainId`, and its announcement's lineage is the entry's `lineage`.
- The act's log line is appended. The log is `<root>/artifacts/agents/authority/power-of-attorney.log`, one fsynced line per admitted act naming the grant, the verb, the class, the main id and the pid.

A successor main keeps the lineage and the lease, so it inherits the grant. A delegate announced under the same lineage does not hold the lease, so it does not. Subagents the main runs in process are the main.

**M6: grant-aware actor selection.** `actorArgs` (`intent_goals.go:152-190`, used by pause and done) asks the grant before its lineage shortcut. When no `--by` or `--lineage` is given and the grant admits the caller, the act is the person's: `--by` comes from the grant proof. Otherwise the shortcut is kept, and the helm's behaviour is unchanged.

**M5.** `disk clean`'s person proof (`intent_disk.go:73`) and split ratification (`goalsync_mutations.go:3075`) take `now` from the command's injected clock, not from `time.Now`.

**Unit 2: the class arm.** `lease.ClassifyPersonAt(root, metasystemRoot, caller, now)` returns `ClassifyAt` unchanged unless that result is not `HUMAN` and `humanauthority.AtAttorney` grants. In that case it returns `Classification{Class: ClassHuman, Attorney: &grant}`. The person-act sites swap to it:
- `humanTerminalCheck` (`process_verbs.go:440`: system start/stop, steward arm, engine enrollment);
- the mission fence (`:490`);
- steward session enrollment (`steward_verbs.go:650`);
- `session stop` (`session_stop.go:44`), whose terminal proof accepts the grant's proof when the class came from the grant;
- brain declare/withdraw (`brain.go:72`);
- taint resolution (`internal/missionrunner/resolve.go:41`);
- a person charging a proof run (`proof_run.go:549`, `:1179`, `:1369`).

`ClassifyAt` itself is unchanged, so the grantee's claim, landing and lease renewal still see `MAIN`. The following class sites are left alone because the holder already passes or acts there as the seat:
- the lease gates;
- breach-stop and handoff attribution;
- goal-sync actor attribution.

**Hard limits.** None of these is ever done under a grant:
- **Adding, extending or widening a grant.** The owner refuses `Helm != nil`.
- **`system enroll` and `helm take`.** They use `ProveTerminal`, which has no arm.
- **The interface's boot proof.**
- **The brain** (`goalsync_mutations.go:774`, `:2776`).
- **Authority-defining settings (M3).** `settings set metasystem.runtimes …` requires `EnrolledTerminalFor(root) && proof.Helm == nil`. Today it has no check at all. It is the key that turns fixture mode on, and fixture mode reclassifies callers and overrides the clock.
- **Rewriting the log.** No verb rewrites it.

Arming and engine enrollment are allowed (ruling). An engine enrolled under the grant keeps re-arming after it ends. That is the machinery running, not the agent acting as the person.

**Trust model (M2).** This is a guardrail and an audit trail for a cooperating agent that runs as the same OS user. It is not a security boundary. That user can edit `metasystem.conf.local`, the ledger clone or the log directly, as it can the helm signature today. Its value is:
- no act as the person without a grant only the person can add;
- the grant is fleet-visible and on every `status`;
- every act is logged;
- limits hold for every path the agent is meant to use.

OS-level isolation is out of scope.

## 6. At the effect (M4), and revoke latency

**Ledger acts.** When the admitter admits, it binds the process to the grant id and the command's clock (`goal.BindAttorneyEffect`). `goal.Publish` then re-checks that the entry is live, against the injected clock, on every `Mutate` at the tip the transaction commits on. The transaction fetches the remote tip first (`CaptureTip`, `txn.go:780`) and lands by compare-and-swap, so the act is serialized with every revoke, local or remote. If the grant is not live, the act is refused with nothing written.

**Non-ledger acts.** Under the grant, a non-ledger act (`system stop`, `disk clean`, `steward arm`) holds a shared flock on `artifacts/agents/authority/attorney.lock`. It takes the lock before its final grant read and keeps it until the process exits. A local `grant revoke` takes the lock exclusively for up to 30 seconds before it publishes. Once that revoke returns, no act admitted before it is still running, and no later act is admitted. If the lock is not free in time, the revoke publishes anyway and says an admitted act was still running. Revocation is never blocked.

**Latency:**
- Expiry: judged by each clock, with no propagation needed.
- Same-machine revoke: the next act.
- Remote revoke: the next ledger act. For non-ledger acts it applies at the grantee's next ledger fetch (any goal write, or `goal sync`), and the revoke's confirmation says so.

## 7. Status

`withHelm` (`intent_helm.go:431-442`) prepends the helm lines first, then the grant line from §2. The grant line appears only in the checkout the grant is for. If the root record is unreadable, status says so, and no grant is in force.

## 8. Units

**Unit 1 (step 1).** The ledger record, the time parser, `GrantGeneral`, `grant add|revoke|list`, the proof arm, the admitter and log, `BindAttorneyEffect`, the grant lock, M5, M6, M3, the status line and the hard-limit checks. After unit 1, the seat's main session can do every goal verb, `disk clean`, split ratification and breach-stop ordering alone, for a stated time, with an audit trail.

**Unit 2.** `ClassifyPersonAt` and the site swaps.

## 9. Witnesses

Each witness fails before its unit.
- **W1.** Parser, with an injected clock and Europe/Amsterdam:
  - `24h`, `8h`, `7d` and `1w` are elapsed time;
  - `HH:MM` means today, or tomorrow once passed;
  - `tomorrow` and a date both give the next local 00:00;
  - more than 168h is refused, naming the latest allowed;
  - the DST gap and repeat are refused;
  - the refused shapes are refused with the examples.
- **W2.** Grammar:
  - a general entry round-trips;
  - forbidden and missing keys are refused;
  - a non-minute `until`, more than 168h, and a relative checkout are refused;
  - `LiveAt` is true at `until-1ns` and false at `until`, when revoked, and at a zero `now`.
- **W3.** `Prove`:
  - with the attorney seam granting, it returns an enrolled proof with `Helm.Grant` and `ValidFor`;
  - with the seam nil or false, it gives today's refusal;
  - the helm answers first.
- **W4.** The admitter refuses:
  - a `MAIN` that does not hold the lease;
  - a holder whose lineage differs;
  - another checkout path;
  - another machine;
  - fixture mode;
  - an unreadable root;
  - a failed log append;
  - `DELEGATE`, `STEWARD` and `SUPERVISION`.

  When it admits, it writes one log line and one stderr line.
- **W5.** Hard limits:
  - a grant, helm or fixture proof cannot add a general grant;
  - under a grant proof, `goal.Grant` refuses;
  - `settings set metasystem.runtimes` refuses without a direct person proof, while any other key needs none.
- **W6.** Effect:
  - after a revoke lands between admission and `Mutate`, the act is refused;
  - after the clock passes `until` between admission and `Mutate`, the act is refused;
  - a revoke waits on a held shared lock and proceeds after the timeout.
- **W7 (M6).** Pause and done from an agent with `METASYSTEM_OWNER_LINEAGE` under a grant act as the person; without a grant, the shortcut is kept.
- **W8.** Intents:
  - `grant add` confirms with the local end;
  - a repeat is `unchanged`;
  - a replacement revokes the old grant;
  - `list` shows the time left;
  - `status` first line;
  - a repeated `revoke` is `unchanged`.
- **W9 (unit 2).**
  - `ClassifyPersonAt` gives `HUMAN` plus `Attorney` for the holder, while `ClassifyAt` still gives `MAIN`;
  - `humanTerminalCheck` refuses before the grant and admits under it;
  - `session stop` accepts a grant proof.
- **W10 (unit 2).** Audit: every non-test `lease.ClassHuman` comparison is in the left-alone allowlist or goes through `ClassifyPersonAt`.

## 10. Out of scope

Each of these is later, when it hurts:
- a UI revoke button;
- a bounded fetch before non-ledger acts;
- a History outcome naming the grant on goal acts;
- `--include-delegates` (D3);
- time forms for scoped grants;
- `helm take` inline;
- telling subagents apart from their main;
- D1 (a grant-derived enrollment re-arms after expiry): accepted by ruling;
- D2 (session-stop `InvokerRef`).

## 11. Build decisions beyond the rulings

1. **M4 for non-ledger acts** is a process-lifetime shared lock against an exclusive, bounded revoke. It is not a per-effect clock re-check. The window left open is that an act admitted before `until` may finish after it, by its own run time.
2. **M3's second half is not built.** "Fixture signature/clock overrides only in test binaries" would gate `fixtureauth`'s environment reads behind a build tag. It touches 168 files that set fixture mode, and 26 test files that build the engine with plain `go build`, so it needs its own goal. The narrower half is built: the `settings set` key takes a direct person proof, and a fixture-mode root never admits a grant.

## 12. Built

Built by Claude on Opus 5.5 on branch `poa`: unit 1 at `837ec7b63`, unit 2 in the commit that adds this section. It departs from the page in four places:
- **Status line.** `status` shows no line when the ledger is unreadable, where §7 said it would name the unreadable ledger. Status reads nothing it cannot parse, and no grant is in force either way.
- **Proof-run class checks.** Only `legacyProofLaunchAllowed` swapped to the grant-aware classifier. `admitProofLaunchWithReadsAndClassifier` and `admitLaneProofLaunch` keep `classifyVerbCaller`, because the holder main already passes there as the goal's coordinator or the lane owner. Turning it into HUMAN would move it off the claim-epoch path.
- **M5.** It has no dedicated witness: the change is the clock argument, and existing split and disk tests cover the path.
- **M3, second half.** It is not built (§11).
