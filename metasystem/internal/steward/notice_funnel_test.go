package steward

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var funnelStart = time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)

type funnelBed struct {
	*healthBed
	at                    time.Time
	decisions, transports int
	allowed               bool
	changeDecision        bool
	transportErr          error
	advance               time.Duration
}

func newFunnelBed(t *testing.T, allowed bool) *funnelBed {
	t.Helper()
	return &funnelBed{healthBed: newHealthBed(t, EnrollmentFixture, ""), at: funnelStart, allowed: allowed}
}

func (b *funnelBed) tick() AlertEpisode {
	b.t.Helper()
	var result TickResult
	deps := tickHealthDependencies{
		prober: b.probe,
		now:    func() time.Time { return b.at },
		evaluate: func(string, string, time.Time, identity.Prober, bool) ([]RoleVerdict, SpendObservation) {
			return []RoleVerdict{roleDead(RoleHookFreshness, "fixture hook is unavailable", "")}, SpendObservation{Valid: true}
		},
		funnel: noticeFunnel{decider: func(Notice) (string, bool) {
			b.decisions++
			// A second decision for this notice contradicts the first answer.
			return "health", b.allowed && (!b.changeDecision || b.decisions == 1)
		}},
		deliver: func(string, string) error {
			b.transports++
			b.at = b.at.Add(b.advance)
			return b.transportErr
		},
	}
	if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, time.Time{}, deps); err != nil {
		b.t.Fatal(err)
	}
	episodes, err := AlertEpisodes(b.root)
	if err != nil || len(episodes) != 1 {
		b.t.Fatalf("one saved episode required: %+v, %v", episodes, err)
	}
	return episodes[0]
}

func TestFunnelOutcomeContract(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"submitted", "failed", "withheld", "withheld-after-pending"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			allowed := name == "submitted" || name == "failed"
			b := newFunnelBed(t, allowed)
			b.changeDecision = true
			wantResult, wantAttempts, wantCalls := TransportSubmitted, 1, 1
			if name == "failed" {
				b.transportErr = errors.New("fixture transport failure")
				wantResult = TransportFailed
			}
			if !allowed {
				wantResult, wantAttempts, wantCalls = TransportWithheld, 0, 0
			}
			if name == "withheld-after-pending" {
				roles := []RoleVerdict{roleDead(RoleHookFreshness, "fixture hook is unavailable", "")}
				digest := healthFindingDigest(roles)
				episode := AlertEpisode{Schema: 1, EpisodeID: nextEpisodeID(digest, nil), Digest: digest,
					Message: "pending hook alert", OpenedAt: b.at, TransportResult: TransportPending,
					Attempts: []AlertAttempt{{Sequence: 1, AttemptedAt: b.at, Result: TransportPending}}}
				if err := saveAlertEpisode(b.root, episode); err != nil {
					t.Fatal(err)
				}
				wantAttempts = 1
			}
			episode := b.tick()
			if episode.TransportResult != wantResult {
				t.Errorf("episode result = %s, want %s", episode.TransportResult, wantResult)
			}
			if len(episode.Attempts) != wantAttempts {
				t.Errorf("attempt count = %d, want %d", len(episode.Attempts), wantAttempts)
			}
			if b.transports != wantCalls {
				t.Errorf("transport calls = %d, want %d", b.transports, wantCalls)
			}
			if b.decisions != 1 {
				t.Errorf("decider calls = %d, want 1", b.decisions)
			}
			for _, attempt := range episode.Attempts {
				if attempt.Result != wantResult || !attempt.CompletedAt.Equal(b.at) {
					t.Errorf("completed attempt = %+v, want %s at %s", attempt, wantResult, b.at)
				}
			}
			records := journalOf(t, b.root)
			if len(records) != 1 {
				t.Fatalf("journal lines = %d, want 1", len(records))
			}
			record := records[0]
			if record.Delivered != (wantResult == TransportSubmitted) {
				t.Errorf("journal delivery = %t, want result %s", record.Delivered, wantResult)
			}
			wantClass, wantError := "", ""
			if !allowed {
				wantClass = "health"
			}
			if b.transportErr != nil {
				wantError = b.transportErr.Error()
			}
			if record.Withheld != wantClass || record.Error != wantError {
				t.Errorf("journal disposition = %+v, want withheld %q error %q", record, wantClass, wantError)
			}
			if allowed {
				raw, err := os.ReadFile(NotificationJournalPath(b.root))
				if err != nil || strings.Contains(string(raw), `"withheld"`) {
					t.Errorf("allowed journal must omit withheld: %s, %v", raw, err)
				}
			}
		})
	}
}

func TestJournalTimeIsReadAfterTheSend(t *testing.T) {
	t.Parallel()
	b := newFunnelBed(t, true)
	b.advance = 7 * time.Second
	episode := b.tick()
	if len(episode.Attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(episode.Attempts))
	}
	attempt := episode.Attempts[0]
	if !attempt.AttemptedAt.Equal(funnelStart) {
		t.Errorf("attempt started at %s, want %s", attempt.AttemptedAt, funnelStart)
	}
	later := funnelStart.Add(7 * time.Second)
	if !attempt.CompletedAt.Equal(later) {
		t.Errorf("completion = %s, want %s", attempt.CompletedAt, later)
	}
	records := journalOf(t, b.root)
	if len(records) != 1 {
		t.Fatalf("journal lines = %d, want 1", len(records))
	}
	if records[0].At != later.Format(time.RFC3339) {
		t.Errorf("journal at = %s, want %s", records[0].At, later.Format(time.RFC3339))
	}
	id := records[0].ID
	if len(id) != 26 {
		t.Fatalf("journal id = %q, want 26 characters", id)
	}
	var millis int64
	for _, c := range id[:10] {
		value := strings.IndexRune(notificationIDAlphabet, c)
		if value < 0 {
			t.Fatalf("invalid journal id %q", id)
		}
		millis = millis*32 + int64(value)
	}
	if millis != later.UnixMilli() {
		t.Errorf("journal id milliseconds = %d, want %d", millis, later.UnixMilli())
	}
}

