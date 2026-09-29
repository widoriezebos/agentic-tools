package evidence

// A person's evidence acts (design engine-owns-disk-lifetimes Part B 3.12
// "The public actions"; Wido 2026-09-28: "export before we then delete
// with the verb because I want a verb to help me clean this in a safe
// way"): show, export and dispose. Removal is only a person's act, of a
// segmented item (Round B2-3, rule 3), from a previewed plan: the preview judges on the accepted ledger as it stands
// and fetches nothing; the execution observes the ledger afresh under the
// bound lock, re-judges every item, revalidates its device, inode and
// inventory digest, skips a held item unless --override (every override
// on the receipt and the tombstone), exports first when asked and removes
// only an item whose export verified. H1: a person is never refused; an
// item whose removal would damage something is declined with what would
// go wrong and the public command that settles it, and the rest completes.
// A removal cut short is only ever rolled back before its commit point
// and has only its set-aside copy removed after it (Round B2-3, rule 2);
// the person then previews again.

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

// Target is one located item: always an item of a segment.
type Target struct {
	Root    Root
	Segment Segment
	Item    Item
}

// DefaultPlanMaxAge bounds the plan dispose executes without --plan
// (Round B2-3, N3-7): only this session's newest preview, and only when
// it is younger than this.
const DefaultPlanMaxAge = 24 * time.Hour

// ErrNotEvidence is a path outside every evidence root.
var ErrNotEvidence = errors.New("not in any evidence root of this host")

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

// NotAnItem is the refusal for any word that is not an enumerated item.
const NotAnItem = "not an item; metasystem evidence show --verbose lists this checkout's items, and --all --verbose every root's"

// NotManagedLine is how a not-managed entry is named (Round B2-3, rule 3).
const NotManagedLine = "not managed: remove by hand if unneeded"

// Enumerate is the inventory of items (Round B2-2, R1; Round B2-3, rule
// 3): every item of every segment of every root, orphan segments
// included. It never yields a structure directory, a segment, a
// tombstone, a sidecar, a stage, an entry set aside for disposal, or an
// entry that is not managed.
func (e Env) Enumerate(ctx context.Context) ([]Target, error) {
	var targets []Target
	for _, root := range e.Roots() {
		for _, segment := range root.Segments {
			items, err := segment.Items(ctx)
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				targets = append(targets, Target{Root: root, Segment: segment, Item: item})
			}
		}
	}
	return targets, nil
}

// NotManaged lists every root's entries that are not managed, with their
// bytes: evidence show names them, dispose refuses them.
func (e Env) NotManaged(ctx context.Context) []ItemView {
	var views []ItemView
	for _, root := range e.Roots() {
		for _, path := range root.NotManaged {
			bytes, _, _ := diskstore.Measure(ctx, path)
			views = append(views, ItemView{Name: filepath.Base(path), Path: path, State: "not managed", Bytes: bytes, Why: NotManagedLine})
		}
	}
	return views
}

