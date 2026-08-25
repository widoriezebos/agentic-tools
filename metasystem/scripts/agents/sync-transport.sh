#!/usr/bin/env bash
set -euo pipefail

# Transport mirrors origin, never the local branch (IL-30,
# retro-2026-08-25): pre-review candidate commits once reached
# transport through a bookkeeping push five hours before the review
# covenant's AGREE. This sync refuses to push any commit origin does
# not already have — the reviewed remote is the only source transport
# reflects, so unreviewed bytes structurally cannot leak.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
branch=${1:-main}

git fetch origin "$branch" --quiet
git push transport "origin/$branch:refs/heads/$branch" 2>&1 | tail -1
