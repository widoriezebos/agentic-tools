package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

// noCitations answers the citation clause with nothing cited.
type noCitations struct{}

func (noCitations) Cited(context.Context, Segment, Item) ([]string, string, string) {
	return nil, "", ""
}

// ledgers is a fake observer: the accepted and the fetched view per
// installation.
type ledgers struct {
	accepted, fetched map[string]LedgerView
	err               error
	fetches           int
}

func (l *ledgers) observe(_ context.Context, installation string, fetch bool) (LedgerView, error) {
	if l.err != nil {
		return LedgerView{}, l.err
	}
	if fetch {
		l.fetches++
		return l.fetched[installation], nil
	}
	return l.accepted[installation], nil
}

func view(identity string, states map[string]string) LedgerView {
	return LedgerView{Tip: "9498700a9", Identity: identity, States: states}
}

const identityA = "01J9LEDGER0000000000000000"

func (bed *boundBed) exclusions(fake *ledgers) *Exclusions {
	bed.segment.LedgerIdentity = identityA
	return &Exclusions{Observe: fake.observe, Fetch: true, Citations: noCitations{}}
}

func (bed *boundBed) judged(exclusions *Exclusions) Bound {
	bound := bed.bound(nil, nil)
	bound.Judge = exclusions.Judge
	return bound
}

func (bed *boundBed) ledger(t *testing.T, lines ...string) string {
	t.Helper()
	path := ReceiptLedgerPath(bed.installation)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func receiptNaming(job string) string {
	return "1790000000|2026-09-20T00:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=claude:opus:" + job + "|note=x"
}

func TestAnOpenGoalHoldsItsChainOnTheFetchedLedger(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	open := bed.chain(t, "reopened", 300, 400, "g-reopened")
	done := bed.chain(t, "finished", 300, 400, "g-done")
	fake := &ledgers{
		// This clone's accepted ref still says done; the remote reopened it.
		accepted: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-reopened": GoalDone, "g-done": GoalDone})},
		fetched:  map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-reopened": GoalOpen, "g-done": GoalDone})},
	}
	bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(open) || !compacted(done) {
		t.Fatalf("the goal reopened on the remote holds its chain: %v %v", compacted(open), compacted(done))
	}
	if fake.fetches != 1 {
		t.Fatalf("one observation per checkout per pass, fetched first: %d", fake.fetches)
	}
	lines := receipts(t, bed.segment)
	if len(lines) != 1 || lines[0].GoalState != GoalDone || lines[0].LedgerIdentity != identityA || lines[0].LedgerTip != "9498700a9" {
		t.Fatalf("the receipt records the observation: %+v", lines)
	}
}

func TestAFailedObservationCompactsNothingInTheSegment(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "")
	position := bed.judged(bed.exclusions(&ledgers{err: errors.New("fetch: could not resolve host")})).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || !strings.Contains(position.Unknown, "ledger not observed: fetch: could not resolve host") {
		t.Fatalf("a failed fetch is Unknown for the whole segment: %+v", position)
	}
}

// Two checkouts sharing a root commit with two adoptions are two ledgers
// (DL4E-03): a goal concluded in one says nothing about the other.
func TestTwoLedgerIdentitiesAreTwoViews(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "forked", 300, 400, "g")
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view("01KOTHERADOPTION0000000000", map[string]string{"g": GoalDone})}}
	position := bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || !strings.Contains(position.Unknown, "ledger identity changed: "+identityA) {
		t.Fatalf("an observed identity other than the index's is Unknown: %+v", position)
	}
	fake.fetched[bed.installation] = view("", map[string]string{"g": GoalDone})
	position = bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || !strings.Contains(position.Unknown, "no ledger identity") {
		t.Fatalf("a root record with no identity is Unknown: %+v", position)
	}
}

func TestLocalModeUnionHoldsAGoalOpenInEitherClone(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "local", 300, 400, "g")
	peer := Context{Installation: "/elsewhere/clone/metasystem", Facts: diskstore.CheckoutFacts{LedgerIdentity: identityA}}
	here := view(identityA, map[string]string{"g": GoalDone})
	here.Local = true
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: here},
		accepted: map[string]LedgerView{peer.Installation: view(identityA, map[string]string{"g": GoalOpen})}}
	exclusions := bed.exclusions(fake)
	exclusions.Peers = []Context{peer}
	bed.judged(exclusions).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) {
		t.Fatal("in local mode a goal open in any armed clone of the identity holds")
	}
}

