package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

func TestDesignSizeLegacyHeadersBuild(t *testing.T) {
	t.Parallel()
	for _, header := range []string{"Changed lines", "Estimated changed lines", "Total changed lines", "Lines production/test", "Production lines"} {
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			for _, person := range []bool{false, true} {
				bed := sizeBed(t)
				if person {
					bed.lineage = ""
				}
				table := "| Unit | " + header + " |\n|---|---|\n| u0 | 200 |\n"
				if header != "Production lines" {
					table = "| Unit | " + header + " | Production lines |\n|---|---|---|\n| u0 | 900 | 200 |\n"
				}
				sizePage(t, bed, "main", "accepted", table)
				code, result, output := sizeBuild(t, bed, "u0")
				if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
					t.Fatalf("header=%s person=%t: code=%d result=%+v output=%s", header, person, code, result, output)
				}
				identity, err := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
				if err != nil {
					t.Fatal(err)
				}
				_, record := designGateRead(t, bed, identity, "u0")
				unit := record.Designs[0].Size.Designs[0].Units[0]
				wantLines := int64(900)
				if header == "Production lines" {
					wantLines = 200
				}
				if record.Verdict != "ok" || unit.Lines != wantLines || unit.Production == nil || *unit.Production != 200 {
					t.Fatalf("header=%s person=%t record=%+v unit=%+v", header, person, record, unit)
				}
			}
		})
	}
}

func TestDesignSizeRepeatedEstimatesBuild(t *testing.T) {
	t.Parallel()
	// The fleet design lists production-only units, then repeats them with test allowances.
	const table = `## Units

| Unit | Intent | Production lines | Areas |
| --- | --- | ---: | --- |
| R1 | Continuing authorization | 240 | internal/steward/ |
| R2 | Person revival | 250 | cmd/metasystem/ |
| R3 | Recovery request | 250 | internal/channel/ |
| R4 | Timestamped usage | 250 | internal/launch/ |
| **Total** | **Four units** | **990** | |

## Estimates

| Unit | Build/read minutes | Focused check minutes included | Production lines | Test-line allowance | Total changed lines |
| --- | ---: | ---: | ---: | ---: | ---: |
| R1 | 85 | 8 | 240 | 300 | 540 |
| R2 | 95 | 9 | 250 | 320 | 570 |
| R3 | 95 | 10 | 250 | 320 | 570 |
| R4 | 85 | 9 | 250 | 300 | 550 |
| **Total** | **360** | **36** | **990** | **1,240** | **2,230** |
`
	for _, summary := range []string{"**Total**", "sum"} {
		t.Run(summary, func(t *testing.T) {
			t.Parallel()
			bed := sizeBed(t)
			path := sizePage(t, bed, "fleet", "accepted", strings.ReplaceAll(table, "**Total**", summary))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			code, result, output := sizeBuild(t, bed, "R1")
			if code != 0 || !slices.Contains(bed.starter.launched(), "build") {
				t.Fatalf("code=%d result=%+v output=%s", code, result, output)
			}
			identity, err := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
			if err != nil {
				t.Fatal(err)
			}
			_, record := designGateRead(t, bed, identity, "R1")
			if record.Verdict != "ok" || record.Designs[0].Size.Count != 4 {
				t.Fatalf("repeated estimates counted as more than four units: %+v", record)
			}
			after, err := os.ReadFile(path)
			if err != nil || !slices.Equal(before, after) {
				t.Fatalf("admission changed the design: %v", err)
			}
		})
	}
}

func TestDesignSizeMainAcceptanceHistoryBuild(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"accepted before declaration merge", "accepted before unlanded declaration", "accepted after declaration", "draft before declaration", "different identity before declaration"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := sizeBed(t)
			path := sizePage(t, bed, "main", "accepted", sizeTable("u", 1, "900", "900"))
			accepted, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			page := relativeOrSame(bed.root(), path)
			conf := relativeOrSame(bed.root(), filepath.Join(bed.stateRoot(), "metasystem.conf"))
			wantExempt := strings.HasPrefix(scenario, "accepted before")
			bed.workOwnersHook = func(o *intentWorkOwners) {
				fallback := o.git
				o.git = func(root string, args ...string) ([]byte, error) {
					switch args[0] {
					case "log":
						if args[len(args)-1] == conf {
							// The branch declared the keys before merging main's accepted page.
							if !slices.Contains(args, "origin/main") {
								return []byte("branch-declaration\n"), nil
							}
							if !slices.Contains(args, "--first-parent") || !slices.Contains(args, "--diff-merges=first-parent") {
								t.Fatalf("main's declaration merge must be observed: %v", args)
							}
							if scenario == "accepted before unlanded declaration" {
								return nil, nil
							}
							return []byte("main-declaration\n"), nil
						}
						boundary := "main-declaration^"
						if scenario == "accepted before unlanded declaration" {
							boundary = "origin/main"
						}
						if args[len(args)-1] != page || !slices.Contains(args, boundary) {
							t.Fatalf("acceptance must precede main's declaration: %v", args)
						}
						if scenario == "accepted after declaration" {
							return nil, nil
						}
						return []byte("main-page\n"), nil
					case "show":
						if args[1] == "branch-declaration:"+page || args[1] == "main-declaration:"+page {
							return nil, errors.New("path does not exist in declaration tree")
						}
						if args[1] != "main-page:"+page {
							t.Fatalf("unexpected historical page: %v", args)
						}
						if scenario == "draft before declaration" {
							return []byte(strings.ReplaceAll(string(accepted), "Status: accepted", "Status: draft")), nil
						}
						if scenario == "different identity before declaration" {
							return []byte(strings.ReplaceAll(string(accepted), "Id: main", "Id: earlier-page")), nil
						}
						return accepted, nil
					case "ls-tree":
						return nil, nil // The page is absent from the branch declaration tree.
					}
					return fallback(root, args...)
				}
			}
			code, result, output := sizeBuild(t, bed, "u0")
			gate := resultData(t, result)["designGate"].(map[string]any)
			if wantExempt {
				if code != 0 || gate["verdict"] != "ok" || !slices.Contains(bed.starter.launched(), "build") {
					t.Fatalf("main's earlier acceptance lost its exemption: code=%d result=%+v output=%s", code, result, output)
				}
				identity, err := bed.designGate.identity(stateroottest.Installation(t, bed.stateRoot()))
				if err != nil {
					t.Fatal(err)
				}
				_, record := designGateRead(t, bed, identity, "u0")
				if !record.Designs[0].Size.Designs[0].SizeExempt || record.Designs[0].Size.Count != 0 {
					t.Fatalf("exemption not retained: %+v", record)
				}
			} else if code != 1 || gate["verdict"] != "design-size-invalid" || len(bed.starter.launched()) != 0 {
				t.Fatalf("later acceptance incorrectly exempted: code=%d result=%+v output=%s", code, result, output)
			}
		})
	}
}
