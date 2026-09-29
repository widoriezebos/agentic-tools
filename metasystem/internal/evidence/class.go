package evidence

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/placement"
)

// BoundClass is the evidence bound as one class of the machine pass (Round
// B2-3: it reports and never compacts or removes). Its plan discovers the
// roots and measures every segment; its apply runs the citation
// generation's next step, the registry of roots and the segment indexes,
// and the blob check and sweep; its finish adds every segment past its cap
// or with a person's removal left open, the machine cap's position and
// every root with its owner to the report.
type BoundClass struct {
	Bound         Bound
	Sync          diskstore.Syncer
	By            string
	UserHome      string
	HomeStateRoot string
	Checkouts     []HostCheckout
	// MachineCap and BlobGrace are the host keys in force; AgeFloor is the
	// longest age floor of the armed checkouts (how long a dangling blob
	// reference is reported before it goes).
	MachineCap int64
	BlobGrace  time.Duration
	AgeFloor   time.Duration
	// Citations is the host's index; nil leaves it where it is.
	Citations *Citations
	// SegmentSettings reads a segment's numbers; nil is SegmentSettings
	// (fixtures pass their own caps).
	SegmentSettings func(Segment) (PassSettings, error)

	positions []diskstore.EvidenceSegment
	lines     []string
	notes     []string
	hostBytes int64
	misplaced []diskstore.Line
}

// Name names the class.
func (c *BoundClass) Name() string { return "evidence bound" }

const (
	keyCitations = "0-citations"
	keyRegistry  = "0-registry"
	keyBlobs     = "1-blobs"
)

// Plan discovers the roots and measures every segment; it plans only the
// bookkeeping steps, and only when the pass applies.
func (c *BoundClass) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	registry, _ := ReadRootsRegistry(RootsRegistryPath(c.HomeStateRoot))
	roots := DiscoverRoots(c.UserHome, c.Checkouts, registry)
	c.positions, c.lines, c.notes, c.hostBytes, c.misplaced = nil, nil, nil, 0, nil
	for _, root := range roots {
		bytes, _, _ := diskstore.Measure(ctx, root.Path)
		c.hostBytes += bytes
		c.lines = append(c.lines, rootLine(root, bytes))
		for _, path := range root.NotManaged {
			if misplaced := placement.Of(path); misplaced.Kind != "" {
				treeBytes, _, _ := diskstore.Measure(ctx, path)
				c.misplaced = append(c.misplaced, diskstore.MisplacedLine(path, misplaced, treeBytes))
			}
		}
		for _, segment := range root.Segments {
			read := c.SegmentSettings
			if read == nil {
				read = SegmentSettings
			}
			settings, err := read(segment)
			if err != nil && segment.Unknown == "" {
				segment.Unknown = err.Error()
			}
			c.positions = append(c.positions, c.Bound.ReportSegment(ctx, segment, settings))
		}
	}
	if c.Bound.Blobs.Dir != "" {
		bytes, _, _ := diskstore.Measure(ctx, c.Bound.Blobs.Dir)
		c.hostBytes += bytes
	}
	if pass.Mode != diskstore.ModeApply {
		return nil, nil
	}
	return []diskstore.Item{
		{Class: c.Name(), Key: keyCitations, Path: c.HomeStateRoot, Verdict: release("advance the citation index")},
		{Class: c.Name(), Key: keyRegistry, Path: RootsRegistryPath(c.HomeStateRoot), Verdict: release("record the host's evidence roots")},
		{Class: c.Name(), Key: keyBlobs, Path: c.Bound.Blobs.Dir, Verdict: release("check the blob references and sweep")},
	}, nil
}

func release(reason string) diskstore.Verdict {
	return diskstore.Verdict{Decision: diskstore.Release, Reason: reason}
}

