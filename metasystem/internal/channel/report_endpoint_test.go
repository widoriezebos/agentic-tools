package channel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ComposeStatusReportAtEndpoint accepts a report root only when it resolves to
// the bound endpoint root; any other root reports the backlog as unavailable
// instead of reading another checkout's ledger.
func TestStatusReportAtEndpointBindsReportRootToEndpointRoot(t *testing.T) {
	t.Parallel()
	landedAt := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	fixture := newReportFixture(t, landedAt, reportGoal("bound-landing", "Land at the endpoint.", goal.StateApproved, "other-machine", landedAt))
	endpoint := goal.Endpoint{Root: fixture.root, Remote: "local", Branch: "refs/heads/main", Repository: endpointLedgerFixture{fixture}}
	window := landedAt.Add(-time.Hour)
	log := []byte("Ship at endpoint\x00bound-landing\n")
	config := func(root string) ReportConfig {
		return ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: landedAt.Add(time.Minute), WindowStart: window, Location: time.UTC}
	}
	compose := func(root string) string {
		t.Helper()
		calls := 0
		text, _, err := ComposeStatusReportAtEndpoint(config(root), endpoint, func(logRoot string, start time.Time) ([]byte, error) {
			calls++
			if logRoot != root || !start.Equal(window) {
				t.Fatalf("landing history read root=%q window=%s, want root=%q window=%s", logRoot, start.Format(time.RFC3339), root, window.Format(time.RFC3339))
			}
			return append([]byte(nil), log...), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("landing history was read %d times, want one", calls)
		}
		return text
	}

	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(fixture.root, alias); err != nil {
		t.Fatal(err)
	}
	direct := fixture.mustCompose(config(fixture.root), window, log)
	if got := compose(alias); got != direct {
		t.Fatalf("symlinked endpoint root changed the report:\n got:\n%s\nwant:\n%s", got, direct)
	}
	if strings.Contains(direct, "Backlog order: unavailable") || !strings.Contains(direct, "Delivered: bound landing — Ship at endpoint") {
		t.Fatalf("bound endpoint report did not read the accepted ledger and landing history:\n%s", direct)
	}

	other := t.TempDir()
	mismatch := compose(other)
	want := "Backlog order: unavailable — " + fmt.Sprintf("status endpoint root %q does not match report root %q", fixture.root, other)
	if !strings.Contains(mismatch, want) || strings.Contains(mismatch, "Delivered:") {
		t.Fatalf("different report root was not refused as unavailable:\n%s\nwant line %q", mismatch, want)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	absent := compose(missing)
	if !strings.Contains(absent, "Backlog order: unavailable — ") || !strings.Contains(absent, missing) || strings.Contains(absent, "does not match") {
		t.Fatalf("unresolvable report root did not surface its resolution error:\n%s", absent)
	}

	gone := filepath.Join(t.TempDir(), "gone")
	endpoint.Root = gone
	unbound := compose(fixture.root)
	if !strings.Contains(unbound, "Backlog order: unavailable — ") || !strings.Contains(unbound, gone) || strings.Contains(unbound, "does not match") {
		t.Fatalf("unresolvable endpoint root did not surface its resolution error:\n%s", unbound)
	}
}

// endpointLedgerFixture also serves the endpoint's single root-record read,
// which names the accepted ledger identity for the brain state.
type endpointLedgerFixture struct{ *reportFixture }

func (f endpointLedgerFixture) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	const rootRecord = "plans/goals/backlog.md"
	if len(prefixes) == 1 && prefixes[0] == rootRecord && f.present && commit == reportAcceptedTip {
		return map[string][]byte{rootRecord: append([]byte(nil), f.files[rootRecord]...)}, nil
	}
	return f.reportFixture.Files(commit, prefixes...)
}
