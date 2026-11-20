package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

const platformFixtureID = "platform:darwin/arm64:internal/unit:TestOne"

func platformFixtureLog(commit, platform, route, scope, verdict string, failed, passed map[string][]string) string {
	osName, arch, _ := strings.Cut(platform, "/")
	var output strings.Builder
	fmt.Fprintf(&output, "landing environment go version fixture; GOOS=%q; GOARCH=%q\n", osName, arch)
	coverage, _ := json.Marshal(passed)
	fmt.Fprintf(&output, "landing passed %s\nlanding clock total 100\n", coverage)
	for unit, tests := range failed {
		fmt.Fprintf(&output, "LANDING-FAILED\t%s\t%s\n", unit, strings.Join(tests, " "))
	}
	fmt.Fprintf(&output, "LANDING-CHECKED\t%d\n", len(failed))
	record, _ := json.Marshal(map[string]any{"commit": commit, "platform": platform, "route": route, "scope": scope, "verdict": verdict, "failed": failed})
	fmt.Fprintf(&output, "landing result %s\n", record)
	return output.String()
}

func platformTestSetting(t *testing.T, b *intentBed, platforms string) string {
	t.Helper()
	parent := b.repo.accepted
	tip, err := b.repo.Build("setting", parent, []goal.Change{{Path: "metasystem.conf", Content: []byte("landing.deciding-platforms=" + platforms + "\n")}}, "declare deciding platforms")
	helmMust(t, err)
	_, err = b.repo.Publish(parent, tip)
	helmMust(t, err)
	helmMust(t, b.repo.AcceptedCAS(parent, tip))
	return tip
}

func platformEntries(t *testing.T, b *intentBed) []goal.TrunkRedEntry {
	t.Helper()
	entries, problems := goal.ParseTrunkRed(b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"])
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return entries
}

func TestPlatformIncidentFilingRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, setting, platform, route, scope, verdict, remove, reason string }{
		{"handover", "", "darwin/arm64", "host", "complete", "red", "", "deciding"},
		{"deciding", "linux/amd64 linux/arm64", "linux/arm64", "host", "complete", "red", "", "decides landings"},
		{"missing environment", "linux/amd64", "darwin/arm64", "host", "complete", "red", "landing environment", "environment"},
		{"old proof", "linux/amd64", "darwin/arm64", "host", "complete", "red", "landing result", "complete"},
		{"truncated", "linux/amd64", "darwin/arm64", "host", "complete", "red", "LANDING-CHECKED", "complete"},
		{"vm", "linux/amd64", "darwin/arm64", "vm", "complete", "red", "", "native"},
		{"scoped", "linux/amd64", "darwin/arm64", "host", "scoped", "red", "", "scoped"},
		{"narrowed", "linux/amd64", "darwin/arm64", "host", "narrowed", "red", "", "narrow"},
		{"green", "linux/amd64", "darwin/arm64", "host", "complete", "green", "", "nothing failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newIntentBed(t, false, nil)
			b.lineage = "m1"
			commit := platformTestSetting(t, b, tc.setting)
			failed := map[string][]string{}
			if tc.verdict == "red" {
				failed["internal/unit"] = []string{"TestOne"}
			}
			data := platformFixtureLog(commit, tc.platform, tc.route, tc.scope, tc.verdict, failed, nil)
			var kept []string
			for _, line := range strings.Split(data, "\n") {
				if tc.remove == "" || !strings.HasPrefix(line, tc.remove) {
					kept = append(kept, line)
				}
			}
			path := filepath.Join(t.TempDir(), "proof.log")
			helmMust(t, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0600))
			code, res := b.runJSON(b.owners(), "incident", "file", path)
			if code == 0 || b.repo.accepted != commit || !strings.Contains(strings.ToLower(res.Summary), tc.reason) {
				t.Fatalf("unexpected filing result: %d %+v", code, res)
			}
		})
	}
}

