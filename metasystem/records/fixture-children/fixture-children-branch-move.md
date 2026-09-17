# fixture-children: what moved to the goal branch, and what did not

Goal fixture-children-cannot-outlive-their-test. Written 2026-09-17 from git, the goal page and the backup. Times are
UTC. This says what happened. The plan in `metasystem/plans/goals-live-on-branches-design.md` section 9 differs.

## What moved, and when
Only unit 5f lived on `origin/goal/fixture-children-cannot-outlive-their-test`. No older unit was moved onto it.
- 2026-09-17 10:56: commit b9286339e, unit 5f, on parent a1f29152e, which was main at 10:43. On origin by 10:59.
- 11:00: a5923a8dd replaced it by a forced update. Same tree 24a36b8f, same parent. It adds the trailer
  `Goal-Unit: fixture-children-cannot-outlive-their-test/5f`. No ref holds b9286339e any more.
- 11:05: the read of a5923a8dd said land, with 0 material findings.
- 16:06: unit 5f landed on main as c6db36b81. The branch still points at a5923a8dd.

## What landed without the branch
Units 1 to 5a landed on main on 2026-09-16: 1 400218350, 2 9cafaae5f, 3a d44c98d87, 3c e1cc059ca, 3b 0dda2d620, 4a
41a545307, 4b c91c3ac21, 5a 7b32588a3. The six stacked units never went onto the branch. They landed on main on
2026-09-17 from 06:23 to 06:27: 5b0 a79fb69fd, 5b d2e1007b5, 5b2 708d52695, 5c 788063932, 5d a3de4a2ab, 5e 0671cc471.
They were proved only as one stack, by the deep proof `proof-mu54noqv-5bdf1f9e702e5fb3`, 54 of 54 groups passed. The
reads of all these units are history. Each one read a worktree, a bare tree or a diff file kept outside the
repository. A commit named in one is a base, never a commit that holds the unit. The landed commits replaced them.

## Earlier read records
The files are `metasystem/records/misc/fixture-children-u<record>.md`. "first" is the unit's first read.

| Record | Unit | Round | Digest named | Verdict | Main commit |
| --- | --- | --- | --- | --- | --- |
| 1-read | 1 | first | names no digest | no verdict line, 2 material | 400218350 |
| 1-read-round2 | 1 | 2 | names no digest | no verdict line, 0 new material | 400218350 |
| 2-read | 2 | first | names no digest | no verdict line, 1 material | 9cafaae5f |
| 2-read-round2 | 2 | 2 | names no digest | no verdict line, 0 new material | 9cafaae5f |
| 3-read | 3 | first | names no digest | no verdict line, 9 material | d44c98d87 e1cc059ca 0dda2d620 |
| 3-read-round2 | 3a 3b | 2 | names no digest | NOT LAND for 3a and 3b | d44c98d87 e1cc059ca 0dda2d620 |
| 3-read-round3 | 3a 3b 3c | 3 | names no digest | LAND for 3a, 3b and 3c | d44c98d87 e1cc059ca 0dda2d620 |
| 3b-read-round4 | 3b | 4 | file fcu3b-r3.diff, no digest | LAND | 0dda2d620 |
| 3b-read-round5 | 3b | 5 | files fcu3b-r3.diff and fcu3b-r4.diff, no digest | LAND | 0dda2d620 |
| 3b-read-round6 | 3b | 6 | file fcu3b-r5.diff, no digest | LAND | 0dda2d620 |
| 4a-read | 4a | first | base tree aac0e4ad52e0a62760f5a26d71e2c6074c490167, fcu4a-r5.diff | NOT LAND | 41a545307 |
| 4a-read-round2 | 4a | 2 | a210078b1c1532a59a7c870db0eb220375206757a1398a8631e5cee1c54b4446 | LAND | 41a545307 |
| 4b-read | 4b | first | base tree 19e7e829a56b8003ba1b8963c35ba1cc97f9c7d8, fcu4b.diff | LAND | c91c3ac21 |
| 5a-read | 5a | first | base tree cc03bb9c, no digest | NOT LAND | 7b32588a3 |
| 5a-read-round2 | 5a | 2 | 2bbf0e750a0457313a6401625899fc46bf9b9474aae5b1a4cd4ec7f341749d48 | NOT LAND | 7b32588a3 |
| 5a-read-round3 | 5a | 3 | 4d3453123b2c37d3236125785809319a5c95dfae78e881621aeb504cace94cf4 | LAND | 7b32588a3 |
| 5b-read | 5b | first | 80a4b53557b061d0d1c90f58e741a0c79fe46a917b3495625b9d9ff165b3fb4d | NOT LAND | d2e1007b5 |
| 5b-read-round5 | 5b0 5b 5b2 | 5 | 129492488e53716f36f763db025caa2e58c61ac5398393c77b09958d959f7032 | LAND | [1] |
| 5c-read | 5c | first | 3538ea9b443a02686a520237c68b5641854e39e1f8d29b2d36df20948860d291 | NOT LAND | 788063932 |
| 5c-read-round5 | 5c | 5 | fa3cb9a67bf5e83f87c1df8dd1869410d128d1cb06b3404c887bd040773993b7 | LAND | 788063932 |
| 5d-read | 5d | first | cf7c633996ed635660a0d75137ccda3354e4e5490ea5e5438d39d8034e5d8687 | NOT LAND | a3de4a2ab |
| 5d-read-round2 | 5d | 2 | 53e096545c0df4d83feb1e3040d109a5580686d5dbf5573cd110e9d748ee7c89 | NOT LAND | a3de4a2ab |
| 5d-read-round3 | 5d | 3 | 745c12e19d6086cc076477889506f71aaa70c5ed5faf16dd310e5402ee982302 | LAND | a3de4a2ab |
| 5e-read | 5e | first | 77b6ee949696cae49c4e85ac70149d0ac76ab001521f8f5e3ab21ac9b9d83b7d | NOT LAND | 0671cc471 |
| 5e-read-round2 | 5e | 2 | 5fd7fd1a3e1715db92a84944c6cf4c8fa52a4bb0dfbd939ccf5ff3e22bc469c6 | LAND | 0671cc471 |

