package evidence

// The evidence bound (design engine-owns-disk-lifetimes Part B 3.12; Wido
// 2026-09-28, option B; Round B2-3 scope cut): per checkout segment of an
// evidence root, the machine pass measures the segment against
// evidence.segment-cap-gib and, past it, only REPORTS by how much, with the
// command pair whose preview shows what a person can remove. Machinery
// never compacts and never removes an item; a person's removal from a
// previewed plan is the only disposal. Unknown (a failed observation, an
// unreadable settings file, an unreadable end time) keeps and says so.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// SegmentIndexSchema names a segment index's format.
const SegmentIndexSchema = "metasystem.evidence-segment/1"

// SegmentIndex is <the evidence root>/segments/<segment>.json: the checkout a
// segment belongs to, recorded when the pass first resolved it, and
// revalidated before the bound acts (3.12 "Checkout identity").
type SegmentIndex struct {
	Schema              string          `json:"schema"`
	GitRoot             string          `json:"gitRoot"`
	Installation        string          `json:"installation"`
	RootCommit          string          `json:"rootCommit,omitempty"`
	LedgerIdentity      string          `json:"ledgerIdentity,omitempty"`
	GitSegment          string          `json:"gitSegment"`
	InstallationSegment string          `json:"installationSegment"`
	FirstSeen           time.Time       `json:"firstSeen"`
	Retired             json.RawMessage `json:"retired,omitempty"`
}

// SegmentIndexPath is a segment's index.
func SegmentIndexPath(root, segment string) string {
	return filepath.Join(root, "segments", segment+".json")
}

// ReadSegmentIndex reads one index; absent is os.ErrNotExist.
func ReadSegmentIndex(root, segment string) (SegmentIndex, error) {
	data, err := os.ReadFile(SegmentIndexPath(root, segment))
	if err != nil {
		return SegmentIndex{}, err
	}
	var index SegmentIndex
	if err := json.Unmarshal(data, &index); err != nil || index.Schema != SegmentIndexSchema {
		if err == nil {
			err = errors.New("not a segment index")
		}
		return SegmentIndex{}, fmt.Errorf("%s is unreadable: %w", SegmentIndexPath(root, segment), err)
	}
	return index, nil
}

// RecordSegmentIndex writes the index idempotently: an index already present
// is kept as it is (its identity is what revalidation compares against); a
// missing ledger identity is filled in once the checkout has one.
func RecordSegmentIndex(root string, facts diskstore.CheckoutFacts, now time.Time, sync diskstore.Syncer) (SegmentIndex, error) {
	segment := diskstore.Segment(facts.GitRoot)
	index, err := ReadSegmentIndex(root, segment)
	switch {
	case err == nil:
		if index.LedgerIdentity != "" || facts.LedgerIdentity == "" || index.Installation != facts.Installation || index.RootCommit != facts.RootCommit {
			return index, nil
		}
		index.LedgerIdentity = facts.LedgerIdentity
	case errors.Is(err, os.ErrNotExist):
		index = SegmentIndex{Schema: SegmentIndexSchema, GitRoot: facts.GitRoot, Installation: facts.Installation, RootCommit: facts.RootCommit,
			LedgerIdentity: facts.LedgerIdentity, GitSegment: segment, InstallationSegment: diskstore.Segment(facts.Installation), FirstSeen: now.UTC()}
	default:
		return SegmentIndex{}, err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return SegmentIndex{}, err
	}
	return index, sync.WriteDurable(SegmentIndexPath(root, segment), append(data, '\n'), fmt.Sprintf("index-%d", now.UnixNano()))
}