func TestPlatformIncidentLifecycle(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	b.lineage = "m1"
	commit := platformTestSetting(t, b, "linux/amd64")
	path := filepath.Join(t.TempDir(), "night.log")
	write := func(data string) { t.Helper(); helmMust(t, os.WriteFile(path, []byte(data), 0600)) }
	act := func(wantOK bool, args ...string) {
		t.Helper()
		code, res := b.runJSON(b.owners(), args...)
		if (code == 0) != wantOK {
			t.Fatalf("%v: %d %+v", args, code, res)
		}
	}
	log := platformFixtureLog(commit, "darwin/arm64", "host", "complete", "red", map[string][]string{"internal/unit": {"TestOne", "TestTwo"}, "build": {}}, nil)
	write(log)
	act(false, "incident", "file", path, "--platform", "linux/amd64")
	partial := platformFixtureLog(commit, "darwin/arm64", "host", "complete", "red", map[string][]string{"build": {}}, nil)
	write(partial)
	act(true, "incident", "file", path)
	write(log)
	act(true, "incident", "file", path)
	entries := platformEntries(t, b)
	if len(entries) != 3 {
		t.Fatalf("filed %d incidents", len(entries))
	}
	find := func(id string) goal.TrunkRedEntry {
		t.Helper()
		for _, entry := range platformEntries(t, b) {
			if entry.ID == id {
				return entry
			}
		}
		t.Fatalf("absent %s", id)
		return goal.TrunkRedEntry{}
	}
	if len(find("platform:darwin/arm64:build").Sightings) != 2 {
		t.Fatal("multi-unit filing lost the earlier sighting")
	}
	p := find(platformFixtureID)
	s := p.Sightings[0]
	if p.Platform != "darwin/arm64" || p.Class != "platform" || p.TestUnit != "internal/unit" || p.TestName != "TestOne" || s.LogPath != path || s.BaseCommit != commit || s.LogDigest != fmt.Sprintf("%x", sha256.Sum256([]byte(log))) || len(p.Holds) != 0 || p.Owner.Machine != "" {
		t.Fatalf("incomplete platform incident %+v", p)
	}
	if _, held := goal.LandingIncident(entries, bedGoal); held {
		t.Fatal("seat held")
	}
	before := b.repo.accepted
	act(true, "incident", "file", path)
	if b.repo.accepted != before {
		t.Fatal("duplicate log published")
	}
	write(strings.Replace(log, "total 100", "total 101", 1))
	act(true, "incident", "file", path)
	if len(find(platformFixtureID).Sightings) != 2 {
		t.Fatal("new run lost")
	}
	act(false, "incident", "claim", platformFixtureID, "--goal", "absent")
	act(true, "incident", "claim", platformFixtureID, "--goal", bedGoal)
	p = find(platformFixtureID)
	if p.FixGoal != bedGoal || p.Owner.How != "taken" {
		t.Fatalf("claim: %+v", p)
	}
	code, out, _ := b.run(b.owners(), "incident", "list")
	if code != 0 || !strings.Contains(out, "Platform incidents") || !strings.Contains(out, "darwin/arm64") || !strings.Contains(out, p.Owner.Machine) {
		t.Fatalf("listing %s", out)
	}
	for _, verb := range []string{"list", "next"} {
		var code int
		var out string
		if verb == "next" {
			var stdout, stderr bytes.Buffer
			deps := b.dependencies()
			deps.proveTerminal = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent session")
			}
			code = runGoalNextWithInputs([]string{"--root", b.root(), "--machine", "mac-cli"}, deps, b.commandNow, &stdout, &stderr)
			out = stdout.String()
		} else {
			code, out, _ = b.run(b.owners(), "goal", verb)
		}
		if code != 0 || strings.Contains(out, "main is red") || strings.Contains(out, "trunk red "+platformFixtureID) {
			t.Fatalf("goal %s: %s", verb, out)
		}
	}
	owners := incidentRecorderFixture(t, b, "linux-lane")
	check := incidentCheckFixture(t, "TestOne")
	check.Commit = commit
	helmMust(t, owners.landingIncidentRecorder(b.root())([]plain.Result{check}))
	entries = platformEntries(t, b)
	red, held := goal.LandingIncident(entries, "unrelated")
	if !held || red.ID != "red:internal/unit:TestOne" || !reflect.DeepEqual(find(platformFixtureID), p) {
		t.Fatal("Linux red did not independently hold")
	}
	owners.plainProve.Git = func(_ string, args ...string) (string, error) { return b.repo.canonical, nil }
	green := plain.Result{Trunk: true, Result: plain.Green, Attempt: "linux-green", Scope: "full", Tree: "tree", FullTree: "tree", At: "now", FullAt: "now", Commit: b.repo.accepted}
	helmMust(t, owners.clearLandingIncidents(b.root(), green))
	if find(red.ID).Closed == nil || !reflect.DeepEqual(find(platformFixtureID), p) {
		t.Fatal("Linux green crossed identities")
	}
	green.Scope, green.Ran = "scoped", []string{"internal/unit"}
	helmMust(t, owners.clearLandingIncidents(b.root(), green))
	if !reflect.DeepEqual(find(platformFixtureID), p) {
		t.Fatal("scoped Linux green changed the platform incident")
	}
	repair := b.repo.accepted
	parent := b.repo.commit(commit).parent
	sibling, err := b.repo.Build("sibling", parent, []goal.Change{{Path: "sibling.marker", Content: []byte("sibling")}}, "independent branch")
	helmMust(t, err)
	passed := map[string][]string{"internal/unit": {"TestOne"}}
	for _, tc := range []struct {
		commit, platform, scope string
		passed                  map[string][]string
	}{
		{parent, "darwin/arm64", "complete", passed}, {sibling, "darwin/arm64", "complete", passed},
		{commit, "darwin/arm64", "complete", passed}, {repair, "linux/amd64", "complete", passed}, {repair, "darwin/amd64", "complete", passed}, {repair, "darwin/arm64", "narrowed", passed}, {repair, "darwin/arm64", "complete", map[string][]string{"internal/unit": {"TestOther"}}},
	} {
		write(platformFixtureLog(tc.commit, tc.platform, "host", tc.scope, "green", nil, tc.passed))
		act(false, "incident", "prove-closed", platformFixtureID, path)
		if !reflect.DeepEqual(find(platformFixtureID), p) {
			t.Fatal("refused closure changed entry")
		}
	}
	closeLog := platformFixtureLog(repair, "darwin/arm64", "host", "complete", "green", nil, passed)
	write(closeLog)
	act(true, "incident", "prove-closed", platformFixtureID, path)
	closed := find(platformFixtureID).Closed
	if closed == nil || closed.Platform != p.Platform || closed.BaseCommit != repair || closed.Attempt == "" || closed.LogPath != path || closed.LogDigest != fmt.Sprintf("%x", sha256.Sum256([]byte(closeLog))) {
		t.Fatalf("closure %+v", closed)
	}
	before = b.repo.accepted
	act(true, "incident", "prove-closed", platformFixtureID, path)
	if b.repo.accepted != before {
		t.Fatal("repeated close published")
	}
	write(log)
	act(true, "incident", "file", path)
	if b.repo.accepted != before {
		t.Fatal("replayed an old red after closure")
	}
	write(strings.Replace(log, "total 100", "total 102", 1))
	act(true, "incident", "file", path)
	reopened := find(platformFixtureID + "-2")
	agent := b.owners()
	agent.prove = unprovable
	if code, _ := b.runJSON(agent, "incident", "close", reopened.ID, "--why", "resolved"); code == 0 {
		t.Fatal("agent hand-closed incident")
	}
	act(true, "incident", "claim", reopened.ID, "--by", "Wido", "--to", "mac-fix", "--goal", bedGoal)
	if owner := find(reopened.ID).Owner; owner.How != "hand" || owner.Machine != "mac-fix" {
		t.Fatalf("assignment: %+v", owner)
	}
	act(true, "incident", "close", reopened.ID, "--by", "Wido", "--reason", "retired this test")
	hand := find(reopened.ID).Closed
	if hand == nil || hand.How != "hand" || hand.By != "Wido" || hand.Why != "retired this test" {
		t.Fatalf("hand close: %+v", hand)
	}
	write(closeLog)
	act(true, "incident", "prove-closed", reopened.ID, path)
	if !reflect.DeepEqual(find(reopened.ID).Closed, hand) {
		t.Fatal("replaced a person's closure")
	}
	code, out, _ = b.run(b.owners(), "incident", "list", "--all")
	if code != 0 || !strings.Contains(out, repair) || !strings.Contains(oneSpaced(out), "closed by darwin/arm64") {
		t.Fatalf("closed listing: %s", out)
	}
}

func witnessPlatformRepeat(t *testing.T, closing bool) {
	b := newIntentBed(t, false, nil)
	b.lineage = "m1"
	commit := platformTestSetting(t, b, "linux/amd64")
	path := filepath.Join(t.TempDir(), "night.log")
	id := "platform:linux/arm64:unit"
	log := platformFixtureLog(commit, "linux/arm64", "host", "complete", "red", map[string][]string{"unit": {}}, nil)
	helmMust(t, os.WriteFile(path, []byte(log), 0600))
	args := []string{"incident", "file", path}
	run := func() {
		t.Helper()
		if code, res := b.runJSON(b.owners(), args...); code != 0 {
			t.Fatalf("%v: %+v", args, res)
		}
	}
	run()
	if closing {
		log = platformFixtureLog(b.repo.accepted, "linux/arm64", "host", "complete", "green", nil, map[string][]string{"unit": {}})
		helmMust(t, os.WriteFile(path, []byte(log), 0600))
		args = []string{"incident", "prove-closed", id, path}
		run()
	}
	before := b.repo.accepted
	run()
	if b.repo.accepted != before {
		t.Fatal("repeated platform check moved the register")
	}
}
