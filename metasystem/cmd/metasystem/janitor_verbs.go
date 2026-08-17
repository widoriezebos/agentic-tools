package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

// runJanitorHeadroom checks free space on the distinct filesystems
// the given paths touch, against a floor. Exit 0 when all meet the
// floor, 3 when any is below (a named, distinguishable refusal a
// startup guard can branch on — not a mechanical error), 2 usage.
// Every result prints as one JSON line so a shell guard can read
// the per-filesystem deficit.
func runJanitorHeadroom(args []string) int {
	f := flag.NewFlagSet("janitor headroom", flag.ContinueOnError)
	floorGB := f.Float64("floor-gb", 2, "required free space per filesystem, in GiB (binary)")
	var paths multiFlag
	f.Var(&paths, "path", "a path whose filesystem to check (repeatable)")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem janitor headroom --path P [--path P ...] [--floor-gb N]")
		return 2
	}
	for _, p := range paths {
		if p == "" {
			fmt.Fprintln(os.Stderr, "janitor headroom: --path must not be empty")
			return 2
		}
	}
	// A floor must be a real non-negative number: NaN silently became
	// zero and negative/-Inf became always-pass (review finding 5).
	if math.IsNaN(*floorGB) || math.IsInf(*floorGB, 0) || *floorGB < 0 {
		fmt.Fprintln(os.Stderr, "janitor headroom: --floor-gb must be a finite non-negative number")
		return 2
	}
	const gib = float64(1 << 30)
	floorFloat := *floorGB * gib
	floorBytes := int64(math.MaxInt64)
	if floorFloat < float64(math.MaxInt64) {
		floorBytes = int64(floorFloat)
	}
	results, err := janitor.Headroom(paths, floorBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	below := false
	for _, r := range results {
		if r.BelowFloor() {
			below = true
		}
		line, _ := json.Marshal(map[string]any{
			"path":       r.Path,
			"freeBytes":  r.FreeBytes,
			"floorBytes": r.FloorBytes,
			"deficit":    r.Deficit(),
			"belowFloor": r.BelowFloor(),
		})
		fmt.Println(string(line))
	}
	if below {
		return 3
	}
	return 0
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprintf("%v", []string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
