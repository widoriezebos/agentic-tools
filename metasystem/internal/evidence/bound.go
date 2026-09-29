package evidence

// The evidence bound (design engine-owns-disk-lifetimes Part B 3.12; Wido
// 2026-09-28, option B): per checkout segment of an evidence root, past
// evidence.segment-cap-gib, the machine pass COMPACTS the eligible item
// with the oldest end time, one item per bound-lock acquisition, measuring
// the segment by a fresh stat walk under the lock before each cap test, and
// stops at the cap. Compaction drops a chain's logs and payloads and a
// bundle's members and keeps the record, the brief, the verdict, the
// cumulative inventory and the tombstone; it touches no blob, reference or
// recipe. Machinery never removes an item: a segment still over after
// compaction is a report line with the bytes each exclusion holds and the
// command pair a person runs. Unknown (a failed observation, an unreadable
// settings file, an unreadable end time, a held lock) does nothing to the
// item or segment and says so.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// SegmentIndex is evidence.root/segments/<segment>.json: the checkout a
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
	Jobs      []string `json:"jobs,omitempty"`
	Compacted bool     `json:"compacted,omitempty"`
	Attempt   string   `json:"attempt,omitempty"`
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
	if _, err := os.Lstat(filepath.Join(path, diskstore.CompactTombstoneName)); err == nil {
		item.Compacted = true
	}
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
	if _, err := os.Lstat(filepath.Join(path, diskstore.CompactTombstoneName)); err == nil {
		item.Compacted = true
	}
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

// KeptFor is a compaction's kept set for an item kind (3.12): a chain keeps
// its manifest, every job record, its brief and every round's return; a
// bundle keeps its recipe and owner file. Both keep the verdict and the
// tombstone.
func KeptFor(kind string) func(rel string) bool {
	switch kind {
	case diskstore.KindChain:
		return diskstore.ChainKept
	case diskstore.KindBundle:
		return func(rel string) bool { return rel == diskstore.DistilledName || rel == diskstore.OwnerFileName }
	}
	return nil
}

