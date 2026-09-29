package evidence

// A person's evidence acts (design engine-owns-disk-lifetimes Part B 3.12
// "The public actions"; Wido 2026-09-28: "export before we then delete
// with the verb because I want a verb to help me clean this in a safe
// way"): show, export and dispose. Removal is only a person's act, from a
// previewed plan: the preview judges on the accepted ledger as it stands
// and fetches nothing; the execution observes the ledger afresh under the
// bound lock, re-judges every item, revalidates its device, inode and
// inventory digest, skips a held item unless --override (every override
// on the receipt and the tombstone), exports first when asked and removes
// only an item whose export verified. H1: a person is never refused; an
// item whose removal would damage something is declined with what would
// go wrong and the public command that settles it, and the rest completes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// statIDs are a file's device and inode.
func statIDs(info os.FileInfo) (uint64, uint64) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev), uint64(stat.Ino)
	}
	return 0, 0
}

// Env is what a person's evidence verb sees.
type Env struct {
	UserHome      string
	HomeStateRoot string
	// Checkouts are the host's armed checkouts; This is the invoking one.
	Checkouts []HostCheckout
	This      HostCheckout
	Now       time.Time
	Entropy   io.Reader
	Sync      diskstore.Syncer
	Observe   Observer
	// Tip reads a checkout's accepted ledger tip (no fetch); nil
	// re-observes every item.
	Tip       func(ctx context.Context, installation string) (string, error)
	Citations *Citations
	// Locks takes a chain's lifecycle locks; the verb waits the reaper's
	// bound.
	Locks Locks
	Blobs diskstore.BlobStore
	By    string
	// Session names the terminal session the verb runs in (its session id).
	Session string
	// SegmentSettings reads a segment's numbers; nil is SegmentSettings.
	SegmentSettings func(Segment) (PassSettings, error)
}

func (e Env) checkouts() []HostCheckout {
	for _, checkout := range e.Checkouts {
		if checkout.Installation == e.This.Installation {
			return e.Checkouts
		}
	}
	return append(append([]HostCheckout(nil), e.Checkouts...), e.This)
}

// Roots are the host's evidence roots.
func (e Env) Roots() []Root {
	registry, _ := ReadRootsRegistry(RootsRegistryPath(e.HomeStateRoot))
	return DiscoverRoots(e.UserHome, e.checkouts(), registry)
}

func (e Env) settingsOf(segment Segment) (PassSettings, error) {
	if e.SegmentSettings != nil {
		return e.SegmentSettings(segment)
	}
	return SegmentSettings(segment)
}

// Target is one located item.
type Target struct {
	Root    Root
	Segment Segment
	Item    Item
	// Unsegmented is a root's top-level entry outside every segment: a
	// person's to remove whole, with no compact form.
	Unsegmented bool
}

// ErrNotEvidence is a path outside every evidence root.
var ErrNotEvidence = errors.New("not in any evidence root of this host")

// structureNames are an evidence root's own directories and files: never an
// item (Round B2, F-3).
var structureNames = map[string]bool{"agents": true, "suite-failures": true, "events": true, "segments": true, "disposals": true, "RETIRED.json": true}