// Resolve accepts only items the inventory enumerates (Round B2-2, R1): a
// name is the item of that name in this checkout's segment; a path is
// accepted only when it is the same file (device and inode, after Lstat,
// never a string or case comparison) as exactly one enumerated item. A
// symlink, a structure directory in any spelling, a segment, a record, a
// path inside an item and anything else is refused.
func (e Env) Resolve(ctx context.Context, argument string) ([]Target, error) {
	targets, err := e.Enumerate(ctx)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(argument) {
		for _, target := range targets {
			segment := target.Segment
			if segment.Context != nil && segment.Context.Installation == e.This.Installation && target.Item.Name == argument {
				return []Target{target}, nil
			}
		}
		return nil, fmt.Errorf("%s: %s", argument, NotAnItem)
	}
	info, err := os.Lstat(argument)
	if err != nil {
		return nil, fmt.Errorf("%s: %s (%v)", argument, NotAnItem, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symlink: %s", argument, NotAnItem)
	}
	var found []Target
	for _, target := range targets {
		candidate, err := os.Lstat(target.Item.Path)
		if err == nil && os.SameFile(info, candidate) {
			found = append(found, target)
		}
	}
	if len(found) != 1 {
		for _, entry := range e.NotManaged(ctx) {
			if candidate, err := os.Lstat(entry.Path); err == nil && os.SameFile(info, candidate) {
				return nil, fmt.Errorf("%s is %s; dispose removes only an item of a segment", argument, NotManagedLine)
			}
		}
		return nil, fmt.Errorf("%s: %s", argument, NotAnItem)
	}
	return found, nil
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
// removed item from its tombstone's cumulative inventory
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

// judgeTarget judges an item by the exclusions (Round B2, F-3) in its
// segment; an item of a segment with no armed context checkout is held.
func judgeTarget(ctx context.Context, exclusions *Exclusions, target Target) Judgement {
	if target.Segment.Context == nil {
		return Judgement{Held: []string{"its segment has no armed context checkout: " + target.Segment.Unknown}}
	}
	return exclusions.Judge(ctx, target.Segment, target.Item)
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
// or under it (3.12 --over-bound; DL4E-12; Round B2-3): its items past the
// age floor, oldest end time first, until the segment's total from a stat
// walk now would be at or under the cap; held items are listed as held and
// do not count. The still-over lines say what it cannot select.
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
			bound := Bound{Now: e.Now, Blobs: e.Blobs}
			total, _, complete := bound.Measure(ctx, segment)
			if !complete || total <= settings.CapBytes {
				continue
			}
			items, err := segment.Items(ctx)
			if err != nil {
				continue
			}
			sort.SliceStable(items, func(i, j int) bool { return items[i].EndedAt.Before(items[j].EndedAt) })
			var held, young, unknown int64
			for _, item := range items {
				if removable, _ := bound.Removable(item, settings.AgeFloor); !removable {
					if item.EndUnknown != "" {
						unknown += item.Bytes
					} else {
						young += item.Bytes
					}
					continue
				}
				if total <= settings.CapBytes {
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
				stillOver = append(stillOver, fmt.Sprintf("still over by %s after this plan in %s: held %s; younger than the age floor %s; end time unknown %s; not managed in its root %s (%s)",
					formatGiB(total-settings.CapBytes), segment.Git, formatGiB(held), formatGiB(young), formatGiB(unknown), formatGiB(notManagedBytes(ctx, root)), NotManagedLine))
			}
		}
	}
	return targets, stillOver
}

func notManagedBytes(ctx context.Context, root Root) int64 {
	var total int64
	for _, entry := range root.NotManaged {
		bytes, _, _ := diskstore.Measure(ctx, entry)
		total += bytes
	}
	return total
}

// Preview writes a person's plan (R15): each item with its step, file count
// and bytes, device, inode and inventory digest, and held or clear judged
// on the accepted ledger as it stands: no fetch, no ref advance. It writes
// exactly the plan file.
func (e Env) Preview(ctx context.Context, targets []Target, stillOver []string, exportDir string) (DisposePlan, error) {
	id, err := diskstore.NewID(e.Now, e.Entropy)
	if err != nil {
		return DisposePlan{}, err
	}
	plan := DisposePlan{Schema: DisposePlanSchema, ID: id, At: e.Now.UTC(), Export: exportDir, StillOver: stillOver, Items: []PlannedDisposal{},
		Session: e.Session}
	exclusions := e.Exclusions(false)
	plan.Ledger = "goal state unknown: no accepted ledger"
	for _, target := range targets {
		planned := e.plan(ctx, target, exclusions)
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
func (e Env) plan(ctx context.Context, target Target, exclusions *Exclusions) PlannedDisposal {
	item := target.Item
	planned := PlannedDisposal{Path: item.Path, Name: item.Name, Kind: item.Kind, Segment: target.Segment.Git, Root: target.Root.Path,
		Step: diskstore.StepRemove, State: "clear"}
	if info, err := os.Lstat(item.Path); err == nil {
		planned.Device, planned.Inode = statIDs(info)
	}
	files, err := diskstore.Inventory(ctx, item.Path)
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
// previewed within DefaultPlanMaxAge of now, or "": another session's
// preview, or an older one, is never executed by default (Round B2-3,
// N3-7).
func NewestDisposePlan(homeStateRoot, session string, now time.Time) string {
	entries, err := os.ReadDir(filepath.Join(homeStateRoot, "stores", "plans"))
	if err != nil {
		return ""
	}
	var ids []string
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if plan, err := ReadDisposePlan(homeStateRoot, id); err == nil && session != "" && plan.Session == session && now.Sub(plan.At) < DefaultPlanMaxAge && !plan.At.After(now) {
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
	// A removal of this item that was cut short is settled first, from its
	// tombstone, and never continued (Round B2-3, rule 2): the person
	// previews again.
	if line, open := e.settleOpen(ctx, planned.Path, disposalLedgerAt(planned)); open {
		outcome.Line = line
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
	if info, err := os.Lstat(planned.Path); err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	} else if device, inode := statIDs(info); device != planned.Device || inode != planned.Inode {
		outcome.Line = planned.Path + ": changed since the preview; run --preview again"
		return outcome
	}
	files, err := diskstore.Inventory(ctx, planned.Path)
	if err != nil || diskstore.InventoryDigest(files) != planned.InventoryDigest {
		outcome.Line = planned.Path + ": changed since the preview; run --preview again"
		return outcome
	}
	judgement := judgeTarget(ctx, exclusions, target)
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
	// A chain's payload is settled only after the exclusions let the item
	// go (Round B2, F-13); a re-mirror that changed the item since the
	// preview sends the person back to it.
	if decline := e.settleChain(target, stage); decline != "" {
		outcome.Line = planned.Path + ": declined: " + decline
		return outcome
	}
	if files, err := diskstore.Inventory(ctx, planned.Path); err != nil || diskstore.InventoryDigest(files) != planned.InventoryDigest {
		outcome.Line = planned.Path + ": its settlement re-mirrored records that differ from the preview; run --preview again"
		return outcome
	}
	receipt := diskstore.DisposalReceipt{At: e.Now.UTC(), Segment: target.Segment.Git, Checkout: target.Segment.CheckoutName(), Kind: target.Item.Kind,
		Item: target.Item.Name, Rule: diskstore.RulePerson, By: e.By, LedgerTip: judgement.LedgerTip, LedgerIdentity: judgement.LedgerIdentity,
		EndedAt: target.Item.EndedAt.Format(time.RFC3339), Goal: target.Item.Goal, GoalState: judgement.GoalState,
		UncoveredReceipts: judgement.Uncovered, Citations: judgement.Citations, Plan: plan.ID, Reason: options.Reason, Overrides: overrides}
	if receipt.ID, err = diskstore.NewReceiptID(e.Now, e.Entropy); err != nil {
		outcome.Line = planned.Path + ": " + err.Error()
		return outcome
	}
	if plan.Export != "" {
		exported, err := diskstore.Export(ctx, diskstore.ExportRequest{Item: planned.Path, Dir: plan.Export, Segment: segmentName(target), Kind: receipt.Kind,
			Checkout: receipt.Checkout, EndedAt: receipt.EndedAt, Blobs: e.Blobs, Now: e.Now, Stage: stage, Sync: e.Sync})
		if err != nil {
			outcome.Line = planned.Path + ": kept: " + err.Error()
			return outcome
		}
		receipt.Export = exported.Ref(plan.Export)
		outcome.Export = exported.Archive
	}
	result, err := diskstore.Dispose(ctx, diskstore.DisposalStep{Item: planned.Path, Receipt: receipt, Ledger: disposalLedger(target), Commit: judgement.Commit,
		Stage: stage, Sync: e.Sync})
	switch {
	case err != nil:
		outcome.Line = planned.Path + ": stopped: " + err.Error() + "; run --preview again"
	case result.RolledBack != "":
		outcome.Line = planned.Path + ": rolled back, nothing removed: " + result.RolledBack + "; run --preview again"
	case result.Already:
		outcome.Done, outcome.Already, outcome.Line = true, true, planned.Path+": already removed"
	default:
		outcome.Done, outcome.Freed = true, result.Receipt.ItemBytesBefore
		outcome.Line = fmt.Sprintf("%s: removed, %d files, receipt %s", planned.Path, result.Receipt.Dropped, result.Receipt.ID)
		if outcome.Export != "" {
			outcome.Line += ", exported to " + outcome.Export
		}
		if target.Item.Kind == diskstore.KindBundle {
			e.dropReferences(result.Tombstone, target)
		}
	}
	return outcome
}

// settleOpen settles a removal of the item that was cut short (Round
// B2-3, rule 2): rolled back before its commit point, its set-aside copy
// removed after it. open is false when there was none; the line says what
// was done and that the person previews again.
func (e Env) settleOpen(ctx context.Context, item, ledger string) (string, bool) {
	_, tombstone, open, err := diskstore.OpenDisposal(item)
	switch {
	case err != nil:
		return item + ": its tombstone cannot be read (" + err.Error() + "); a person decides", true
	case !open:
		return "", false
	}
	stage, err := diskstore.NewID(e.Now, e.Entropy)
	if err != nil {
		return item + ": " + err.Error(), true
	}
	settled, err := diskstore.SettlePersonDisposal(ctx, item, ledger, e.Sync, stage)
	switch {
	case err != nil:
		return item + ": its removal (receipt " + tombstone.Receipt + ") was cut short and could not be settled: " + err.Error(), true
	case settled.Finished:
		return item + ": its removal (receipt " + tombstone.Receipt + ") was committed; its set-aside copy is now removed", true
	}
	return item + ": its removal that was cut short was rolled back, the item is back; run --preview again", true
}

// SettleOpen settles every removal of this host's segments that was cut
// short (evidence show; Round B2-3, rule 2), under the bound lock without
// waiting; held, it settles nothing and says so.
func (e Env) SettleOpen(ctx context.Context) []string {
	var open []string
	for _, root := range e.Roots() {
		for _, segment := range root.Segments {
			for _, directory := range segment.Dirs() {
				if directory != "" {
					open = append(open, diskstore.OpenRemovals(directory)...)
				}
			}
		}
	}
	if len(open) == 0 {
		return nil
	}
	lock, err := diskstore.TryBoundExclusive(diskstore.BoundLockPath(e.HomeStateRoot))
	if err != nil {
		return []string{fmt.Sprintf("%d removal(s) cut short are settled once the bound lock is free (a disposal is in its step)", len(open))}
	}
	defer lock.Release()
	var lines []string
	for _, item := range open {
		line, _ := e.settleOpen(ctx, item, disposalLedgerAt(PlannedDisposal{Path: item}))
		lines = append(lines, line)
	}
	return lines
}

func segmentName(target Target) string {
	if target.Item.Kind == diskstore.KindEvents {
		return target.Segment.Installation
	}
	return target.Segment.Git
}

// disposalLedger is the item's receipt ledger: its segment's.
func disposalLedger(target Target) string {
	return filepath.Join(target.Root.Path, "disposals", segmentName(target)+".jsonl")
}

// disposalLedgerAt is the ledger of an item known by its path alone
// (<root>/<agents|suite-failures|events>/<segment>/<item>), as it is
// while the item is set aside.
func disposalLedgerAt(planned PlannedDisposal) string {
	segment := filepath.Dir(planned.Path)
	return filepath.Join(filepath.Dir(filepath.Dir(segment)), "disposals", filepath.Base(segment)+".jsonl")
}

// alreadyDone answers a repeat: a removed item succeeds and writes nothing
// (R-129).
func (e Env) alreadyDone(planned PlannedDisposal) string {
	if tombstone, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(planned.Path)); err == nil && tombstone.State == diskstore.StateDone {
		if _, err := os.Lstat(planned.Path); errors.Is(err, os.ErrNotExist) {
			return planned.Path + ": already removed (receipt " + tombstone.Receipt + ")"
		}
	}
	return ""
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

// settleChain is the damage protection for a chain (3.12, DL4C-15, DL4E-11): a
// closed chain whose payload is still in its checkout is settled by the
// verb itself (every terminal job re-mirrored, then the collector's own
// collection for this one chain), unless a registered, unreleased
// workspace store of the chain keeps that payload; a chain that is not
// closed is declined naming the one action that changes its state.
func (e Env) settleChain(target Target, stage string) string {
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
			result, err := diskstore.Export(ctx, diskstore.ExportRequest{Item: target.Item.Path, Dir: dir, Segment: segmentName(target), Kind: target.Item.Kind,
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
	Name    string    `json:"name"`
	Kind    string    `json:"kind,omitempty"`
	Path    string    `json:"path"`
	State   string    `json:"state"`
	EndedAt time.Time `json:"endedAt,omitempty"`
	Bytes   int64     `json:"bytes"`
	// Removable: past the age floor with a known end time, so
	// --over-bound may select it (a held item is still judged).
	Removable bool     `json:"removable"`
	Why       string   `json:"why,omitempty"`
	Held      []string `json:"held,omitempty"`
}

// SegmentView is this checkout's segment for evidence show.
type SegmentView struct {
	Checkout string                    `json:"checkout"`
	Root     string                    `json:"root"`
	Position diskstore.EvidenceSegment `json:"position"`
	AgeFloor time.Duration             `json:"ageFloor"`
	Items    []ItemView                `json:"items"`
	Ledger   string                    `json:"ledger"`
	Unknown  string                    `json:"unknown,omitempty"`
	// Settled are the removals cut short that this show settled (Round
	// B2-3, rule 2).
	Settled []string `json:"settled,omitempty"`
	// NotManaged are the root's entries outside every segment.
	NotManaged []ItemView `json:"notManaged,omitempty"`
}

// Show reads this checkout's segment: its total against its cap, every
// item's state, whether --over-bound may select it and the exclusion that
// holds it (judged on the accepted ledger as it stands), the over-the-bound
// line with the command pair, and the root's entries that are not managed.
// Its one write is settling a removal that was cut short (Round B2-3, rule
// 2).
func (e Env) Show(ctx context.Context) SegmentView {
	view := SegmentView{Checkout: e.This.Facts.GitRoot, Ledger: "goal state unknown: no accepted ledger"}
	view.Settled = e.SettleOpen(ctx)
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
	for _, path := range root.NotManaged {
		bytes, _, _ := diskstore.Measure(ctx, path)
		view.NotManaged = append(view.NotManaged, ItemView{Name: filepath.Base(path), Path: path, State: "not managed", Bytes: bytes, Why: NotManagedLine})
	}
	settings, err := e.settingsOf(segment)
	if err != nil {
		view.Unknown = err.Error()
		return view
	}
	view.AgeFloor = settings.AgeFloor
	bound := Bound{Now: e.Now, Blobs: e.Blobs}
	view.Position = bound.ReportSegment(ctx, segment, settings)
	view.Unknown = segment.Unknown
	exclusions := e.Exclusions(false)
	if observed, unknown := exclusions.observe(ctx, segment); unknown == "" {
		view.Ledger = "judged on the accepted ledger " + short12(observed.Tip)
		if !observed.Committed.IsZero() {
			view.Ledger += ", " + e.Now.Sub(observed.Committed).Round(time.Minute).String() + " old"
		}
	} else {
		view.Ledger = unknown
	}
	items, _ := segment.Items(ctx)
	sort.SliceStable(items, func(i, j int) bool { return items[i].EndedAt.Before(items[j].EndedAt) })
	for _, item := range items {
		entry := ItemView{Name: item.Name, Kind: item.Kind, Path: item.Path, State: "live", EndedAt: item.EndedAt, Bytes: item.Bytes}
		entry.Removable, entry.Why = bound.Removable(item, settings.AgeFloor)
		if entry.Removable && segment.Unknown == "" {
			if reason := blocking(exclusions.Judge(ctx, segment, item)); reason != "" {
				entry.Held = strings.Split(reason, "; ")
			}
		}
		view.Items = append(view.Items, entry)
	}
	return view
}

// Lines renders a segment view: a short summary by default; every item
// with --verbose.
func (v SegmentView) Lines(verbose bool) []string {
	var lines []string
	for _, line := range v.Settled {
		lines = append(lines, "settled: "+line)
	}
	if v.Root == "" {
		return append(lines, "evidence of "+v.Checkout+": "+v.Unknown)
	}
	position := v.Position
	lines = append(lines, fmt.Sprintf("evidence of %s: segment %s in %s: %s of the %s cap%s; age floor %d days",
		v.Checkout, position.Segment, v.Root, formatGiB(position.TotalBytes), formatGiB(position.CapBytes), charges(position.BlobChargeBytes), int(v.AgeFloor.Hours()/24)))
	var heldCount, removable int
	for _, item := range v.Items {
		if len(item.Held) > 0 {
			heldCount++
		} else if item.Removable {
			removable++
		}
	}
	lines = append(lines, fmt.Sprintf("  %d item(s): %d past the age floor and clear, %d held by an exclusion; %s",
		len(v.Items), removable, heldCount, v.Ledger))
	if v.Unknown != "" {
		lines = append(lines, "  Unknown to the bound: "+v.Unknown)
	}
	for _, line := range position.Pending {
		lines = append(lines, "  unfinished: "+line)
	}
	if position.Over {
		lines = append(lines, position.Lines(verbose)...)
	}
	if count := len(v.NotManaged); count > 0 {
		var bytes int64
		for _, entry := range v.NotManaged {
			bytes += entry.Bytes
		}
		lines = append(lines, fmt.Sprintf("  %d entr%s of %s outside every segment, %s: %s", count, plural(count, "y", "ies"), v.Root, formatGiB(bytes), NotManagedLine))
		if verbose {
			for _, entry := range v.NotManaged {
				lines = append(lines, fmt.Sprintf("    %s, %s: %s", entry.Path, formatGiB(entry.Bytes), NotManagedLine))
			}
		}
	}
	if verbose {
		for _, item := range v.Items {
			line := fmt.Sprintf("    %s %s, ended %s, %s", item.Kind, item.Name, item.EndedAt.Format("2006-01-02"), formatGiB(item.Bytes))
			switch {
			case len(item.Held) > 0:
				line += "; held: " + strings.Join(item.Held, "; ")
			case item.Removable:
				line += "; past the age floor, clear"
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
