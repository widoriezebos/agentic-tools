package evidence

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// BoundClass is the evidence bound as one class of the machine pass: its
// plan lists, read-only, what the pass would compact; its apply runs the
// citation generation's next step, the registry, each segment's items
// oldest first (one item per bound-lock acquisition), the blob check and
// sweep, and the machine cap; its finish adds each segment's position and
// every root with its owner to the report.
type BoundClass struct {
	Bound         Bound
	UserHome      string
	HomeStateRoot string
	Checkouts     []HostCheckout
	// MachineCap and BlobGrace are the host keys in force; AgeFloor is the
	// longest age floor of the armed checkouts (how long a dangling blob
	// reference is reported before it goes).
	MachineCap int64
	BlobGrace  time.Duration
	AgeFloor   time.Duration
	Observe    Observer
	// Tip reads a checkout's accepted ledger tip per item (nil re-observes).
	Tip func(ctx context.Context, installation string) (string, error)
	// Citations is the host's index; nil keeps every item (Unknown).
	Citations *Citations
	// SegmentSettings reads a segment's numbers; nil is SegmentSettings
	// (fixtures pass their own caps).
	SegmentSettings func(Segment) (PassSettings, error)

	roots     []Root
	segments  map[string]Segment
	settings  map[string]PassSettings
	positions map[string]*diskstore.EvidenceSegment
	exclusion *Exclusions
	held      map[string]map[string]string
	lines     []string
	notes     []string
}

// Name names the class.
func (c *BoundClass) Name() string { return "evidence bound" }

const (
	keyCitations = "0-citations"
	keyRegistry  = "0-registry"
	keyBlobs     = "1-blobs"
	keyMachine   = "~machine-cap"
)

// Plan discovers the roots and lists the candidates of every segment over
// its cap, oldest first; in a preview each is judged on the accepted
// ledger as it stands, fetching nothing (R15).
func (c *BoundClass) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	registry, _ := ReadRootsRegistry(RootsRegistryPath(c.HomeStateRoot))
	c.roots = DiscoverRoots(c.UserHome, c.Checkouts, registry)
	c.segments, c.settings, c.positions = map[string]Segment{}, map[string]PassSettings{}, map[string]*diskstore.EvidenceSegment{}
	c.held = map[string]map[string]string{}
	var peers []Context
	var unreadable []string
	for _, checkout := range c.Checkouts {
		if checkout.FactsErr != nil {
			unreadable = append(unreadable, checkout.Installation)
		}
		if checkout.SettingsErr == nil && checkout.FactsErr == nil {
			peers = append(peers, Context{Installation: checkout.Installation, Facts: checkout.Facts, Settings: checkout.Settings})
		}
	}
	c.exclusion = &Exclusions{Observe: c.Observe, Fetch: pass.Mode == diskstore.ModeApply, Peers: peers, Unreadable: unreadable, Tip: c.Tip}
	if c.Citations != nil {
		c.exclusion.Citations = c.Citations
	}
	c.lines, c.notes = nil, nil
	var items []diskstore.Item
	if pass.Mode == diskstore.ModeApply {
		items = append(items, diskstore.Item{Class: c.Name(), Key: keyCitations, Path: c.HomeStateRoot, Verdict: release("advance the citation index")},
			diskstore.Item{Class: c.Name(), Key: keyRegistry, Path: RootsRegistryPath(c.HomeStateRoot), Verdict: release("record the host's evidence roots")})
	}
	for _, root := range c.roots {
		c.lines = append(c.lines, rootLine(ctx, root))
		for _, segment := range root.Segments {
			key := segment.Git
			if key == "" {
				key = "events-" + segment.Installation
			}
			position := &diskstore.EvidenceSegment{Segment: key, Root: segment.Root, Checkout: segment.CheckoutName(), Held: map[string]int64{}}
			c.positions[key] = position
			if segment.Unknown != "" {
				position.Unknown = segment.Unknown
				position.TotalBytes, position.BlobChargeBytes, _ = c.Bound.Measure(ctx, segment)
				continue
			}
			read := c.SegmentSettings
			if read == nil {
				read = SegmentSettings
			}
			settings, err := read(segment)
			if err != nil {
				position.Unknown = err.Error()
				continue
			}
			c.segments[key], c.settings[key] = segment, settings
			position.CapBytes = settings.CapBytes
			items = append(items, c.planSegment(ctx, pass, key, segment, settings, position)...)
		}
	}
	if pass.Mode == diskstore.ModeApply {
		items = append(items, diskstore.Item{Class: c.Name(), Key: keyBlobs, Path: c.Bound.Blobs.Dir, Verdict: release("check the blob references and sweep")},
			diskstore.Item{Class: c.Name(), Key: keyMachine, Path: c.HomeStateRoot, Verdict: release("apply the machine cap")})
	}
	return items, nil
}

func release(reason string) diskstore.Verdict {
	return diskstore.Verdict{Decision: diskstore.Release, Reason: reason}
}

