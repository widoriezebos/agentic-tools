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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

type observerRepository struct {
	goal.Repository
	capture func() (string, error)
	release func()
	err     error
}

func (r observerRepository) Capture(string) (string, error)  { return r.capture() }
func (r observerRepository) Accepted() (string, bool, error) { return "", false, r.err }
func (r observerRepository) Release(string) error {
	if r.release != nil {
		r.release()
	}
	return nil
}

func TestGoalLedgerObserverUsesInjectedDeadline(t *testing.T) {
	t.Parallel()
	for _, fetch := range []bool{false, true} {
		t.Run(map[bool]string{false: "offline", true: "fetch"}[fetch], func(t *testing.T) {
			t.Parallel()
			failure := errors.New("fixture ledger unavailable")
			deadlines := make(chan time.Duration, 1)
			endpoint := goal.Endpoint{Repository: observerRepository{
				err: failure, capture: func() (string, error) {
					if !fetch {
						t.Error("offline observation fetched")
					}
					return "", failure
				},
			}, ProjectionDeadline: func(wait time.Duration) <-chan time.Time {
				deadlines <- wait
				return make(chan time.Time)
			}}
			observe := goalLedgerObserver(func() time.Time { return boundNow }, func(installation string) (goal.Endpoint, error) {
				if installation != "fixture-installation" {
					t.Errorf("installation=%q", installation)
				}
				return endpoint, nil
			})
			view, err := observe(context.Background(), "fixture-installation", fetch)
			if !errors.Is(err, failure) || view.Tip != "" || len(view.States) != 0 {
				t.Fatalf("failed observation view=%+v err=%v", view, err)
			}
			if fetch {
				select {
				case wait := <-deadlines:
					if wait != 4*time.Second {
						t.Fatalf("projection budget=%s; want 4s", wait)
					}
				default:
					t.Fatal("fresh observation bypassed its injected deadline")
				}
			}
			if len(deadlines) != 0 {
				t.Fatal("observation armed an unexpected deadline")
			}
		})
	}
}

func TestGoalLedgerObserverAbandonsProjectionOnDeadlineOrCancellation(t *testing.T) {
	t.Parallel()
	for _, cancelPass := range []bool{false, true} {
		t.Run(map[bool]string{false: "projection deadline", true: "pass cancellation"}[cancelPass], func(t *testing.T) {
			t.Parallel()
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			deadline := make(chan time.Time)
			deadlines := make(chan time.Duration, 1)
			endpoint := goal.Endpoint{Repository: observerRepository{
				capture: func() (string, error) {
					close(started)
					<-release
					return "", errors.New("fixture fetch released")
				}, release: func() { close(finished) },
			}, ProjectionDeadline: func(wait time.Duration) <-chan time.Time {
				deadlines <- wait
				return deadline
			}}
			observe := goalLedgerObserver(func() time.Time { return boundNow }, func(string) (goal.Endpoint, error) { return endpoint, nil })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			t.Cleanup(func() { close(release); <-finished })
			type answer struct {
				view LedgerView
				err  error
			}
			done := make(chan answer, 1)
			go func() {
				view, err := observe(ctx, "fixture-installation", true)
				done <- answer{view, err}
			}()
			<-started
			select {
			case wait := <-deadlines:
				if wait != 4*time.Second {
					t.Fatalf("projection budget=%s; want 4s", wait)
				}
			default:
				t.Fatal("fresh observation bypassed its injected deadline")
			}
			select {
			case result := <-done:
				t.Fatalf("observation returned before its deadline or cancellation: %+v", result)
			default:
			}
			if cancelPass {
				cancel()
			} else {
				close(deadline)
			}
			result := <-done
			if result.view.Tip != "" || len(result.view.States) != 0 {
				t.Fatalf("abandoned observation returned ledger facts: %+v", result.view)
			}
			if cancelPass {
				if !errors.Is(result.err, context.Canceled) {
					t.Fatalf("pass cancellation error=%v", result.err)
				}
			} else if result.err == nil || !strings.Contains(result.err.Error(), "fetch timed out after 4s") {
				t.Fatalf("projection deadline error=%v", result.err)
			}
		})
	}
}

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

// judgedPass is every item of the segment judged as a person's dispose
// judges it: clear by path, the segment-wide Unknown, each judgement.
type judgedPass struct {
	clear      map[string]bool
	unknown    string
	judgements map[string]Judgement
}

func (bed *boundBed) judge(exclusions *Exclusions) judgedPass {
	pass := judgedPass{clear: map[string]bool{}, judgements: map[string]Judgement{}}
	items, _ := bed.segment.Items(context.Background())
	for _, item := range items {
		judgement := exclusions.Judge(context.Background(), bed.segment, item)
		if judgement.SegmentUnknown != "" && pass.unknown == "" {
			pass.unknown = judgement.SegmentUnknown
		}
		pass.clear[item.Path] = blocking(judgement) == ""
		pass.judgements[item.Path] = judgement
	}
	return pass
}

