# Typing the historical designs

The machine's proposal for the kit's own history in `metasystem/plans`, for a human to
review. `metasystem project type-designs` rewrites this table from the files and the
ledger; `--apply` reads it back and types every line that says `resolved`.

The goal is proposed from the file name, the status from what the ledger says about
that goal. A line is `resolved` only when the file's own opening agrees with both;
otherwise it is `unresolved` and the note says why. Edit a line's goal or status, or
change `unresolved` to `resolved`, and `--apply` honours what you wrote. A line that
says `record` is already typed and a line that says `malformed` carries a head no
file name can repair; `--apply` leaves both alone.

| file | proposed goal | proposed status | resolution | note |
| --- | --- | --- | --- | --- |
| account-provenance-design.md | account-provenance | done | resolved |  |
| actionable-metrics-design.md | actionable-metrics | accepted | unresolved | the file names goal continuous-self-improvement |
| alert-channel-design.md |  |  | unresolved | no goal matched |
| application-testing-contract-design.md |  |  | unresolved | no goal matched |
| backlog-git-sync-design.md | backlog-git-sync | done | resolved |  |
| backlog-ordered-by-priority-conclusion-vs-compaction-design.md |  |  | unresolved | no goal matched |
| backlog-ordered-by-priority-design.md | backlog-ordered-by-priority | done | unresolved | the file says accepted |
| breach-clock-and-budget-honesty-design.md | breach-clock-and-budget-honesty | done | resolved |  |
| brief-declares-the-round-boundary-design.md | brief-declares-the-round-boundary | accepted | resolved |  |
| budget-extends-by-consumption-design.md |  |  | unresolved | no goal matched |
| builder-proves-each-rule-by-mutation-design.md | builder-proves-each-rule-by-mutation | accepted | resolved |  |
| capped-round-continues-instead-of-restarting-design.md | capped-round-continues-instead-of-restarting | done | resolved |  |
| chain-landing-after-base-move-recertifies-design.md | chain-landing-after-base-move-recertifies | done | unresolved | the file says accepted |
| channel-landing-notice-design.md |  |  | unresolved | no goal matched |
| codex-handshake-design.md |  |  | unresolved | no goal matched |
| coordinator-context-stays-under-budget-design.md | coordinator-context-stays-under-budget | done | resolved |  |
| coordinator-loop-prevention-design.md | coordinator-loop-prevention | done | resolved |  |
| coordinator-loop-prevention-testing-design.md |  |  | unresolved | no goal matched |
| coordinator-wakes-on-events-not-polls-design.md | coordinator-wakes-on-events-not-polls | done | resolved |  |
| counselor-design.md | counselor | accepted | resolved |  |
| critique-always-design.md | critique-always | done | resolved |  |
| critique-closes-on-folded-proof-design.md | critique-closes-on-folded-proof | accepted | resolved |  |
| deep-battery-design.md |  |  | unresolved | no goal matched |
| deep-sections-cannot-stay-red-unseen-design.md | deep-sections-cannot-stay-red-unseen | done | resolved |  |
| defect-analysis-gate-design.md | defect-analysis-gate | accepted | resolved |  |
| delegate-job-liveness-design.md | delegate-job-liveness | done | resolved |  |
| delegate-proof-runs-inside-the-sandbox-design.md | delegate-proof-runs-inside-the-sandbox | done | unresolved | the file names goal delegate-rounds-reuse-a-warm-gate |
| delegate-rounds-reuse-a-warm-gate-design.md | delegate-rounds-reuse-a-warm-gate | done | resolved |  |
| delegate-sandbox-runs-the-beds-design.md | delegate-sandbox-runs-the-beds | accepted | resolved |  |
| delivery-candidate-is-the-workspace-design.md |  |  | unresolved | no goal matched |
| delivery-receipt-stops-at-first-failure-design.md | delivery-receipt-stops-at-first-failure | done | resolved |  |
| disk-hygiene-design.md | disk-hygiene | done | resolved |  |
| dispatch-cap-settlement-design.md |  |  | unresolved | no goal matched |
| engine-rebuild-rearm-design.md |  |  | unresolved | no goal matched |
| engine-runs-re-arm-on-a-landed-engine-design.md |  |  | unresolved | no goal matched |
| fable-5-1-rollover-design.md |  |  | unresolved | no goal matched |
| fable-model-alias-design.md | fable-model-alias | done | resolved |  |
| failed-job-attention-design.md | failed-job-attention | accepted | resolved |  |
| fixture-children-cannot-outlive-their-test-design.md | fixture-children-cannot-outlive-their-test | done | resolved |  |
| fleet-channel-gateway-design.md | fleet-channel-gateway | accepted | resolved |  |
| fleet-doctor-repairs-what-stops-other-seats-design.md | fleet-doctor-repairs-what-stops-other-seats | accepted | resolved |  |
| fleet-join-bootstrap-design.md | fleet-join-bootstrap | accepted | resolved |  |
| fleet-pull-design.md | fleet-pull | accepted | resolved |  |
| fleet-slack-channel-design.md | fleet-slack-channel | done | resolved |  |
| fleet-slack-channel-slice2-design.md |  |  | unresolved | no goal matched |
| goal-abandoned-with-a-reason-design.md | goal-abandoned-with-a-reason | done | resolved |  |
| goal-scope-bounds-design.md | goal-scope-bounds | accepted | resolved |  |
| goals-live-on-branches-design.md | goals-live-on-branches | done | resolved |  |
| host-health-role-design.md | host-health-role | accepted | resolved |  |
| host-implementer-wall-design.md | host-implementer-wall | done | resolved |  |
| host-runtime-setup-design.md | host-runtime-setup | done | unresolved | the file names goal runtime-install-execution |
| human-approval-for-execution-design.md | human-approval-for-execution | done | resolved |  |
| human-break-glass-commit-keeps-the-ledger-valid-design.md | human-break-glass-commit-keeps-the-ledger-valid | accepted | resolved |  |
| human-carried-landing-carry-design.md | human-carried-landing-carry | done | resolved |  |
| human-carried-landing-design.md | human-carried-landing | accepted | resolved |  |
| human-goal-verbs-forgiving-design.md | human-goal-verbs-forgiving | done | unresolved | the file names goal verbs-match-intent |
| human-proof-fits-the-act-design.md | human-proof-fits-the-act | done | unresolved | the file says accepted |
| hung-proof-attempts-design.md |  |  | unresolved | no goal matched |
| job-record-birth-token-design.md | job-record-birth-token | accepted | resolved |  |
| land-ready-work-lands-without-a-claim-slot-design.md | land-ready-work-lands-without-a-claim-slot | done | resolved |  |
| landing-receipt-survives-records-drift-design.md | landing-receipt-survives-records-drift | done | resolved |  |
| landing-runs-standard-design.md |  |  | unresolved | no goal matched |
| ledger-attention-design.md | ledger-attention | done | resolved |  |
| live-records-landing-design.md |  |  | unresolved | no goal matched |
| memory-architecture-design.md | memory-architecture | done | unresolved | the file says draft |
| metasystem-stop-design.md |  |  | unresolved | no goal matched |
| metasystem-stop-verb-design.md | metasystem-stop-verb | done | resolved |  |
| missionrunner-patience-design.md |  |  | unresolved | no goal matched |
| operator-surface-design.md |  |  | unresolved | no goal matched |
| path-class-manifest-design.md | path-class-manifest | done | resolved |  |
| process-steward-design.md | process-steward | superseded | resolved |  |
| proof-admission-fits-a-seat-proving-several-units-design.md | proof-admission-fits-a-seat-proving-several-units | done | resolved |  |
| proof-groups-detect-hangs-by-progress-not-the-clock-design.md | proof-groups-detect-hangs-by-progress-not-the-clock | done | resolved |  |
| proof-groups-progress-hang-design.md |  |  | unresolved | no goal matched |
| proof-harness-custody-design.md |  |  | unresolved | no goal matched |
| proof-run-cost-and-liveness-design.md | proof-run-cost-and-liveness | done | resolved |  |
| receipt-admission-attribution-design.md |  |  | unresolved | no goal matched |
| red-on-main-gets-an-owner-on-the-ledger-design.md | red-on-main-gets-an-owner-on-the-ledger | done | resolved |  |
| registered-wait-matches-the-runtime-session-design.md | registered-wait-matches-the-runtime-session | superseded | resolved |  |
| retained-proof-reuse-crosses-claims-and-attempts-design.md | retained-proof-reuse-crosses-claims-and-attempts | done | resolved |  |
| role-context-composition-design.md | role-context-composition | accepted | resolved |  |
| role-lane-packets-design.md | role-lane-packets | superseded | resolved |  |
| seat-mutual-awareness-design.md | seat-mutual-awareness | accepted | unresolved | the file says done |
| seat-successor-continues-a-handoff-without-a-human-design.md | seat-successor-continues-a-handoff-without-a-human | accepted | resolved |  |
| seats-spend-tokens-in-bounded-sessions-design.md | seats-spend-tokens-in-bounded-sessions | accepted | resolved |  |
| severity-tiered-rigor-design.md | severity-tiered-rigor | done | resolved |  |
| severity-tiered-rigor-p2-design.md | severity-tiered-rigor-p2 | done | unresolved | the file names goal severity-tiered-rigor |
| small-change-lane-design.md | small-change-lane | accepted | resolved |  |
| spend-cap-retirement-design.md |  |  | unresolved | no goal matched |
| spend-fence-reports-tokens-per-model-and-cause-design.md | spend-fence-reports-tokens-per-model-and-cause | accepted | resolved |  |
| stop-decisions-record-deadline-evidence-design.md | stop-decisions-record-deadline-evidence | done | resolved |  |
| stop-hook-never-forces-an-empty-turn-design.md | stop-hook-never-forces-an-empty-turn | accepted | resolved |  |
| stop-infrastructure-allows-the-seat-to-stop-design.md | stop-infrastructure-allows-the-seat-to-stop | done | unresolved | the file names goal stop-hook-never-forces-an-empty-turn |
| stop-message-fits-one-screen-design.md |  |  | unresolved | no goal matched |
| suite-custody-design.md | suite-custody | superseded | resolved |  |
| supervision-hook-root-design.md |  |  | unresolved | no goal matched |
| time-bound-tests-design.md |  |  | unresolved | no goal matched |
| token-spend-fence-design.md | token-spend-fence | accepted | resolved |  |
| turn-verdict-hardening-design.md | turn-verdict-hardening | done | resolved |  |
| two-bars-caller-class-design.md |  |  | unresolved | no goal matched |
| two-bars-for-changes-design.md | two-bars-for-changes | done | unresolved | the file says accepted |
| verification-loop-design.md |  |  | unresolved | no goal matched |
