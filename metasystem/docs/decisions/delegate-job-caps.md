# Delegate job caps: per-pair caps and the fence

- Kind: decision
- Id: 01M3MKDZMWKMVMZ36W7SVQPVT5
- Status: accepted

The delegate-caps chain, 2026-08-09 to 08-11, behind ruling R-137-m1e. The
live mechanism is `internal/mission/fence.go` (`AuthorizeCapWithClock`,
`RefuseBudgetCap`), `internal/supervise/ceiling.go`,
`internal/config/validate.go` and `internal/delegation/admission.go`.
Distilled 2026-09-28 from `records/delegate-delivery/delegate-caps.md` (Why;
D-1, D-1a, D-2, D-3, D-5, D-7; AUTH-R2 resolutions 001-008), removed that day
(tag `records-archive-2026-09-28`).

- Why per pair: bm-2's uniform 15-minute job cap killed a Devin/swe-1-7
  implementer that held 1,322 compiling lines, and the mission shipped a
  skeleton (acceptance 1/53 on both reps). The cause was the configuration,
  not the model or the CLI. A cap is keyed on (runtime, canonical model) and
  declared where the binding is declared.
- The fence authorizes, dispatch asks. `mission fence-authorize-cap` is the
  only resolver of a mission job's cap. Rejected: dispatch resolving mission
  caps from configuration, which creates a second, mutable authority.
- The contract is the only transport. Pair caps live in the signed contract
  (and its sealed exposure) and nowhere else. A mission refuses unsigned
  `--cap-min` above the authorized value and refuses `.local`/env cap keys.
  Rejected: a configuration copy checked for equality at preflight, because it
  drifts after preflight and leaves two authorities. Host turn caps stay
  contract-only for the same reason, with no config form.
- The pin is the raw-file sha256 (`approvedContractSha256`), not the canonical
  signed-content digest, because drift detection needs exact bytes (Approval
  line and whitespace included). It governs every signed limit the fence reads
  (pair cap, `fence.job-cap-min`, wall clock), not only pair caps. Only the
  runner writes it, at start and resume, from the bytes that preflight just
  verified. Nothing re-hashes it later. Pinned by
  TestAuthorizeCapRefusesPinnedContractDrift and
  TestAuthorizeCapRefusesWhitespaceOnlyDrift.
- The deadline is truncated from the persisted fence start, never from the
  lease's `startedAt`, which resets on resume and would silently extend the
  signed wall clock. A timeout that truncation caused is attributed to
  `wall-clock-hours`, so the human is asked to amend the right term.
- The watcher ceiling is derived from config sources plus `--max-cap` only.
  Arming is repo-wide and cannot enumerate contracts, so a mission that needs
  more raises the ceiling at arming. Dispatch compares against the ceiling the
  watcher attests, never a recomputed one. `--rearm` refuses to lower the
  ceiling below a reserved cap, because a live job's budget cannot be
  invalidated retroactively.
- A non-canonical `cap.min.*` key is refused with its canonical form. It is
  never normalized, because a lone `cap.min.devin.swe-1.7` would otherwise
  fall back silently to the default.
