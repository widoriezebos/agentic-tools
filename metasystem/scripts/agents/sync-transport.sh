#!/usr/bin/env bash
set -euo pipefail

# Transport mirrors origin, never the local branch: this sync fetches
# origin's branch head by its FULL ref into the exact tracking ref,
# then pushes that tracking ref — a commit origin has not accepted,
# a tag shadowing the branch name, or a stale tracking ref cannot
# select what lands. The branch argument is validated as one whole
# byte string before git sees it; an explicitly empty argument is an
# error, not a default.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
if (( $# >= 1 )); then
  branch=$1
else
  branch=main
fi

case "$branch" in
  ''|-*|*..*|*//*|*' '*|*$'\n'*|*$'\t'*)
    echo "sync-transport refused: branch name '$branch' is not a plain branch" >&2
    exit 2
    ;;
esac
printf '%s' "$branch" | LC_ALL=C grep -zEq '^[A-Za-z0-9][A-Za-z0-9._/-]*$' || {
  echo "sync-transport refused: branch name '$branch' is not a plain branch" >&2
  exit 2
}

git fetch --quiet origin "+refs/heads/$branch:refs/remotes/origin/$branch" || {
  echo "sync-transport refused: origin has no branch '$branch'" >&2
  exit 1
}
git push transport "refs/remotes/origin/$branch:refs/heads/$branch" 2>&1 | tail -1