// segmentName12 reports a segment directory's name: twelve hex digits.
func segmentName12(name string) bool {
	if len(name) != 12 {
		return false
	}
	for _, r := range name {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// Resolve finds exactly the items a word names (Round B2, F-3): an item's
// path is that item; a segment's path is each of that segment's items; a
// root's top-level entry that is not its structure is that one unsegmented
// item (a legacy chain directly under agents/ is one too); a name is the
// item of that name in this checkout's segment. A root's structure
// directories, the root itself and a path inside an item are refused.
func (e Env) Resolve(ctx context.Context, argument string) ([]Target, error) {
	roots := e.Roots()
	if !filepath.IsAbs(argument) {
		for _, root := range roots {
			for _, segment := range root.Segments {
				if segment.Context == nil || segment.Context.Installation != e.This.Installation {
					continue
				}
				items, err := segment.Items(ctx)
				if err != nil {
					return nil, err
				}
				for _, item := range items {
					if item.Name == argument {
						return []Target{{Root: root, Segment: segment, Item: item}}, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("no item named %s in this checkout's segment; name it by its path", argument)
	}
	clean := filepath.Clean(argument)
	for _, root := range roots {
		rel, err := filepath.Rel(root.Path, clean)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if rel == "." {
			return nil, fmt.Errorf("%s is an evidence root, not an item; name its items or segments", clean)
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		structural := parts[0] == "agents" || parts[0] == "suite-failures" || parts[0] == "events"
		switch {
		case len(parts) == 1 && structureNames[parts[0]]:
			return nil, fmt.Errorf("%s is the root's %s directory, never an item; name a segment (%s/<segment>) or an item", clean, parts[0], clean)
		case len(parts) == 1:
			bytes, _, _ := diskstore.Measure(ctx, clean)
			return []Target{{Root: root, Unsegmented: true, Item: Item{Kind: diskstore.KindUnsegmented, Name: parts[0], Path: clean, Bytes: bytes}}}, nil
		case !structural:
			return nil, fmt.Errorf("%s lies inside %s; name the item itself", clean, filepath.Join(root.Path, parts[0]))
		case len(parts) == 2 && parts[0] == "agents" && !segmentName12(parts[1]):
			// A legacy chain mirrored before segments: unsegmented, judged
			// by the exclusions like every item.
			item := chainItem(clean)
			item.Kind = diskstore.KindUnsegmented
			item.Bytes, _, _ = diskstore.Measure(ctx, clean)
			return []Target{{Root: root, Unsegmented: true, Item: item}}, nil
		case len(parts) == 2:
			segment := segmentNamed(root, parts[0], parts[1])
			items, err := segment.Items(ctx)
			if err != nil {
				return nil, err
			}
			var targets []Target
			for _, item := range items {
				if filepath.Dir(item.Path) == clean {
					targets = append(targets, Target{Root: root, Segment: segment, Item: item})
				}
			}
			if len(targets) == 0 {
				return nil, fmt.Errorf("segment %s holds no item", clean)
			}
			return targets, nil
		case len(parts) == 3:
			item, err := itemAt(ctx, parts[0], clean)
			if err != nil {
				return nil, err
			}
			return []Target{{Root: root, Segment: segmentNamed(root, parts[0], parts[1]), Item: item}}, nil
		default:
			return nil, fmt.Errorf("%s lies inside the item %s; name the item itself (metasystem evidence show PATH answers for a file)", clean,
				filepath.Join(root.Path, parts[0], parts[1], parts[2]))
		}
	}
	return nil, fmt.Errorf("%s is %w", argument, ErrNotEvidence)
}

// Locate is Resolve for a word that must name exactly one item.
func (e Env) Locate(ctx context.Context, argument string) (Target, error) {
	targets, err := e.Resolve(ctx, argument)
	if err != nil {
		return Target{}, err
	}
	if len(targets) != 1 {
		return Target{}, fmt.Errorf("%s names %d items, not one", argument, len(targets))
	}
	return targets[0], nil
}

func segmentNamed(root Root, directory, name string) Segment {
	for _, segment := range root.Segments {
		if directory == "events" && segment.Installation == name || directory != "events" && segment.Git == name {
			return segment
		}
	}
	segment := Segment{Root: root.Path, Git: name, Unknown: "orphan segment " + name}
	if directory == "events" {
		segment = Segment{Root: root.Path, Installation: name, Unknown: "orphan segment events/" + name}
	}
	return segment
}

func itemAt(ctx context.Context, directory, path string) (Item, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Item{}, err
	}
	var item Item
	switch directory {
	case "agents":
		item = chainItem(path)
	case "suite-failures":
		item = bundleItem(path)
	default:
		item = Item{Kind: diskstore.KindEvents, Name: filepath.Base(path), Path: path}
		if match := eventsStamp.FindStringSubmatch(filepath.Base(path)); match != nil {
			item.EndedAt, _ = time.Parse("20060102T150405Z", match[1])
		}
	}
	_ = info
	item.Bytes, _, _ = diskstore.Measure(ctx, path)
	return item, nil
}

// PointerAnswer is what a path a record holds points at now (3.12 "The
// tombstone and the pointer").
type PointerAnswer struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Line   string `json:"line"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// Pointer resolves any path a record may hold: a live file, a file of a
// compacted or removed item from its tombstone's cumulative inventory
// (with the export when there was one), or a distilled file's logical
// original through its recipe line.
func Pointer(ctx context.Context, path string) PointerAnswer {
	clean := filepath.Clean(path)
	answer := PointerAnswer{Path: clean}
	if info, err := os.Lstat(clean); err == nil && info.Mode().IsRegular() {
		digest, size, _ := diskstore.FileDigest(ctx, clean)
		answer.State, answer.Size, answer.SHA256 = "live", size, digest
		answer.Line = fmt.Sprintf("%s: a live file, %d bytes, sha256 %s", clean, size, digest)
		return answer
	}
	for item := clean; item != filepath.Dir(item); item = filepath.Dir(item) {
		rel, _ := filepath.Rel(item, clean)
		rel = filepath.ToSlash(rel)
		if removed, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(item)); err == nil {
			return fromTombstone(answer, diskstore.RemovedTombstonePath(item), removed, rel, "removed")
		}
		if compacted, err := diskstore.ReadTombstone(filepath.Join(item, diskstore.CompactTombstoneName)); err == nil {
			return fromTombstone(answer, filepath.Join(item, diskstore.CompactTombstoneName), compacted, rel, "compacted")
		}
		if _, lines, present, err := diskstore.ReadDistilled(item); err == nil && present {
			for _, line := range lines {
				if line.Path == rel && line.Restores() {
					answer.State, answer.Size, answer.SHA256 = "distilled", line.Size, line.SHA256
					answer.Line = fmt.Sprintf("%s: distilled (%s) into %s; %d bytes, sha256 %s; restored through %s", clean, line.Kind, line.Replacement, line.Size, line.SHA256,
						filepath.Join(item, diskstore.DistilledName))
					return answer
				}
			}
		}
	}
	answer.State, answer.Line = "absent", clean+": no such evidence"
	return answer
}

func fromTombstone(answer PointerAnswer, tombstonePath string, tombstone diskstore.Tombstone, rel, state string) PointerAnswer {
	files, err := diskstore.TombstoneFiles(tombstonePath, tombstone)
	if err != nil {
		answer.State, answer.Line = "unknown", answer.Path+": the tombstone's inventory cannot be read: "+err.Error()
		return answer
	}
	// The cumulative history: an earlier tombstone carried whole answers
	// for every path that lay in the item then.
	for _, entry := range tombstone.History {
		var earlier diskstore.Tombstone
		if json.Unmarshal(entry.Tombstone, &earlier) == nil {
			files = append(files, earlier.Files...)
		}
	}
	for _, file := range files {
		if file.Path != rel && !(rel == "." && file.Path == tombstone.Item) {
			continue
		}
		answer.State, answer.Size, answer.SHA256 = state, file.Size, file.SHA256
		if file.Original != nil {
			answer.SHA256 = file.Original.SHA256
		}
		answer.Line = fmt.Sprintf("%s: %d bytes, sha256 %s; %s on %s under rule %s, receipt %s", answer.Path, answer.Size, answer.SHA256,
			state, tombstone.At.Format("2006-01-02"), tombstone.Rule, tombstone.Receipt)
		if tombstone.Export != nil {
			answer.Line += fmt.Sprintf("; exported to %s (%s)", tombstone.Export.Archive, tombstone.Export.ArchiveSHA256)
		}
		return answer
	}
	answer.State = state
	answer.Line = fmt.Sprintf("%s: the item was %s on %s under rule %s, receipt %s; this path was not in it", answer.Path, state,
		tombstone.At.Format("2006-01-02"), tombstone.Rule, tombstone.Receipt)
	return answer
}

// judgeTarget judges every item by the exclusions (Round B2, F-3): a
// segmented item in its segment; an unsegmented entry against its root's
// context checkout, held when the root has none.
func judgeTarget(ctx context.Context, exclusions *Exclusions, target Target) Judgement {
	if !target.Unsegmented {
		if target.Segment.Context == nil {
			return Judgement{Held: []string{"its segment has no armed context checkout: " + target.Segment.Unknown}}
		}
		return exclusions.Judge(ctx, target.Segment, target.Item)
	}
	for _, segment := range target.Root.Segments {
		if segment.Context != nil && segment.Unknown == "" {
			return exclusions.Judge(ctx, segment, target.Item)
		}
	}
	return Judgement{Held: []string{"its root has no armed context checkout to judge it against"}}
}

// DisposePlanSchema names a person's disposal plan.
const DisposePlanSchema = "metasystem.evidence-dispose-plan/1"

// PlannedDisposal is one item of a plan.
type PlannedDisposal struct {
	Path            string   `json:"path"`
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	Segment         string   `json:"segment,omitempty"`
	Root            string   `json:"root"`
	Step            string   `json:"step"`
	Files           int      `json:"files"`
	Bytes           int64    `json:"bytes"`
	Device          uint64   `json:"device,omitempty"`
	Inode           uint64   `json:"inode,omitempty"`
	InventoryDigest string   `json:"inventoryDigest"`
	Held            []string `json:"held,omitempty"`
	State           string   `json:"state"`
	// Decline is damage the step would do (H1): the item is declined with
	// the public command that settles it.
	Decline string `json:"decline,omitempty"`
}

// DisposePlan is a person's preview.
type DisposePlan struct {
	Schema    string            `json:"schema"`
	ID        string            `json:"id"`
	At        time.Time         `json:"at"`
	LedgerTip string            `json:"ledgerTip,omitempty"`
	Ledger    string            `json:"ledger"`
	Export    string            `json:"export,omitempty"`
	Compact   bool              `json:"compact,omitempty"`
	Items     []PlannedDisposal `json:"items"`
	StillOver []string          `json:"stillOver,omitempty"`
	// Session is the terminal session that previewed it: execution without
	// --plan takes only this session's newest preview (Round B2, F-8).
	Session string `json:"session,omitempty"`
}

// PlanPath is where a plan is written.
func PlanPath(homeStateRoot, id string) string {
	return filepath.Join(homeStateRoot, "stores", "plans", id+".json")
}

// OverBound is the removal set that brings every segment over its cap to
// or under it (3.12 --over-bound; DL4E-12): its compacted items and its
// events archives past the age floor, oldest end time first, until the
// segment's total from a stat walk now would be at or under the cap; held
// items are listed as held and do not count; an uncompacted eligible item
// is never in the set. The still-over lines say what it cannot select.
func (e Env) OverBound(ctx context.Context, judge func(Segment, Item) Judgement) ([]Target, []string) {
	var targets []Target
	var stillOver []string
	for _, root := range e.Roots() {
		for _, segment := range root.Segments {
			if segment.Unknown != "" || segment.Context == nil {
				continue
			}
			settings, err := e.settingsOf(segment)
			if err != nil {
				continue
			}
			total, _, complete := Bound{Blobs: e.Blobs}.Measure(ctx, segment)
			if !complete || total <= settings.CapBytes {
				continue
			}
			items, err := segment.Items(ctx)
			if err != nil {
				continue
			}
			sort.SliceStable(items, func(i, j int) bool { return items[i].EndedAt.Before(items[j].EndedAt) })
			var held, young, awaiting int64
			for _, item := range items {
				removable := item.EndUnknown == "" && e.Now.Sub(item.EndedAt) >= settings.AgeFloor && (item.Compacted || item.Kind == diskstore.KindEvents)
				switch {
				case !removable && item.EndUnknown == "" && e.Now.Sub(item.EndedAt) < settings.AgeFloor:
					young += item.Bytes
					continue
				case !removable:
					awaiting += item.Bytes
					continue
				case total <= settings.CapBytes:
					continue
				}
				targets = append(targets, Target{Root: root, Segment: segment, Item: item})
				if reason := blocking(judge(segment, item)); reason != "" {
					held += item.Bytes
					continue
				}
				total -= item.Bytes
			}
			if total > settings.CapBytes {
				stillOver = append(stillOver, fmt.Sprintf("still over by %s after this plan in %s: held %s; younger than the age floor %s; awaiting compaction %s; unsegmented %s (name them explicitly)",
					formatGiB(total-settings.CapBytes), segment.Git, formatGiB(held), formatGiB(young), formatGiB(awaiting), formatGiB(unsegmentedBytes(ctx, root))))
			}
		}
	}
	return targets, stillOver
}

func unsegmentedBytes(ctx context.Context, root Root) int64 {
	var total int64
	for _, entry := range root.Unsegmented {
		bytes, _, _ := diskstore.Measure(ctx, entry)
		total += bytes
	}
	return total
}

// Preview writes a person's plan (R15): each item with its step, file count
// and bytes, device, inode and inventory digest, and held or clear judged
// on the accepted ledger as it stands: no fetch, no ref advance. It writes
// exactly the plan file.
func (e Env) Preview(ctx context.Context, targets []Target, stillOver []string, compact bool, exportDir string) (DisposePlan, error) {
	id, err := diskstore.NewID(e.Now, e.Entropy)
	if err != nil {
		return DisposePlan{}, err
	}
	plan := DisposePlan{Schema: DisposePlanSchema, ID: id, At: e.Now.UTC(), Export: exportDir, Compact: compact, StillOver: stillOver, Items: []PlannedDisposal{},
		Session: e.Session}
	exclusions := e.Exclusions(false)
	plan.Ledger = "goal state unknown: no accepted ledger"
	for _, target := range targets {
		planned := e.plan(ctx, target, compact, exclusions)
		if target.Segment.Context != nil && plan.LedgerTip == "" {
			if view, unknown := exclusions.observe(ctx, target.Segment); unknown == "" {
				plan.LedgerTip = view.Tip
				plan.Ledger = "judged on the accepted ledger " + short12(view.Tip)
				if !view.Committed.IsZero() {
					age := e.Now.Sub(view.Committed).Round(time.Minute)
					plan.Ledger += ", " + age.String() + " old"
					if age > goal.StaleThreshold {
						plan.Ledger += "; execution observes the ledger afresh and re-judges every item"
					}
				}
			}
		}
		plan.Items = append(plan.Items, planned)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return plan, err
	}
	return plan, e.Sync.WriteDurable(PlanPath(e.HomeStateRoot, id), append(data, '\n'), "plan-"+id)
}

// Exclusions are this environment's judge: every armed checkout as a
// peer, the unreadable ones named, the accepted-tip check per item.
func (e Env) Exclusions(fetch bool) *Exclusions {
	var unreadable []string
	for _, checkout := range e.checkouts() {
		if checkout.FactsErr != nil {
			unreadable = append(unreadable, checkout.Installation)
		}
	}
	return &Exclusions{Observe: e.Observe, Fetch: fetch, Peers: e.peers(), Unreadable: unreadable, Tip: e.Tip, Citations: e.citer()}
}

func (e Env) peers() []Context {
	var peers []Context
	for _, checkout := range e.checkouts() {
		if checkout.SettingsErr == nil && checkout.FactsErr == nil {
			peers = append(peers, Context{Installation: checkout.Installation, Facts: checkout.Facts, Settings: checkout.Settings})
		}
	}
	return peers
}

func (e Env) citer() Citer {
	if e.Citations == nil {
		return nil
	}
	return e.Citations
}

// plan is one item's preview.
func (e Env) plan(ctx context.Context, target Target, compact bool, exclusions *Exclusions) PlannedDisposal {
	item := target.Item
	planned := PlannedDisposal{Path: item.Path, Name: item.Name, Kind: item.Kind, Segment: target.Segment.Git, Root: target.Root.Path,
		Step: diskstore.StepRemove, State: "clear"}
	if compact {
		planned.Step = diskstore.StepCompact
	}
	if info, err := os.Lstat(item.Path); err == nil {
		planned.Device, planned.Inode = statIDs(info)
	}
	files, err := diskstore.Inventory(ctx, item.Path, nil)
	if err != nil {
		planned.Decline = "the item cannot be read: " + err.Error()
		planned.State = "declined"
		return planned
	}
	planned.InventoryDigest = diskstore.InventoryDigest(files)
	for _, file := range files {
		if file.Original == nil {
			planned.Files++
			planned.Bytes += file.Size
		}
	}
	if compact && (item.Compacted || KeptFor(item.Kind) == nil || target.Unsegmented) {
		planned.Decline = fmt.Sprintf("no compact form; without --compact the item is removed whole: %d files, %d bytes", planned.Files, planned.Bytes)
		planned.State = "declined"
		return planned
	}
	if reason := blocking(judgeTarget(ctx, exclusions, target)); reason != "" {
		planned.Held = strings.Split(reason, "; ")
		planned.State = "held"
	}
	return planned
}

// ReadDisposePlan reads a plan by id.
func ReadDisposePlan(homeStateRoot, id string) (DisposePlan, error) {
	data, err := os.ReadFile(PlanPath(homeStateRoot, id))
	if err != nil {
		return DisposePlan{}, err
	}
	var plan DisposePlan
	if err := json.Unmarshal(data, &plan); err != nil || plan.Schema != DisposePlanSchema {
		if err == nil {
			err = errors.New("not an evidence disposal plan")
		}
		return DisposePlan{}, fmt.Errorf("plan %s is unreadable: %w; metasystem evidence dispose --preview writes one", id, err)
	}
	return plan, nil
}

// NewestDisposePlan is the newest evidence disposal plan this session
// previewed, or "": another session's preview is never executed by
// default.
func NewestDisposePlan(homeStateRoot, session string) string {
	entries, err := os.ReadDir(filepath.Join(homeStateRoot, "stores", "plans"))
	if err != nil {
		return ""
	}
	var ids []string
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if plan, err := ReadDisposePlan(homeStateRoot, id); err == nil && session != "" && plan.Session == session {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

// Outcome is what an execution did with one planned item.
type Outcome struct {
	Path    string `json:"path"`
	Done    bool   `json:"done"`
	Already bool   `json:"already,omitempty"`
	// Line says what happened and, for an item not done, what settles it.
	Line   string `json:"line"`
	Freed  int64  `json:"freed,omitempty"`
	Export string `json:"export,omitempty"`
}

// ExecuteOptions shape an execution.
type ExecuteOptions struct {
	Override bool
	Reason   string
}

// Execute performs a plan: per item, under the bound lock (waiting its one
// item) and a chain's lifecycle locks (waiting the reaper's bound), after
// a fresh ledger observation, with the item revalidated and re-judged.
func (e Env) Execute(ctx context.Context, plan DisposePlan, options ExecuteOptions) []Outcome {
	exclusions := e.Exclusions(true)
	var outcomes []Outcome
	for _, planned := range plan.Items {
		outcomes = append(outcomes, e.executeOne(ctx, plan, planned, options, exclusions))
	}
	return outcomes
}

func (e Env) executeOne(ctx context.Context, plan DisposePlan, planned PlannedDisposal, options ExecuteOptions, exclusions *Exclusions) Outcome {
	outcome := Outcome{Path: planned.Path}
	if planned.Decline != "" {
		outcome.Line = planned.Path + ": declined: " + planned.Decline
		return outcome
	}
	lock, err := diskstore.BoundExclusive(diskstore.BoundLockPath(e.HomeStateRoot))
	if err != nil {
		outcome.Line = planned.Path + ": the bound lock cannot be taken: " + err.Error() + "; repeat metasystem evidence dispose --plan " + plan.ID
		return outcome
	}
	defer lock.Release()
	if already := e.alreadyDone(planned); already != "" {
		outcome.Done, outcome.Already, outcome.Line = true, true, already
		return outcome
	}
	target, err := e.Locate(ctx, planned.Path)
	if err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	}
	if target.Item.Kind == diskstore.KindChain && target.Segment.Context != nil && e.Locks != nil {
		release, holder, err := e.Locks(target.Segment.Context.Installation, chainJobs(target.Segment, target.Item), ReaperBound)
		if err != nil || holder != "" {
			if err != nil {
				holder = err.Error()
			}
			outcome.Line = planned.Path + ": declined: a mirror or reap is in this chain (" + holder + "); repeat metasystem evidence dispose --plan " + plan.ID + " once it has finished"
			return outcome
		}
		defer release()
	}
	stage, err := diskstore.NewID(e.Now, e.Entropy)
	if err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	}
	ledger := disposalLedger(target)
	judgement := judgeTarget(ctx, exclusions, target)
	if recovered, handled := e.recover(ctx, target, ledger, stage, judgement, options); handled {
		return recovered
	}
	if info, err := os.Lstat(planned.Path); err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	} else if device, inode := statIDs(info); device != planned.Device || inode != planned.Inode {
		outcome.Line = planned.Path + ": changed since the preview; run --preview again"
		return outcome
	}
	files, err := diskstore.Inventory(ctx, planned.Path, nil)
	if err != nil || diskstore.InventoryDigest(files) != planned.InventoryDigest {
		outcome.Line = planned.Path + ": changed since the preview; run --preview again"
		return outcome
	}
	var overrides []string
	if judgement.SegmentUnknown != "" && strings.HasPrefix(judgement.SegmentUnknown, "ledger not observed") {
		judgement.Held = append(judgement.Held, "ledger-not-observed")
		judgement.SegmentUnknown = ""
	}
	if reason := blocking(judgement); reason != "" {
		if !options.Override {
			outcome.Line = planned.Path + ": held: " + reason + "; --override takes it anyway"
			return outcome
		}
		overrides = strings.Split(reason, "; ")
	}
	// Settlement re-mirrors only after the exclusions let the item go
	// (Round B2, F-13); a re-mirror that changed the item since the
	// preview sends the person back to it.
	if decline := e.settle(ctx, target, stage); decline != "" {
		outcome.Line = planned.Path + ": declined: " + decline
		return outcome
	}
	if files, err := diskstore.Inventory(ctx, planned.Path, nil); err != nil || diskstore.InventoryDigest(files) != planned.InventoryDigest {
		outcome.Line = planned.Path + ": its settlement re-mirrored records that differ from the preview; run --preview again"
		return outcome
	}
	receipt := diskstore.DisposalReceipt{At: e.Now.UTC(), Segment: target.Segment.Git, Checkout: target.Segment.CheckoutName(), Kind: target.Item.Kind,
		Item: target.Item.Name, Rule: diskstore.RulePerson, By: e.By, LedgerTip: judgement.LedgerTip, LedgerIdentity: judgement.LedgerIdentity,
		EndedAt: target.Item.EndedAt.Format(time.RFC3339), Goal: target.Item.Goal, GoalState: judgement.GoalState,
		UncoveredReceipts: judgement.Uncovered, Citations: judgement.Citations, Plan: plan.ID, Reason: options.Reason, Overrides: overrides}
	if target.Unsegmented {
		receipt.Kind = diskstore.KindUnsegmented
	}
	if receipt.ID, err = diskstore.NewReceiptID(e.Now, e.Entropy); err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	}
	if plan.Export != "" && planned.Step == diskstore.StepRemove {
		exported, err := diskstore.Export(ctx, diskstore.ExportRequest{Item: planned.Path, Dir: plan.Export, Segment: segmentName(target), Kind: receipt.Kind,
			Checkout: receipt.Checkout, EndedAt: receipt.EndedAt, Blobs: e.Blobs, Now: e.Now, Stage: stage, Sync: e.Sync})
		if err != nil {
			outcome.Line = planned.Path + ": kept: " + err.Error()
			return outcome
		}
		receipt.Export = exported.Ref(plan.Export)
		outcome.Export = exported.Archive
	}
	step := diskstore.DisposalStep{Item: planned.Path, Receipt: receipt, Ledger: ledger, Commit: judgement.Commit, Stage: stage, Sync: e.Sync}
	if planned.Step == diskstore.StepCompact {
		verdict, err := Verdict(target.Item, e.Blobs)
		if err != nil {
			outcome.Line = planned.Path + ": not compacted: " + err.Error()
			return outcome
		}
		step.Kept, step.Verdict = KeptFor(target.Item.Kind), verdict
	}
	result, err := diskstore.Dispose(ctx, step)
	switch {
	case err != nil:
		outcome.Line = planned.Path + ": stopped: " + err.Error() + "; repeat metasystem evidence dispose --plan " + plan.ID
	case result.RolledBack != "":
		outcome.Line = planned.Path + ": rolled back, nothing removed: " + result.RolledBack + "; run --preview again"
	case result.Already:
		outcome.Done, outcome.Already, outcome.Line = true, true, planned.Path+": already "+pastTense(planned.Step)
	default:
		outcome.Done, outcome.Freed = true, result.Receipt.ItemBytesBefore-result.Receipt.ItemBytesAfter
		outcome.Line = fmt.Sprintf("%s: %s, %d files, receipt %s", planned.Path, pastTense(planned.Step), result.Receipt.Dropped, result.Receipt.ID)
		if outcome.Export != "" {
			outcome.Line += ", exported to " + outcome.Export
		}
		if planned.Step == diskstore.StepRemove && target.Item.Kind == diskstore.KindBundle {
			e.dropReferences(result.Tombstone, target)
		}
	}
	return outcome
}

func pastTense(step string) string {
	if step == diskstore.StepCompact {
		return "compacted"
	}
	return "removed"
}

func segmentName(target Target) string {
	switch {
	case target.Unsegmented:
		return "unsegmented"
	case target.Item.Kind == diskstore.KindEvents:
		return target.Segment.Installation
	}
	return target.Segment.Git
}

// disposalLedger is the item's receipt ledger: its segment's, or
// unsegmented.jsonl for a root's unsegmented entry.
func disposalLedger(target Target) string {
	name := segmentName(target)
	return filepath.Join(target.Root.Path, "disposals", name+".jsonl")
}

// alreadyDone answers a repeat: a removed item, or a compacted one asked
// to compact again, succeeds and writes nothing (R-129).
func (e Env) alreadyDone(planned PlannedDisposal) string {
	if tombstone, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(planned.Path)); err == nil && tombstone.State == diskstore.StateDone {
		if _, err := os.Lstat(planned.Path); errors.Is(err, os.ErrNotExist) {
			return planned.Path + ": already removed (receipt " + tombstone.Receipt + ")"
		}
	}
	if planned.Step == diskstore.StepCompact {
		if tombstone, err := diskstore.ReadTombstone(filepath.Join(planned.Path, diskstore.CompactTombstoneName)); err == nil && tombstone.State == diskstore.StateDone {
			return planned.Path + ": already compacted (receipt " + tombstone.Receipt + ")"
		}
	}
	return ""
}

// recover settles an unfinished disposal of the item first: committed, it
// is finished; uncommitted, it is re-judged and a recorded export is
// re-verified on the disk before it may continue (DL4E-05).
func (e Env) recover(ctx context.Context, target Target, ledger, stage string, judgement Judgement, options ExecuteOptions) (Outcome, bool) {
	_, tombstone, open, err := diskstore.OpenDisposal(target.Item.Path)
	if err != nil || !open {
		return Outcome{}, false
	}
	outcome := Outcome{Path: target.Item.Path}
	result, err := diskstore.RecoverDisposal(ctx, target.Item.Path, ledger, diskstore.DisposalReceipt{Kind: tombstone.Kind, Item: tombstone.Item,
		Segment: tombstone.Segment, Rule: tombstone.Rule, By: tombstone.By, Export: tombstone.Export, Overrides: tombstone.Overrides}, e.Sync, stage,
		func(tombstone diskstore.Tombstone) diskstore.Recovery {
			if tombstone.Export != nil {
				if got, _, err := diskstore.FileDigest(ctx, tombstone.Export.Archive); err != nil || got != tombstone.Export.ArchiveSHA256 {
					return diskstore.Recovery{Reason: "kept: export not verified after restart"}
				}
			}
			if reason := blocking(judgement); reason != "" && !options.Override {
				return diskstore.Recovery{Reason: "held: " + reason}
			}
			return diskstore.Recovery{Continue: true, Commit: judgement.Commit}
		})
	switch {
	case err != nil:
		outcome.Line = target.Item.Path + ": its unfinished disposal could not be settled: " + err.Error()
	case result.RolledBack != "":
		outcome.Line = target.Item.Path + ": its unfinished disposal was rolled back: " + result.RolledBack
	default:
		outcome.Done, outcome.Line = true, target.Item.Path+": its unfinished disposal was finished, receipt "+tombstone.Receipt
	}
	return outcome, true
}

// dropReferences drops a removed bundle's blob references after its commit
// point (3.12: the disposer drops them itself; the check drops any left).
func (e Env) dropReferences(tombstonePath string, target Target) {
	tombstone, err := diskstore.ReadTombstone(tombstonePath)
	if err != nil {
		return
	}
	files, err := diskstore.TombstoneFiles(tombstonePath, tombstone)
	if err != nil {
		return
	}
	release, err := e.Blobs.TryExclusive()
	if err != nil {
		return // the reference check drops them later
	}
	defer release()
	referrer := segmentName(target) + "-" + target.Item.Name
	for _, file := range files {
		if file.Original != nil && file.Original.Kind == diskstore.RecipeBlob {
			_ = os.Remove(e.Blobs.RefPath(file.Original.SHA256, referrer))
		}
	}
}

// settle is the damage protection for a chain (3.12, DL4C-15, DL4E-11): a
// closed chain whose payload is still in its checkout is settled by the
// verb itself (every terminal job re-mirrored, then the collector's own
// collection for this one chain), unless a registered, unreleased
// workspace store of the chain keeps that payload; a chain that is not
// closed is declined naming the one action that changes its state.
func (e Env) settle(ctx context.Context, target Target, stage string) string {
	if target.Item.Kind != diskstore.KindChain || target.Segment.Context == nil {
		return ""
	}
	installation := target.Segment.Context.Installation
	agents := filepath.Join(installation, "artifacts", "agents")
	if _, err := os.Stat(filepath.Join(agents, target.Item.Name)); errors.Is(err, os.ErrNotExist) {
		return ""
	}
	records, _ := chainRecords(filepath.Join(agents, "jobs"), target.Item.Name)
	root := rootRecord(records, target.Item.Name)
	switch {
	case len(records) == 0 || root == nil:
		return "its checkout's records cannot be read; run metasystem system check in " + installation
	case anyLive(records):
		return "a round is live; metasystem work stop j2:" + target.Item.Name + " ends it"
	case root["chainClosed"] != true:
		if _, reviewed := root["reviews"]; !reviewed {
			return "every round is terminal and unreviewed; metasystem work review j2:" + target.Item.Name + " reviews it"
		}
		return "the chain is not closed; metasystem work finish j2:" + target.Item.Name + " closes it"
	}
	registry := diskstore.CheckoutRegistry(installation)
	records2, _ := registry.Inventory()
	for _, record := range records2 {
		if record.Owner.Kind == diskstore.OwnerDelegate && record.Owner.Ref == target.Item.Name && record.State != diskstore.StateReleased {
			return "its workspace store " + record.ID + " is registered and not released, and it keeps the chain's payload; metasystem disk clean --preview names what settles it"
		}
	}
	result := filepath.Join(e.HomeStateRoot, "stores", ".settle-"+stage+".json")
	defer os.Remove(result)
	for _, record := range records {
		job, _ := record["jobId"].(string)
		if job == "" {
			continue
		}
		if err := dispatch.Mirror(installation, target.Segment.Context.Facts.GitRoot, target.Root.Path, target.Item.Name, job, result); err != nil {
			return "its re-mirror failed (" + err.Error() + "); run metasystem system check in " + installation
		}
	}
	collected, reason, err := CollectChain(installation, target.Root.Path, target.Item.Name)
	switch {
	case err != nil:
		return "its payload could not be collected: " + err.Error()
	case !collected:
		return "its payload stays in " + installation + ": " + reason + "; run metasystem system check there"
	}
	_ = ctx
	return ""
}

// ExportTargets exports each target to dir (both audiences): under the
// bound lock and a chain's lifecycle locks, so nothing lands in an item
// while it is archived.
func (e Env) ExportTargets(ctx context.Context, targets []Target, dir string) []Outcome {
	var outcomes []Outcome
	for _, target := range targets {
		outcome := Outcome{Path: target.Item.Path}
		lock, err := diskstore.BoundExclusive(diskstore.BoundLockPath(e.HomeStateRoot))
		if err != nil {
			outcome.Line = target.Item.Path + ": the bound lock cannot be taken: " + err.Error()
			outcomes = append(outcomes, outcome)
			continue
		}
		func() {
			defer lock.Release()
			if target.Item.Kind == diskstore.KindChain && target.Segment.Context != nil && e.Locks != nil {
				release, holder, err := e.Locks(target.Segment.Context.Installation, chainJobs(target.Segment, target.Item), ReaperBound)
				if err != nil || holder != "" {
					if err != nil {
						holder = err.Error()
					}
					outcome.Line = target.Item.Path + ": not exported: a mirror or reap is in this chain (" + holder + "); repeat once it has finished"
					return
				}
				defer release()
			}
			stage, _ := diskstore.NewID(e.Now, e.Entropy)
			kind := target.Item.Kind
			if target.Unsegmented {
				kind = diskstore.KindUnsegmented
			}
			result, err := diskstore.Export(ctx, diskstore.ExportRequest{Item: target.Item.Path, Dir: dir, Segment: segmentName(target), Kind: kind,
				Checkout: target.Segment.CheckoutName(), EndedAt: target.Item.EndedAt.Format(time.RFC3339), Blobs: e.Blobs, Now: e.Now, Stage: stage, Sync: e.Sync})
			switch {
			case err != nil:
				outcome.Line = target.Item.Path + ": " + err.Error()
			case result.Already:
				outcome.Done, outcome.Already, outcome.Export = true, true, result.Archive
				outcome.Line = target.Item.Path + ": already exported, verified " + result.VerifiedAt.Format(time.RFC3339) + " (" + result.Archive + ")"
			default:
				outcome.Done, outcome.Export = true, result.Archive
				outcome.Line = fmt.Sprintf("%s: exported to %s (%s)", target.Item.Path, result.Archive, result.ArchiveSHA256[:12])
			}
		}()
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

// ExportDirFor is --to, else evidence.export-dir, else the input refusal
// naming both; a directory the export would damage is declined.
func (e Env) ExportDirFor(to string, stores []string) (string, string) {
	dir := to
	if dir == "" {
		dir = e.This.Settings.Values[config.DiskEvidenceExportDirKey]
	}
	if dir == "" {
		return "", "name where the copies go: --to DIR (or --export DIR), or set " + config.DiskEvidenceExportDirKey + " in metasystem.conf.local; nothing was done"
	}
	var roots, checkouts []string
	for _, root := range e.Roots() {
		roots = append(roots, root.Path)
	}
	for _, checkout := range e.checkouts() {
		checkouts = append(checkouts, checkout.Facts.GitRoot, checkout.Installation)
	}
	if problem := diskstore.ExportDirProblem(dir, roots, checkouts, stores); problem != "" {
		return "", problem
	}
	return dir, ""
}

// ItemView is one item as evidence show prints it.
type ItemView struct {
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Path     string    `json:"path"`
	State    string    `json:"state"`
	EndedAt  time.Time `json:"endedAt,omitempty"`
	Bytes    int64     `json:"bytes"`
	Eligible bool      `json:"eligible"`
	Why      string    `json:"why,omitempty"`
	Held     []string  `json:"held,omitempty"`
}

// SegmentView is this checkout's segment for evidence show.
type SegmentView struct {
	Checkout string                    `json:"checkout"`
	Root     string                    `json:"root"`
	Position diskstore.EvidenceSegment `json:"position"`
	AgeFloor time.Duration             `json:"ageFloor"`
	Items    []ItemView                `json:"items"`
	Ledger   string                    `json:"ledger"`
	// NextPass are the items the next pass would compact, oldest first.
	NextPass []string `json:"nextPass,omitempty"`
	Unknown  string   `json:"unknown,omitempty"`
	// Open are disposals a person must finish or roll back.
	Open []string `json:"open,omitempty"`
}

// Show reads this checkout's segment: its total against its cap, every
// item's state, eligibility and the exclusion that holds it (judged on the
// accepted ledger as it stands), what the next pass would compact, and the
// over-the-bound line with the command pair. It writes nothing.
func (e Env) Show(ctx context.Context) SegmentView {
	view := SegmentView{Checkout: e.This.Facts.GitRoot, Ledger: "goal state unknown: no accepted ledger"}
	var segment Segment
	var root Root
	for _, candidate := range e.Roots() {
		for _, s := range candidate.Segments {
			if s.Context != nil && s.Context.Installation == e.This.Installation {
				segment, root = s, candidate
			}
		}
	}
	if segment.Root == "" {
		root := e.This.Settings.EvidenceRoot.Path
		switch {
		case e.This.SettingsErr != nil:
			view.Unknown = "its settings cannot be read: " + e.This.SettingsErr.Error() + "; metasystem settings check names the fix"
		case root == "":
			view.Unknown = "no evidence root resolves for it; metasystem settings check names the fix"
		default:
			view.Unknown = "nothing is in its evidence root " + root + " yet; metasystem evidence show --all names every root of this host with its owner"
		}
		return view
	}
	view.Root = root.Path
	settings, err := e.settingsOf(segment)
	if err != nil {
		view.Unknown = err.Error()
		return view
	}
	view.AgeFloor = settings.AgeFloor
	bound := Bound{Now: e.Now, Blobs: e.Blobs}
	position := diskstore.EvidenceSegment{Segment: segment.Git, Root: segment.Root, Checkout: segment.CheckoutName(), CapBytes: settings.CapBytes, Held: map[string]int64{}}
	position.TotalBytes, position.BlobChargeBytes, _ = bound.Measure(ctx, segment)
	if segment.Unknown != "" {
		view.Unknown, position.Unknown = segment.Unknown, segment.Unknown
	}
	exclusions := e.Exclusions(false)
	if observed, unknown := exclusions.observe(ctx, segment); unknown == "" {
		view.Ledger = "judged on the accepted ledger " + short12(observed.Tip)
		if !observed.Committed.IsZero() {
			view.Ledger += ", " + e.Now.Sub(observed.Committed).Round(time.Minute).String() + " old"
		}
	} else {
		view.Ledger = unknown
	}
	view.Open = segment.OpenPersonDisposals(ctx)
	items, _ := segment.Items(ctx)
	sort.SliceStable(items, func(i, j int) bool { return items[i].EndedAt.Before(items[j].EndedAt) })
	held := map[string]string{}
	estimate := position.TotalBytes
	for _, item := range items {
		state := "live"
		if item.Compacted {
			state = "compacted"
		}
		entry := ItemView{Name: item.Name, Kind: item.Kind, Path: item.Path, State: state, EndedAt: item.EndedAt, Bytes: item.Bytes}
		entry.Eligible, entry.Why = bound.Candidate(segment, item, settings.AgeFloor)
		if entry.Eligible && segment.Unknown == "" {
			if reason := blocking(exclusions.Judge(ctx, segment, item)); reason != "" {
				entry.Eligible, entry.Held = false, strings.Split(reason, "; ")
				held[item.Name] = reason
			} else if estimate > settings.CapBytes {
				view.NextPass = append(view.NextPass, item.Name)
				estimate -= item.Bytes
			}
		}
		view.Items = append(view.Items, entry)
	}
	if position.Unknown == "" {
		bound.finishPosition(ctx, segment, settings, &position, held)
	}
	view.Position = position
	return view
}

// Lines renders a segment view: a short summary by default; every item
// with --verbose.
func (v SegmentView) Lines(verbose bool) []string {
	if v.Root == "" {
		return []string{"evidence of " + v.Checkout + ": " + v.Unknown}
	}
	var lines []string
	position := v.Position
	lines = append(lines, fmt.Sprintf("evidence of %s: segment %s in %s: %s of the %s cap%s; age floor %d days",
		v.Checkout, position.Segment, v.Root, formatGiB(position.TotalBytes), formatGiB(position.CapBytes), charges(position.BlobChargeBytes), int(v.AgeFloor.Hours()/24)))
	counts := map[string]int{}
	var heldCount, eligible int
	for _, item := range v.Items {
		counts[item.State]++
		if len(item.Held) > 0 {
			heldCount++
		}
		if item.Eligible {
			eligible++
		}
	}
	lines = append(lines, fmt.Sprintf("  %d item(s): %d live, %d compacted; %d eligible for compaction, %d held by an exclusion; %s",
		len(v.Items), counts["live"], counts["compacted"], eligible, heldCount, v.Ledger))
	if v.Unknown != "" {
		lines = append(lines, "  Unknown to the bound: "+v.Unknown)
	}
	for _, line := range v.Open {
		lines = append(lines, "  unfinished: "+line)
	}
	if len(v.NextPass) > 0 {
		lines = append(lines, fmt.Sprintf("  the next pass would compact %d item(s): %s", len(v.NextPass), examplesOf(v.NextPass)))
	}
	if position.Over {
		lines = append(lines, position.Lines(verbose)...)
	}
	if verbose {
		for _, item := range v.Items {
			line := fmt.Sprintf("    %s %s %s, ended %s, %s", item.State, item.Kind, item.Name, item.EndedAt.Format("2006-01-02"), formatGiB(item.Bytes))
			switch {
			case len(item.Held) > 0:
				line += "; held: " + strings.Join(item.Held, "; ")
			case item.Eligible:
				line += "; eligible"
			default:
				line += "; " + item.Why
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func charges(bytes int64) string {
	if bytes == 0 {
		return ""
	}
	return " (blob charges " + formatGiB(bytes) + ")"
}

func examplesOf(names []string) string {
	if len(names) <= 3 {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:3], ", ") + fmt.Sprintf(" and %d more", len(names)-3)
}
