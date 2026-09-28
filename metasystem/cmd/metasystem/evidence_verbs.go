package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The evidence family owns the durable-evidence lifecycle: raw run evidence
// under gitignored artifacts/ is mirrored to the evidence root before it
// counts as disposable, and disposable evidence eventually gets disposed of.

// evidenceGC is one collection pass for the checkout at root, with this
// process as the lease caller: an empty evidenceRoot means the checkout's
// resolved evidence root. Collection lines go to out, refusals to errOut.
func evidenceGC(root, evidenceRoot string, grace float64, out, errOut io.Writer) int {
	checkout, err := canonicalPath(root)
	if err != nil {
		fmt.Fprintln(errOut, "evidence-gc:", err)
		return 1
	}
	caller := int64(os.Getpid())
	holder, err := lease.RequireHolder(checkout, caller, nil)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	collect := func() error {
		if _, err := lease.RequireHolder(checkout, caller, holder.ClaimEpoch); err != nil {
			return err
		}
		target, err := evidenceGCTarget(checkout, evidenceRoot, nil)
		if err != nil {
			return err
		}
		return evidence.GC(checkout, target, grace, out)
	}
	if err := lease.WithHeld(checkout, caller, holder.ClaimEpoch, collect); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

// evidenceGCTarget is the root a collection pass reads: the explicit one,
// else the checkout's evidence root as the owner resolves it under lookup
// (nil is os.LookupEnv).
func evidenceGCTarget(checkout, explicit string, lookup func(string) (string, bool)) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	resolved, err := config.ResolveEvidenceRoot(config.EvidenceRootParams{
		ConfPath: filepath.Join(checkout, "metasystem.conf"), LookupEnv: lookup})
	if err != nil {
		return "", fmt.Errorf("evidence-gc refused: %v", err)
	}
	return resolved.Path, nil
}

// evidenceGCDefaultGrace is METASYSTEM_CHAIN_GRACE_SECONDS, else 5400 seconds.
func evidenceGCDefaultGrace() float64 {
	if value, err := strconv.ParseFloat(os.Getenv("METASYSTEM_CHAIN_GRACE_SECONDS"), 64); err == nil {
		return value
	}
	return 5400
}
