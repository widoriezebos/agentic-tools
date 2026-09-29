package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// The disk owners read the landing lane through the host lane resolver
// (U12), never the seat's own landing.batch-root: the lane the host's
// record names is the one whose landing batches hold attempts, even when
// the seat's setting names another checkout.
func TestLandingLaneRootsReadTheHostLaneNotTheSeatSetting(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	seatSetting := filepath.Join(t.TempDir(), "seat-landing")
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\nlanding.batch-root="+seatSetting+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hostLane := filepath.Join(t.TempDir(), "host-lane")
	var asked []string
	resolve := func(root string, _ time.Time) (string, bool, error) {
		asked = append(asked, root)
		return hostLane, true, nil
	}
	roots, err := landingLaneRootsWith(resolve, installation, attemptsNow)
	if err != nil || len(roots) == 0 || roots[0] != hostLane || slices.Contains(roots, seatSetting) {
		t.Fatalf("roots %v err %v: want the host lane %s first and never the seat setting", roots, err, hostLane)
	}
	if !slices.Equal(asked, []string{installation}) {
		t.Fatalf("the resolver is asked for the installation once: %v", asked)
	}
	unconfigured := func(string, time.Time) (string, bool, error) { return "", false, nil }
	if roots, err := landingLaneRootsWith(unconfigured, installation, attemptsNow); err != nil || len(roots) != 0 {
		t.Fatalf("no host lane reads no lane: %v %v", roots, err)
	}
}

// An unresolvable lane, and an unbound resolver, fail closed: the lookup
// is an error, which holds the landing batches' kind and the clone report.
func TestLandingLaneRootsFailClosed(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	broken := func(string, time.Time) (string, bool, error) { return "", true, errors.New("lane record unreadable") }
	if _, err := landingLaneRootsWith(broken, installation, attemptsNow); err == nil || !strings.Contains(err.Error(), "lane record unreadable") {
		t.Fatalf("an unresolvable lane is an error: %v", err)
	}
	if _, err := landingLaneRootsWith(nil, installation, attemptsNow); err == nil {
		t.Fatal("an unbound lane resolver is an error, never no lane")
	}
	configuredEmpty := func(string, time.Time) (string, bool, error) { return "", true, nil }
	if _, err := landingLaneRootsWith(configuredEmpty, installation, attemptsNow); err == nil {
		t.Fatal("a configured lane with no root is an error")
	}
}

// The unowned-clone report never lists the landing lane's checkout, and an
// unresolvable lane holds the report as the unreadable registry does.
func TestUnownedClonesExcludeTheLandingLane(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	checkout := filepath.Join(parent, "checkout")
	for _, dir := range []string{filepath.Join(checkout, ".git"), filepath.Join(parent, "clone", ".git"), filepath.Join(parent, "lane", ".git")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	git := func(_ context.Context, dir string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "rev-list" && dir == checkout:
			return []byte("root1\n"), nil
		case args[0] == "cat-file":
			return nil, nil
		}
		return nil, stewardGitNotFound{}
	}
	class := UnownedClones{GitRoot: checkout, Git: git, Lane: []string{filepath.Join(parent, "lane")}}
	items, err := class.Plan(context.Background(), nil)
	if err != nil || len(items) != 1 || items[0].Path != filepath.Join(parent, "clone") {
		t.Fatalf("only the clone is listed, never the lane: %+v %v", items, err)
	}
	held := UnownedClones{GitRoot: checkout, Git: git, LaneErr: errors.New("lane record unreadable")}
	items, _ = held.Plan(context.Background(), nil)
	if len(items) != 1 || items[0].Verdict.Decision != diskstore.Pending || !strings.Contains(items[0].Verdict.Reason, "landing lane") {
		t.Fatalf("an unresolvable lane holds the report: %+v", items)
	}
}