// planSegment lists a segment's candidates past its cap, oldest first,
// each keyed so the pass visits them in that order.
func (c *BoundClass) planSegment(ctx context.Context, pass *diskstore.Pass, key string, segment Segment, settings PassSettings,
	position *diskstore.EvidenceSegment) []diskstore.Item {
	total, charges, complete := c.Bound.Measure(ctx, segment)
	position.TotalBytes, position.BlobChargeBytes = total, charges
	if !complete || total <= settings.CapBytes {
		return nil
	}
	candidates, _ := c.candidates(ctx, segment, settings)
	var items []diskstore.Item
	estimate := total
	for rank, item := range candidates {
		if estimate <= settings.CapBytes {
			break
		}
		planned := diskstore.Item{Class: c.Name(), Key: fmt.Sprintf("%s/%05d/%s", key, rank, item.Name), Path: item.Path,
			Verdict: release("compact: the oldest eligible item past " + config.DiskEvidenceSegmentCapKey)}
		if pass.Mode == diskstore.ModePreview {
			if reason := blocking(c.exclusion.Judge(ctx, segment, item)); reason != "" {
				planned.Verdict = diskstore.Verdict{Decision: diskstore.Keep, Reason: "held: " + reason, Command: "metasystem evidence show " + item.Path}
				items = append(items, planned)
				continue
			}
		}
		estimate -= item.Bytes
		items = append(items, planned)
	}
	return items
}

func (c *BoundClass) candidates(ctx context.Context, segment Segment, settings PassSettings) ([]Item, error) {
	items, err := segment.Items(ctx)
	if err != nil {
		return nil, err
	}
	var candidates []Item
	for _, item := range items {
		if ok, _ := c.Bound.Candidate(segment, item, settings.AgeFloor); ok {
			candidates = append(candidates, item)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].EndedAt.Before(candidates[j].EndedAt) })
	return candidates, nil
}

// Apply runs one planned step.
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
		if err := RecordRoots(RootsRegistryPath(c.HomeStateRoot), c.Checkouts, c.Bound.Now, c.Bound.Sync); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the registry of evidence roots could not be written: " + err.Error(), Command: "metasystem disk show"}
		}
		for _, checkout := range c.Checkouts {
			if checkout.SettingsErr == nil && checkout.FactsErr == nil && checkout.Settings.EvidenceRoot.Path != "" {
				_, _ = RecordSegmentIndex(checkout.Settings.EvidenceRoot.Path, checkout.Facts, c.Bound.Now, c.Bound.Sync)
			}
		}
		return diskstore.Verdict{Decision: diskstore.Wait}
	case keyBlobs:
		armed := map[string]bool{}
		for _, checkout := range c.Checkouts {
			armed[checkout.Installation] = checkout.SettingsErr == nil
		}
		result := diskstore.BlobCheck{Blobs: c.Bound.Blobs, Now: c.Bound.Now, Grace: c.BlobGrace, AgeFloor: c.AgeFloor, By: c.Bound.By,
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
		return diskstore.Verdict{Decision: diskstore.Wait}
	case keyMachine:
		bound := c.Bound
		bound.Judge = c.exclusion.Judge
		host := bound.CompactHost(ctx, c.roots, c.settings, c.MachineCap, c.positions)
		c.notes = append(c.notes, host.Lines()...)
		if host.Compacted > 0 {
			return release(fmt.Sprintf("machine cap: compacted %d item(s)", host.Compacted))
		}
		return diskstore.Verdict{Decision: diskstore.Wait}
	}
	key, _, _ := strings.Cut(item.Key, "/")
	segment, known := c.segments[key]
	if !known {
		return diskstore.Verdict{Decision: diskstore.Wait}
	}
	candidates, err := c.candidates(ctx, segment, c.settings[key])
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
	}
	for _, candidate := range candidates {
		if candidate.Path != item.Path {
			continue
		}
		bound := c.Bound
		bound.Judge = c.exclusion.Judge
		position := c.positions[key]
		before := position.Compacted
		outcome := bound.compactOne(ctx, segment, c.settings[key], candidate, position, bound.segmentCap(segment, c.settings[key]))
		switch {
		case position.Compacted > before:
			return release("compacted under " + config.DiskEvidenceSegmentCapKey)
		case outcome.held != "":
			if c.held[key] == nil {
				c.held[key] = map[string]string{}
			}
			c.held[key][candidate.Name] = outcome.held
			return diskstore.Verdict{Decision: diskstore.Keep, Reason: "held: " + outcome.held, Command: "metasystem evidence show " + candidate.Path}
		}
		return diskstore.Verdict{Decision: diskstore.Wait}
	}
	return diskstore.Verdict{Decision: diskstore.Wait}
}

// Finish adds every segment's position and every root's line.
func (c *BoundClass) Finish(pass *diskstore.Pass, report *diskstore.Report) {
	ctx := context.Background()
	keys := make([]string, 0, len(c.positions))
	for key := range c.positions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		position := c.positions[key]
		if segment, known := c.segments[key]; known && position.Unknown == "" {
			c.Bound.finishPosition(ctx, segment, c.settings[key], position, c.held[key])
		}
		if position.Unknown != "" || position.Over || position.Compacted > 0 || len(position.Pending) > 0 {
			report.Evidence = append(report.Evidence, *position)
		}
	}
	report.EvidenceRoots = append(report.EvidenceRoots, c.lines...)
	report.Notes = append(report.Notes, c.notes...)
}

// rootLine is one root with its owner and bytes.
func rootLine(ctx context.Context, root Root) string {
	bytes, _, _ := diskstore.Measure(ctx, root.Path)
	line := fmt.Sprintf("%s: %s, %s", root.Path, root.Owner, formatGiB(bytes))
	if len(root.Unsegmented) > 0 {
		line += fmt.Sprintf("; %d unsegmented entr%s (a person's to dispose of: metasystem evidence dispose PATH)", len(root.Unsegmented), plural(len(root.Unsegmented), "y", "ies"))
	}
	return line
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
