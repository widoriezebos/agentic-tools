# VM validation sweep — 2026-08-30

The first Debian-guest validation across the ~30 landings since the
last guest run, executed per Wido's speed-up package (via m1).

## Protocol facts

- Transport re-established per the standing dual-push law: bare
  repository at `/Users/wido/LocalStorage/transport/agentic-tools.git`
  (the guest's mounted path is identical), `main` pushed from
  `origin/main`.
- Guest checkout `~/agentic-tools` fetched the transport remote and
  hard-reset to `transport/main`.
- HEAD equality CONFIRMED before any run (the standing law: never
  trust green without confirming VM HEAD): host `origin/main` =
  guest HEAD = `e35898c397b2255a507e5d3035a432787f44b6be`.
- Engine rebuilt in the guest (go1.26.5 linux/amd64); full
  `scripts/validate-metasystem.sh` run inside the guest, 13m10s.

## Verdict: RED — two platform findings, both opened as goals

1. `winddown-zombie-ownership-linux` (m2's seam, the slice-2
   wind-down): a zombie-holding group misclassifies as provably
   foreign on Linux — empty `/proc/<pid>/cmdline` reads as
   known-empty argv → NOT-OURS, and zombie-only group liveness
   differs from darwin. Two wind-down tests red on the guest,
   green on darwin. No wrong signal is ever sent; the defect is
   classification and evidence.
2. `supervise-start-gate-linux-red` (m1's seam): the launch owner
   accepted a blocked start gate
   (`TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate`).

Everything else in the guest run was green. Guest evidence:
`artifacts/agents/suite-failures/20260830T095355Z-745160` in the
guest checkout.

## What this run means

This is the genuinely different context the retired battery
pretended to be: both findings are platform divergences darwin
could never show, caught by one 13-minute guest run. The
retirement evidence is now real — the guest sweep found what the
tombstoned battery was believed to cover.
