package landing

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
)

// The path classes and landing classes compiled into the engine are not the
// landing's judge: the running engine need not be built at the landing's
// base. A landing reads both from its base tree, so a base whose classes
// differ from the engine's decides by its own.
func TestObservePolicyIsTheBaseTreesNotTheEngines(t *testing.T) {
	t.Parallel()
	f := newRepositoryObservationFixture(t)
	engine, err := pathclass.Load()
	if err != nil {
		t.Fatal(err)
	}
	if engine.Class("product.txt") != pathclass.Unclassified {
		t.Fatal("the fixture needs a path the engine's own classes leave unclassified")
	}
	f.base(pathclass.SourcePath, string(pathclass.Source())+"install:product.txt runtime\n")
	c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\n+carried\n", "product.txt")
	c.declare("product.txt", observationText("before\n"), observationText("before\ncarried\n"))
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
	// The engine's classes would refuse path-unclassified; the base's row
	// makes it a runtime path.
	if got.Code != "runtime-path-refused" {
		t.Fatalf("the base's runtime row did not decide: %+v", got)
	}
}

// A candidate that edits the compiled-in classes' source is judged by the
// base's classes: its own row does not classify its own product path, and
// the classes file is on the tier-1 floor.
func TestObserveCandidateClassEditIsJudgedByTheBase(t *testing.T) {
	t.Parallel()
	f := newRepositoryObservationFixture(t)
	before := string(f.baseFiles[pathclass.SourcePath])
	c := f.comparison(observeTreeB, "diff --git a/product.txt b/product.txt\n+changed\n", "product.txt", pathclass.SourcePath)
	c.declare("product.txt", observationText("before\n"), observationText("changed\n"))
	c.declare(pathclass.SourcePath, &before, observationText(before+"install:product.txt record\n"))
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
	if got.Bar != BarRefusal || got.Verdict != "would-refuse" {
		t.Fatalf("a candidate's own class edit decided its landing: %+v", got)
	}
}

// The landing that moves the policy into the engine source is judged by a
// base that still keeps it at the legacy path.
func TestObservePolicyFallsBackToTheLegacyBasePath(t *testing.T) {
	t.Parallel()
	f := newRepositoryObservationFixture(t)
	pathClasses := f.baseFiles[pathclass.SourcePath]
	landingClasses := f.baseFiles[LandingClassesSourcePath]
	if !bytes.Equal(landingClasses, landingClassesSource) {
		t.Fatal("the fixture's base landing classes are not the engine's")
	}
	delete(f.baseFiles, pathclass.SourcePath)
	delete(f.baseFiles, LandingClassesSourcePath)
	f.base(pathclass.LegacySourcePath, string(pathClasses))
	f.base(legacyLandingClassesSourcePath, string(landingClasses))
	c := f.comparison(observeTreeB, "diff --git a/memory/receipts.log b/memory/receipts.log\n+receipt=moved\n", "memory/receipts.log")
	c.declare("memory/receipts.log", observationText("receipt=existing\n"), observationText("receipt=existing\nreceipt=moved\n"))
	c.declare(pathclass.SourcePath, nil, nil)
	c.declare(LandingClassesSourcePath, nil, nil)
	got := c.observe(ObserveParams{RepoRoot: f.root, CandidateTree: c.candidate, DirectFix: "register-carriage"})
	if got.Bar != BarDirectFix || got.Code != "register-carriage" {
		t.Fatalf("a pre-move base could not judge the move: %+v", got)
	}
	if strings.Contains(got.Provenance, "unreadable") {
		t.Fatalf("provenance %q", got.Provenance)
	}
}
