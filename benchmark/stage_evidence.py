#!/usr/bin/env python3
"""Stage a produced repository for a held-out grader (KI-42).

A live mission's ACP transport leaves its fifo pair in the round
artifacts, and a named pipe cannot be copied by a grader's evidence
walk — grading died on one (bm-2dc rep 2, 2026-08-24). The registered
graders are immutable (versions.lock), so the KIT stages: regular
files, directories, and symlinks ride; fifos, sockets, and devices
are skipped and PRINTED one per line on stdout — never silently (the
kit's no-silent-caps rule). The stage is a copy; the evidence itself
is never touched.

Usage: stage_evidence.py <source> <stage-destination>
Exit 0 with skipped paths on stdout; nonzero on any staging failure.
"""

import os
import shutil
import stat
import sys


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: stage_evidence.py <source> <stage>", file=sys.stderr)
        return 2
    source, stage = sys.argv[1], sys.argv[2]
    skipped = []
    for root, dirs, files in os.walk(source):
        rel = os.path.relpath(root, source)
        dest_dir = os.path.join(stage, rel) if rel != "." else stage
        os.makedirs(dest_dir, exist_ok=True)
        for name in list(dirs):
            src = os.path.join(root, name)
            if os.path.islink(src):
                os.symlink(os.readlink(src), os.path.join(dest_dir, name))
                dirs.remove(name)
        for name in files:
            src = os.path.join(root, name)
            dst = os.path.join(dest_dir, name)
            mode = os.lstat(src).st_mode
            if stat.S_ISLNK(mode):
                os.symlink(os.readlink(src), dst)
            elif stat.S_ISREG(mode):
                shutil.copy2(src, dst)
            else:
                skipped.append(os.path.join(rel, name) if rel != "." else name)
    for path in skipped:
        print(path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
