# Live specimens: run processes that outlived their launcher

Captured 2026-09-16 08:32:19 CEST on m1c. NOTHING WAS KILLED to take this, and none of these belong to m1c.

The specimen m1e named (pid 3607, goal.test under a dead bash launcher in .claude/worktrees/ccf-s2-U2e) had already exited when m1c looked, four minutes later. These were found instead by sweeping for processes whose parent is init.

## Every orphan of a run, as observed
```
 1872     1  1872 10-23:12:24   0.0 S    /opt/homebrew/bin/limactl hostagent --pidfile /Users/wido/.lima/metasystem-debian/ha.pid --socket /Users/wido/.lima/metasystem-debian/ha.sock --guestagent /opt/homebrew/share/lima/lima-guestagent.Linux-aarch64.gz --nerd
 2185     1  2185 10-23:11:45   0.0 Ss   ssh: /Users/wido/.lima/metasystem-debian/ssh.sock [mux]                                   
 2961     1  2961       12:43   2.7 Ss   bash /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/scripts/agents/adapters/codex.sh dispatch --job m1b-cw5b4-1789539562 --start-gate /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/
17600     1 17600    10:59:53   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr5-1789500482/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell1709226971/002/artifacts/agents/steward/engine-pins/generation-1-adac8ea71
19900     1 19900    10:48:55   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr5-1789500482/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell400373082/002/artifacts/agents/steward/engine-pins/generation-1-adac8ea717
42698     1 42698       57:42   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/artifacts/agents/steward/engine-pins/generation-138-70da97552b5485cad67d7e04d3ac2d1b889e245849d43520c64e6a8eb9fd10f9 steward run --repo /Users/wido/LocalStora
44109     1 44109    10:56:04   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr5-1789500482/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell3985806400/002/artifacts/agents/steward/engine-pins/generation-1-adac8ea71
46085     1 46085    11:19:55   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr3-1789499161/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell1730147082/002/artifacts/agents/steward/engine-pins/generation-1-ed292b739
46894     1 46894    10:55:01   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr5-1789500482/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell3968497694/002/artifacts/agents/steward/engine-pins/generation-1-adac8ea71
55076     1 55076    10:05:27   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr8-1789503783/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell617352424/002/artifacts/agents/steward/engine-pins/generation-1-67a8c5ce08
62426     1 59388    21:00:29   0.0 S    /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/worktrees/steward-19174402a56e2c30/metasystem/bin/metasystem channel fake serve --dir /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T//metas
69076     1 69076    10:03:49   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.git/worktrees/m1b-u1cr8-1789503783/metasystem-build-cache/go-tmp/TestPendingWaitFromChildShell3005466794/002/artifacts/agents/steward/engine-pins/generation-1-67a8c5ce0
73832     1 73832       26:53   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/steward/engine-pins/generation-226-7724a4fb33bb9a60ae50a8098bb29f97868879c2aa69784089fa81f94a288edc steward run --repo /Users/wido/LocalStora
75080     1 75080       26:45   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/steward/engine-pins/generation-226-7724a4fb33bb9a60ae50a8098bb29f97868879c2aa69784089fa81f94a288edc supervise owner --repo /Users/wido/LocalS
77561     1 77561       52:49   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/artifacts/agents/steward/engine-pins/generation-138-70da97552b5485cad67d7e04d3ac2d1b889e245849d43520c64e6a8eb9fd10f9 supervise owner --repo /Users/wido/LocalS
81870     1 81870 07-08:27:46   0.0 Ss   /Users/wido/metasystem-evidence/agentic-tools/coordinator-loop-prevention/public-canary-20260908T205906Z/existing-app-valid/artifacts/agents/steward/engine-pins/generation-1-62fde1efe01e2813d5626ef58141ae07b34365b87e9ad
90376     1 90376       25:52   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1c/metasystem/artifacts/agents/steward/engine-pins/generation-136-23f2a00a45df7745781075788656c69f91e10653f98ee9bd60e119683f653a5d steward run --repo /Users/wido/LocalStora
91587     1 91587       25:45   0.0 Ss   /Users/wido/LocalStorage/GitHub/agentic-tools-m1c/metasystem/artifacts/agents/steward/engine-pins/generation-136-23f2a00a45df7745781075788656c69f91e10653f98ee9bd60e119683f653a5d supervise owner --repo /Users/wido/LocalS
```

## What they are
- Eight or more binaries named TestPendingWaitFrom... live in m1b's per-run build caches (.git/worktrees/m1b-u1cr3, -u1cr5, -u1cr8/metasystem-build-cache/go-tmp). Ages at capture: ten to eleven hours. They are a Go test's own child processes, still running long after the test, the run and the worktree that produced them.
- One metasystem binary from m1b's steward worktree, aged 21 hours.
- One codex.sh dispatch from m1b, aged 12 minutes, parent gone.
- One engine-pin binary from m1e's checkout, aged 57 minutes, parent gone.
- All of them have been reparented to init (ppid 1) and each is its own process-group leader, so nothing that started them can reap them by group.

## Why this is the goal, not a curiosity
Three shapes of one defect are now observed on this machine within twelve hours. At 04:38, sixteen TERM-ignoring busy loops from internal/goal/attention_test.go outlived their test by three and a half hours and took the load average to 40. At 08:01, a goal.test binary outlived the bash launcher that started it and ran on its own 45-minute timeout. And here, test children and engine binaries from three different worktrees have outlived their runs by ten to twenty-one hours. The rule is the same in each: a run's processes die with the run, whatever ignores whatever, and reaping cannot depend on the dying parent's cooperation or on a process group that no longer has a leader.

These belong to m1b and m1e; m1c has not touched them and has asked their owners to reap them.

## Second live producer, identified by m1b at 08:45

The eight TestPendingWaitFromChildShell binaries are children of member C of
registered-wait-matches-the-runtime-session, which LANDED as 53c6d2ee7 earlier
this morning. The fixture starts child shells through exec and its cleanup does
not reach every grandchild, so trunk currently ships a live producer of this
defect. m1b reaped the nine processes and is not opening a separate goal (Wido
stopped new work); it folds into fixture-children-cannot-outlive-their-test and
m1b will fix the fixture after the housekeeping pass.