func TestAnUncoveredReceiptHoldsTheChainItNamesUntilARetroCoversIt(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	named := bed.chain(t, "named", 300, 400, "")
	unnamed := bed.chain(t, "unnamed", 300, 400, "")
	path := bed.ledger(t, receiptNaming("named"))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(named) || !compacted(unnamed) {
		t.Fatalf("an uncovered receipt holds the chain it names; an unnamed item is not held: %v %v", compacted(named), compacted(unnamed))
	}
	coverage := receipt.CoverageOf(readText(t, path))
	retro := "1790000100|2026-09-21T00:00:00Z|RETRO|note=covered|covered=" + strings.Join(coverage.Digests, ",")
	bed.ledger(t, receiptNaming("named"), retro)
	bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if !compacted(named) {
		t.Fatal("once a RETRO row covers the line, the chain is eligible")
	}
}

// A line merged into the ledger by git between the judgement and the
// receipt append makes the step roll back with nothing dropped
// (DL4E-07); the next pass holds the item the line names.
func TestALedgerChangedUnderTheJudgementRollsBack(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "merged", 300, 400, "")
	path := bed.ledger(t, receiptNaming("someone-else"))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	exclusions := bed.exclusions(fake)
	bound := bed.judged(exclusions)
	bound.Judge = func(ctx context.Context, segment Segment, item Item) Judgement {
		judgement := exclusions.Judge(ctx, segment, item)
		// The fixture's git merge lands after the judgement.
		handle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		handle.WriteString(receiptNaming("merged") + "\n")
		handle.Close()
		return judgement
	}
	position := bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || len(receipts(t, bed.segment)) != 0 || !strings.Contains(strings.Join(position.Pending, "\n"), "receipt ledger changed during the judgement") {
		t.Fatalf("the step rolls back with no receipt: %+v", position)
	}
	bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) {
		t.Fatal("the next pass holds the item the merged line names")
	}
}

func TestAMalformedReceiptLedgerCompactsNothing(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "")
	bed.ledger(t, "<<<<<<< HEAD", receiptNaming("x"))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	position := bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || !strings.Contains(position.Unknown, "malformed") {
		t.Fatalf("a malformed receipt ledger compacts nothing in the segment: %+v", position)
	}
}

// bundle writes a distilled bundle of the segment with an owner file.
func (bed *boundBed) bundle(t *testing.T, name string, daysAgo int, owner diskstore.BundleOwner) string {
	t.Helper()
	dir := filepath.Join(bed.root, "suite-failures", bed.segment.Git, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	header, _ := json.Marshal(diskstore.DistilledHeader{Schema: diskstore.DistilledSchema, Created: boundNow.Add(-time.Duration(daysAgo) * 24 * time.Hour)})
	if err := os.WriteFile(filepath.Join(dir, diskstore.DistilledName), append(header, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.log"), make([]byte, 400*kib), 0o644); err != nil {
		t.Fatal(err)
	}
	owner.WrittenBy, owner.WrittenAt = "bundle-writer", boundNow
	if _, err := diskstore.WriteBundleOwner(dir, owner, diskstore.Syncer{}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A bundle's goal is on the bundle (DL4D-06): its attempt record may be
// pruned and its goal reopened, and OWNER.json still holds it.
func TestTheOwnerFileHoldsABundleOfAnOpenGoal(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	held := bed.bundle(t, "20260801T000000Z-watchdog-proof-a", 150, diskstore.BundleOwner{Attempt: "proof-a", Goal: "g-open", LedgerIdentity: identityA})
	none := bed.bundle(t, "20260801T000000Z-watchdog-standalone-1", 150, diskstore.BundleOwner{Attempt: diskstore.AttemptStandalone, Goal: diskstore.GoalNone})
	unknown := bed.bundle(t, "20260801T000000Z-detached-legacy", 150, diskstore.BundleOwner{Attempt: diskstore.GoalUnknown, Goal: diskstore.GoalUnknown})
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-open": GoalOpen})}}
	bed.judged(bed.exclusions(fake)).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(held) || !compacted(none) || compacted(unknown) {
		t.Fatalf("open goal held, goal none compacted, goal unknown never: %v %v %v", compacted(held), compacted(none), compacted(unknown))
	}
	for _, kept := range []string{diskstore.DistilledName, diskstore.OwnerFileName, diskstore.VerdictName} {
		if _, err := os.Stat(filepath.Join(none, kept)); err != nil {
			t.Fatalf("a compacted bundle keeps %s: %v", kept, err)
		}
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSegmentSettingsReadTheContextCheckout(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	bed.segment.Context.Settings = diskstore.Settings{Values: map[string]string{"evidence.segment-cap-gib": "3", "evidence.age-floor-days": "7"}}
	settings, err := SegmentSettings(bed.segment)
	if err != nil || settings.CapBytes != 3<<30 || settings.AgeFloor != 7*24*time.Hour {
		t.Fatalf("%+v %v", settings, err)
	}
	orphan := bed.segment
	orphan.Context = nil
	if _, err := SegmentSettings(orphan); err == nil {
		t.Fatal("an orphan segment has no settings")
	}
}
