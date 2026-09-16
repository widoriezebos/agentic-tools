FC-1 — CLOSED: The shared custodian watches the exact launcher chain independently of the test binary, kills the still-live test owner when a watched launcher disappears, and witness 4 exercises that shape in seconds.
FC-2 — CLOSED: The custodian is placed in its own session outside the suite, bed, launcher, watchdog, and delegate kill domains; witnesses 3–6, 10, and 11 cover owner death, group cancellation, a non-leashed detached grandchild, and a stopped child without relying on cooperative cleanup.
FC-3 — CLOSED: Executable-path matches are actionable only when an exact ownership record names a dead or identity-mismatched owner; live-owner and indeterminate matches are not signalled, and witness 8 distinguishes those classes.
FC-4 — CLOSED: The tag and recorded-child paths now retain the platform-native `identity.Ref`, reject non-native exact values, and re-prove that ref immediately before every signal; witnesses 1 and 2 cover Darwin/Linux encoding and PID replacement without signalling the replacement.
FC-5 — CLOSED: Witnesses 10 and 11 preserve and positively assert the stopped runner, detached TERM-ignoring member, host readiness, and held host's TERM resistance before testing owner-loss reaping.

# New material findings

## FC-6 — Per-test cleanup is keyed only by the package test binary and can kill a parallel test's live fixture

- **Page sections:** 3.1–3.2, 3.5, witness 8.
- **Evidence checked by reading:** The environment tag contains `<owner ref>|<test name>`, but `FixtureSurvivors(owner Ref)` and cleanup step 3 select by `owner Ref` alone. Every test in one package process has the same exact owner ref. Therefore, when one `t.Parallel` test cleans up, its scan can select and SIGKILL a sibling test's correctly live tagged children. Witness 8 uses different live and dead owner refs, so it cannot catch two fixture identities under the same live owner.
- **Failure caused:** The new reaper can kill a legitimate active fixture, the safety failure FC-3 was meant to exclude through the census path.
- **Required design change:** Give each fixture a unique full key (owner ref plus test identity/nonce), scope cooperative cleanup and any live-owner `--owner` operation to that full key, and reserve owner-ref-wide scans for the moment the test binary itself is proved dead. Add a fixture-table witness with two live fixture keys under the same owner; cleaning one must signal only its process.

## FC-7 — The census consumers have no positive witness, so the DONE proof passes if health and the proof-run launcher never use the census

- **Page sections:** 3.5, unit 7, section 8.
- **Evidence checked by reading:** Witness 8 tests `proc fixture-survivors` classification directly, but no witness drives a certain survivor through `metasystem health` or through the proof-run launcher's post-suite `--reap` call. Unit 7 names no witness. The DONE check that health prints no survivor on a quiet seat is purely negative and still passes if the new health scan is omitted; the supplemental future aborted proof is explicitly not a gate.
- **Failure caused:** Rule 4's second net can be absent from both operational consumers while all eleven required witnesses and DONE checks pass.
- **Required design change:** Add deterministic positive consumer witnesses using the process-table fixture and injected signal sink: health must name the certain dead-owned survivor and exit non-zero, and a proof-run post-suite path must select/reap that exact class and include it in the verdict. They must also retain the live-owner no-signal case and require no real leaked process.

## FC-8 — The controlled run-owner propagation is unwitnessed; witness 4 proves only the fallback path

- **Page sections:** 3.3, units 4 and 7, witness 4.
- **Evidence checked by reading:** Witness 4 deliberately leaves `METASYSTEM_RUN_OWNER` unset and exercises the heuristic that stops at the first non-Go ancestor. No witness proves that `metasystem test run`, the proof-run launcher, `go-gate.sh`, or a bed exports the correct native exact ref, or that the custodian consumes that explicit ref as the top of its watched chain. Those exporters are split across units 4 and 7, and neither unit has a positive propagation witness.
- **Failure caused:** An omitted or malformed exporter can leave a controlled test watching only an intermediate suite shell; death of the actual run launcher above that live shell need not trigger owner death, while the fallback-only witness remains green.
- **Required design change:** Add seconds-long producer/consumer contract witnesses for the explicit variable: each controlled launch point must export its own parseable exact ref, and a custodian must retain that ancestor in its chain. At least one process witness must kill the exported launcher while an intermediate process remains alive and observe the test owner and tagged child disappear.

# Non-material notes

- The binding rule is preserved: the revised fixtures still positively establish an unresponsive git, a detached bed member, a stopped runner, and a ready held host that ignores TERM before owner loss makes reaping possible.
- Both named producers and all three observed shapes are routed to mechanisms and witnesses: the hanging-git fixture, the landed wait-verb exec/grandchild fixture, TERM resistance, init reparenting, and an exec-hidden grandchild.
- The custodian must not carry the same actionable fixture tag as the children it scans (or it must explicitly exclude its own exact ref). The bed witnesses would catch a self-kill, so this is an implementation clarification rather than an additional blocker.
- The estimated 220–260-line core units are tight once cross-platform code and subprocess witnesses are counted. The stated rule to split at a witness boundary before a computed diff exceeds 300 lines preserves the cap, but the actual totals cannot be known before implementation.
- The page's live-owner meaning for `--owner <ref>` should be made explicit: bed-exit callers are vouching for teardown, whereas an ordinary census still requires a dead exact owner. Existing bed witnesses should expose the wrong interpretation.

# Unchecked items

- No proposed witness was run because this is a pre-implementation design.
- I did not inspect files outside the supplied goal, round-1 critique, preserved specimen, repository instructions/critique guidance, the review page, and source locations or components the page cites.
- I did not independently inventory every process-spawning fixture beyond the named producers and bound fixture family.
- Whether same-user Darwin `KERN_PROCARGS2` exposes environment reliably remains unproved, as the page records; implementation unit 1 must resolve it without weakening an unreadable process into an actionable match.
- Exact changed-line totals and the behavior of not-yet-written launcher/health wiring cannot be checked before implementation.

# Tool calls used

20 tool calls total: 18 read-only shell calls before writing, one `apply_patch` call creating this file, and one readback call. No state-changing git command was used.
