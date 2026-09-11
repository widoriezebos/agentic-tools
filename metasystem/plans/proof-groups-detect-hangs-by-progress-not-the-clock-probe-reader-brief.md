Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Build brief: the protected worker probe reads a newer candidate's result

Goal proof-groups-detect-hangs-by-progress-not-the-clock, a mechanical
step that slice 1 needs in order to land. The landing receipt is minted
by the retained destination engine; its protected probe
`emit-zero-tests` (metasystem/cmd/metasystem/test_protection.go,
runFrozenWorkerProbe) runs the candidate engine as a test worker and
reads the worker's result through readTestingWorkerResult, which
decodes with DisallowUnknownFields. Slice 1 adds four fields to every
group record (progressRule, cpuSeconds, longestSilentSeconds,
longestZeroCpuSeconds), so the probe fails with `json: unknown field
"progressRule"` and no receipt can be minted for a candidate that adds
a record field. Observed on the seat at 05:34Z on the receipt for tree
c7600667a03abba2c45db1e4c4a27ac109a597b1.

## Mandate

1. In runFrozenWorkerProbe read the candidate worker's result with a
   reader that ignores fields it does not know (plain json.Unmarshal of
   proofrun.TestResult, then proofrun.ValidateTestResult), keeping
   every judgement the probe makes today (exit status, one group,
   collectionComplete false, status invalid, no forged reuse). The
   strict reader stays for the destination's own worker results
   (metasystem/cmd/metasystem/test.go readTestingWorkerResult and
   readStrictJSON are unchanged).
2. A test in metasystem/cmd/metasystem: a worker result file whose
   group carries an unknown field is accepted by the probe's reader and
   still judged by status and collectionComplete; the same file is
   refused by readTestingWorkerResult.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 -run
'Probe|Protection|Worker' ./cmd/metasystem. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Never
delete written work.
