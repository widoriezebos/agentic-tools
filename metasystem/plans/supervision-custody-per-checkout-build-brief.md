Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Build brief: supervision custody is per checkout; nothing crosses

Goal `supervision-custody-per-checkout` (tier 3, approved under R-79-m2, Wido's word of 2026-09-05: "This is serious and needs to be addressed immediately."; box 8 hours / 10 attempts / 1 active job / 3 review rounds). A code critic reviews before landing. This item is first in line on this machine.

## What happened

On 2026-09-05 at 01:22:18Z the machine-wide supervision registry (`~/.metasystem/armed-checkouts.jsonl`, one file per user per machine, custody keyed by the canonical checkout path per `docs/design/supervision-registry.md` REG-1) recorded this checkout's supervision components as `exited`, reason `terminated`. At that moment the supervision fixture's stop-hook-monitor scenario (preserved on `preserve/shv-build1-r9`, rounds 2 to 9 of chain shv-build1) had armed a temporary root in a temp directory with this session's main-process identity (`--pid $$` of the suite's shell, via `scripts/agents/arm-supervision.sh`) and this user's registry, and then shut that root down in its cleanup. The checkout's owner log shows it healthy up to the second it was killed. Health then reported the owner (lock owner pid 16315) and the watcher dead and the census stale, and every dispatch was refused until `metasystem up --repo .` at 01:29Z. It happened again at 01:35Z with round 9.

## The law

One machine runs many checkouts with many supervisors. A shutdown, takeover, relaunch, reap or sweep issued for checkout A must never terminate or retire the owner, watcher or runner of checkout B, whatever main-process identity or session name the request carries. Victim selection is by canonical checkout path, never by main pid, session tag or registry position.

## What to build, in this order

1. **The invariant test first**, in `internal/supervise` (or `internal/registry` where the selection lives): arm two real checkouts (two git worktrees or copies under a temporary directory, each with its own `artifacts/agents`) plus a third temporary root, all against one registry file in a temporary home, under the same main identity and then under different ones; then issue for one of them each of: shutdown, takeover by a fresh arm, relaunch, reap of a dead component, registry sweep; assert after each that the other two's owner, watcher and runner processes are alive and their registry rows untouched. Run it against the current code; it must reproduce the cross-kill (or show which operation crosses) before you fix anything. Report which operation and which code path selected the victim.
2. **The fix** in that path: select by canonical checkout path only; a request that names a main identity or session tag not matching the checkout's own row is refused with a message naming both paths, never acted on.
3. **The fixture rule**: one paragraph in `docs/orchestration.md` under the fixture guidance: a scenario that brings up supervision does so only with its own registry home (`METASYSTEM_SUPERVISION_HOME` or the mechanism the operator-layout scenario uses) and its own main identity, and shuts down only what it started; and a self-check in `scripts/agents/supervision-fixtures.sh` that fails the suite if any scenario arms with the seat's registry home or the suite shell's pid as the main.

## Verification

`go test ./internal/supervise/... ./internal/registry/...` with the new test green after the fix and red before it (show both); `gofmt -l`, `go vet`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on the touched packages; `bash -n` on the fixture. Your sandbox cannot run the process-owning suite (KI-15); the orchestrator runs `supervision-fixtures.sh` seat-side and checks `metasystem health --repo .` afterwards. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `internal/supervise`, `internal/registry`, `scripts/agents/arm-supervision.sh` if the arm path is where the identity is bound, `scripts/agents/supervision-fixtures.sh` for the self-check only, and `docs/orchestration.md`. Do not touch the stop-hook scenario's own legs (another goal owns them).
