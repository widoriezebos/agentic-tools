# Dispositions: enroll-login-crit2 (fresh final critique), round 1

Chain under review: enroll-login-build1 at its final work round
enroll-login-build1-r2 (reviewed tree
e89e011050946b35c7643929903f823d62af381d). Critic: enroll-login-crit2,
fresh session, zero material findings. Orchestrator: m1b.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| ELF-01 | noted | True as read: the first-read reason names the program when the owner is the failing fact. Decision D5 prescribes that sentence and a real setuid login always runs with effective uid 0, so the sub-case exists only in the fake reader. No artifact changes. | none |
| ELF-02 | noted | True as read: the second-read login-and-owner re-check sits behind equality checks that make it unreachable; defensive, no behavior. No artifact changes. | none |
| ELF-03 | noted | True as read: the workaround sentence rides every unreadable refusal, including a proof's. The advice applies there too and the recorded outcome codes are unchanged. No artifact changes. | none |

Gaps the critic named, answered by the orchestrator: the live enrollment
gap was closed by running the enrollment walk with the kernel reader
from this Mac's own Terminal.app shell (pid 61288) to its root-owned
login (pid 61287, executable /usr/bin/login, owner 0, arguments
withheld, parent Terminal): it enrolled, with the login recorded as the
session leader. The fixture-bed gap stands as stated: the dispatch and
goal command-line beds were not run for this landing; the gate was the
static checks, the identity, human-authority and whole command
packages, the parent reader's caller packages, the linux cross-build,
and the live proof.
