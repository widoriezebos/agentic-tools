#!/bin/bash
set -u
usage() { echo "usage: $0 <tag> <round> <previous-task-file> <out-file> [--constraints <n>]" >&2; exit 2; }
[ $# -ge 4 ] || usage
tag=$1 round=$2 previous=$3 out=$4; shift 4
constraints=0
if [ $# -gt 0 ]; then [ "$1" = --constraints ] && [ $# -eq 2 ] || usage; constraints=$2; fi
require_marker() { grep -Fq -- "$1" "$previous" || { echo "previous task is missing required text: $1" >&2; exit 1; }; }
case $round in
  2)
    for marker in \
      'for revision 1 of' \
      'review round 1 of 3' \
      "$tag-critique-r1.md" \
      '-design-r1.md' \
      "$tag-design-brief-r1.md"
    do
      require_marker "$marker"
    done
    sed -e 's/for revision 1 of/for revision 2 of/' -e 's/review round 1 of 3/review round 2 of 3/' \
      -e "s|$tag-critique-r1.md|$tag-critique-r2.md|" -e 's/-design-r1.md/-design-r2.md/' \
      -e "s|$tag-design-brief-r1.md|$tag-design-brief-r2.md|" \
      -e 's|written against a checkout 65 code files behind this worktree.s origin/main: a cited line number that is merely shifted is NOT a finding; a cited symbol, branch, file or behaviour that does not exist at origin/main IS.|revision 2 was written in this worktree against its own tree: a cited symbol, branch, file, line or behaviour that does not exist here IS a finding.|' "$previous" >"$out"
    cat >>"$out" <<EOF

Round 1's critique is artifacts/reports/$tag-critique-r1.md in this worktree. The page's "Fold of critique r1" section claims FIXED, REFUTED or DEFERRED for each round-1 finding. Check every claim FIRST: a FIXED that does not actually fix the failure is material; a REFUTED whose file:line evidence is wrong is material; a DEFERRED that leaves a DONE condition unmet is material. Do not re-raise a round-1 finding that was correctly fixed or correctly refuted. Then attack the content the fold added, with the same attack list.
EOF
    ;;
  3)
    for marker in 'for revision 2 of the design' 'review round 2 of 3.' "$tag-critique-r2.md in this worktree's" '-design-r2.md (untracked' "Its brief is artifacts/reports/$tag-design-brief-r2.md and" 'The design was revision 2 was written' "Round 1's critique is"; do require_marker "$marker"; done
    sed -e 's/for revision 2 of the design/for revision 3 of the design/' -e 's/review round 2 of 3\./review round 3 of 3, the LAST round./' \
      -e "s|$tag-critique-r2.md in this worktree's|$tag-critique-r3.md in this worktree's|" -e 's/-design-r2.md (untracked/-design-r3.md (untracked/' \
      -e "s|Its brief is artifacts/reports/$tag-design-brief-r2.md and|Its brief is artifacts/reports/$tag-design-brief-r3.md (the seat's fold prompt with its binding seat constraints C1-$constraints; check conformance to them, do not re-argue them) and|" \
      -e 's/The design was revision 2 was written/The design revision 3 was written/' "$previous" | sed '/^Round 1.s critique is/,$d' >"$out"
    cat >>"$out" <<EOF
Round 2's critique is artifacts/reports/$tag-critique-r2.md in this worktree, and round 1's is $tag-critique-r1.md beside it. The page's "Fold of critique r2" section claims FIXED, REFUTED, DEFERRED or ESCALATED for each round-2 finding. Check every claim FIRST, running each critic's named mutation against the r3 text:
- A FIXED that does not actually fix the failure is material.
- A REFUTED whose file:line evidence is wrong is material.
- A DEFERRED or ESCALATED that leaves a DONE condition unmet without an Escalation section is material.
Do not re-raise a finding that was correctly fixed or refuted in an earlier round. Then attack the content the fold added, using the same attack list. This is the last round: separate what blocks a build (material) from what a builder can settle inside a unit (not material), and be exact about which is which.
EOF
    ;;
  *) usage;;
esac