[1] is a79fb69fd, d2e1007b5 and 708d52695. That read named one diff for 5b0, 5b and 5b2, so no single commit matches
it. 5a round 3 and both 5b records printed all 64 digits. 4a round 2 and the 5c, 5d and 5e records printed only the
first and last digits, and 5a round 2 named `fcu5a-r3-diff.sha256` without its value. For those nine, the full value
is from the matching `.sha256` file in the backup. The unit 3 reads cover what later landed as 3a, 3c and 3b.

## Unit 5f: the first read bound to a commit
5f had three reads. The first named diff sha256 41b8085c7d40584b68a5ac12096be53901c4f8d8a88eb4692279f2fab96aa25b and
said NOT LAND. The second named 2d7414f2...dde2 and said land. Both are history. The third says "Commit read:
a5923a8ddbfd29390354333cc1b9ffdd58543a6a" and "VERDICT: land (0 material findings)". These three records were only in
the m1c session scratchpad, as `read-fcu5f.md`, `read-fcu5f-fix.md` and `read-fcu5f-r3.md`. They are carried with this
record as `fixture-children-u5f-read.md`, `fixture-children-u5f-read-round2.md` and
`fixture-children-u5f-read-round3.md`. Checked for this record: `git diff --numstat a5923a8dd^ a5923a8dd` gives 11
files, 415 lines added and 97 removed. `git diff --numstat c6db36b81^ c6db36b81` gives the same 11 rows and one more,
a single added row in `metasystem/memory/receipts.log`. `git ls-tree -r` shows equal blob ids for 10 of the 11 files.
The blob of `metasystem/testing.json` differs because main changed that file between a1f29152e and fae71a23d, the
parent of c6db36b81. The unit's own change to it is one line out and one line in at both commits, and the changed
lines hash to the same sha256. c6db36b81 landed as unit 4 of 5 of a stack under the merge proof
`proof-mu5phhmb-72bf534216305871`.

## Records carried with this one
This record and the seven below live in `metasystem/records/fixture-children/`, not in `metasystem/records/misc/`.
They ride one `Goal-Plan` commit on the goal branch, and a `Goal-Plan` commit refuses a path under `records/misc/`:
on a goal branch that directory holds only the one prose record a `Goal-Read` commit carries beside its attestation.

The first four were only in `/Users/wido/LocalStorage/hact-20260912/backup-m1c-fixture-children/g18/`, under the same
names. They are copied next to this file byte for byte. Each line gives the file and its sha256.
- `fixture-children-amendment-witness9.md` 33bf21f030f34f801ee68bda2e762e8e013929aa1c97adf107b1dd6eeca20b2b
- `fixture-children-amendment-rev6.md` d8f8cb984bf46c36fa9c5623888cccb97ab1d794cf8827c5959e77ae03bf412f
- `fixture-children-u5d-decisions.md` c6820c46399c9861cd25cb4cfe9fedbfa31f9d9af6b521caf2327908eb71bf41
- `fixture-children-custodian-seat-rulings.md` cd9b534e22645c3bc301a88b32b114991efd96c1a738debd1d98e5b9e2868932

The three 5f reads are byte copies of the scratchpad files, under the names `metasystem/records/misc/` uses for this
goal's reads.
- `fixture-children-u5f-read.md` 54de5b3e2493ffebcc1e261f05c07c390876f2f200a653ebd9f01d76d167fd05
- `fixture-children-u5f-read-round2.md` 64396e34073f73d881f0cefcd85af99acd1e8023d39caa571265d2d6413cd438
- `fixture-children-u5f-read-round3.md` 646cb83a4c633f5ed1fdc72c983501db6639bf45f80fff42f3e3c9492d159e71

## Still unbuilt
Unit 5g and units 6 to 12. No commit on any ref names them, and 5f's is the only `Goal-Unit` trailer for this goal.
The goal is parked. Its page says they build on the branch once goals-live-on-branches unit 3 is on main.
