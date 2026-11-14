package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func impactCacheBed(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(impactAdapterBed(t))
	helmMust(t, err)
	var contract testpolicy.Contract
	helmMust(t, json.Unmarshal(mustRead(t, filepath.Join(root, "testing.json")), &contract))
	for i := range contract.Groups {
		contract.Groups[i].Argv = []string{"/bin/sh", "-c", "printf '%s\\n' " + contract.Groups[i].ID + " >> groups-ran"}
	}
	data, err := json.Marshal(contract)
	helmMust(t, err)
	impactWrite(t, root, "testing.json", string(data))
	return root
}

func impactCacheSources(t *testing.T, root string, environment []string, path string) impactSources {
	t.Helper()
	return impactSources{environment: environment,
		getenv: func(key string) string {
			if key == "METASYSTEM_CHECK_SELECTION" {
				return path
			}
			for _, entry := range environment {
				if value, ok := strings.CutPrefix(entry, key+"="); ok {
					return value
				}
			}
			return ""
		},
		get: func(config.GetParams) (string, int, error) { return "100", 0, nil },
		git: func(dir string, args ...string) (string, error) {
			if dir != root {
				t.Fatalf("unexpected Git directory %s", dir)
			}
			switch strings.Join(args, " ") {
			case "rev-parse --verify base^{commit}":
				return "base", nil
			case "rev-parse base^{tree}":
				return "base-tree", nil
			case "merge-base --is-ancestor base HEAD":
				return "", nil
			case "show -s --format=%s base":
				return "base subject", nil
			case "write-tree":
				return "candidate-tree", nil
			}
			t.Fatalf("unexpected Git request: %v", args)
			return "", fmt.Errorf("unexpected Git request")
		},
		impact: func(string, string, bool) (goadapter.Impact, error) { return goadapter.Impact{}, nil },
	}
}

func runImpactCache(t *testing.T, root string, sources impactSources) string {
	t.Helper()
	var out, problem bytes.Buffer
	if code := sources.run([]string{"--root", root, "--base", "base", "--check"}, &out, &problem, ""); code != 0 {
		t.Fatalf("impact: %d %s %s", code, &out, &problem)
	}
	return out.String()
}

func TestLaneFixCacheIsCheapOnly(t *testing.T) {
	t.Parallel()
	root := impactCacheBed(t)
	environment := fixtureCommandEnvironment(t, "METASYSTEM_TESTING_WORKERS=2")
	sources := impactCacheSources(t, root, environment, "")
	runImpactCache(t, root, sources)
	cachePath := filepath.Join(plain.Dir(root), "repair-check.json")
	before := mustRead(t, cachePath)
	var cached repairCheck
	helmMust(t, json.Unmarshal(before, &cached))
	if cached.Evidence == nil || !cached.Evidence.Strength.Complete || !cached.Evidence.Strength.Cheap || cached.Evidence.Strength.Audited || !strings.HasPrefix(cached.Evidence.Source, "test impact ") || cached.Evidence.Selection.BaseTree != "base-tree" {
		t.Fatalf("cache is not complete cheap evidence: %+v", cached)
	}
	markers := mustRead(t, filepath.Join(root, "groups-ran"))
	if output := runImpactCache(t, root, sources); !strings.Contains(output, "check reused: "+cached.Evidence.Source) || !bytes.Equal(markers, mustRead(t, filepath.Join(root, "groups-ran"))) || !bytes.Equal(before, mustRead(t, cachePath)) {
		t.Fatalf("lane fix did not reuse its cache unchanged: %s", output)
	}
	decision := launch.ApplicableCheck(cached.Evidence, cachePath, nil, launch.CheckNeed{Tree: cached.Tree, Base: cached.Base, Strength: "audited", Context: cached.Evidence.Selection})
	if decision.Reuse != nil || decision.Field != "strength" {
		t.Fatalf("publication accepted cheap cache: %+v", decision)
	}
	proveCheapCacheForPublication(t, cached)
	sources.get = func(config.GetParams) (string, int, error) { return "99", 0, nil }
	if output := runImpactCache(t, root, sources); !strings.Contains(output, "settings differs") || bytes.Equal(markers, mustRead(t, filepath.Join(root, "groups-ran"))) {
		t.Fatalf("changed setting reused cheap cache: %s", output)
	}
}

