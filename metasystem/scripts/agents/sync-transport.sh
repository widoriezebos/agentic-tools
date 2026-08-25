#!/usr/bin/env bash
set -euo pipefail

# Transport mirrors origin, never the local branch: this sync pushes
# origin's own ref, so a commit origin has not accepted structurally
# cannot reach transport through it. The branch argument is validated
# before it touches git — an option-shaped or ref-magic string must
# refuse, not select surprising refs.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
branch=${1:-main}

case "$branch" in
  -*|*..*|*//*|*' '*|'')
    echo "sync-transport refused: branch name '$branch' is not a plain branch" >&2
    exit 2
    ;;
esac
printf '%s' "$branch" | LC_ALL=C grep -Eq '^[A-Za-z0-9][A-Za-z0-9._/-]*$' || {
  echo "sync-transport refused: branch name '$branch' is not a plain branch" >&2
  exit 2
}

git fetch --quiet origin -- "$branch"
git rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null || {
  echo "sync-transport refused: origin has no branch '$branch'" >&2
  exit 1
}
git push transport "refs/remotes/origin/$branch:refs/heads/$branch" 2>&1 | tail -1
