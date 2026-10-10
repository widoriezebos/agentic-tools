# Task: revise the design runs-advance-on-their-own after critique round 1

Working Mode: Design
Revise plans/designs/runs-advance-on-their-own.md in place (keep Status: draft). Round 1 (Opus) found 4 material findings, each with a concrete change; apply them and add a "Round 1 findings and changes" table. Cap: unit <=250 production lines, <=5 units; cite code on origin/main and correct citations that moved (launch.go:306/631 are at 314/639).

1. D5: the settings and budget boundary owners do not exist (internal/config/defaults.go:72 goal.raise consumption deferred, :74 settings.apply unit-boundary application not implemented) and runs-admit-fresh-units-design-brief.md owns that consumption. D5 checks only the bound handoff, predecessor death and the existing engine re-arm (internal/steward/runner.go:368 RearmAtBoundary); remove settings and budget from D5's ordering and its test.
2. D3: under seat.driver=person an agent's `work wait` (cmd/metasystem/intent_work.go:1741-1752 waitUnit -> runner.Continue) only observes and renders the next act; only an invocation with directPersonProof (intent_work.go:2455) drives. Add the mutation: an agent's wait starts a step under the helm -> red.
3. D1: suite queue time already exists (test.go:723,913,923 QueueDurationMS from HostResourceLease.Waited(), host_resources.go:152, carried in the cost record). D1 uses it; its only new suite behavior is keeping the acquire wait out of deadlineCheck (test.go:884,917). No second queue-time record. Consider the review tree's UNIT_WAIT_RETRY re-entry (internal/launch/read_sequence.go:105) as D1's pending state home.
4. D4: the worker submits dispositions and brief through the existing public `work revise j2:<review> --dispositions --brief` (internal/launch/unit_revise.go:71); the driver only wakes the worker and observes the resulting revision. Remove "driver calls revise" from D4 and its test.
Return: the material count you believe remains and the unit table with lines.