func (bed *boundBed) ledger(t *testing.T, lines ...string) string {
	t.Helper()
	path, err := ReceiptLedgerPath(bed.installation)
	if err != nil {
		t.Fatal(err)
	}
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
	exclusions := bed.exclusions(fake)
	tips := 0
	exclusions.Tip = func(context.Context, string) (string, error) { tips++; return "9498700a9", nil }
	pass := bed.judge(exclusions)
	if tips == 0 {
		t.Fatal("each item after the first checks the accepted tip")
	}
	if pass.clear[open] || !pass.clear[done] {
		t.Fatalf("the goal reopened on the remote holds its chain: %v %v", pass.clear[open], pass.clear[done])
	}
	if fake.fetches != 1 {
		t.Fatalf("one observation per checkout per pass, fetched first: %d", fake.fetches)
	}
	if judgement := pass.judgements[done]; judgement.GoalState != GoalDone || judgement.LedgerIdentity != identityA || judgement.LedgerTip != "9498700a9" {
		t.Fatalf("the judgement records the observation for the receipt: %+v", judgement)
	}
}

func TestAFailedObservationHoldsEveryItemOfTheSegment(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "")
	pass := bed.judge(bed.exclusions(&ledgers{err: errors.New("fetch: could not resolve host")}))
	if pass.clear[dir] || !strings.Contains(pass.unknown, "ledger not observed: fetch: could not resolve host") {
		t.Fatalf("a failed fetch is Unknown for the whole segment: %+v", pass)
	}
}

// Two checkouts sharing a root commit with two adoptions are two ledgers
// (DL4E-03): a goal concluded in one says nothing about the other.
func TestTwoLedgerIdentitiesAreTwoViews(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "forked", 300, 400, "g")
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view("01KOTHERADOPTION0000000000", map[string]string{"g": GoalDone})}}
	pass := bed.judge(bed.exclusions(fake))
	if pass.clear[dir] || !strings.Contains(pass.unknown, "ledger identity changed: "+identityA) {
		t.Fatalf("an observed identity other than the index's is Unknown: %+v", pass)
	}
	fake.fetched[bed.installation] = view("", map[string]string{"g": GoalDone})
	pass = bed.judge(bed.exclusions(fake))
	if pass.clear[dir] || !strings.Contains(pass.unknown, "no ledger identity") {
		t.Fatalf("a root record with no identity is Unknown: %+v", pass)
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
	if bed.judge(exclusions).clear[dir] {
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
	pass := bed.judge(bed.exclusions(fake))
	if pass.clear[named] || !pass.clear[unnamed] {
		t.Fatalf("an uncovered receipt holds the chain it names; an unnamed item is not held: %v %v", pass.clear[named], pass.clear[unnamed])
	}
	coverage := receipt.CoverageOf(readText(t, path))
	retro := "1790000100|2026-09-21T00:00:00Z|RETRO|note=covered|covered=" + strings.Join(coverage.Digests, ",")
	bed.ledger(t, receiptNaming("named"), retro)
	if !bed.judge(bed.exclusions(fake)).clear[named] {
		t.Fatal("once a RETRO row covers the line, the chain is clear")
	}
}

// A line merged into the ledger by git between the judgement and the
// receipt append fails the commit check, so the removal rolls back with
// nothing removed (DL4E-07); the next judgement holds the item the line
// names.
func TestALedgerChangedUnderTheJudgementFailsTheCommitCheck(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "merged", 300, 400, "")
	path := bed.ledger(t, receiptNaming("someone-else"))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	judgement := bed.judge(bed.exclusions(fake)).judgements[dir]
	if blocking(judgement) != "" || judgement.Commit == nil || judgement.Commit() != nil {
		t.Fatalf("clear, with a commit check that passes on an unchanged ledger: %+v", judgement)
	}
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	handle.WriteString(receiptNaming("merged") + "\n")
	handle.Close()
	if err := judgement.Commit(); err == nil || !strings.Contains(err.Error(), "receipt ledger changed during the judgement") {
		t.Fatalf("the commit check fails and says why: %v", err)
	}
	if bed.judge(bed.exclusions(fake)).clear[dir] {
		t.Fatal("the next judgement holds the item the merged line names")
	}
}

func TestAMalformedReceiptLedgerHoldsTheSegment(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "")
	bed.ledger(t, "<<<<<<< HEAD", receiptNaming("x"))
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	pass := bed.judge(bed.exclusions(fake))
	if pass.clear[dir] || !strings.Contains(pass.unknown, "malformed") {
		t.Fatalf("a malformed receipt ledger holds every item of the segment: %+v", pass)
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
	log := append([]byte("=== RUN   TestLanding\n--- FAIL: TestLanding (0.01s)\n    land_test.go:42: want green, got red\nFAIL\tgithub.com/x/landing\t0.2s\n"), make([]byte, 400*kib)...)
	if err := os.WriteFile(filepath.Join(dir, "run.log"), log, 0o644); err != nil {
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
	pass := bed.judge(bed.exclusions(fake))
	if pass.clear[held] || !pass.clear[none] || pass.clear[unknown] {
		t.Fatalf("open goal held, goal none clear, goal unknown never: %v %v %v", pass.clear[held], pass.clear[none], pass.clear[unknown])
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
