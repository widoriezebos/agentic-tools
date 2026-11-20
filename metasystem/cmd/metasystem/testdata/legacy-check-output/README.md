# Legacy unit check output

Captured by running `TestUnitCheckLegacyOutput` against commit
`96317af9e773480b82410594db78558db6c37054`, then verified without rewriting
fixtures against both omitted and explicit legacy declarations at that commit.
Each fixture preserves stdout, stderr, and the public command exit status.
The public route is `metasystem test run --unit-run RUN`, with and without
`--verbose`. Git and model launches are per-test fakes; the declared shell
commands execute and print fixed text. No output normalization is applied.
