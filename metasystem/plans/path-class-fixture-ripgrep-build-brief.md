Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-fixture-ripgrep)
Date: 2026-09-04

# Build brief: the path-class fixture must not depend on ripgrep

Goal `path-class-fixture-ripgrep` (tier 1, approved by Wido 2026-09-04, box 1 hour / 3 attempts / 1 active job / no review round). Slice: one fixture script. Chain-unique finding ids are not needed; this chain has no critic.

## The defect

`scripts/agents/path-class-fixtures.sh`, test `TestDeletedListsHaveNoReader`, calls `rg` twice (the install-tree search around line 101 and the repository-tree search around line 116). Ripgrep is not in the declared command inventory of `docs/project-rules.md`. On a Mac without it the subshell exits 127, the status check `[[ $search_status -eq 1 ]]` fails, and the fixture reports a false reader of the deleted tables.

## What to build

Replace both `rg` calls with `grep` so the leg behaves identically on every supported host:

- `grep -rnE -I --exclude-dir=reviews --exclude=journey.md "$pattern" -- "${install_paths[@]}"` for the install-tree search (the two ripgrep globs excluded `docs/reviews/**` and `docs/journey.md`; the grep exclusions are by base name, which is acceptable here because no other `reviews` directory or `journey.md` is a behavior source).
- `grep -rnE -I "$pattern" -- "${repo_paths[@]}"` for the repository-tree search.
- `-I` (skip binary files) is required: ripgrep skipped binaries by default, grep would otherwise report `Binary file … matches` for the built engine and exit 0, a false reader.
- Keep the status contract: exit 1 means no reader (pass), 0 means a reader was found (fail with the listing), any other status (2, 127) fails loudly with the captured output so a broken search never passes silently. Extend the failure message so a non-1, non-0 status says the search itself failed and names the status.
- The pattern is already an extended regular expression (`a|b|c`), so `-E` keeps its meaning.

## Verification

Run `scripts/agents/path-class-fixtures.sh` seat-side is the orchestrator's job (KI-15: your sandbox cannot run process-owning fixture suites). In your sandbox: run `bash -n` on the script, and run the two grep commands by hand against the checkout with the same paths the test computes, showing exit status 1 for the current tree and exit status 0 after planting a temporary file containing `neverDirectFix` under a behavior path (remove it afterwards). Show both transcripts in the return.

## Bounds

Touch only `scripts/agents/path-class-fixtures.sh`. Do not add ripgrep to the inventory, do not change any other test, do not edit docs. Return within the box.
