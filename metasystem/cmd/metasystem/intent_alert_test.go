package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

var alertTestNow = time.Date(2026, 9, 30, 11, 10, 0, 0, time.UTC)

// alertBed is one host: a seat checkout and the landing lane's checkout,
// both nested (the checkout root holds .git, its metasystem/ installation
// the steward's store), and the host home whose lane record names the
// landing checkout.
type alertBed struct {
	home, seat, landing string
}

func newAlertBed(t *testing.T) alertBed {
	t.Helper()
	base := realpath.Resolve(t.TempDir())
	bed := alertBed{home: filepath.Join(base, "home"), seat: filepath.Join(base, "seat"), landing: filepath.Join(base, "landing")}
	for _, top := range []string{bed.seat, bed.landing} {
		helmMust(t, os.MkdirAll(filepath.Join(top, ".git"), 0o755), os.MkdirAll(filepath.Join(top, "metasystem"), 0o755),
			os.WriteFile(filepath.Join(top, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644))
	}
	layout, err := lane.NewLayout(bed.landing)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(bed.home, layout, "Wido", alertTestNow); err != nil {
		t.Fatal(err)
	}
	return bed
}

func (bed alertBed) store(top string) string { return filepath.Join(top, "metasystem") }

func (bed alertBed) owners() intentOwners {
	top := func(path string) (string, error) {
		for dir := path; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				return dir, nil
			}
			if filepath.Dir(dir) == dir {
				return "", errors.New("not a repository")
			}
		}
	}
	return intentOwners{resolver: stateroot.NewResolver(top, os.Executable), alerts: alertOwners{
		home: func() (string, error) { return bed.home, nil },
		now:  func() time.Time { return alertTestNow },
		invoker: func() (steward.AlertInvoker, error) {
			return steward.AlertInvoker{Pid: 4242, PidStartedAt: 77, UID: 501}, nil
		},
	}}
}

func (bed alertBed) run(t *testing.T, cwd string, words ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, cwd, bed.owners())
	return code, stdout.String(), stderr.String()
}

// open opens one pattern episode for work in the store of a checkout.
func (bed alertBed) open(t *testing.T, top, work string) steward.AlertEpisode {
	t.Helper()
	episode, created, err := steward.OpenAlert(bed.store(top), steward.AlertOpening{Owner: steward.PatternOwner("stagnation"), Work: work,
		Since: alertTestNow.Add(-2 * time.Hour), Message: "The landing lane has worked on batch " + work[:5] + " for 2 hours without landing it.",
		Evidence: []steward.AlertEvidence{{Record: "artifacts/agents/landing-batches/" + work + ".json", At: "2026-09-30T08:59:51Z", Fact: "counted active time reached batch-hours=2"}},
		Now:      alertTestNow, Deliver: func(string, string) error { return nil }})
	if err != nil || !created {
		t.Fatalf("open alert: %v %v", created, err)
	}
	return episode
}

func alertIDs(t *testing.T, stdout string) []string {
	t.Helper()
	var result struct {
		Data struct {
			Alerts []struct{ ID string } `json:"alerts"`
		}
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("alert list --json: %v\n%s", err, stdout)
	}
	var ids []string
	for _, alert := range result.Data.Alerts {
		ids = append(ids, alert.ID)
	}
	return ids
}

// From a seat's checkout the alert verbs read the seat's own store and the
// host lane's, list each open alert in two lines under its qualified id,
// and acknowledge and clear the lane's alert; a repeat succeeds and changes
// nothing.
func TestAlertVerbsFromSeatCheckout(t *testing.T) {
	t.Parallel()
	bed := newAlertBed(t)
	laneEpisode := bed.open(t, bed.landing, "4gr18nm8t3nyev9sssda9jgtsq")
	seatEpisode := bed.open(t, bed.seat, "0000000000000000000000000a")
	laneID, seatID := "lane/"+laneEpisode.EpisodeID, "here/"+seatEpisode.EpisodeID

	code, stdout, stderr := bed.run(t, bed.seat, "alert", "list")
	if code != 0 || !strings.Contains(stdout, "2 open alerts") {
		t.Fatalf("alert list = %d %q %q", code, stdout, stderr)
	}
	for _, want := range []string{
		"  " + laneID + "  The landing lane has worked on batch 4gr18 for 2 hours without landing it.\n    run: metasystem alert ack " + laneID + "\n",
		"  " + seatID + "  The landing lane has worked on batch 00000 for 2 hours without landing it.\n    run: metasystem alert ack " + seatID + "\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("alert list lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "landing-batches") {
		t.Fatalf("evidence shows without --verbose:\n%s", stdout)
	}
	if _, verbose, _ := bed.run(t, bed.seat, "alert", "list", "--verbose"); !strings.Contains(verbose, "evidence 2026-09-30T08:59:51Z artifacts/agents/landing-batches/4gr18nm8t3nyev9sssda9jgtsq.json") ||
		!strings.Contains(verbose, "work 4gr18nm8t3nyev9sssda9jgtsq, kept in "+bed.store(bed.landing)) {
		t.Fatalf("--verbose lacks the work, evidence or checkout:\n%s", verbose)
	}

	for _, act := range []string{"ack", "clear"} {
		if code, stdout, stderr := bed.run(t, bed.seat, "alert", act, laneID); code != 0 || !strings.Contains(stdout, laneID) {
			t.Fatalf("alert %s = %d %q %q", act, code, stdout, stderr)
		}
		if code, stdout, stderr := bed.run(t, bed.seat, "alert", act, laneID, "--json"); code != 0 || !strings.Contains(stdout, `"outcome": "unchanged"`) {
			t.Fatalf("a repeated alert %s = %d %q %q", act, code, stdout, stderr)
		}
	}
	episodes, err := steward.AlertEpisodes(bed.store(bed.landing))
	if err != nil || len(episodes) != 1 || !episodes[0].Acknowledged || !episodes[0].Cleared || !episodes[0].Suppressed {
		t.Fatalf("the lane's episode after ack and clear: %+v %v", episodes, err)
	}
	if _, stdout, _ := bed.run(t, bed.seat, "alert", "list", "--json"); strings.Join(alertIDs(t, stdout), ",") != seatID {
		t.Fatalf("the open alerts after the clear: %s", stdout)
	}
	if _, stdout, _ := bed.run(t, bed.seat, "alert", "list", "--all", "--json"); len(alertIDs(t, stdout)) != 2 {
		t.Fatalf("--all does not list the cleared alert: %s", stdout)
	}
	// On the lane's checkout the two names are one store, shown as lane.
	_, stdout, _ = bed.run(t, bed.landing, "alert", "list", "--all", "--json")
	if strings.Join(alertIDs(t, stdout), ",") != laneID {
		t.Fatalf("alert list on the lane checkout: %s", stdout)
	}
	if code, _, stderr := bed.run(t, bed.landing, "alert", "ack", "here/"+laneEpisode.EpisodeID); code != 0 {
		t.Fatalf("here/ on the lane checkout does not name the lane's store: %d %s", code, stderr)
	}
}

