# Brief: machinery-measures-its-own-process U1b-fixes (tests for a fix already made)

Working Mode: Implement
U1b is committed (b9e66a86b) and stopped with two material findings in cmd/metasystem/intent_process_measure.go unitMeasures; m1e already made the fix (uncommitted): (1) a finished run (every step terminal with a parseable collection time) has a window end at its last collection: questions opened at or after it are dropped and interval ends are clipped to it, so a finished run's person time never grows; (2) a run with no started step has no window and reports person time unavailable (not 0). TestProcessStepCostPublicStatus now expects exact (not lower-bound) person time for its finished run. Do not change production code unless a test shows the fix is wrong.

Write tests only, in cmd/metasystem/intent_process_measure_test.go through the public status route: (a) a finished run, then a goal question opened after its last collection (a later unit's) and answered: the run's person hours do not change (mutation: no window end, red); (b) a run whose only step has not started: person time shows unavailable, not 0 (mutation: no unavailable marker, red).

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -run '^(TestProcessStepCostPublicStatus|<your new tests>)$' ./cmd/metasystem/. Every new test calls t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and each test with its mutation result.