func TestQueueDispositions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, nonce := range []string{"a-withheld", "handoff-b", "c-submitted"} {
		intent := testIntent(nonce)
		if nonce == "handoff-b" {
			intent.Nonce, intent.Reason, intent.Handoff = "b", seatHandoffReason, &HandoffBinding{}
		}
		if err := MintIntent(root, intent); err != nil {
			t.Fatal(err)
		}
	}
	for _, nonce := range []string{"a-withheld", "handoff-b", "c-submitted", "verdict-" + string(VerdictStalledDead), "handoff-absent"} {
		if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: nonce}); err != nil {
			t.Fatal(err)
		}
	}
	decisions, transports := 0, 0
	sent, err := deliverPendingWithDependencies(root, pendingDeliveryDependencies{
		transport: func(string, string) error { transports++; return nil },
		funnel: noticeFunnel{clock: func() time.Time { return funnelStart }, decider: func(n Notice) (string, bool) {
			decisions++
			return "health", n.Ref == "c-submitted"
		}},
	})
	if err != nil || sent != 1 {
		t.Errorf("queue delivered = %d, error = %v, want 1 and nil", sent, err)
	}
	if transports != 1 || decisions != 3 {
		t.Errorf("queue transport calls = %d, decisions = %d, want 1 and 3", transports, decisions)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Errorf("queue must be empty: %+v, %v", pending, err)
	}
	intents, err := LiveIntents(root)
	if err != nil || len(intents) != 3 {
		t.Fatalf("live intents = %+v, %v", intents, err)
	}
	for _, intent := range intents {
		if intent.Notified != (intent.Nonce == "c-submitted") {
			t.Errorf("intent %s notified = %t", intent.Nonce, intent.Notified)
		}
	}
	records := journalOf(t, root)
	if len(records) != 3 {
		t.Fatalf("journal lines = %d, want 3", len(records))
	}
	want := map[string]bool{"a-withheld": false, "handoff-b": false, "c-submitted": true}
	for _, record := range records {
		delivered, exists := want[record.Ref]
		if !exists {
			t.Errorf("unexpected journal reference %s", record.Ref)
		}
		if record.Delivered != delivered || (!delivered && record.Withheld != "health") || (delivered && record.Withheld != "") {
			t.Errorf("wrong queue disposition: %+v", record)
		}
		delete(want, record.Ref)
	}
	if len(want) != 0 {
		t.Errorf("missing journal references: %v", want)
	}
}

func TestNoRepeatAfterDurableSuccess(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"submitted", "withheld"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newFunnelBed(t, name == "submitted")
			first := b.tick()
			second := b.tick()
			wantCalls := 0
			if b.allowed {
				wantCalls = 1
			}
			if b.transports != wantCalls {
				t.Errorf("transport calls after two passes = %d, want %d", b.transports, wantCalls)
			}
			if records := journalOf(t, b.root); len(records) != 1 {
				t.Errorf("journal lines after two passes = %d, want 1", len(records))
			}
			if first.EpisodeID != second.EpisodeID || first.TransportResult != second.TransportResult {
				t.Errorf("durable success changed: %+v, %+v", first, second)
			}
			if b.decisions != 1 {
				t.Errorf("decider calls after two passes = %d, want 1", b.decisions)
			}
		})
	}
}

func TestTransportFailureRetriesOnce(t *testing.T) {
	t.Parallel()
	b := newFunnelBed(t, true)
	b.transportErr = errors.New("fixture transport failure")
	first := b.tick()
	if first.TransportResult != TransportFailed || b.transports != 1 {
		t.Fatalf("first pass = %+v, calls %d", first, b.transports)
	}
	// A new composition reads the durable records in the same repository.
	restarted := &funnelBed{healthBed: b.healthBed, at: b.at, allowed: true}
	second := restarted.tick()
	if second.TransportResult != TransportSubmitted || len(second.Attempts) != 2 || restarted.transports != 1 {
		t.Fatalf("retry did not submit: %+v, calls %d", second, restarted.transports)
	}
	third := restarted.tick()
	if restarted.transports != 1 || len(third.Attempts) != 2 {
		t.Errorf("settled retry repeated: %+v, calls %d", third, restarted.transports)
	}
	if first.EpisodeID != second.EpisodeID || second.EpisodeID != third.EpisodeID {
		t.Errorf("retry changed episode identity")
	}
}

func TestSettledChecksUseOneHelper(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "episodeSettled" {
				continue
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				binary, ok := node.(*ast.BinaryExpr)
				if !ok || (binary.Op != token.EQL && binary.Op != token.NEQ) {
					return true
				}
				isResult := func(expr ast.Expr) bool {
					selector, ok := expr.(*ast.SelectorExpr)
					return ok && selector.Sel.Name == "TransportResult"
				}
				isSubmitted := func(expr ast.Expr) bool {
					ident, ok := expr.(*ast.Ident)
					return ok && ident.Name == "TransportSubmitted"
				}
				if (isResult(binary.X) && isSubmitted(binary.Y)) || (isResult(binary.Y) && isSubmitted(binary.X)) {
					t.Errorf("%s: submitted comparison outside episodeSettled", fset.Position(binary.Pos()))
				}
				return true
			})
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no production files checked")
	}
}
