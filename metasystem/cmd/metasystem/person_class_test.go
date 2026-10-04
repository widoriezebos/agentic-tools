package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// W9: a person-act check that decides by class sees the main session a live
// general power of attorney admits as a person; without a grant, or without
// a clock, the classifier's answer stands.
func TestPersonVerbCallerAdmitsTheGrantee(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)
	main := func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: lease.ClassMain, MainId: "main-1"}, nil
	}
	clock := func(string) (time.Time, error) { return now, nil }
	grant := func(string, int64, time.Time) (humanauthority.HelmGrant, bool) {
		return humanauthority.HelmGrant{By: "wido", Grant: "01M-grant"}, true
	}
	if got, err := personVerbCallerWith("/seat", 80, main, clock, grant); err != nil || got.Class != lease.ClassHuman || got.MainId != "main-1" {
		t.Fatalf("under a grant = %+v %v", got, err)
	}
	for label, c := range map[string]struct {
		clock func(string) (time.Time, error)
		admit func(string, int64, time.Time) (humanauthority.HelmGrant, bool)
	}{
		"no grant": {clock, func(string, int64, time.Time) (humanauthority.HelmGrant, bool) {
			return humanauthority.HelmGrant{}, false
		}},
		"helm only": {clock, func(string, int64, time.Time) (humanauthority.HelmGrant, bool) {
			return humanauthority.HelmGrant{By: "wido"}, true
		}},
		"no seam":    {clock, nil},
		"no clock":   {func(string) (time.Time, error) { return time.Time{}, errors.New("unreadable") }, grant},
		"zero clock": {func(string) (time.Time, error) { return time.Time{}, nil }, grant},
	} {
		if got, err := personVerbCallerWith("/seat", 80, main, c.clock, c.admit); err != nil || got.Class != lease.ClassMain {
			t.Errorf("%s = %+v %v", label, got, err)
		}
	}
}

// personActWiring are the person-act checks that decide by class, each wired
// to the grant-aware classifier (unit 2 of general-power-of-attorney.md).
var personActWiring = map[string]string{
	"cmd/metasystem/process_verbs.go#defaultProcessOwners":               "personClassifyAt",
	"cmd/metasystem/process_verbs.go#requireHumanTerminal":               "personClassifyAt",
	"cmd/metasystem/intent_owner_calls.go#missionLaunchTo":               "personClassifyAt",
	"cmd/metasystem/steward_verbs.go#runStewardArmWith":                  "personClassifyAt",
	"cmd/metasystem/brain.go#defaultBrainActDependencies":                "personVerbCaller",
	"cmd/metasystem/proof_run.go#legacyProofLaunchAllowed":               "personVerbCaller",
	"cmd/metasystem/session_stop.go#":                                    "personClassify",
	"internal/missionrunner/resolve.go#ResolveTaint":                     "ClassifyPersonAt",
	"cmd/metasystem/session_stop.go#authorizeSessionStop":                "proveSessionStopAttorney",
	"cmd/metasystem/intent_goals.go#actorArgs":                           "attorneyActor",
	"cmd/metasystem/intent_work.go#runIntentSettingsSet":                 "directPersonProof",
	"cmd/metasystem/intent_roster.go#runIntentRosterSet":                 "directPersonProof",
	"cmd/metasystem/intent_planning.go#runIntentGrant":                   "runIntentGrantEverything",
	"cmd/metasystem/intent_grant_everything.go#runIntentGrantEverything": "EnrolledTerminalFor",
}

// classHumanSites are every production function outside internal/lease that
// compares a class with lease.ClassHuman: the person-act checks above, which
// read the grant-aware classifier, and the sites left alone on purpose (the
// holder already passes there, or the class names who acts, not whether a
// person may).
var classHumanSites = []string{
	"cmd/metasystem/brain.go#brainHumanAct",
	"cmd/metasystem/context_verbs.go#runContextHandoffWithInputs",
	"cmd/metasystem/delegate_host.go#BreachStopOrderingHuman",
	"cmd/metasystem/dispatch_verbs.go#breachStopOrderingHumanWith",
	"cmd/metasystem/goalsync_mutations.go#brainHumanWordClassificationWithFacts",
	"cmd/metasystem/goalsync_mutations.go#mainSplitRatification",
	"cmd/metasystem/goalsync_mutations.go#runGoalDischargeReviewObligationWithDependencies",
	"cmd/metasystem/goalsync_mutations.go#syncReqClassifiedWithTerminalGradeAtWithDependencies",
	"cmd/metasystem/person_class.go#personVerbCallerWith",
	"cmd/metasystem/process_verbs.go#humanTerminalCheck",
	"cmd/metasystem/process_verbs.go#missionFenceBeforeArmFor",
	"cmd/metasystem/proof_run.go#admitProofLaunchWithReadsAndClassifier",
	"cmd/metasystem/proof_run.go#legacyProofLaunchAllowed",
	"cmd/metasystem/session_stop.go#authorizeSessionStop",
	"cmd/metasystem/steward_verbs.go#sessionCallerCheck",
	"internal/delegation/admission.go#breachStop",
	"internal/missionrunner/resolve.go#ResolveTaint",
	"internal/steward/handoff_capture.go#",
	"internal/steward/handoff_capture.go#handoffCancelReason",
	"internal/ui/act/act.go#Fixture",
	"internal/ui/act/act.go#Prove",
	"internal/ui/act/act.go#SignedIn",
	"internal/ui/act/act.go#assemble",
}

// W10: every person-act check that decides by class reads the grant-aware
// classifier, and no new production comparison with lease.ClassHuman appears
// without being named here: a new person-act site is either wired to the
// grant or listed as one the grant leaves alone.
func TestAuditPersonActSitesReadTheGrant(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	var sites []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(module, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(module, path)
			rel = filepath.ToSlash(rel)
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, source, 0)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				name := ""
				if fn, ok := decl.(*ast.FuncDecl); ok {
					name = fn.Name.Name
				}
				key := rel + "#" + name
				text := string(source[fset.Position(decl.Pos()).Offset:fset.Position(decl.End()).Offset])
				bodies[key] += text
				if !strings.HasPrefix(rel, "internal/lease/") && strings.Contains(text, "ClassHuman") && !strings.HasPrefix(strings.TrimSpace(text), "const") &&
					(strings.Contains(text, "== lease.ClassHuman") || strings.Contains(text, "!= lease.ClassHuman") || strings.Contains(text, "case lease.ClassHuman") ||
						strings.Contains(text, "Class == handoffClassHuman") || strings.Contains(text, "Class != handoffClassHuman") || strings.Contains(text, "handoffClassHuman    =")) {
					sites = append(sites, key)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for key, want := range personActWiring {
		if !strings.Contains(bodies[key], want) {
			t.Errorf("%s does not read %s: a person-act check there no longer sees the power of attorney", key, want)
		}
	}
	sort.Strings(sites)
	sites = compactStrings(sites)
	known := map[string]bool{}
	for _, site := range classHumanSites {
		known[site] = true
	}
	for _, site := range sites {
		if !known[site] {
			t.Errorf("%s compares a class with lease.ClassHuman: wire it through lease.ClassifyPersonAt (personClassifyAt) or name it in classHumanSites as a site the grant leaves alone", site)
		}
	}
}

func compactStrings(values []string) []string {
	var out []string
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
