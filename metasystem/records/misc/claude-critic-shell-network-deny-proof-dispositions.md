# claude-critic-shell-network-deny: the first live proof and its dispositions

Chain ccn-build1 landed at d471864f; the critic ccn-proof1 dispatched
by that engine (with a shell) reported preset none and network allow
in its own record: the tracked metasystem.conf pins the critic roles
to the none preset and an explicit key wins over the new dispatch
default. Its curl was refused anyway, because the claude sandbox's
allow shape denies egress (goal claude-network-allow-does-not-allow),
so a refused curl cannot certify this change; the proof is the
recorded effective network.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Real: the shipped configuration overrides the landed default on this repository; the orchestrator's brief had not surveyed metasystem.conf. | Chain ccn-fix1 (plans/claude-critic-shell-network-deny-conf-brief.md) sets the two keys to critic. |
| F-2 | accepted | Real: with egress denied under the allow shape too, a curl refusal proves nothing about this change. The proof becomes the recorded effective network of a dispatched critic and the fixture assertions. | The second proof critic reads its own record; the fixture bed pins the rule. |
| F-3 | accepted | The bed pinned only the code-critic's effective network. | Chain ccn-fix1 adds the design-critic (and warden where dispatched) assertions. |