// The same on-disk id in both stores is guidance, not a denial: a bare id
// answers with both qualified ids, and each qualified id acts on its store.
func TestDuplicateLocalIdsNeedQualification(t *testing.T) {
	t.Parallel()
	bed := newAlertBed(t)
	work := "4gr18nm8t3nyev9sssda9jgtsq"
	onLane, onSeat := bed.open(t, bed.landing, work), bed.open(t, bed.seat, work)
	if onLane.EpisodeID != onSeat.EpisodeID {
		t.Fatalf("the fixture needs one id in both stores: %s %s", onLane.EpisodeID, onSeat.EpisodeID)
	}
	id := onLane.EpisodeID
	code, stdout, stderr := bed.run(t, bed.seat, "alert", "ack", id)
	if code != 2 || !strings.Contains(stderr, "lane/"+id+" and here/"+id) || !strings.Contains(stderr, "metasystem alert ack lane/"+id) {
		t.Fatalf("a bare id in both stores = %d %q %q", code, stdout, stderr)
	}
	if episodes, _ := steward.AlertEpisodes(bed.store(bed.seat)); episodes[0].Acknowledged {
		t.Fatal("the ambiguous ack changed a store")
	}
	if code, _, stderr := bed.run(t, bed.seat, "alert", "ack", "here/"+id); code != 0 {
		t.Fatalf("here/%s = %d %s", id, code, stderr)
	}
	seatEpisodes, _ := steward.AlertEpisodes(bed.store(bed.seat))
	laneEpisodes, _ := steward.AlertEpisodes(bed.store(bed.landing))
	if !seatEpisodes[0].Acknowledged || laneEpisodes[0].Acknowledged {
		t.Fatalf("here/ acted on the wrong store: seat %+v lane %+v", seatEpisodes[0], laneEpisodes[0])
	}
	// A bare id in one store is enough.
	other := bed.open(t, bed.landing, "0000000000000000000000000b")
	if code, _, stderr := bed.run(t, bed.seat, "alert", "clear", other.EpisodeID); code != 0 {
		t.Fatalf("a unique bare id = %d %s", code, stderr)
	}
}

// Session status's first line names the open behaviour alerts.
func TestSessionStatusNamesOpenPatternAlerts(t *testing.T) {
	t.Parallel()
	c := layoutCase{name: "session-status", args: []string{"session", "status", "--id", "1", "--root", "ROOT"}, bed: stopReportLayoutBed}
	bed := c.bed(t)
	env := layoutEnv(t, bed.now, "", "")
	bed.owners.textEnv = func(io.Writer) textui.Env { return env }
	root := bed.words["ROOT"]
	for _, work := range []string{"0000000000000000000000000a", "0000000000000000000000000b"} {
		if _, _, err := steward.OpenAlert(root, steward.AlertOpening{Owner: steward.PatternOwner("stagnation"), Work: work, Since: alertTestNow,
			Message: "The landing lane has worked on batch 00000 for 2 hours without landing it.", Now: alertTestNow, Deliver: func(string, string) error { return nil }}); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runLayoutCase(t, c, bed)
	lines := strings.Split(stdout, "\n")
	if code != 0 || !strings.Contains(lines[0], "2 behaviour alerts are open") || !strings.Contains(lines[1], "metasystem alert list") {
		t.Fatalf("session status = %d:\n%s%s", code, stdout, stderr)
	}
}
