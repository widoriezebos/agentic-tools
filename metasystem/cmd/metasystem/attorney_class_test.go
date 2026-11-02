package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestTheClassTableRunsEachRowsRealVerb(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		class, argv, outcome string
		human, successor     bool
	}{
		{"ordering", "goal prioritize standing-validation 1", intentConfirmed, false, false},
		{"ordering", "goal pin standing-validation m1e", intentConfirmed, false, false},
		{"ordering", "goal group standing-validation arc-1", intentConfirmed, false, false},
		{"acks", "goal prioritize standing-validation 1", intentRefused, false, false},
		{"acks", "goal pin standing-validation m1e", intentRefused, false, false},
		{"acks", "goal group standing-validation arc-1", intentConfirmed, false, false},
		{"recovery", "machine revive mac-cli", intentRefused, false, false},
		{"recovery", "goal approve f", intentConfirmed, false, false},
		{"recovery", "goal approve n", intentRefused, false, false},
		{"recovery", "goal approve f --budget norm", intentRefused, false, false},
		{"recovery", "goal approve f --id n", intentRefused, false, false},
		{"stale-releases", "goal release c --reason ended", intentConfirmed, false, false},
		{"stale-releases", "goal release d --reason ended", intentRefused, false, false},
		{"stale-releases", "goal release c --reason ended --arc", intentRefused, false, false},
		{"ordering", "goal release c --reason ended", intentRefused, false, false},
		{"ordering", "goal prioritize standing-validation 2", intentConfirmed, false, true},
		{"ordering", "goal budget standing-validation 1d/10/720m/1/3", intentConfirmed, true, false},
		{"ordering", "goal budget standing-validation 1d/10/720m/1/3", intentRefused, false, false},
	} {
		t.Run(row.class+"/"+row.argv+fmt.Sprint(row.human, row.successor), func(t *testing.T) {
			t.Parallel()
			b := newGrantEverythingBed(t)
			now, _ := b.commandNow(b.root())
			b.addGoal(queuedIntentGoal("f", 3))
			b.addGoal(queuedIntentGoal("n", 3))
			for _, id := range []string{"c", "d"} {
				file := b.goalFile(bedGoal)
				file.Id, file.Arc, file.Claimed.Lineage = id, "dead-arc", "lin-dead"
				if id == "d" {
					file.Claimed.Machine = "mac-other"
				}
				for i := range file.History {
					file.History[i].Targets = []string{id}
				}
				b.addGoal(file)
			}
			g := b.goalFile(bedGoal)
			g.State, g.Claimed, g.StopCapability = goal.StateApproved, nil, nil
			b.addGoal(g)
			stamp := now.Add(-time.Hour).Format(time.RFC3339)
			flake := goal.TrunkRedEntry{ID: goal.FlakeIdentity("fast", "TestFlake"), Identity: goal.FlakeIdentity("fast", "TestFlake"), Class: goal.TrunkRedClassFlake, Group: "fast", TestUnit: "fast", TestName: "TestFlake", Status: "failed",
				Failures: []goal.TrunkRedFailure{{Name: "TestFlake", Classname: "fast", Report: "red.log", Status: "failed"}}, FixGoal: "f", Opened: stamp,
				Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", BaseCommit: "base-1", BaseTree: "tree-1", LogPath: "red.log", Rerun: &goal.TrunkRedRerun{Attempt: "attempt-2", LogPath: "green.log"}, SeenAt: stamp, Opid: goal.Opid("01J5X0000000000000000000X1", "mac-cli", "lane")}}}
			ordinary := flake
			ordinary.ID, ordinary.Identity, ordinary.Class, ordinary.FixGoal = "tr-fast-main000001", "tr-fast-main000001", goal.TrunkRedClassTrunkRed, "n"
			b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"] = goal.RenderTrunkRed([]goal.TrunkRedEntry{flake, ordinary})
			grantCode, result := b.runJSON(b.owners(), "grant", "add", "--acts", "everything", "--classes", row.class, "--lineage", "seat", "--for", "24h")
			if grantCode != 0 || result.Outcome != intentConfirmed {
				t.Fatalf("person's grant: %d %+v", grantCode, result)
			}
			grant := b.rootRecord().PowerOfAttorney[0].ID
			b.lineage, b.holder.OwnerLineage = launch.SeatOwnerLineage, launch.SeatOwnerLineage
			if row.successor {
				b.holder.MainId = "main-2"
			}
			argv := strings.Fields(row.argv)
			var trace bytes.Buffer
			a := &attorneyAdmitter{owners: attorneyAdmitOwners{endpoint: b.dependencies().endpoint,
				args: func() []string { return append(append([]string(nil), argv...), "--json") }, machine: b.dependencies().machine,
				fixture: func(string) bool { return false }, holder: func(string) (lease.CurrentHolderView, error) { return b.holder, nil },
				classify: func(string, int64) (lease.Classification, error) {
					return lease.Classification{Class: lease.ClassMain, MainId: b.holder.MainId}, nil
				},
				lockShared: func(string) error { return nil }, stderr: &trace, notice: func(n admissionNotice) { fmt.Fprintln(n.w, n.line) },
			}}
			owners := b.owners()
			owners.attorney.admit = a.admit
			owners.machines.registryPath = func() (string, error) { return filepath.Join(b.root(), "host-registry"), nil }
			owners.machines.nickname = func(string) (string, bool) { return "mac-cli", true }
			home := t.TempDir()
			owners.landing.home = func() (string, error) { return home, nil }
			walks, admissions := 0, 0
			owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
				p, err := humanauthority.Prove(root, 95, person(), at)
				if err == nil {
					t.Fatal("seat caller reached the enrolled terminal")
				}
				walks++
				if grant, ok := a.admit(root, 95, at); ok {
					admissions++
					return humanauthority.HelmProof(root, grant, at)
				}
				return p, err
			}
			if row.human {
				owners.prove = b.prove
				b.lineage = ""
			}
			before := b.repo.accepted
			code, got := b.runJSON(owners, argv...)
			if got.Outcome != row.outcome || (code == 0) != (row.outcome == intentConfirmed) {
				t.Fatalf("%s under %s: code=%d %+v, trace=%s", row.argv, row.class, code, got, trace.String())
			}
			if strings.HasPrefix(row.argv, "machine revive") && walks == 0 {
				t.Fatal("revive did not reach the direct-person check")
			}
			if row.outcome == intentRefused {
				if b.repo.accepted != before {
					t.Fatal("refusal changed the ledger")
				}
				return
			}
			id := argv[2]
			if strings.HasPrefix(row.argv, "goal approve") {
				id = "f"
			}
			file := b.goalFile(id)
			last := file.History[len(file.History)-1]
			session := row.class == "acks"
			if session {
				if last.Actor != "mac-cli+"+launch.SeatOwnerLineage || last.AuthorityRuling != "" {
					t.Fatalf("session's group: %+v", last)
				}
			} else if last.Actor != "human:Wido" || (!row.human && !strings.Contains(trace.String(), "grant "+grant)) || (row.human && last.AuthorityRuling != "") {
				t.Fatalf("person's history: %+v", last)
			}
			if !session && !row.human && (walks == 0 || admissions == 0) {
				t.Fatal("owner did not ask the production admitter after its walk refused")
			}
			switch argv[1] {
			case "prioritize":
				if fmt.Sprint(file.Priority) != argv[3] {
					t.Fatalf("priority=%d", file.Priority)
				}
			case "pin":
				if file.Pinned != "m1e" {
					t.Fatalf("pin=%s", file.Pinned)
				}
			case "group":
				if file.Arc != "arc-1" {
					t.Fatalf("arc=%s", file.Arc)
				}
			case "approve":
				if file.State != goal.StateApproved {
					t.Fatalf("approved state=%s", file.State)
				}
			case "release":
				if file.Claimed != nil {
					t.Fatal("claim was not released")
				}
			}
			t.Logf("%s: %s; history actor=%s grant=%s", row.argv, got.Summary, last.Actor, grant)
		})
	}
	// These verbs act as the session without asking the general grant.
	for _, row := range []struct {
		argv  []string
		class string
		bed   func(*testing.T) layoutBed
	}{
		{[]string{"question", "withdraw", "q2", "--reason", "decided"}, "acks", questionLayoutBed},
		{[]string{"work", "stop", "j2:design-r2-4f1c"}, "recovery", jobsLayoutBed},
	} {
		bed := row.bed(t)
		now, err := bed.owners.commandNow(bed.cwd)
		if err != nil {
			t.Fatal(err)
		}
		personProof := enrolledPersonProver(t, bed.cwd, now)
		personOwners := bed.owners
		personOwners.prove = personProof
		var granted bytes.Buffer
		if code := runIntentIn(mustIntentCommand(t, "grant add"), []string{"--acts", "everything", "--classes", row.class, "--lineage", "seat", "--for", "24h", "--json"}, &granted, &granted, bed.cwd, personOwners); code != 0 {
			t.Fatalf("grant for %v: %d %s", row.argv, code, granted.String())
		}
		a := &attorneyAdmitter{owners: attorneyAdmitOwners{endpoint: bed.owners.dependencies.endpoint, args: func() []string { return row.argv }, machine: bed.owners.dependencies.machine,
			fixture: func(string) bool { return false }, holder: func(string) (lease.CurrentHolderView, error) {
				return lease.CurrentHolderView{MainId: "main-1", OwnerLineage: launch.SeatOwnerLineage}, nil
			},
			classify: func(string, int64) (lease.Classification, error) {
				return lease.Classification{Class: lease.ClassMain, MainId: "main-1"}, nil
			}, lockShared: func(string) error { return nil }}}
		bed.owners.dependencies.ownerLineage = func() string { return launch.SeatOwnerLineage }
		bed.owners.attorney.admit = a.admit
		bed.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
			p, err := humanauthority.Prove(root, 95, person(), at)
			if err == nil {
				t.Fatal("seat reached the enrolled terminal")
			}
			if grant, ok := a.admit(root, 95, at); ok {
				return humanauthority.HelmProof(root, grant, at)
			}
			return p, err
		}
		bed.owners.processes.process.classify = func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassMain}, nil
		}
		command, rest, _ := resolveIntentArgv(row.argv)
		var out bytes.Buffer
		if code := runIntentIn(command, append(rest, "--json"), &out, &out, bed.cwd, bed.owners); code != 0 {
			t.Fatalf("%v: %d %s", row.argv, code, out.String())
		}
	}
	alert := newAlertBed(t)
	episode := alert.open(t, alert.seat, "4gr18nm8t3nyev9sssda9jgtsq")
	if code, stdout, stderr := alert.run(t, alert.seat, "alert", "ack", "here/"+episode.EpisodeID); code != 0 {
		t.Fatalf("ack: %d %s%s", code, stdout, stderr)
	}
	selection := newSelectionBed(t)
	before := idemTreeDigest(t, selection.home)
	if code, out := selection.run(t, selection.seat, "landing", "resume", "--json"); code == 0 || !strings.Contains(out, "enrolled") {
		t.Fatalf("resume: %d %s", code, out)
	}
	idemSameTree(t, "refused resume", before, idemTreeDigest(t, selection.home))
}
