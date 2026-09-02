# Idle-with-backlog root cause — codex critique of the seat's analysis

Critic: codex gpt-5.6-sol (idle-rootcause-critique-c). 11 findings,
10 material. Verdict on the seat's analysis: AMENDED AND PARTLY
REFUTED. The true root cause and the 'impossible' bar are recorded on
goal idle-with-backlog-alarm.

## F-ROOT-001 [critical|material=True]

Root-cause verdict: AMENDED AND PARTLY REFUTED. The missing effective Stop hook and claimed-goal-delivery's no-claim blind spot are confirmed, but the assertion that hook-freshness death was only status and that there were zero live enforcers is false. The steward opened and transport-submitted an alert 96 minutes before the human intervention. The unexplained failure is therefore downstream of detection as well: transport submission was treated as sufficient despite proving neither human receipt nor acknowledgment. Implementing fix B as new escalation would duplicate shipped behavior and leave the demonstrated loss path intact.

Evidence: metasystem/internal/steward/health.go:285 and :387-390, metasystem/internal/steward/tick.go:267-295, metasystem/internal/steward/alert_episode.go:17-18, and metasystem/artifacts/agents/steward/alerts/alert-99020c96056dc6af-1.json establish the complete dead-to-alert-to-transport path and its timing.

## F-PROPERTY-002 [critical|material=True]

Fixes A, B, and C do not meet Wido's stated bar that idle-with-backlog become impossible. Fix A is explicitly detection-only, fix B is notification-only, and a correctly loaded fix C blocks an unchanged queue only once per session digest. A residual execution is lawful under this design: the first Stop is blocked, the seat responds again without claiming, the second Stop passes for the unchanged queue, and an alert is submitted after grace without any machinery resuming work. The design needs a causal invariant through claim, dispatch, or an explicitly authorized stop, plus a fixture proving that the second unchanged Stop cannot become quiet.

Evidence: metasystem/plans/goals/idle-with-backlog-alarm.md:6 says detection only. metasystem/internal/goal/turnverdict.go:124-130 and :411-425 persist blocked queue digests and deliberately return display-only after the first block. metasystem/internal/steward/alert_episode.go:315-321 stops retrying once a transport accepts an episode.

## F-CONFIG-003 [high|material=True]

The incident proves that the parent-level settings were not effective, but it does not prove the analysis's general claim that Claude intentionally loads project hooks only from the session directory. Current official documentation says parent settings are discovered, while the preserved runtime contradicts it. Moreover, the reviewed commit's nested copy is not a valid implementation of fix C: it retains parent-layout commands that resolve a nonexistent metasystem/metasystem directory, and it did not arm the live session. An implementer needs an explicit project-root resolution and activation/restart contract, not a file-copy instruction based on an unverified loader theory.

Evidence: The preserved session shows current working directory /home/wido.guest/m0/agentic-tools/metasystem and no metasystem hook under Claude Code 2.1.237. metasystem/.claude/settings.json:9, :24, and :35 append /metasystem. Official current documentation says configuration is discovered from the working directory and parents, leaving the version-specific omission cause unresolved.

## F-PROBE-004 [high|material=True]

The ensureGuardEnrolled probe cannot, unchanged, verify that a turn hook is loaded. Its proof works because it resolves and directly executes the same file Git invokes. Directly executing supervision-hook.sh would prove script behavior while bypassing the disputed Claude settings loader and event registry—the exact boundary that failed here. Only the nonce-and-distinct-result protocol is reusable; the test must be driven through a real provider lifecycle event and bind the observed acknowledgment to the current session and active settings source.

Evidence: metasystem/cmd/metasystem/goalsync_verbs.go:119-149 directly executes the resolved pre-commit file and checks its nonce plus exit status. The incident's static hook check at metasystem/scripts/validate-metasystem.sh:2187-2203 accepted the parent settings while the preserved Claude Stop summaries showed that the runtime had not registered them.

## F-FRESHNESS-005 [high|material=True]

Fix B watches the wrong notion of freshness. After any one successful hook emission, the current hook-freshness role can stay alive forever because it compares neither age nor current-session expected turns. A one-time enrollment probe proposed by fix C could therefore make the watchdog permanently green even after later hook unloading. Conversely, the role is evaluated unconditionally and immediately declares a fresh headless or cron installation dead when no turn hook is expected. The design needs an applicability rule and a current-session expected-event-to-emission join before choosing alert thresholds.

Evidence: metasystem/internal/steward/health.go:243-260 evaluates hook-freshness unconditionally. Lines 285-320 accept any internally consistent last EMITTED attempt without an age or session check. metasystem/internal/steward/component_evidence.go:200-219 stores a turn digest but health never joins it to an active session or expected turn.

