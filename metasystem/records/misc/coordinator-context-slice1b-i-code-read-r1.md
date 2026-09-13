# Opus read of coordinator-context slice 1b-i (2026-09-13, code-critique)

# CCB slice 1b-i: independent code critique (Opus)

Reviewed: the worktree diff (9 files, the same content as ccb-1bi.diff), the brief, the builder's result, and design sections 2 and 8b (F-4, rows 01 to 06).

Checks I ran: `go build ./...` ok. `go vet` on both packages ok. `go test -race -count=1 ./internal/output/` ok, with 70.1% coverage. The four focused cmd tests pass. I also ran a scratch copy of `fail_step` against a working engine, a missing engine and an unwritable root, scratch probes of Detect, Spill and Prune, and the engine's `audit coverage-ratchet` with the new package added.

## Layer 1: conformance

- The API matches the brief (output.go:20-156). The `publishTestingResult(root, path, result)` signature is as specified, and both callers pass `prepared.Installation` (test.go:837, 945).
- Output at or under the bound is byte-identical. `printJSON` is `json.Marshal` plus `Println` (helpers.go:14-21), which is exactly what the new inline branch does.
- The `output` family is registered after `janitor` (main.go:409-416).
- The `fail_step` order is as specified (land.sh:278-297). The EXIT trap and the push-retry tail (land.sh:1165) are untouched.
- The fixture assertions are as specified (land-fixtures.sh:1560-1564).
- No slice 1b-ii file was touched.
- Errexit is off at every `fail_step` call: `set -uo pipefail` at line 6 and `set +e` at line 165. A failed spill cannot abort the script. In the scratch runs the exit status stayed 73 and the tail still printed, with the working engine, the missing engine and the unwritable root.
- `$root` is the installation (land.sh:12), the same directory as `test run`'s ControlRoot, including in the prefixed layout. `artifacts/` is ignored by metasystem/.gitignore.

## Layer 2: findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | The new package `internal/output` has no coverage-ratchet floor. Every full `go-gate.sh` run will refuse. | go-gate.sh:776-786 runs `audit coverage-ratchet` over `go list ./internal/...`. coverage.go:97-104 refuses any measured package without a floor. Neither coverage-ratchet.json nor coverage-ratchet-linux.json lists `internal/output`. I ran the engine's ratchet with the measured 70.1%: `coverage ratchet: package internal/output (70.1%) has no ratchet floor; register it`, rc=1. Nothing catches this before landing: the builder ran only `--fast`, and commit.sh:413 also runs only `--fast`, which skips the ratchet. A human commit therefore lands cleanly, and the next `section/go-engine-gate` (in testing.json `cadence`) goes red. Files to change: both ratchet baselines. |
| N-1 | medium | no | No delivery-selected group runs any of the six new Go tests. | No testing.json group lists `internal/output`. `command-interface-smoke` names only `TestTestListCheckPlanAndVerifyWithoutLaunching`. The first time the contract runs these tests is the cadence full gate, which F-1 breaks. The brief's condition for editing testing.json ("the gate reports it unowned") can never fire from `--fast`. |
| N-2 | low | no | If the engine predates the `output` family, the spill failure prints the engine's full usage text, 436 lines, after the tail. | main.go:715-717 prints `usage()` on an unknown family. `fail_step` captures that with `2>&1` and prints it after the tail (land.sh:293-294). Measured: `ms nosuchfamily verb 2>&1 \| wc -l` = 436. This only happens between a pull and an engine rebuild. The exit status and the tail are unaffected. |
| N-3 | low | no | Spill and Prune treat a symlinked output directory differently. | `MkdirAll` and `OpenFile` follow a symlinked `artifacts/agents/output`, but Prune refuses it (output.go:131). Probe: the spill succeeded through the symlink, and Prune then returned "must not be a symlink". Files keep arriving and retention can never remove them. |
| N-4 | low | no | Prune stops at the first error and returns `nil`, so removals already made go unreported. | output.go:142-151: an ENOENT from Lstat or Remove, for example when two prunes race, returns `nil, err`. Files already deleted are never printed, and the CLI exits 1. Row 06 says "reports each". |
| N-5 | low | no | Detect decodes all seven fields, not only the two discriminator keys. | Probe: `{"outputMode":"file","schemaVersion":1,"bytes":"12"}` returns false. Because Go matches JSON keys case-insensitively, `{"OUTPUTMODE":"file","SchemaVersion":1}` returns true. No producer emits either form. A TestResult can never read as an envelope: it has no outputMode key, and `schemaVersion:"1"` returns false. An envelope with extra keys returns true. |
| N-6 | low | no | Several tests prove less than their names suggest. | The spill file name (UTC stamp, no colons, pid, nonce) is never asserted, even though output_test.go:18 passes a +01:00 time. The `<=` bound is untested: the spill test overshoots by at least 512 bytes and the inline test uses a tiny result. `TestPublishTestingResultSpillsOverTheBound` checks neither `Bytes`/`Digest` against the file nor that the path is under `root`. In the prune test, the symlink survives because its own (unaged) mtime is fresh, so the `IsRegular` skip is only indirectly proven. The fixture greps combined stdout and stderr, so it does not isolate stderr. |
| N-7 | low | no | `Line()` separates fields with spaces, so a root containing a space makes `path=` ambiguous for a later parser (the slice 2 Stop hook). | output.go:113 |
| N-8 | info | no | Prune's refusal of a negative window goes beyond the brief. It is reasonable and tested. | output.go:117, output_verbs.go:64 |

Other checks that found no defect:

- **Spill:** O_EXCL retries up to 8 times on EEXIST. The forced-collision test fails if the retry is removed. 32 concurrent writers produce 32 files. Files are 0600 and the directory 0700.
- **Path escapes:** the verb regex, the extension allowlist and the absolute-root check stop `../` in the verb or extension (probed).
- **Prune edges:** a missing directory returns `nil, nil`. A file exactly at the cutoff survives (`!Before`). Directories survive.
- **Coverage-delta branch:** it still prints the full step output after the spill.
- **Staticcheck:** I found nothing it would flag by reading. I did not run it; the builder's fast gate reports staticcheck passed.

## Verdict

1 material finding (F-1).

It is not fit to land by human commit yet. It would land, but the next cadence or full gate would refuse. Register a floor for `internal/output` in both ratchet baselines (70.1% measured on darwin). After that it is fit, pending the land bed's step-failure leg on the seat and the separate full `cmd/metasystem` run.

Disposition (seat): F-1 folded in the landing (floors for internal/output and, found by the same check, internal/hostload from 0c621e4b, in both ratchet baselines; the linux hostload floor seeded below the darwin value until a linux run re-seeds it). N-1 to N-8 recorded, not changed.
