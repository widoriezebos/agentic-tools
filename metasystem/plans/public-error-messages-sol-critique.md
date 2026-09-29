### Findings

- **SOL-EM-01 — high.** Read evidence: `metasystem/internal/report/frontier.go:frontierStatusWithGit` checks whether the caller is in a repository only when reading the frontier file fails. Outside a repository, a readable file passed with `--file` is printed and exits 0, so EM-20 still reports a false success. **Smallest fix:** check the repository before reading the file.

- **SOL-EM-02 — medium.** Read evidence: `metasystem/internal/report/frontier.go:frontierStatusWithGit` treats every file read error inside a repository as an absent frontier. For example, `--file .` reports “no frontier recorded” and exits 0 although `.` cannot be read as a file. **Smallest fix:** report non-absence read errors as unreadable with a nonzero exit.

- **SOL-EM-03 — low.** Read evidence: `metasystem/cmd/metasystem/intent_references.go:noReference` says `metasystem test run` printed the proof ID *when it started*, but `metasystem/cmd/metasystem/intent_work.go:runIntentTest` prints that reference after the test runner returns. **Smallest fix:** say the ID appears in the output of `metasystem test run`, without claiming when.

### Deferred

The builder reported one failing test on an unchanged path. Tests were not rerun for this read-only critique.

VERDICT: 3 material

Codex session ID: 01a0ea8f-74b9-7481-813d-6b1f3b71020e
Resume in Codex: codex resume 01a0ea8f-74b9-7481-813d-6b1f3b71020e
