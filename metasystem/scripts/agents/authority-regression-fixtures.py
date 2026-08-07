#!/usr/bin/env python3
"""Regression proofs for the environment-free control-plane authority matrix."""

from __future__ import annotations

import importlib.util
import os
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
OWNER = ROOT / "scripts/agents/control-plane-authority.py"
SPEC = importlib.util.spec_from_file_location("control_plane_authority", OWNER)
assert SPEC and SPEC.loader
AUTHORITY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(AUTHORITY)


def refused(mode: str, classification: dict, job: str | None = None) -> None:
    try:
        AUTHORITY.authorize(mode, classification, job)
    except PermissionError:
        return
    raise AssertionError((mode, classification, job))


def main() -> int:
    holder = {"class": "MAIN", "holder": True, "mainId": "holder"}
    advisor = {"class": "MAIN", "holder": False, "mainId": "advisor"}
    delegate = {"class": "DELEGATE", "holder": False}
    supervision = {"class": "SUPERVISION", "holder": False}
    adapter = {"class": "ADAPTER-SUPERVISOR", "holder": False, "jobId": "owned-job"}
    human = {"class": "HUMAN", "holder": False}

    for mode in ("holder-only", "record-writer", "adapter-writer"):
        AUTHORITY.authorize(mode, holder, "any-job")
        AUTHORITY.authorize(mode, human, "any-job")
        refused(mode, advisor, "any-job")
        refused(mode, delegate, "any-job")
    AUTHORITY.authorize("record-writer", supervision, "job")
    AUTHORITY.authorize("supervision-only", supervision, None)
    refused("supervision-only", advisor)
    refused("supervision-only", holder)
    AUTHORITY.authorize("adapter-writer", adapter, "owned-job")
    AUTHORITY.authorize("record-writer", adapter, "owned-job")
    refused("adapter-writer", adapter, "foreign-job")
    refused("holder-only", adapter, "owned-job")

    # WC-1: even a caller that supplies the implementation's retired marker
    # cannot bypass a positive delegate classification.
    environment = dict(os.environ)
    environment["METASYSTEM" + "_LEASE_FENCE"] = "fixture"
    result = subprocess.run(
        [
            str(OWNER),
            "--mode",
            "record-writer",
            "--classification",
            '{"class":"DELEGATE","holder":false}',
            "--job",
            "foreign-job",
        ],
        env=environment,
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode != 0
    assert "DELEGATE" in result.stderr

    sources = [
        ROOT / "scripts/agents/dispatch.sh",
        ROOT / "scripts/agents/worktree-lease.py",
        ROOT / "scripts/agents/commit.sh",
        ROOT / "scripts/agents/evidence-gc.sh",
    ]
    retired = "METASYSTEM" + "_LEASE_FENCE"
    assert all(retired not in path.read_text(encoding="utf-8") for path in sources)

    dispatch = (ROOT / "scripts/agents/dispatch.sh").read_text(encoding="utf-8")
    wait_body = dispatch.split("wait_for_job()", 1)[1].split("aggregate_chain_usage()", 1)[0]
    # WC-3: both terminal collection and live reaping in --wait re-enter
    # through the lease-held verb; no bare reap remains in the wait loop.
    assert wait_body.count("lease_run_held") == 2
    assert "__reap-held" in wait_body and "reap_one \"$job\"" not in wait_body
    reap_body = dispatch.split("reap_jobs()", 1)[1].split("internal_register_custody()", 1)[0]
    # WC-9: interval reaping authenticates supervision before it enters its
    # destructive loop, while the start gate lets arming publish custody first.
    authority_offset = reap_body.index("internal_authority supervision-only")
    loop_offset = reap_body.index("while true")
    assert "start_gate" in reap_body and authority_offset < loop_offset
    print("authority regression fixtures: PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