// Revalidate compares an index with the checkout observed now: a reused
// checkout path, a changed installation or a re-adopted ledger makes the
// segment Unknown with the reason.
func (index SegmentIndex) Revalidate(facts diskstore.CheckoutFacts) string {
	switch {
	case index.Installation != facts.Installation || index.RootCommit != "" && facts.RootCommit != "" && index.RootCommit != facts.RootCommit:
		return "checkout path reused: the index records " + index.Installation + " at root commit " + short12(index.RootCommit)
	case index.LedgerIdentity == "":
		return "the segment's checkout has no ledger identity recorded yet"
	case facts.LedgerIdentity == "":
		return "the checkout's accepted ledger has no identity"
	case index.LedgerIdentity != facts.LedgerIdentity:
		return "ledger identity changed: " + index.LedgerIdentity + " now " + facts.LedgerIdentity
	}
	return ""
}

func short12(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

// Context is a segment's context checkout: the armed checkout whose git
// root and installation hash to the segment.
type Context struct {
	Installation string
	Facts        diskstore.CheckoutFacts
	Settings     diskstore.Settings
}

// Segment is one checkout's segment of one evidence root.
type Segment struct {
	Root string
	// Git and Installation are the segment names under agents/ and
	// suite-failures/, and under events/.
	Git, Installation string
	Context           *Context
	// LedgerIdentity is the identity the segment index recorded; every
	// observation must return it.
	LedgerIdentity string
	// Unknown, when set, is why the bound does nothing here this pass.
	Unknown string
}

// Dirs are the segment's three directories.
// A segment known only by one of its names (an orphan events segment has
// no git-root name) has "" for the directories it lacks: never the parent.
func (s Segment) Dirs() []string {
	dir := func(parent, name string) string {
		if name == "" {
			return ""
		}
		return filepath.Join(s.Root, parent, name)
	}
	return []string{dir("agents", s.Git), dir("suite-failures", s.Git), dir("events", s.Installation)}
}

// Ledger is the segment's disposals ledger.
func (s Segment) Ledger() string { return filepath.Join(s.Root, "disposals", s.Git+".jsonl") }

// CheckoutName names the segment's checkout for a person.
func (s Segment) CheckoutName() string {
	if s.Context != nil {
		return s.Context.Facts.GitRoot
	}
	return ""
}

// Item is one evidence item of a segment: a chain, a bundle or an events
// archive.
type Item struct {
	Kind    string    `json:"kind"`
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	EndedAt time.Time `json:"endedAt,omitempty"`
	// EndUnknown is why the end time cannot be read (Unknown keeps).
	EndUnknown string `json:"endUnknown,omitempty"`
	Goal       string `json:"goal,omitempty"`
	Closed     bool   `json:"closed,omitempty"`
	// Jobs are the job ids whose records the item holds (a chain).
	Jobs    []string `json:"jobs,omitempty"`
	Attempt string   `json:"attempt,omitempty"`
	// OwnerLedger is the ledger identity a bundle's owner file records.
	OwnerLedger string `json:"ownerLedger,omitempty"`
	// Unsettled is why a bundle is not settled.
	Unsettled string `json:"unsettled,omitempty"`
	Bytes     int64  `json:"bytes"`
}

var eventsStamp = regexp.MustCompile(`^events-(\d{8}T\d{6}Z)`)

// Items lists the segment's items with their facts. It reads only.
func (s Segment) Items(ctx context.Context) ([]Item, error) {
	var items []Item
	for index, directory := range s.Dirs() {
		if directory == "" {
			continue
		}
		entries, err := os.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return items, err
			}
			name := entry.Name()
			if diskstore.IsDisposalRecord(name) || diskstore.IsPartial(name) {
				continue
			}
			path := filepath.Join(directory, name)
			var item Item
			switch {
			case index == 0 && entry.IsDir():
				item = chainItem(path)
			case index == 1 && entry.IsDir():
				item = bundleItem(path)
			case index == 2 && entry.Type().IsRegular() && eventsStamp.MatchString(name):
				item = Item{Kind: diskstore.KindEvents, Name: name, Path: path}
				stamp, err := time.Parse("20060102T150405Z", eventsStamp.FindStringSubmatch(name)[1])
				if err != nil {
					item.EndUnknown = "its file name stamp does not parse"
				}
				item.EndedAt = stamp.UTC()
			default:
				continue
			}
			item.Bytes, _, _ = diskstore.Measure(ctx, path)
			items = append(items, item)
		}
	}
	return items, nil
}

