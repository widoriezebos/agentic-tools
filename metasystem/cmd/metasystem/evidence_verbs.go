package main

import (
	"flag"
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

// runEvidenceGC runs one collection pass: collect closed terminal chains
// verified against the mirror manifest, prune mirrored job records past the
// grace window, sweep per-job residue, prune empty non-spine directories, and
// copy-then-age flight-recorder archives. The pass is a checkout write: it
// runs as the lease holder (this process is the supplied caller), under the
// lease lock at the holder's claim epoch, or ungated for a HUMAN caller.
// The evidence root defaults to the checkout's configured evidence.root and
// the grace window to METASYSTEM_CHAIN_GRACE_SECONDS or 5400 seconds.
func runEvidenceGC(args []string) int {
	flags := flag.NewFlagSet("evidence gc", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	evidenceRoot := flags.String("evidence", "", "durable evidence root (absolute path; default evidence.root)")
	grace := flags.Float64("grace-seconds", evidenceGCDefaultGrace(), "seconds a mirrored terminal chain's job records stay after mirroring")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal evidence gc [--root DIR] [--evidence DIR] [--grace-seconds SEC]")
		return 2
	}
	return evidenceGC(*root, *evidenceRoot, *grace, os.Stdout, os.Stderr)
}

// evidenceGC is one collection pass for the checkout at root, with this
// process as the lease caller: an empty evidenceRoot means the checkout's
// configured evidence.root. Collection lines go to out, refusals to errOut.
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
		target := evidenceRoot
		if target == "" {
			target, _, _ = config.Get(config.GetParams{
				Key: "evidence.root", Default: "", DefaultSet: true,
				ConfPath: filepath.Join(checkout, "metasystem.conf"),
			})
		}
		if !filepath.IsAbs(target) {
			return fmt.Errorf("evidence-gc refused: evidence.root is not configured")
		}
		return evidence.GC(checkout, target, grace, out)
	}
	if err := lease.WithHeld(checkout, caller, holder.ClaimEpoch, collect); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

// evidenceGCDefaultGrace is METASYSTEM_CHAIN_GRACE_SECONDS, else 5400 seconds.
func evidenceGCDefaultGrace() float64 {
	if value, err := strconv.ParseFloat(os.Getenv("METASYSTEM_CHAIN_GRACE_SECONDS"), 64); err == nil {
		return value
	}
	return 5400
}
