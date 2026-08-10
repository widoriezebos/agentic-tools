package validate

import (
	"path/filepath"
	"testing"
)

func writeJob(t *testing.T, root, jobID, record string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "artifacts", "agents", "jobs", jobID+".json"), record)
}

func TestCodeCritiqueClaimAccepts(t *testing.T) {
	root := t.TempDir()
	writeJob(t, root, "imp-1", `{"jobId":"imp-1","role":"implementer","parentJob":null}`)
	writeJob(t, root, "crit-1", `{"jobId":"crit-1","role":"code-critic","parentJob":null,"reviews":"imp-1"}`)
	delegates := []string{"claude:strong:imp-1", "claude:strong:crit-1"}
	if !CodeCritiqueClaim(root, delegates) {
		t.Fatal("a top-level code-critic reviewing a delegate implementer must verify")
	}
}

func TestCodeCritiqueClaimRejects(t *testing.T) {
	root := t.TempDir()
	writeJob(t, root, "imp-1", `{"jobId":"imp-1","role":"implementer","parentJob":null}`)
	// The critic reviews a job that is not among the delegates.
	writeJob(t, root, "crit-1", `{"jobId":"crit-1","role":"code-critic","parentJob":null,"reviews":"imp-9"}`)
	// A nested critic (parentJob set) is not a top-level chain.
	writeJob(t, root, "crit-2", `{"jobId":"crit-2","role":"code-critic","parentJob":"imp-1","reviews":"imp-1"}`)
	delegates := []string{"claude:strong:imp-1", "claude:strong:crit-1", "claude:strong:crit-2"}
	if CodeCritiqueClaim(root, delegates) {
		t.Fatal("no top-level critic names a delegate implementer; the claim must be refused")
	}
	if CodeCritiqueClaim(root, nil) {
		t.Fatal("an empty delegate list must be refused")
	}
}

func TestWaiverFactsResolvesClassAndStream(t *testing.T) {
	root := t.TempDir()
	writeJob(t, root, "imp-1",
		`{"jobId":"imp-1","role":"implementer","parentJob":"chain-root","critiqueWaived":{"class":"mechanical"}}`)
	writeJob(t, root, "chain-root", `{"jobId":"chain-root","role":"orchestrator","parentJob":null}`)
	writeFile(t, filepath.Join(root, "artifacts", "agents", "chain-root", "brief.md"),
		"# Brief\nMission Stream: stream-a\n")
	class, stream := WaiverFacts(root, []string{"claude:strong:imp-1"})
	if class != "mechanical" || stream != "stream-a" {
		t.Fatalf("facts = %s/%s, want mechanical/stream-a", class, stream)
	}
}

func TestWaiverFactsDefaults(t *testing.T) {
	root := t.TempDir()
	writeJob(t, root, "imp-1", `{"jobId":"imp-1","role":"implementer","parentJob":null}`)
	class, stream := WaiverFacts(root, []string{"claude:strong:imp-1"})
	if class != "none" || stream != "none" {
		t.Fatalf("facts = %s/%s, want none/none for an unwaived delegate", class, stream)
	}

	// A waiver whose chain root has no brief resolves to the standalone stream.
	writeJob(t, root, "imp-2",
		`{"jobId":"imp-2","role":"implementer","parentJob":null,"critiqueWaived":{"class":"trivial"}}`)
	class, stream = WaiverFacts(root, []string{"claude:strong:imp-2"})
	if class != "trivial" || stream != "standalone" {
		t.Fatalf("facts = %s/%s, want trivial/standalone", class, stream)
	}
}