func chainItem(path string) Item {
	name := filepath.Base(path)
	item := Item{Kind: diskstore.KindChain, Name: name, Path: path}
	matches, _ := filepath.Glob(filepath.Join(path, "jobs", "*.json"))
	sort.Strings(matches)
	var newest time.Time
	for _, match := range matches {
		record, err := readJSONObject(match)
		if err != nil {
			item.EndUnknown = "a job record is unreadable: " + err.Error()
			continue
		}
		item.Jobs = append(item.Jobs, strings.TrimSuffix(filepath.Base(match), ".json"))
		if record["jobId"] == name {
			item.Closed = record["chainClosed"] == true
			item.Goal, _ = record["goalId"].(string)
		}
		ended, _ := record["endedAt"].(string)
		at, err := time.Parse(time.RFC3339, ended)
		if err != nil {
			item.EndUnknown = "job " + filepath.Base(match) + " has no readable endedAt"
			continue
		}
		if at.After(newest) {
			newest = at
		}
	}
	if len(matches) == 0 {
		item.EndUnknown = "it holds no job record"
	}
	item.EndedAt = newest.UTC()
	return item
}

func bundleItem(path string) Item {
	item := Item{Kind: diskstore.KindBundle, Name: filepath.Base(path), Path: path}
	header, lines, present, err := diskstore.ReadDistilled(path)
	switch {
	case err != nil:
		item.EndUnknown = err.Error()
	case !present:
		item.EndUnknown, item.Unsettled = "it has no DISTILLED.txt", "not distilled"
	default:
		item.EndedAt = header.Created.UTC()
		for _, line := range lines {
			if line.Restores() && pathPresent(filepath.Join(path, filepath.FromSlash(line.Path))) {
				item.Unsettled = "a distillation is unfinished"
			}
		}
	}
	owner, err := diskstore.ReadBundleOwner(path)
	switch {
	case err != nil:
		item.Goal = diskstore.GoalUnknown
	default:
		item.Goal, item.Attempt, item.OwnerLedger = owner.Goal, owner.Attempt, owner.LedgerIdentity
	}
	return item
}

