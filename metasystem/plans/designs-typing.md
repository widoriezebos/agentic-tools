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
| account-provenance-design.md | account-provenance | done | record | already declares 01M3A2YHDKCXX1GQ1ZX2BZW4JC |
| actionable-metrics-design.md | actionable-metrics | accepted | record | already declares 01M3A3TATH53FVDB4SWP5V064R |
| alert-channel-design.md | alert-escalation-channel | done | record | already declares 01M3A3TATHRM3ZZTV190DJHQ7X |
| application-testing-contract-design.md | coordinator-loop-prevention | done | record | already declares 01M3A3TATHF24QSCBWSAJ12X0G |
| backlog-git-sync-design.md | backlog-git-sync | done | record | already declares 01M3A2YHDKHNMXKEAW4HZT2NGZ |
| backlog-ordered-by-priority-conclusion-vs-compaction-design.md | backlog-ordered-by-priority | done | record | already declares 01M3A3TATHZSNCZS97XH3WPPPJ |
| backlog-ordered-by-priority-design.md | backlog-ordered-by-priority | done | record | already declares 01M3A3TATHW37JEKBTRNZWJC7E |
| breach-clock-and-budget-honesty-design.md | breach-clock-and-budget-honesty | done | record | already declares 01M3A2YHDK31ZK3YEWCE7A1589 |
| brief-declares-the-round-boundary-design.md | brief-declares-the-round-boundary | accepted | record | already declares 01M3A2YHDKRCNJXVDTHD98E2C4 |
| budget-extends-by-consumption-design.md | budget-extends-by-consumption-and-breach-parks | done | record | already declares 01M3A3TATHVCADQTPBZ6PDPXHS |
| builder-proves-each-rule-by-mutation-design.md | builder-proves-each-rule-by-mutation | accepted | record | already declares 01M3A2YHDKNK8S6MM1BKCYCPMC |
| capped-round-continues-instead-of-restarting-design.md | capped-round-continues-instead-of-restarting | done | record | already declares 01M3A2YHDKG5HVK1Y2XQGEM23G |
| chain-landing-after-base-move-recertifies-design.md | chain-landing-after-base-move-recertifies | done | record | already declares 01M3A3TATH3CQHPZERYVKF397W |
| channel-landing-notice-design.md | channel-tells-me-when-something-lands | done | record | already declares 01M3A3TATH0XP94HDPH666T1C0 |
| codex-handshake-design.md | codex-handshake-budget-load-fragile | done | record | already declares 01M3A3TATHTJGFHZDA2FQ41CJ4 |
| coordinator-context-stays-under-budget-design.md | coordinator-context-stays-under-budget | done | record | already declares 01M3A2YHDKJS1BXQ203RAERCAD |
| coordinator-loop-prevention-design.md | coordinator-loop-prevention | done | record | already declares 01M3A2YHDKBQA1PEKHRCXB0Y8Q |
| coordinator-loop-prevention-testing-design.md | coordinator-loop-prevention | done | record | already declares 01M3A3TATH5BCZRV0WWBYVT2Q5 |
| coordinator-wakes-on-events-not-polls-design.md | coordinator-wakes-on-events-not-polls | done | record | already declares 01M3A2YHDK3DM5YS2DSBPWV22W |
| counselor-design.md | counselor | accepted | record | already declares 01M3A2YHDKEHB8GB76VK6A8W1N |
| critique-always-design.md | critique-always | done | record | already declares 01M3A2YHDKR2GJGT38B5Y0PHZN |
| critique-closes-on-folded-proof-design.md | critique-closes-on-folded-proof | accepted | record | already declares 01M3A2YHDKNSS10646SPQPNTKB |
| deep-battery-design.md | deep-battery-under-ten-minutes | done | record | already declares 01M3A3TATHK4AECGDAFARP750M |
| deep-sections-cannot-stay-red-unseen-design.md | deep-sections-cannot-stay-red-unseen | done | record | already declares 01M3A2YHDM2VCX7NQPGH7MXT95 |
| defect-analysis-gate-design.md | defect-analysis-gate | accepted | record | already declares 01M3A2YHDMJY93Q1PFHVN3XE6P |
| delegate-job-liveness-design.md | delegate-job-liveness | done | record | already declares 01M3A2YHDMKT7HMV0RKMM563KV |
| delegate-proof-runs-inside-the-sandbox-design.md | delegate-proof-runs-inside-the-sandbox | done | record | already declares 01M3A3TATHV7HXE8TS399W5FNJ |
| delegate-rounds-reuse-a-warm-gate-design.md | delegate-rounds-reuse-a-warm-gate | done | record | already declares 01M3A2YHDMFJRHBGQCEK0PYASV |
| delegate-sandbox-runs-the-beds-design.md | delegate-sandbox-runs-the-beds | accepted | record | already declares 01M3A2YHDMCEX6Q46T5BPA91F7 |
| delivery-candidate-is-the-workspace-design.md | delivery-candidate-is-the-workspace-not-the-ledger | done | record | already declares 01M3A3TATJ65FH40316H16JPJK |
| delivery-receipt-stops-at-first-failure-design.md | delivery-receipt-stops-at-first-failure | done | record | already declares 01M3A2YHDMXVB5D6J4FVNXT6VF |
| disk-hygiene-design.md | disk-hygiene | done | record | already declares 01M3A2YHDM2Z0KWBWQGND5594N |
| dispatch-cap-settlement-design.md | dispatch-cap-necessity | done | record | already declares 01M3A3TATJ64MW1D6S24VT9KQW |
| engine-rebuild-rearm-design.md | engine-rebuild-rearms-itself | done | record | already declares 01M3A3TATJ77BKJQFMKM7YNRQN |
| engine-runs-re-arm-on-a-landed-engine-design.md | engine-runs-re-arm-themselves-on-a-landed-engine | done | record | already declares 01M3A3TATJFFXH4Q4EXV7R8QND |
| fable-5-1-rollover-design.md | fable-5-1-model-rollover | done | record | already declares 01M3A3TATJ03N72B0WQDHH9DR9 |
| fable-model-alias-design.md | fable-model-alias | done | record | already declares 01M3A2YHDM2KFBQDV57GQRM3DX |
| failed-job-attention-design.md | failed-job-attention | accepted | record | already declares 01M3A2YHDMF72GCQ9EE257WK4T |
| fixture-children-cannot-outlive-their-test-design.md | fixture-children-cannot-outlive-their-test | done | record | already declares 01M3A2YHDMX5B1JTEYZNZ2N1KB |
| fleet-channel-gateway-design.md | fleet-channel-gateway | accepted | record | already declares 01M3A2YHDMBPXK67XBZDAMC8GP |
| fleet-doctor-repairs-what-stops-other-seats-design.md | fleet-doctor-repairs-what-stops-other-seats | accepted | record | already declares 01M3A2YHDMXBFPCK89427K1003 |
| fleet-join-bootstrap-design.md | fleet-join-bootstrap | accepted | record | already declares 01M3A2YHDMRDF85KG9ZR1CMXZ4 |
| fleet-pull-design.md | fleet-pull | accepted | record | already declares 01M3A2YHDMG2GZQ1WPPM1F0WTB |
| fleet-slack-channel-design.md | fleet-slack-channel | done | record | already declares 01M3A2YHDMADA13NBKF2MW56PZ |
| fleet-slack-channel-slice2-design.md | fleet-slack-channel | done | record | already declares 01M3A3TATJD02C652Z4NJX5NMT |
| goal-abandoned-with-a-reason-design.md | goal-abandoned-with-a-reason | done | record | already declares 01M3A2YHDM58J3QN8D9WNE2MN2 |
| goal-scope-bounds-design.md | goal-scope-bounds | accepted | record | already declares 01M3A2YHDM5JQWVGGGNA6HE6DM |
| goals-live-on-branches-design.md | goals-live-on-branches | done | record | already declares 01M3A2YHDM5Y2ZP4KHR300BJX4 |
| host-health-role-design.md | host-health-role | accepted | record | already declares 01M3A2YHDMGPFXHR8D1VDVX28C |
| host-implementer-wall-design.md | host-implementer-wall | done | record | already declares 01M3A2YHDMWD569BF8QNKKY8N9 |
| host-runtime-setup-design.md | host-runtime-setup | done | record | already declares 01M3A3TATJFZ3Q7JH0RVQZPE64 |
| human-approval-for-execution-design.md | human-approval-for-execution | done | record | already declares 01M3A2YHDM9FHK0CZBKKK1MS0D |
| human-break-glass-commit-keeps-the-ledger-valid-design.md | human-break-glass-commit-keeps-the-ledger-valid | accepted | record | already declares 01M3A2YHDM5CW8JEFDFBM42XM9 |
| human-carried-landing-carry-design.md | human-carried-landing-carry | done | record | already declares 01M3A2YHDMZ3P6VP1HP5WFPD1G |
| human-carried-landing-design.md | human-carried-landing | accepted | record | already declares 01M3A2YHDMKV7SET8MT3FFT8GS |
| human-goal-verbs-forgiving-design.md | human-goal-verbs-forgiving | done | record | already declares 01M3A3TATJCCS1WXTZCEG51BTK |
| human-proof-fits-the-act-design.md | human-proof-fits-the-act | done | record | already declares 01M3A3TATJ1A92Q1Y8NZPQE3KH |
| hung-proof-attempts-design.md | hung-proof-attempts-end-at-their-deadline | done | record | already declares 01M3A3TATJ7GK6KZ0R45VTJRBA |
| job-record-birth-token-design.md | job-record-birth-token | accepted | record | already declares 01M3A2YHDMDN8SWPGP84DX307V |
| land-ready-work-lands-without-a-claim-slot-design.md | land-ready-work-lands-without-a-claim-slot | done | record | already declares 01M3A2YHDM9VT15RJP0801CG8S |
| landing-receipt-survives-records-drift-design.md | landing-receipt-survives-records-drift | done | record | already declares 01M3A2YHDMGB0JNP7E0HB0VWFC |
| landing-runs-standard-design.md | landing-runs-standard-deep-runs-at-cadence | done | record | already declares 01M3A3TATJEQBVR8QK6JHGNQS1 |
| ledger-attention-design.md | ledger-attention | done | record | already declares 01M3A2YHDMK0HB6N074ZFQYZCA |
| live-records-landing-design.md | digest-landing-race | done | record | already declares 01M3A3TATJTT7DX0T003ND3QT8 |
| memory-architecture-design.md | memory-architecture | done | record | already declares 01M3A3TATJ5S075929NTZXM61T |
| metasystem-stop-design.md | metasystem-stop-verb | done | record | already declares 01M3A3TATJVS5JJJPZJXP0ZYQY |
| metasystem-stop-verb-design.md | metasystem-stop-verb | done | record | already declares 01M3A2YHDMGS19GJ1FB0J4M46J |
| missionrunner-patience-design.md | missionrunner-terminate-flake | accepted | record | already declares 01M3A3TATJGAQR8W09HPHQ1XEF |
| operator-surface-design.md |  | done | record | already declares 01M3A3TATJ5BGZRG5KKQHDSDMV |
| path-class-manifest-design.md | path-class-manifest | done | record | already declares 01M3A2YHDM8KVA3FVGP6RD7AE6 |
| process-steward-design.md | process-steward | superseded | record | already declares 01M3A2YHDMG6HKD9J5AQ3VG1Q3 |
| proof-admission-fits-a-seat-proving-several-units-design.md | proof-admission-fits-a-seat-proving-several-units | done | record | already declares 01M3A2YHDM3PZKPF8X41GSMA1R |
| proof-groups-detect-hangs-by-progress-not-the-clock-design.md | proof-groups-detect-hangs-by-progress-not-the-clock | done | record | already declares 01M3A2YHDM5GCAN114MP546CM1 |
| proof-groups-progress-hang-design.md | proof-groups-detect-hangs-by-progress-not-the-clock | done | record | already declares 01M3A3TATJACP8PRYV9BSW9KQG |
| proof-harness-custody-design.md | proof-harness-process-custody | done | record | already declares 01M3A3TATJ2VCQMZGAGBT8XTG8 |
| proof-run-cost-and-liveness-design.md | proof-run-cost-and-liveness | done | record | already declares 01M3A2YHDMM1ACWPYWY9EYD1HF |
| receipt-admission-attribution-design.md | receipt-admission-caps-concurrent-batteries | done | record | already declares 01M3A3TATJBMGTM0WE0YG1V0G8 |
| red-on-main-gets-an-owner-on-the-ledger-design.md | red-on-main-gets-an-owner-on-the-ledger | done | record | already declares 01M3A2YHDM5XV063F8Y33RVK95 |
| registered-wait-matches-the-runtime-session-design.md | registered-wait-matches-the-runtime-session | superseded | record | already declares 01M3A2YHDMV0SVQ4QJMWQWTHF1 |
| retained-proof-reuse-crosses-claims-and-attempts-design.md | retained-proof-reuse-crosses-claims-and-attempts | done | record | already declares 01M3A2YHDMA8MNA59MK25AJ87Z |
| role-context-composition-design.md | role-context-composition | accepted | record | already declares 01M3A2YHDMH8SYSGHE815A5B6Q |
| role-lane-packets-design.md | role-lane-packets | superseded | record | already declares 01M3A2YHDMZNZEV7P4BXTYPJTA |
| seat-mutual-awareness-design.md | seat-mutual-awareness | accepted | record | already declares 01M3A3TATJ3NB3JC43W342M8GC |
| seat-successor-continues-a-handoff-without-a-human-design.md | seat-successor-continues-a-handoff-without-a-human | accepted | record | already declares 01M3A2YHDN6AQBSRQC6ZHRZA24 |
| seats-spend-tokens-in-bounded-sessions-design.md | seats-spend-tokens-in-bounded-sessions | accepted | record | already declares 01M3A2YHDNH7RQJ6JV67ZW44K8 |
| severity-tiered-rigor-design.md | severity-tiered-rigor | done | record | already declares 01M3A2YHDND3FTGC116G78F28T |
| severity-tiered-rigor-p2-design.md | severity-tiered-rigor-p2 | done | record | already declares 01M3A3TATJGFK45GE96NCM3XBD |
| small-change-lane-design.md | small-change-lane | accepted | record | already declares 01M3A2YHDNX4ED00Y4PVBQTV29 |
| spend-cap-retirement-design.md | native-spend-cap-retirement | done | record | already declares 01M3A3TATJ3YEVJPPZ3R9DETDK |
| spend-fence-reports-tokens-per-model-and-cause-design.md | spend-fence-reports-tokens-per-model-and-cause | accepted | record | already declares 01M3A2YHDNJGCMYACYCAMFRRB1 |
| stop-decisions-record-deadline-evidence-design.md | stop-decisions-record-deadline-evidence | done | record | already declares 01M3A2YHDNQ4X9KAZN3MHX30SW |
| stop-hook-never-forces-an-empty-turn-design.md | stop-hook-never-forces-an-empty-turn | accepted | record | already declares 01M3A2YHDNKDH07EKHPMG4MEWZ |
| stop-infrastructure-allows-the-seat-to-stop-design.md | stop-infrastructure-allows-the-seat-to-stop | done | record | already declares 01M3A3TATJ9PSBKDWET84P0HFN |
| stop-message-fits-one-screen-design.md | stop-refusal-fits-on-one-screen | done | record | already declares 01M3A3TATJDBQG64QGR3WRQSJ7 |
| suite-custody-design.md | suite-custody | superseded | record | already declares 01M3A2YHDN2BT7CTG7CT37ZRYE |
| supervision-hook-root-design.md | supervision-hook-wrong-root hook-root-resolver-design | done | record | already declares 01M3A3TATJZRAA32FQBA9R6ZYC |
| time-bound-tests-design.md | time-bound-tests-run-on-artificial-clocks | done | record | already declares 01M3A3TATJT5V7NZDD5BR23RRC |
| token-spend-fence-design.md | token-spend-fence | accepted | record | already declares 01M3A2YHDN94AP79J2RT1875W1 |
| turn-verdict-hardening-design.md | turn-verdict-hardening | done | record | already declares 01M3A2YHDN5JGGCENJ1SY5198V |
| two-bars-caller-class-design.md | two-bars-for-changes | done | record | already declares 01M3A3TATJ28WCJ6S86JGXX1XK |
| two-bars-for-changes-design.md | two-bars-for-changes | done | record | already declares 01M3A3TATKTQ5T0F4PR6E353N9 |
| verification-loop-design.md | trunk-versus-candidate-is-a-verification-step failures-show-observed-against-expected round-proof-feeds-the-next-brief delegate-sandbox-runs-the-beds every-round-gets-an-independent-read | accepted | record | already declares 01M3A3TATKKHVQ8X49KNA4YDNB |
