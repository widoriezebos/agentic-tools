package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStatusMergeReadBoundIsOnlyAFinding(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, state, conf string
		minutes           float64
		finding           bool
	}{
		{"under", "reviewing", "", 19, false},
		{"at bound", "reviewing", "", 20, false},
		{"over", "reviewing", "", 21, true},
		{"completed", "done", "", 21, false},
		{"minutes only", "reviewing", "", 21, true},
		{"override", "reviewing", "landing.read-minutes=25\n", 21, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := holdLaneFixture(t, nil)
			data, err := os.ReadFile("testdata/lane-merge-read/fix.json")
			var fix plain.Fix
			if err != nil || json.Unmarshal(data, &fix) != nil {
				t.Fatalf("real fix: %s %v", data, err)
			}
			fix.State = tc.state
			fix.ReadStartedAt = laneTestNow.UTC().Format(time.RFC3339Nano)
			fix.ReadMinutes = tc.minutes
			if tc.name == "minutes only" {
				fix.ReadStartedAt = ""
			}
			if err := plain.WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte(tc.conf), 0600); err != nil {
				t.Fatal(err)
			}
			b.owners.landing.plainProve.Now = func() time.Time {
				elapsed := tc.minutes
				if tc.state == "done" {
					elapsed += 100
				}
				return laneTestNow.Add(time.Duration(elapsed) * time.Minute)
			}
			code, output := b.run(t, b.root, "status")
			if code != 0 || strings.Contains(oneSpaced(output), "merge read ran 21 min, bound 20") != tc.finding {
				t.Fatalf("status exit=%d %s", code, output)
			}
			code, output = b.run(t, b.root, "status", "--json")
			var result struct{ Data plain.Status }
			if code != 0 || json.Unmarshal([]byte(output), &result) != nil || (result.Data.MergeReadFinding != "") != tc.finding || result.Data.Stop != nil {
				t.Fatalf("advisory status exit=%d %s", code, output)
			}
		})
	}
}

func TestLandingStatusMergeReadSkipsClosedRecordsAndReportsDamageOnce(t *testing.T) {
	t.Parallel()
	for _, damaged := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed history", true: "damaged record"}[damaged], func(t *testing.T) {
			t.Parallel()
			b, _ := holdLaneFixture(t, nil)
			data, err := os.ReadFile("testdata/lane-merge-read/fix.json")
			var fix plain.Fix
			if err != nil || json.Unmarshal(data, &fix) != nil {
				t.Fatalf("fixture: %v", err)
			}
			fix.State, fix.ReadMinutes = "done", 80
			if err := plain.WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			fix.Attempt, fix.State, fix.ReadMinutes = "open-merge", "reviewing", 19
			if err := plain.WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			if damaged {
				if err := os.WriteFile(filepath.Join(plain.Dir(b.install), "fixes", "broken.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, output := b.run(t, b.root, "status", "--json")
			var result struct{ Data plain.Status }
			if code != 0 || json.Unmarshal([]byte(output), &result) != nil || result.Data.MergeReadFinding != "" {
				t.Fatalf("status=%d %s", code, output)
			}
			count := 0
			for _, problem := range result.Data.Problems {
				if strings.Contains(problem, "unexpected end of JSON input") {
					count++
				}
			}
			if damaged && count != 1 || !damaged && count != 0 {
				t.Fatalf("fix read errors=%d: %+v", count, result.Data.Problems)
			}
		})
	}
}

func TestLaneMergeResolutionRequiresPathsAndReviewableFallback(t *testing.T) {
	t.Parallel()
	for _, paths := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing paths", true: "second-parent fallback"}[paths], func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			owners := b.workOwners()
			owners.work.git = func(_ string, args ...string) ([]byte, error) {
				switch args[0] {
				case "rev-list":
					return []byte("merge first second\n"), nil
				case "merge-tree":
					return []byte("automatic\n"), nil
				case "diff":
					if !paths {
						return []byte("diff --git a/source.go b/source.go\n+resolution\n"), nil
					}
					if args[2] == "second" {
						return []byte("diff --git a/source.go b/source.go\n+main\n"), nil
					}
					return nil, nil
				default:
					t.Fatalf("unexpected git: %v", args)
					return nil, nil
				}
			}
			layout, err := owners.resolver.ResolveLayout(b.root())
			if err != nil {
				t.Fatal(err)
			}
			inv := &intentInvocation{owners: owners, layout: layout}
			fix := &plain.Fix{Commit: "merge"}
			if paths {
				fix.Paths = []conflict.Path{{Path: "source.go", Class: conflict.Builder}}
			}
			diff, fallbackParent, err := inv.mergeResolutionDiff(layout.GitRoot, fix)
			if !paths && err == nil {
				t.Fatal("a merge without recorded conflicts must stay pending")
			}
			if paths && (err != nil || fallbackParent != 2 || !strings.Contains(string(diff), "+main")) {
				t.Fatalf("second-parent resolution=%s error=%v", diff, err)
			}
		})
	}
}

func TestLaneMergeResolutionUsesInstallationPrefixForGeneratedPaths(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{".", "metasystem"} {
		for _, parent := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/parent-%d", prefix, parent), func(t *testing.T) {
				t.Parallel()
				b := newWorkBed(t)
				owners := b.workOwners()
				data, err := os.ReadFile("testdata/lane-merge-read/fix.json")
				var fix plain.Fix
				if err != nil || json.Unmarshal(data, &fix) != nil {
					t.Fatalf("fixture: %v", err)
				}
				calls := 0
				owners.work.git = func(_ string, args ...string) ([]byte, error) {
					switch args[0] {
					case "rev-list":
						return []byte("merge first second\n"), nil
					case "merge-tree":
						return []byte("automatic\n"), nil
					case "diff":
						for _, set := range fix.Generated {
							for _, path := range set.Paths {
								want := ":(exclude)" + filepath.ToSlash(filepath.Join(prefix, path))
								if !slices.Contains(args, want) {
									t.Fatalf("generated exclusion %q missing: %v", want, args)
								}
							}
						}
						if want := []string{"automatic", "first", "second"}[calls]; args[2] != want {
							t.Fatalf("diff base=%q want %q: %v", args[2], want, args)
						}
						for _, path := range fix.Paths {
							if slices.Contains(args, ":(literal)"+path.Path) != (calls > 0) {
								t.Fatalf("only fallbacks must use recorded conflicts: %v", args)
							}
						}
						calls++
						if calls == parent+1 {
							return []byte("resolution"), nil
						}
						return nil, nil
					default:
						t.Fatalf("unexpected git: %v", args)
						return nil, nil
					}
				}
				layout, err := owners.resolver.ResolveLayout(b.root())
				if err != nil {
					t.Fatal(err)
				}
				layout.InstallationRel = prefix
				inv := &intentInvocation{owners: owners, layout: layout}
				if diff, fallbackParent, err := inv.mergeResolutionDiff(layout.GitRoot, &fix); err != nil || fallbackParent != parent || string(diff) != "resolution" || calls != parent+1 {
					t.Fatalf("resolution=%s fallback=%d calls=%d error=%v", diff, fallbackParent, calls, err)
				}
			})
		}
	}
}
