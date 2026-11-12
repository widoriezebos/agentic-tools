package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestImpactWritesItsExecutedSelection(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"cheap", "static-only", "publication-only", "red", "stale-red", "unset", "cache", "settings", "workers", "ceiling", "base", "replay", "replay-plan", "plan", "write-error"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			root, err := filepath.EvalSymlinks(impactAdapterBed(t))
			helmMust(t, err)
			path := filepath.Join(root, "check", "selection.json")
			marker := filepath.Join(root, "groups-ran")
			data, err := os.ReadFile(filepath.Join(root, "testing.json"))
			helmMust(t, err)
			var contract testpolicy.Contract
			helmMust(t, json.Unmarshal(data, &contract))
			prerequisite := contract.Groups[2]
			prerequisite.ID = "refusal-register-standard"
			contract.Groups[2].Requires = []string{prerequisite.ID}
			contract.Groups = append(contract.Groups, prerequisite)
			for i := range contract.Groups {
				group := &contract.Groups[i]
				group.Argv = []string{"/bin/sh", "-c", `test -z "$METASYSTEM_CHECK_SELECTION" && test ! -e ` + shellCommand([]string{path}) + " && printf '%s\\n' " + shellCommand([]string{group.ID}) + " >> " + shellCommand([]string{marker})}
				if row == "write-error" && group.ID == "fast-static-build" {
					group.Argv[2] += "; printf blocked > " + shellCommand([]string{filepath.Join(root, "check")})
				}
				if (row == "red" || row == "stale-red") && group.ID == "canary" {
					group.Argv[2] += "; exit 1"
				}
			}
			data, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, root, "testing.json", string(data))
			// A direct local-file read disagrees with the injected effective setting.
			impactWrite(t, root, "metasystem.conf.local", "landing.impact-max-share=1\n")
			if row == "stale-red" {
				impactWrite(t, root, "check/selection.json", "stale selection")
			}
			if row == "cache" {
				_, err := repairCheckCache(root, repairCheck{Tree: "current-tree", Base: "resolved-base"}, true)
				helmMust(t, err)
			}
			environment := fixtureCommandEnvironment(t, "LANDING_PROOF_BASE=environment-base", "METASYSTEM_TESTING_WORKERS=2", proofrun.TestWorkersEnvironment+"=2", "METASYSTEM_CHECK_SELECTION="+path)
			workers := 2
			if row == "workers" || row == "ceiling" {
				environment = slices.DeleteFunc(environment, func(entry string) bool {
					return strings.HasPrefix(entry, "METASYSTEM_TESTING_WORKERS=") || strings.HasPrefix(entry, proofrun.TestWorkersEnvironment+"=")
				})
				environment = append(environment, "METASYSTEM_TESTING_WORKERS=7")
				workers = 7
				if row == "ceiling" {
					workers = 1
				}
				environment = append(environment, proofrun.TestWorkersEnvironment+fmt.Sprintf("=%d", workers))
			}
			if row == "unset" {
				environment = append(environment, "METASYSTEM_CHECK_SELECTION=")
			}
			limit, reads := "100", 0
			if row == "settings" {
				limit = "73"
			} else if row == "static-only" {
				limit = "1"
			}
			requestedBase := "explicit-base"
			sources := impactSources{
				environment: environment,
				getenv: func(name string) string {
					value := ""
					for _, entry := range environment {
						if found, ok := strings.CutPrefix(entry, name+"="); ok {
							value = found
						}
					}
					return value
				},
				get: func(p config.GetParams) (string, int, error) {
					if p.Key != "landing.impact-max-share" || p.ConfPath != filepath.Join(root, "metasystem.conf") {
						t.Fatalf("unexpected setting read: %+v", p)
					}
					reads++
					return limit, 0, nil
				},
				git: func(_ string, args ...string) (string, error) {
					switch strings.Join(args, " ") {
					case "rev-parse --verify " + requestedBase + "^{commit}":
						return "resolved-base", nil
					case "merge-base --is-ancestor resolved-base HEAD":
						return "", nil
					case "show -s --format=%s resolved-base":
						return "fixture base", nil
					case "rev-parse resolved-base^{tree}":
						return "base-tree", nil
					case "write-tree":
						return "current-tree", nil
					default:
						t.Fatalf("unexpected Git request: %v", args)
						return "", fmt.Errorf("unexpected Git request")
					}
				},
				impact: func(_ string, base string, _ bool) (goadapter.Impact, error) {
					if base != "resolved-base" || strings.HasPrefix(row, "replay") || row == "publication-only" {
						t.Fatalf("unexpected impact request for %s: %s", row, base)
					}
					return goadapter.Impact{Paths: []string{"internal/launch/value.go"}, NotRunnable: 3,
						Packages: []testpolicy.Group{{ID: "unit/internal/launch", Adapter: "go", Packages: []string{"./internal/launch"}, Tests: json.RawMessage(`"all"`)}}}, nil
				},
			}
			args := []string{"--root", root, "--base", requestedBase, "--check"}
			only := ""
			if row == "publication-only" {
				args = append(args, "--publication-only")
			} else if strings.HasPrefix(row, "replay") {
				only = "canary internal/launch=TestValue"
				if row == "replay-plan" {
					args = append(args, "--plan")
				}
			} else if row == "unset" {
				args = args[:len(args)-1]
			} else if row == "plan" {
				args = append(args, "--plan")
			}
			var out, problem bytes.Buffer
			code := sources.run(args, &out, &problem, only)
			if row == "red" || row == "stale-red" || row == "write-error" {
				if code != 1 {
					t.Fatalf("failed run exit=%d out=%s problem=%s", code, &out, &problem)
				}
				if row != "write-error" {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("failed run left selection: %v", err)
					}
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit=%d out=%s problem=%s", code, &out, &problem)
			}
			if row == "unset" || row == "plan" {
				helmMust(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
					if err == nil && entry.Name() == "selection.json" {
						t.Errorf("unexpected selection file: %s", path)
					}
					return err
				}))
				return
			}
			data, err = os.ReadFile(path)
			helmMust(t, err)
			var selection launch.CheckSelection
			helmMust(t, json.Unmarshal(data, &selection))
			wantGroups := []string{"refusal-register-standard", "fast-static-build", "canary", "unit/internal/launch"}
			wantTests, mode := map[string][]string{"internal/launch": {"TestValue"}}, "impact"
			if row == "static-only" || row == "publication-only" {
				wantGroups, wantTests, mode = wantGroups[:2], map[string][]string{}, row
			} else if strings.HasPrefix(row, "replay") {
				wantGroups = wantGroups[2:]
			}
			contractData, _ := json.Marshal(contract)
			if selection.Version != 1 || selection.VerdictPolicyVersion != 1 || selection.Mode != mode || !slices.Equal(selection.Groups, wantGroups) || !reflect.DeepEqual(selection.PackageTests, wantTests) || selection.ContractDigest != bytesSHA256(contractData) {
				t.Fatalf("selection=%+v want groups=%v tests=%v mode=%s", selection, wantGroups, wantTests, mode)
			}
			if strings.HasPrefix(row, "replay") {
				if selection.Base != "" || selection.BaseTree != "" || selection.BaseReason != "replay" {
					t.Fatalf("replay guessed a base: %+v", selection)
				}
			} else if selection.Base != "resolved-base" || selection.BaseTree != "base-tree" || selection.BaseReason != "" {
				t.Fatalf("comparison base: %+v", selection)
			}
			if selection.Workers != workers || selection.GOOS == "" || selection.GOARCH == "" || !strings.HasPrefix(selection.GoVersion, "go version ") || len(selection.EnvironmentSHA256) != 64 {
				t.Fatalf("context: %+v", selection)
			}
			executable, err := os.Executable()
			helmMust(t, err)
			engineDigest, err := fileSHA256(executable)
			helmMust(t, err)
			if selection.EngineDigest != engineDigest || (row != "publication-only" && !strings.HasPrefix(row, "replay") && (selection.Settings["landing.impact-max-share"] != limit || reads != 1)) {
				t.Fatalf("effective context: %+v, setting reads=%d", selection, reads)
			}
			for _, group := range contract.Groups {
				if slices.Contains(wantGroups, group.ID) {
					definition, _ := json.Marshal(group)
					if selection.GroupDigests[group.ID] != bytesSHA256(definition) {
						t.Fatalf("group %s definition digest missing or wrong: %+v", group.ID, selection)
					}
				}
			}
			if !strings.HasPrefix(row, "replay") {
				data, err = os.ReadFile(marker)
				helmMust(t, err)
				wantMarkers := slices.DeleteFunc(slices.Clone(wantGroups), func(id string) bool { return strings.HasPrefix(id, "unit/") })
				if !slices.Equal(strings.Fields(string(data)), wantMarkers) {
					t.Fatalf("executed groups=%s want=%v", data, wantMarkers)
				}
			}
			if !strings.HasPrefix(row, "replay") && row != "publication-only" && selection.NotRunnable != 3 {
				t.Fatalf("not runnable count: %+v", selection)
			}
			if _, err := os.Stat(filepath.Join(plain.Dir(root), "repair-check.json")); row != "cache" && err == nil {
				t.Fatal("evidence run unexpectedly wrote the repair cache")
			}
		})
	}
}