func proveCheapCacheForPublication(t *testing.T, cached repairCheck) {
	t.Helper()
	b, _, run, runner := unitCarryIntentBed(t, false)
	b.manager.Supervisor = publicationIntentStarter{b.starter}
	runner.Git = publicationIntentGit{runner.Git, "", cached.Tree}
	runner.PublicationCheck = func(string, string, launch.UnitCheck, launch.CheckSelection) (launch.CheckSelection, []string, error) {
		return cached.Evidence.Selection, []string{"fast-static-build"}, nil
	}
	var mismatch string
	runner.CheckMismatch = func(message string) { mismatch = message }
	launches := len(b.starter.launched())
	err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		planPath := filepath.Join(review.Round.Directory, "plan.json")
		plan, err := launch.ReadUnitPlan(planPath)
		if err != nil {
			return err
		}
		plan.Check = &launch.UnitCheck{Directory: b.worktree}
		transferWriteJSON(t, planPath, plan)
		transferWriteJSON(t, filepath.Join(review.Round.Directory, "check-repair", "evidence.json"), cached.Evidence)
		subject := *review.Subject
		_, err = runner.ProveRoundResult(review, b.worktree, cached.Base, &subject, retain)
		if subject.GateReuse != nil {
			t.Fatalf("publication reused the lane-fix cache: %+v", subject.GateReuse)
		}
		return err
	})
	if err != nil || !strings.Contains(mismatch, "strength differs") || len(b.starter.launched()) != launches+1 {
		t.Fatalf("publication did not rerun cheap cache: %v %s", err, mismatch)
	}
}

func TestWholeCheckEvidenceIgnoresTheCheapCache(t *testing.T) {
	t.Parallel()
	if root := os.Getenv("METASYSTEM_IMPACT_CACHE_CHILD"); root != "" {
		path := os.Getenv("METASYSTEM_CHECK_SELECTION")
		sources := impactCacheSources(t, root, os.Environ(), "")
		runImpactCache(t, root, sources)
		cachePath := filepath.Join(plain.Dir(root), "repair-check.json")
		cache, markers := mustRead(t, cachePath), mustRead(t, filepath.Join(root, "groups-ran"))
		// The same environment and selection match this cache if it is read.
		var retained repairCheck
		helmMust(t, json.Unmarshal(cache, &retained))
		if decision, _ := repairCheckCache(root, retained, false); decision.Reuse == nil {
			t.Fatalf("fixture cache does not match: %+v", decision)
		}
		sources.getenv = func(key string) string {
			if key == "METASYSTEM_CHECK_SELECTION" {
				return path
			}
			return os.Getenv(key)
		}
		runImpactCache(t, root, sources)
		if !bytes.Equal(cache, mustRead(t, cachePath)) || !bytes.Equal(append(slices.Clone(markers), markers...), mustRead(t, filepath.Join(root, "groups-ran"))) {
			t.Fatal("whole check skipped groups or overwrote the cheap cache")
		}
		return
	}
	root := impactCacheBed(t)
	cheap := shellCommand([]string{os.Args[0], "-test.run=^TestWholeCheckEvidenceIgnoresTheCheapCache$"})
	check := launch.UnitCheck{Base: "base", Directory: root, Cheap: cheap, Audits: "printf audited", Minutes: 15,
		Environment: fixtureCommandEnvironment(t, "METASYSTEM_IMPACT_CACHE_CHILD="+root, "METASYSTEM_TESTING_WORKERS=2")}
	check.Declaration = &launch.UnitDeclaration{Values: [4]string{cheap, check.Audits, "15", "full"}, SourceSHA256: "declaration"}
	execution, exits, err := check.Run(root, t.TempDir(), launch.CheckContext{Worktree: root,
		Tree: func() (string, error) { return "candidate-tree", nil },
		Now:  func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("whole check: %v %+v", err, exits)
	}
	var evidence launch.CheckEvidence
	helmMust(t, json.Unmarshal(mustRead(t, filepath.Join(execution, "evidence.json")), &evidence))
	if !evidence.Strength.Complete || !evidence.Strength.Audited || evidence.Source != filepath.Base(execution) || !slices.Contains(evidence.Selection.Groups, "fast-static-build") {
		t.Fatalf("whole check did not own complete evidence: %+v", evidence)
	}
}