// Apply runs one planned bookkeeping step.
func (c *BoundClass) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	switch item.Key {
	case keyCitations:
		if c.Citations == nil {
			return diskstore.Verdict{Decision: diskstore.Wait}
		}
		// The generation takes at most half of what is left of the pass.
		step, cancel := context.WithTimeout(ctx, pass.Remaining(ctx)/2)
		defer cancel()
		// The index is the bound's bookkeeping, never a release.
		if _, err := c.Citations.Step(step); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the citation index could not advance: " + err.Error(), Command: "metasystem disk show"}
		}
		return diskstore.Verdict{Decision: diskstore.Wait}
	case keyRegistry:
		if err := RecordRoots(RootsRegistryPath(c.HomeStateRoot), c.Checkouts, c.Bound.Now, c.Sync); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the registry of evidence roots could not be written: " + err.Error(), Command: "metasystem disk show"}
		}
		for _, checkout := range c.Checkouts {
			if checkout.SettingsErr == nil && checkout.FactsErr == nil && checkout.Settings.EvidenceRoot.Path != "" {
				_, _ = RecordSegmentIndex(checkout.Settings.EvidenceRoot.Path, checkout.Facts, c.Bound.Now, c.Sync)
			}
		}
		return diskstore.Verdict{Decision: diskstore.Wait}
	case keyBlobs:
		armed := map[string]bool{}
		for _, checkout := range c.Checkouts {
			armed[checkout.Installation] = checkout.SettingsErr == nil
		}
		result := diskstore.BlobCheck{Blobs: c.Bound.Blobs, Now: c.Bound.Now, Grace: c.BlobGrace, AgeFloor: c.AgeFloor, By: c.By,
			Armed: func(installation string) bool { return armed[installation] }}.Run(ctx)
		for _, dangling := range result.Dangling {
			c.notes = append(c.notes, "dangling blob reference: "+dangling)
		}
		if result.Pending != "" {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: result.Pending, Command: "metasystem disk clean"}
		}
		if len(result.Removed)+len(result.Dropped) > 0 {
			return release(fmt.Sprintf("blob store: %d reference(s) dropped, %d blob(s) and stage(s) removed", len(result.Dropped), len(result.Removed)))
		}
	}
	return diskstore.Verdict{Decision: diskstore.Wait}
}

// Finish adds every segment a person has something to decide about, the
// machine cap's position and every root's line.
func (c *BoundClass) Finish(pass *diskstore.Pass, report *diskstore.Report) {
	positions := append([]diskstore.EvidenceSegment(nil), c.positions...)
	sort.SliceStable(positions, func(i, j int) bool { return positions[i].Segment < positions[j].Segment })
	for _, position := range positions {
		if position.Unknown != "" || position.Over || len(position.Pending) > 0 {
			report.Evidence = append(report.Evidence, position)
		}
	}
	if c.MachineCap > 0 && c.hostBytes > c.MachineCap {
		c.notes = append(c.notes, fmt.Sprintf("machine cap: the host's evidence holds %s, over %s = %s by %s; metasystem evidence show --all names every root, and metasystem evidence dispose --over-bound --export DIR --preview shows what a person can remove",
			formatGiB(c.hostBytes), config.DiskEvidenceMachineCapKey, formatGiB(c.MachineCap), formatGiB(c.hostBytes-c.MachineCap)))
	}
	report.EvidenceRoots = append(report.EvidenceRoots, c.lines...)
	report.Misplaced = append(report.Misplaced, c.misplaced...)
	report.Notes = append(report.Notes, c.notes...)
}

// rootLine is one root with its owner and bytes.
func rootLine(root Root, bytes int64) string {
	line := fmt.Sprintf("%s: %s, %s", root.Path, root.Owner, formatGiB(bytes))
	if count := len(root.NotManaged); count > 0 {
		line += fmt.Sprintf("; %d entr%s not managed (metasystem evidence show --all --verbose lists them; remove by hand if unneeded)", count, plural(count, "y", "ies"))
	}
	return line
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
