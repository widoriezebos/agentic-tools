# Dispositions: benchmark-validity closure, round 1

Chain design-critic-20260807t063006z-8fb2. All eight material findings accepted and folded; BV-1-9 noted and folded as wording.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BV-1-1 | accepted | V-2 was a no-op; the real defect sat in the runner record and was reproduced by hand. | V-2 rebuilt: collect-don't-abort closure, close-mirrors-diff, completion publishes after closure. |
| BV-1-2 | accepted | Provision cannot know the eleven identity fields. | V-3 moved to the cohort driver, with a single-run path. |
| BV-1-3 | accepted | Requiring new fields under version 1 breaks archives silently. | V-4: schema version 2, kit version bump, extractor dispatches by declared version. |
| BV-1-4 | accepted | Ruler changes are human-approved and version-bumped; kit validation is not acceptance. | Ownership rewritten; the human's in-session ratification recorded verbatim; kit and metasystem commits separated. |
| BV-1-5 | accepted | Multi-key modelUsage had no contract; dictionary order was accidental policy. | One key = effective; none = unreported; several = multi-model sentinel; both latter fail the roster gate closed. |
| BV-1-6 | accepted | Parking is suspension, not termination; shared closure destroys resume continuation. | V-2c: chains close on completion only; parked chains stay open under the lease. |
| BV-1-7 | accepted | The return duplicates the model and extraction cross-checks the copies. | Result telemetry flows through the normalization point; the fixture asserts both copies agree. |
| BV-1-8 | accepted | The producer legitimately emits failure censuses without those fields. | V-4 requires the fields for SUCCESS shapes and requires them null for the failure shapes. |
| BV-1-9 | noted | Runtime telemetry is candidate-side evidence, not attestation, under the declared trust model. | Wording now says exactly that. |