## F-OWNER-006 [high|material=True]

The analysis missed an existing steward decision owner whose semantics are part of the root cause: queued backlog without a local claim is deliberately classified as no work, producing no action. Adding fix A only as a parallel health role would let the same tick report both no-work and dead-for-idleness. The design must assign one authoritative no-claim/backlog predicate or explicitly reconcile the core decision and health outcomes; otherwise an implementer will leave the active steward control path inert while adding another status path.

Evidence: metasystem/internal/steward/openwork.go:30-63 counts queued goals but returns WorkNone when none is owned locally. metasystem/internal/steward/verdict.go:90-97 maps that result to VerdictNoWork and ActNone. Fix A is scoped only as a new health verdict in metasystem/plans/goals/idle-with-backlog-alarm.md:4-6.

## F-WORK-007 [high|material=True]

Fix A's phrase 'nothing runs' is not implemented by its narrower test for no non-terminal delegate-job records. A stale running record with a dead process suppresses the literal alarm even though nothing runs; a live mission runner, gate, monitored run, or other productive worker is absent from that test and can cause a false alarm. The existing worker census covers those classes but also counts the live main session that was idle in this incident, so it cannot be substituted blindly. The design must define productive, relevant, live work and its ownership.

Evidence: metasystem/plans/goals/idle-with-backlog-alarm.md:4 uses absence of non-terminal delegate jobs. metasystem/internal/steward/verdict.go:41-55 documents a broader census over sessions, delegate jobs, gates, mission runners, and monitored runs. Neither source defines which live classes actually discharge idle-with-backlog.

## F-CLOCK-008 [high|material=True]

The fifteen-minute grace has no defined clock. The design does not say whether time begins when the claim is released, when work becomes claimable, when the first steward tick observes both, or when the last productive process ends; nor does it define resets, persistence across steward restart, concurrent transitions, or clock regression. These choices produce materially different alerts and displayed idle durations, so the boundary tests named in fix A cannot be written deterministically from the design.

Evidence: metasystem/plans/goals/idle-with-backlog-alarm.md:4-6 names a configurable grace and duration but no transition state. metasystem/internal/steward/health.go:323-413 persists failure counters only after a role is already dead; it supplies no pre-failure idle interval owner.

## F-LEDGER-009 [high|material=True]

Fix A does not bind 'claimable budgeted' to a canonical, fresh ledger verdict. The existing turn verdict's queued frontier includes every queued item, while goal.Next correctly filters dependencies and foreign pins; the goal even leaves foreign-pin handling as a freedom. Adjacent steward health roles call offline Project with fetch disabled, where a projection older than thirty minutes remains usable with only a banner. An implementer could therefore count nonclaimable rows, miss remotely added work indefinitely, or alert on remotely resolved work. The design must select the canonical Next semantics, define budget validity, and specify the fail-safe outcome for a stale projection.

Evidence: metasystem/internal/goal/turnverdict.go:534-571 hashes all queued rows. metasystem/internal/goal/project.go:89-123 defines dependency- and pin-aware claimability, while :24-78 allows stale offline state. metasystem/plans/goals/idle-with-backlog-alarm.md:6 leaves foreign-pin handling open despite claimability already having a shipped meaning.

## F-ALERT-010 [high|material=True]

Fix B's alert behavior is both already present and observably storm-prone. Alert identity hashes the complete set of unhealthy role names and statuses, not the hook role's own episode; any companion role changing state resolves one aggregate episode and mints another while hook-freshness remains continuously dead. This produced seventeen transport submissions with the same hook failure in about forty-two hours and no acknowledgments. 'Same severity as steward-runner death' is also ambiguous: hook death currently alerts on its first observation, while steward-runner death is initially eligible for automatic healing. The design needs role-specific episode identity, applicability, retry or acknowledgment semantics, and an exact threshold before escalating hook absence.

Evidence: metasystem/internal/steward/health.go:1116-1124 hashes every non-alive role/status pair. metasystem/internal/steward/alert_episode.go:270-297 rotates episodes when that aggregate changes. The seventeen matching files under metasystem/artifacts/agents/steward/alerts/ were all TRANSPORT_SUBMITTED and unacknowledged. metasystem/internal/steward/health.go:387-398 gives no-remedy hook death and automatically healable steward-runner death different escalation timing.

## F-CONFIRM-011 [low|material=False]

The analysis correctly states that claimed-goal-delivery has no no-claim verdict.

Evidence: metasystem/internal/steward/delivery.go:60-83 builds entries only for this machine's claimed goals and explicitly returns alive with 'there are no goals claimed by this machine' when that set is empty.