// Verdict is a compaction's VERDICT.txt. For a chain: one line per round
// (round|job|role|status|endedAt|claimed model|gaps=<n>|the first line of
// whatWasDone, at most 300 characters) from its record and return. For a
// bundle: the failure facts it holds (Round B2, F-6): every failing test
// and its failure lines, panics and exit lines, read from its members as
// they stand, gzipped ones inflated and blob-backed ones read from the
// host's store. A bundle whose failure facts cannot be extracted is not
// compacted: the error says so and the item is kept for a person.
func Verdict(item Item, blobs diskstore.BlobStore) ([]byte, error) {
	var lines []string
	switch item.Kind {
	case diskstore.KindChain:
		for _, job := range item.Jobs {
			record, err := readJSONObject(filepath.Join(item.Path, "jobs", job+".json"))
			if err != nil {
				continue
			}
			round := fmt.Sprint(record["round"])
			if value, ok := record["round"].(float64); ok {
				round = fmt.Sprintf("%d", int64(value))
			}
			returned, _ := readJSONObject(filepath.Join(item.Path, "rounds", round, "return.json"))
			claimed := ""
			if value, ok := returned["claimed"].(map[string]any); ok {
				claimed, _ = value["model"].(string)
			}
			gaps := 0
			if value, ok := returned["gaps"].([]any); ok {
				gaps = len(value)
			}
			done, _ := returned["whatWasDone"].(string)
			done, _, _ = strings.Cut(done, "\n")
			if len(done) > 300 {
				done = done[:300]
			}
			lines = append(lines, strings.Join([]string{round, job, str(record["role"]), str(record["status"]), str(record["endedAt"]),
				claimed, fmt.Sprintf("gaps=%d", gaps), done}, "|"))
		}
	case diskstore.KindBundle:
		facts, err := failureFacts(item.Path, blobs)
		if err != nil {
			return nil, err
		}
		if len(facts) == 0 {
			return nil, errors.New("its failure facts cannot be extracted (no failing test, panic or exit line in any member); it is kept for a person")
		}
		lines = append([]string{"failure facts of a suite-failure bundle of attempt " + item.Attempt + ":"}, facts...)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// pathPresent reports any entry at path, a directory included.
func pathPresent(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func str(value any) string {
	text, _ := value.(string)
	return text
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

// Bound is one pass of the bound over segments.
type Bound struct {
	// BoundLock is ~/.metasystem/stores/.bound.flock.
	BoundLock string
	Now       time.Time
	Entropy   io.Reader
	Sync      diskstore.Syncer
	By        string
	Judge     Judge
	Locks     Locks
	Blobs     diskstore.BlobStore
}

// Candidate reports whether machinery may compact the item now (3.12
// "Eligibility"), and why not.
func (b Bound) Candidate(segment Segment, item Item, ageFloor time.Duration) (bool, string) {
	switch {
	case item.Kind == diskstore.KindEvents:
		return false, "an events archive has no compact form"
	case item.Compacted:
		return false, "already compacted"
	case item.EndUnknown != "":
		return false, "end time unknown: " + item.EndUnknown
	case b.Now.Sub(item.EndedAt) < ageFloor:
		return false, "younger than the age floor"
	}
	return b.Settled(segment, item)
}

// Settled is eligibility (c): a closed chain whose payload the collector
// took in its armed, readable installation; a bundle fully distilled with a
// known goal.
func (b Bound) Settled(segment Segment, item Item) (bool, string) {
	switch item.Kind {
	case diskstore.KindChain:
		if !item.Closed {
			return false, "the chain is not closed"
		}
		if segment.Context == nil {
			return false, "its checkout is not armed"
		}
		if _, err := os.Lstat(filepath.Join(segment.Context.Installation, "artifacts", "agents", item.Name)); err == nil {
			return false, "its payload is still in " + segment.Context.Installation
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, "its installation cannot be read: " + err.Error()
		}
	case diskstore.KindBundle:
		if item.Unsettled != "" {
			return false, item.Unsettled
		}
		if item.Goal == diskstore.GoalUnknown || item.Goal == "" {
			return false, "its owner's goal is unknown"
		}
	}
	return true, ""
}

// Measure is the segment's total under the lock (3.12 "Measurement"): a
// stat walk of its three directories and its disposals ledger, plus the
// blob charges of its bundles' recipes; complete false when the walk did
// not finish within the context.
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
	if b.Blobs.Dir == "" {
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

// PassSettings are the numbers one segment's loop uses.
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

// CompactSegment runs the bound's loop over one segment (3.12 "The bound
// lock and the one step, oldest first"): per candidate, oldest end time
// first, it takes the bound lock and the chain's lifecycle locks without
// waiting, recovers an unfinished disposal of the item, measures the
// segment afresh, stops at or under the cap, judges the exclusions and
// compacts; then it reports the segment's position.
func (b Bound) CompactSegment(ctx context.Context, segment Segment, settings PassSettings) diskstore.EvidenceSegment {
	position := diskstore.EvidenceSegment{Segment: segment.Git, Root: segment.Root, Checkout: segment.CheckoutName(), CapBytes: settings.CapBytes, Held: map[string]int64{}}
	if segment.Unknown != "" {
		position.Unknown = segment.Unknown
		position.TotalBytes, position.BlobChargeBytes, _ = b.Measure(ctx, segment)
		return position
	}
	items, err := segment.Items(ctx)
	if err != nil {
		position.Unknown = "the segment cannot be listed: " + err.Error()
		return position
	}
	position.Pending = append(position.Pending, segment.OpenPersonDisposals(ctx)...)
	var candidates []Item
	for _, item := range items {
		if ok, _ := b.Candidate(segment, item, settings.AgeFloor); ok {
			candidates = append(candidates, item)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].EndedAt.Before(candidates[j].EndedAt) })
	held := map[string]string{}
	for _, item := range candidates {
		if ctx.Err() != nil {
			position.Pending = append(position.Pending, "the pass budget ran out before "+item.Name)
			break
		}
		outcome := b.compactOne(ctx, segment, settings, item, &position, b.segmentCap(segment, settings))
		if outcome.stop {
			break
		}
		if outcome.held != "" {
			held[item.Name] = outcome.held
		}
	}
	b.finishPosition(ctx, segment, settings, &position, held)
	return position
}

type stepOutcome struct {
	stop bool
	held string
}

// capTest is what one step measures, against which cap, under which rule:
// a segment's total against evidence.segment-cap-gib (rule bound), or the
// host's physical evidence bytes against evidence.machine-cap-gib (rule
// machine-cap).
type capTest struct {
	measure  func(ctx context.Context) (total, charges int64, complete bool)
	capBytes int64
	rule     string
	// where names what was measured, for a walk cut short.
	where string
}

func (b Bound) segmentCap(segment Segment, settings PassSettings) capTest {
	return capTest{measure: func(ctx context.Context) (int64, int64, bool) { return b.Measure(ctx, segment) },
		capBytes: settings.CapBytes, rule: diskstore.RuleBound, where: segment.Root}
}

// compactOne is one item's critical section.
func (b Bound) compactOne(ctx context.Context, segment Segment, settings PassSettings, item Item, position *diskstore.EvidenceSegment, test capTest) stepOutcome {
	lock, err := diskstore.TryBoundExclusive(b.BoundLock)
	if err != nil {
		position.Pending = append(position.Pending, "the bound lock is held (a disposal or another pass is in its step); the rest waits for the next pass")
		return stepOutcome{stop: true}
	}
	defer lock.Release()
	if item.Kind == diskstore.KindChain && b.Locks != nil {
		release, holder, err := b.Locks(segment.Context.Installation, chainJobs(segment, item), 0)
		if err != nil || holder != "" {
			reason := holder
			if err != nil {
				reason = err.Error()
			}
			position.Pending = append(position.Pending, item.Name+": a mirror or reap holds its job lifecycle lock ("+reason+"); skipped this pass")
			return stepOutcome{}
		}
		defer release()
	}
	stage, err := diskstore.NewID(b.Now, b.Entropy)
	if err != nil {
		position.Pending = append(position.Pending, item.Name+": "+err.Error())
		return stepOutcome{}
	}
	judgement := Judgement{}
	judged := false
	judge := func() Judgement {
		if !judged {
			judged = true
			judgement = b.judge(ctx, segment, item)
		}
		return judgement
	}
	if _, tombstone, open, err := diskstore.OpenDisposal(item.Path); err != nil {
		position.Pending = append(position.Pending, item.Name+": its tombstone is unreadable ("+err.Error()+"); a person decides: metasystem evidence show "+item.Path)
		return stepOutcome{}
	} else if open && !tombstone.MachineOwned() {
		position.Pending = append(position.Pending, PersonsOpenDisposal(item.Path, tombstone))
		return stepOutcome{}
	} else if open {
		_, err := diskstore.RecoverDisposal(ctx, item.Path, segment.Ledger(), b.receipt(segment, settings, item, Judgement{}, test.rule), b.Sync, stage,
			func(diskstore.Tombstone) diskstore.Recovery {
				current := judge()
				if reason := blocking(current); reason != "" {
					return diskstore.Recovery{Reason: reason}
				}
				return diskstore.Recovery{Continue: true, Commit: current.Commit}
			})
		if err != nil {
			position.Pending = append(position.Pending, item.Name+": its unfinished disposal could not be recovered: "+err.Error())
			return stepOutcome{}
		}
		return stepOutcome{}
	}
	total, charges, complete := test.measure(ctx)
	if test.rule == diskstore.RuleBound {
		position.TotalBytes, position.BlobChargeBytes = total, charges
	}
	if !complete {
		position.Pending = append(position.Pending, fmt.Sprintf("measurement did not finish within %s (reached %s of %s)",
			config.DiskSweepBudgetKey, formatGiB(total), test.where))
		return stepOutcome{stop: true}
	}
	if total <= test.capBytes {
		return stepOutcome{stop: true}
	}
	current := judge()
	if current.SegmentUnknown != "" {
		position.Unknown = current.SegmentUnknown
		return stepOutcome{stop: true}
	}
	if reason := blocking(current); reason != "" {
		return stepOutcome{held: reason}
	}
	verdict, err := Verdict(item, b.Blobs)
	if err != nil {
		position.Pending = append(position.Pending, item.Name+": not compacted: "+err.Error())
		return stepOutcome{held: "failure facts not extractable"}
	}
	receipt := b.receipt(segment, settings, item, current, test.rule)
	if receipt.ID, err = diskstore.NewReceiptID(b.Now, b.Entropy); err != nil {
		position.Pending = append(position.Pending, item.Name+": "+err.Error())
		return stepOutcome{}
	}
	receipt.SegmentBytesBefore, receipt.BlobChargeBytes = total, charges
	result, err := diskstore.Dispose(ctx, diskstore.DisposalStep{Item: item.Path, Receipt: receipt, Ledger: segment.Ledger(),
		Kept: KeptFor(item.Kind), Verdict: verdict, Commit: current.Commit, Stage: stage, Sync: b.Sync})
	switch {
	case err != nil:
		position.Pending = append(position.Pending, item.Name+": compaction stopped: "+err.Error())
	case result.RolledBack != "":
		position.Pending = append(position.Pending, item.Name+": rolled back, nothing dropped: "+result.RolledBack)
	case !result.Already:
		position.Compacted++
		position.FreedBytes += result.Receipt.ItemBytesBefore - result.Receipt.ItemBytesAfter
	}
	return stepOutcome{}
}

func (b Bound) judge(ctx context.Context, segment Segment, item Item) Judgement {
	if b.Judge == nil {
		return Judgement{Unknown: "the exclusions are not judged in this engine"}
	}
	return b.Judge(ctx, segment, item)
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

func (b Bound) receipt(segment Segment, settings PassSettings, item Item, judgement Judgement, rule string) diskstore.DisposalReceipt {
	return diskstore.DisposalReceipt{At: b.Now.UTC(), Segment: segment.Git, Checkout: segment.CheckoutName(), Kind: item.Kind, Item: item.Name,
		Rule: rule, By: b.By, LedgerTip: judgement.LedgerTip, LedgerIdentity: judgement.LedgerIdentity, Settings: settings.Values,
		EndedAt: item.EndedAt.Format(time.RFC3339), Goal: item.Goal, GoalState: judgement.GoalState, UncoveredReceipts: judgement.Uncovered,
		Citations: judgement.Citations}
}

// PersonsOpenDisposal is the line for a disposal the machine never finishes
// (Round B2, F-4): a person's removal or compaction interrupted before its
// commit point, with the command that finishes or rolls it back (the
// person's path re-verifies a recorded export first).
func PersonsOpenDisposal(item string, tombstone diskstore.Tombstone) string {
	command := "metasystem evidence dispose " + item + " --preview, then metasystem evidence dispose --plan ID"
	if tombstone.Plan != "" {
		command = "metasystem evidence dispose --plan " + tombstone.Plan
	}
	return fmt.Sprintf("%s: a %s by %s (rule %s) stopped before its commit point; the machine leaves it for a person: %s finishes or rolls it back",
		item, tombstone.Step, tombstone.By, tombstone.Rule, command)
}

// OpenPersonDisposals lists every disposal in the segment the machine
// leaves for a person, each with the command that settles it: removals
// found by their begun tombstones (the item may be set aside), and begun
// compactions that are not the machine's own.
func (s Segment) OpenPersonDisposals(ctx context.Context) []string {
	var lines []string
	seen := map[string]bool{}
	for _, directory := range s.Dirs() {
		if directory == "" {
			continue
		}
		for _, item := range diskstore.OpenRemovals(directory) {
			seen[item] = true
			_, tombstone, open, err := diskstore.OpenDisposal(item)
			switch {
			case err != nil:
				lines = append(lines, item+": its removal tombstone is unreadable ("+err.Error()+"); a person decides: metasystem evidence show "+item)
			case open:
				lines = append(lines, PersonsOpenDisposal(item, tombstone))
			}
		}
	}
	items, _ := s.Items(ctx)
	for _, item := range items {
		if seen[item.Path] {
			continue
		}
		if _, tombstone, open, err := diskstore.OpenDisposal(item.Path); err == nil && open && !tombstone.MachineOwned() {
			lines = append(lines, PersonsOpenDisposal(item.Path, tombstone))
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

// finishPosition measures the segment once more after the loop and, when it
// is still over its cap, fills in what holds it and what a person may
// remove, with the command pair.
func (b Bound) finishPosition(ctx context.Context, segment Segment, settings PassSettings, position *diskstore.EvidenceSegment, held map[string]string) {
	total, charges, complete := b.Measure(ctx, segment)
	if complete {
		position.TotalBytes, position.BlobChargeBytes = total, charges
	}
	if position.Unknown != "" || position.TotalBytes <= settings.CapBytes {
		return
	}
	position.Over = true
	items, _ := segment.Items(ctx)
	for _, item := range items {
		if reason, ok := held[item.Name]; ok {
			position.Held[reason] += item.Bytes
			continue
		}
		switch {
		case item.Compacted || item.Kind == diskstore.KindEvents && item.EndUnknown == "" && b.Now.Sub(item.EndedAt) >= settings.AgeFloor:
			position.Removable++
			position.RemovableBytes += item.Bytes
			if position.Oldest.IsZero() || item.EndedAt.Before(position.Oldest) {
				position.Oldest = item.EndedAt
			}
		case item.EndUnknown == "" && b.Now.Sub(item.EndedAt) < settings.AgeFloor:
			position.YoungBytes += item.Bytes
		default:
			position.AwaitingBytes += item.Bytes
		}
	}
	position.Commands = CommandPair(exportDirOf(segment))
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