// pathPresent reports any entry at path, a directory included.
func pathPresent(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Judgement is the exclusions' answer for one item (3.12 clauses 1 to 3),
// taken under the bound lock after the segment's ledger observation.
type Judgement struct {
	// Held names each exclusion that holds the item ("goal G open",
	// "named by N receipts no retro covered", "cited by FILE").
	Held []string
	// Unknown is an Unknown for this item; SegmentUnknown for the whole
	// segment (a failed observation, an unreadable ledger).
	Unknown        string
	SegmentUnknown string
	GoalState      string
	Uncovered      int
	Citations      int
	LedgerTip      string
	LedgerIdentity string
	// Commit is the commit-point check (the receipt ledger re-hashed):
	// an error rolls the step back.
	Commit func() error
}

// Judge answers the exclusions for one item of a segment.
type Judge func(ctx context.Context, segment Segment, item Item) Judgement

// Locks takes a chain's job lifecycle locks in an installation, without
// waiting when wait is zero; held names the holder.
type Locks func(installation string, jobs []string, wait time.Duration) (release func(), held string, err error)

// Bound measures segments: the stat walk and the blob charges.
type Bound struct {
	Now   time.Time
	Blobs diskstore.BlobStore
}

// Removable reports whether a person's --over-bound set may select the item
// (3.12; Round B2-3): its end time is known and older than the age floor.
func (b Bound) Removable(item Item, ageFloor time.Duration) (bool, string) {
	switch {
	case item.EndUnknown != "":
		return false, "end time unknown: " + item.EndUnknown
	case b.Now.Sub(item.EndedAt) < ageFloor:
		return false, "younger than the age floor"
	}
	return true, ""
}

// Measure is the segment's total (3.12 "Measurement"): a stat walk of its
// three directories and its disposals ledger, plus the blob charges of its
// bundles' recipes; complete false when the walk did not finish within the
// context.
func (b Bound) Measure(ctx context.Context, segment Segment) (total, charges int64, complete bool) {
	complete = true
	for _, directory := range segment.Dirs() {
		if directory == "" {
			continue
		}
		bytes, _, done := diskstore.Measure(ctx, directory)
		total += bytes
		complete = complete && done
	}
	ledger, _, done := diskstore.Measure(ctx, segment.Ledger())
	total += ledger
	complete = complete && done && ctx.Err() == nil
	charges = b.blobCharges(ctx, segment)
	return total + charges, charges, complete && ctx.Err() == nil
}

// blobCharges charges every blob a segment's recipes name, in full, once
// per segment (DL4C-12).
func (b Bound) blobCharges(ctx context.Context, segment Segment) int64 {
	if b.Blobs.Dir == "" || segment.Git == "" {
		return 0
	}
	seen := map[string]bool{}
	var charges int64
	bundles, _ := os.ReadDir(filepath.Join(segment.Root, "suite-failures", segment.Git))
	for _, bundle := range bundles {
		if ctx.Err() != nil || !bundle.IsDir() {
			continue
		}
		_, lines, _, err := diskstore.ReadDistilled(filepath.Join(segment.Root, "suite-failures", segment.Git, bundle.Name()))
		if err != nil {
			continue
		}
		for _, line := range lines {
			if line.Kind != diskstore.RecipeBlob || seen[line.SHA256] {
				continue
			}
			seen[line.SHA256] = true
			bytes, _, _ := diskstore.Measure(ctx, b.Blobs.Path(line.SHA256))
			charges += bytes
		}
	}
	return charges
}

// PassSettings are the numbers one segment uses.
type PassSettings struct {
	CapBytes int64
	AgeFloor time.Duration
	// Values are recorded in each receipt and tombstone.
	Values map[string]string
}

// SegmentSettings reads a segment's settings from its context checkout.
func SegmentSettings(segment Segment) (PassSettings, error) {
	if segment.Context == nil {
		return PassSettings{}, errors.New("the segment has no context checkout")
	}
	settings := segment.Context.Settings
	return PassSettings{CapBytes: settings.Bytes(config.DiskEvidenceSegmentCapKey), AgeFloor: settings.Duration(config.DiskEvidenceAgeFloorKey),
		Values: map[string]string{config.DiskEvidenceSegmentCapKey: settings.Values[config.DiskEvidenceSegmentCapKey],
			config.DiskEvidenceAgeFloorKey: settings.Values[config.DiskEvidenceAgeFloorKey]}}, nil
}

// ReportSegment is one segment's position (Round B2-3, rule 1): its total
// against its cap and, past it, by how much with the command pair; the
// removals a person left open are pending. It reads only.
func (b Bound) ReportSegment(ctx context.Context, segment Segment, settings PassSettings) diskstore.EvidenceSegment {
	position := diskstore.EvidenceSegment{Segment: segment.Git, Root: segment.Root, Checkout: segment.CheckoutName(), CapBytes: settings.CapBytes}
	if position.Segment == "" {
		position.Segment = "events-" + segment.Installation
	}
	total, charges, complete := b.Measure(ctx, segment)
	position.TotalBytes, position.BlobChargeBytes = total, charges
	position.Pending = segment.OpenPersonDisposals()
	switch {
	case segment.Unknown != "":
		position.Unknown = segment.Unknown
		return position
	case !complete:
		position.Pending = append(position.Pending, fmt.Sprintf("measurement did not finish within %s (reached %s of %s)", config.DiskSweepBudgetKey, formatGiB(total), segment.Root))
		return position
	case total <= settings.CapBytes:
		return position
	}
	position.Over = true
	items, _ := segment.Items(ctx)
	for _, item := range items {
		if ok, _ := b.Removable(item, settings.AgeFloor); ok && (position.Oldest.IsZero() || item.EndedAt.Before(position.Oldest)) {
			position.Oldest = item.EndedAt
		}
	}
	position.Commands = CommandPair(exportDirOf(segment))
	return position
}

// blocking is why a judgement keeps the item, or "".
func blocking(judgement Judgement) string {
	switch {
	case judgement.SegmentUnknown != "":
		return judgement.SegmentUnknown
	case judgement.Unknown != "":
		return "unknown: " + judgement.Unknown
	case len(judgement.Held) > 0:
		return strings.Join(judgement.Held, "; ")
	}
	return ""
}

// PersonsOpenDisposal is the line for a person's removal that was cut
// short: show only reports it; evidence dispose, in any form, settles it
// first (Round B2-3, rule 2, as amended: show shows).
func PersonsOpenDisposal(item string, _ diskstore.Tombstone) string {
	return "an interrupted removal of " + item + " is open: metasystem evidence dispose settles it (rolls it back), then preview again"
}

// OpenPersonDisposals lists every removal in the segment that was cut
// short, found by its begun tombstone (the item may be set aside).
func (s Segment) OpenPersonDisposals() []string {
	var lines []string
	for _, directory := range s.Dirs() {
		if directory == "" {
			continue
		}
		for _, item := range diskstore.OpenRemovals(directory) {
			_, tombstone, open, err := diskstore.OpenDisposal(item)
			switch {
			case err != nil:
				lines = append(lines, item+": its removal tombstone is unreadable ("+err.Error()+"); a person decides")
			case open:
				lines = append(lines, PersonsOpenDisposal(item, tombstone))
			}
		}
	}
	return lines
}

// chainJobs is the chain's job set: every job record the item holds plus
// every jobs/<chain>*.json of its installation (final: a closed chain
// refuses a follow-up).
func chainJobs(segment Segment, item Item) []string {
	jobs := map[string]bool{}
	for _, job := range item.Jobs {
		jobs[job] = true
	}
	member := regexp.MustCompile("^" + regexp.QuoteMeta(item.Name) + `(-r[0-9]+)?$`)
	if segment.Context != nil {
		matches, _ := filepath.Glob(filepath.Join(segment.Context.Installation, "artifacts", "agents", "jobs", item.Name+"*.json"))
		for _, match := range matches {
			if stem := strings.TrimSuffix(filepath.Base(match), ".json"); member.MatchString(stem) {
				jobs[stem] = true
			}
		}
	}
	var sorted []string
	for job := range jobs {
		sorted = append(sorted, job)
	}
	sort.Strings(sorted)
	return sorted
}

func exportDirOf(segment Segment) string {
	if segment.Context == nil {
		return ""
	}
	return segment.Context.Settings.Values[config.DiskEvidenceExportDirKey]
}

// CommandPair is the ready pair disk show prints for a segment over its
// bound (3.12): the preview with the export directory spelled from
// evidence.export-dir when set, then the execution of the plan it prints.
func CommandPair(exportDir string) []string {
	if exportDir == "" {
		return []string{"metasystem evidence dispose --over-bound --export DIR --preview",
			"metasystem evidence dispose --plan ID",
			"(choose a directory outside every evidence root and checkout, or set " + config.DiskEvidenceExportDirKey + ")"}
	}
	return []string{"metasystem evidence dispose --over-bound --export " + exportDir + " --preview", "metasystem evidence dispose --plan ID"}
}

func formatGiB(bytes int64) string { return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(1<<30)) }
