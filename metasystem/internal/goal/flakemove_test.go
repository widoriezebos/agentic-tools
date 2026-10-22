package goal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const flakeMoveGoal = "tests-observe-only-what-they-inject-2"

func TestFlakeRecordsOnlyReadsTheChange(t *testing.T) {
	t.Parallel()
	cases := []string{"ordinary-name", "stacked", "two-tests-one-record", "extra-instruction", "red-entry-beside-sighting", "closure-beside-sighting", "cadence-changed", "same-timestamp-sorts-first", "old-entry-removed", "history-reason", "discarded-prose", "verb-only-changed", "register-only", "design-edit-flake-fix", "opened-fix", "record-plus-edit", "unlinked-goal", "broken-register", "archived", "fixture"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			id := flakeMoveGoal
			if name == "design-edit-flake-fix" {
				id = "flake-fix-metasystem-internal-x-testy"
			}
			f := vGoal(id, StateQueued)
			f.NextStep = "Keep this step."
			e, client := fakeGoalEndpoint(t, f)
			args := flakeFixture()
			args.ObserveOnly, args.Where, args.Repeat = true, "main", "alone"
			if name == "two-tests-one-record" {
				args.Tests = []string{"TestOne", "TestTwo"}
			}
			if name == "same-timestamp-sorts-first" {
				args.Tests = []string{"TestZ"}
			}
			record := func(n int) {
				t.Helper()
				args.Attempt, args.Rerun.Attempt = fmt.Sprintf("red-%d", n), fmt.Sprintf("green-%d", n)
				r := flakeRequest(e, n)
				if name == "same-timestamp-sorts-first" {
					r.Now = flakeRequest(e, 0).Now
				}
				if got, err := RecordFlake(r, args); err != nil || got.Publish.Outcome != OutcomeConfirmed {
					t.Fatalf("record: %+v %v", got, err)
				}
			}
			record(0)
			seed := client.store.commits[client.store.canonical]
			entries, problems := ParseTrunkRed(seed.files[trunkRedPath])
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			if name != "register-only" && name != "opened-fix" {
				for i := range entries {
					entries[i].FixGoal, entries[i].FixOpenedAt = id, f.OpenedAt
				}
			}
			seed.files[trunkRedPath] = RenderTrunkRed(entries)
			client.store.commits[client.store.canonical] = seed
			snapshot := func() map[string][]byte {
				files := map[string][]byte{}
				for path, data := range client.store.commits[client.store.canonical].files {
					files[path] = bytes.TrimSpace(bytes.Clone(data))
				}
				return files
			}
			a := snapshot()
			record(1)
			b := snapshot()
			if name == "stacked" || name == "fixture" || name == "opened-fix" {
				record(2)
			}
			c := snapshot()
			if name == "same-timestamp-sorts-first" {
				for n := 0; ; n++ {
					candidate := fmt.Sprintf("TestNew%d", n)
					if FlakeIdentity(args.Unit, candidate) < FlakeIdentity(args.Unit, "TestZ") {
						args.Tests = []string{candidate}
						break
					}
				}
				record(2)
				c = snapshot()
			}
			if name == "opened-fix" {
				args.Fix = &FlakeFix{Identity: FlakeIdentity(args.Unit, "TestOne"), ID: "new-flake-fix", Intent: "Fix the flaky test.", Basis: "Two confirmed sightings.", Next: "Repair it.", Tier: 3, Priority: 1}
				record(3)
				c = snapshot()
			}
			path := livePath(id)
			edit := func(change func(*GoalFile)) {
				got, problems := ParseFile(c[path])
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				change(got)
				c[path] = bytes.TrimSpace(RenderFile(got))
			}
			switch name {
			case "extra-instruction":
				edit(func(g *GoalFile) { g.NextStep += " Also change the implementation." })
			case "design-edit-flake-fix", "record-plus-edit":
				next := "Change the design."
				if got, err := Edit(flakeRequest(e, 3), id, EditFields{NextStep: &next}); err != nil || got.Outcome != OutcomeConfirmed {
					t.Fatalf("edit: %+v %v", got, err)
				}
				c = snapshot()
			case "history-reason":
				edit(func(g *GoalFile) { g.History[len(g.History)-1].Reason = "Do something extra." })
			case "verb-only-changed":
				edit(func(g *GoalFile) { g.History[len(g.History)-1].Verb = "edit" })
			case "discarded-prose":
				body, _, ok := splitIntegrity(c[path])
				if !ok {
					t.Fatal("missing integrity")
				}
				body = bytes.Replace(body, []byte("History:\n"), []byte("History:\n\nUnrecorded instructions.\n"), 1)
				c[path] = []byte(string(body) + "Integrity: sha256=" + IntegrityDigest(body))
			case "red-entry-beside-sighting", "closure-beside-sighting", "old-entry-removed", "cadence-changed":
				entries, problems := ParseTrunkRed(c[trunkRedPath])
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				switch name {
				case "red-entry-beside-sighting":
					entries = append(entries, testTrunkRedEntry("red:unit:TestOther", "red:unit:TestOther", "2026-10-04T12:01:00Z"))
				case "closure-beside-sighting":
					entries[0].Closed = &TrunkRedClosure{At: "2026-10-04T12:01:00Z", How: "hand", Opid: flakeRequest(e, 4).opid(), By: "wido", Why: "Resolved."}
				case "old-entry-removed":
					extra := testTrunkRedEntry("red:unit:TestOther", "red:unit:TestOther", "2026-10-04T10:00:00Z")
					prior, _ := ParseTrunkRed(a[trunkRedPath])
					a[trunkRedPath] = RenderTrunkRed(append(prior, extra))
				}
				c[trunkRedPath] = RenderTrunkRed(entries)
				if name == "cadence-changed" {
					status := cadenceStatusFixture(cadenceKeyFixture(), flakeRequest(e, 1).Now, "passed")
					status.Opid = flakeRequest(e, 1).opid()
					c[trunkRedPath] = renderTrunkRedState(entries, &status, nil)
				}
			case "unlinked-goal", "archived":
				target := livePath("unlinked-goal")
				if name == "archived" {
					target = recordsGoalsPrefix + id + ".md"
				}
				a[target], c[target] = a[path], c[path]
				delete(a, path)
				delete(c, path)
			case "broken-register":
				c[trunkRedPath] = []byte("broken")
			}
			if name != "broken-register" {
				if _, problems := ParseTrunkRed(c[trunkRedPath]); len(problems) != 0 {
					t.Fatalf("tampered register must remain valid: %v", problems)
				}
			}
			before, after := map[string][]byte{}, map[string][]byte{}
			for path := range a {
				if !bytes.Equal(a[path], c[path]) {
					before[path], after[path] = a[path], c[path]
				}
			}
			for path := range c {
				if _, ok := a[path]; !ok {
					after[path] = c[path]
				}
			}
			want := slices.Contains([]string{"ordinary-name", "stacked", "two-tests-one-record", "register-only", "same-timestamp-sorts-first", "fixture"}, name)
			if got := FlakeRecordsOnly(before, after); got != want {
				t.Fatalf("flake-only comparison = %v, want %v; changed paths %d", got, want, len(after))
			}
			if name == "fixture" {
				for i, tree := range []map[string][]byte{a, b, c} {
					for _, path := range []string{trunkRedPath, livePath(id)} {
						file := filepath.Join("..", "landing", "plain", "testdata", "flake-record", string(rune('a'+i)), filepath.Base(path))
						if os.Getenv("METASYSTEM_WRITE_FLAKE_FIXTURE") == "1" {
							if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(file, tree[path], 0o644); err != nil {
								t.Fatal(err)
							}
						}
						got, err := os.ReadFile(file)
						if err != nil || !bytes.Equal(got, tree[path]) {
							t.Fatalf("fixture %s differs from recorded transaction: %v", file, err)
						}
					}
				}
			}
		})
	}
}

func TestFlakeRecordPathNamesOnlyTheRegisterAndLiveGoalPages(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"plans/goals/trunk-red.json":          true,
		"plans/goals/a-flake-is-not-a-red.md": true,
		"plans/goals/done/old-goal.md":        false,
		"plans/goals/notes.txt":               false,
		"plans/designs/fixture.md":            false,
		"docs/project-rules.md":               false,
	} {
		if got := FlakeRecordPath(name); got != want {
			t.Errorf("FlakeRecordPath(%q) = %v, want %v", name, got, want)
		}
	}
}
