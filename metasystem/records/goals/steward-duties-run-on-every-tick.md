# steward-duties-run-on-every-tick

- State: done
- Priority: 1
- Sequence: 51
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="finding 323 (03:12 10-15, runs-advance extraction read): when the keeper reports AgentFailed on a held reservation, landingStewardTick returns before the steward's duties (cmd/metasystem/landing_agent.go:490), so stale questions, incidents and flake fixes wait until a person runs work stop; and the production Unclaimed wiring (landing_agent.go:398) has no test — removing it passes every test"
- Tier: 2
- Intent: the lane's steward duties run on every tick whatever the keeper says about its agent, and the keeper's recovery wiring is proven in production shape
- Origin: human
- Next step: U1 brief (Astra to zero or stop rule), Codex build by hand after the lane fix, Opus read, land by hand, deploy
- Concluded: U1 landed by hand (lane steward duties run on every tick of the lane it keeps; a pending reservation reported and retried): read LAND, whole command package gate green, on main.
- OpenedAt: 2026-10-15T06:29:31Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T06:29:39Z revision=2 opid=DGHCK67ZWVZE7TYSSKXN47ECDM-m1e-670f65d0 authority=proven digest=51db15c0a1f46c7c87655b211e32fb1a497585f0cae1affd442b07d340ce52ea episode=2

History:
- 2026-10-15T06:29:31Z 4VJBK9ZZGBB4NMSF8Z0MT79J3W-m1e-670f65d0 open actor=human:Wido targets=steward-duties-run-on-every-tick
- 2026-10-15T06:29:39Z DGHCK67ZWVZE7TYSSKXN47ECDM-m1e-670f65d0 approve actor=human:Wido targets=steward-duties-run-on-every-tick
- 2026-10-15T06:29:46Z 8CDD4QX17JV04GZB63N052NCGR-m1e-670f65d0 set-priority actor=human:Wido targets=steward-duties-run-on-every-tick reason=priority-order subject=steward-duties-run-on-every-tick from=unranked to=1:52 requested-sequence=append
- 2026-10-15T08:14:38Z 1FAA39GRBBSHDHG15EPFF0G3KK-m1e-670f65d0 done actor=human:Wido targets=a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,lane-proof-finds-its-own-engine,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does reason=priority-order from=1:52 to=1:51
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers
Integrity: sha256=14c85b58574ca1479b33005f4b2f75a227c5f5bab8034a42f083d3419de8156d
