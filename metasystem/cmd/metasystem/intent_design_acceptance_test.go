package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const acceptanceUnits = "\n## Units\n\n| Unit | Purpose | Production lines |\n| --- | --- | --- |\n| reader | Read the whole page | 40 |\n"

// The repository remains the real goal transaction's only transport seam.
type designAcceptanceRepository struct {
	goal.Repository
	before  func() error
	capture func() error
	after   func() (goal.CASOutcome, error)
}

func (r *designAcceptanceRepository) Capture(op string) (string, error) {
	if r.capture != nil {
		if err := r.capture(); err != nil {
			return "", err
		}
	}
	return r.Repository.Capture(op)
}

func (r *designAcceptanceRepository) Build(op, parent string, changes []goal.Change, message string) (string, error) {
	if r.before != nil {
		if err := r.before(); err != nil {
			return "", err
		}
	}
	return r.Repository.Build(op, parent, changes, message)
}

func (r *designAcceptanceRepository) Publish(parent, commit string) (goal.CASOutcome, error) {
	result, err := r.Repository.Publish(parent, commit)
	if err == nil && r.after != nil {
		return r.after()
	}
	return result, err
}

func acceptanceBuild(t *testing.T, b *designLoopBed, wantAccepted bool) {
	t.Helper()
	bed := newDesignGateBed(t, 2)
	bed.lineage = "builder"
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		original := owners.git
		owners.git = func(dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "rev-parse base-commit^{tree}" {
				return []byte(strings.Repeat("b", 40)), nil
			}
			if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf") {
				return []byte("proof.cheap=true\nproof.audits=true\nproof.deadline=15\n"), nil
			}
			return original(dir, args...)
		}
	}
	setDesignGateMode(t, bed, "refuse")
	shared := b.goalFile(bed.id)
	clone := bed.goalFile(bed.id)
	clone.DesignExits, clone.ReviewObligations = shared.DesignExits, shared.ReviewObligations
	bed.addGoal(clone)
	path, _ := designGatePage(t, bed, "")
	if err := os.WriteFile(path, mustRead(t, b.design), 0600); err != nil {
		t.Fatal(err)
	}
	_, listed := bed.runJSON(bed.owners(), "design", "list", "--goal", bed.id)
	list, err := json.Marshal(listed.Data)
	visible, _, _ := project.ParseRecord(path, string(mustRead(t, path)))
	if err != nil || !bytes.Contains(list, []byte(`"status":"`+visible.Status+`"`)) {
		t.Fatalf("status selector: %+v %v", listed, err)
	}
	brief := bed.brief("acceptance.md", "Build the reader.\n")
	code, result, _ := bed.work("work", "build", bed.id, "reader", "--brief", brief, "--lines", "40", "--read-tool-calls", "12")
	if wantAccepted {
		if code != 0 || !strings.Contains(strings.Join(bed.starter.launched(), ","), "build") {
			t.Fatalf("committed clone build: %d %+v", code, result)
		}
	} else if code == 0 || result.Outcome != intentRefused || len(bed.starter.launched()) != 0 {
		t.Fatalf("pending acceptance built: %d %+v", code, result)
	}
}

func TestDesignReviewAcceptanceKeepsUnresolvedHistoryPending(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"unresolved finding", "held policy", "wrong owner"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			b, _, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			template := b.decide(b.review(), nil)
			expected := mustRead(t, b.design)
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			switch cause {
			case "unresolved finding":
				b.register(1, 2, []int64{0}, map[string]any{"findingId": "earlier:F1"})
			case "held policy":
				owners.work = intentWorkOwners{units: func(stateroot.Layout) *launch.UnitRunner {
					return &launch.UnitRunner{Manager: &launch.Manager{}, ReviewPolicy: func() (string, error) { return "person", nil }}
				}}
			case "wrong owner":
				b.lineage = "another-session"
			}
			code, result := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template)
			if code == 0 || len(b.goalFile(bedGoal).DesignExits) != 0 || b.job("rev1")["chainClosed"] == true || !bytes.Equal(expected, mustRead(t, b.design)) {
				t.Fatalf("%s granted acceptance: %d %+v", cause, code, result)
			}
		})
	}
}

func TestDesignReviewPublishesAcceptance(t *testing.T) {
	t.Parallel()
	t.Run("person forms", testDesignReviewPersonActs)
	t.Run("concrete fold", testDesignReviewConcreteFoldPublishesExit)
	b, _, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	expected := mustRead(t, b.design)
	template := b.decide(b.review(), nil)
	result := b.review("--dispositions", template)
	if result.Outcome != intentConfirmed || b.closes != 1 {
		t.Fatalf("acceptance: %+v", result)
	}
	file := b.goalFile(bedGoal)
	if len(file.DesignExits) != 1 {
		t.Fatalf("shared acceptance missing: %+v", file)
	}
	exit := file.DesignExits[0]
	page := mustRead(t, b.design)
	if _, err := os.Stat(b.design + ".publication-lock"); !os.IsNotExist(err) {
		t.Fatalf("publication left a lock beside the design: %v", err)
	}
	locks, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "design-publication", "*.lock"))
	if err != nil || len(locks) != 1 {
		t.Fatalf("publication lock is missing from the state root: %v %v", locks, err)
	}
	before, _ := project.DesignBodyDigest(b.design, expected)
	if exit.State != "committed" || exit.Expected != string(expected) || exit.Page != string(page) || exit.BodySHA256 != before || exit.Root != "rev1" || exit.Round != 1 || len(exit.Units) != 1 || exit.Units[0] != "reader" || len(exit.Items) != 0 || exit.Dispositions != string(mustRead(t, template)) {
		t.Fatalf("acceptance payload: %+v", exit)
	}
	if !strings.Contains(string(page), "- Status: accepted\n") || !strings.Contains(string(page), "on 0 material findings folded as 0 unit acceptance items (convergence "+exit.Operation+")") {
		t.Fatalf("accepted head: %s", page)
	}
	acceptanceBuild(t, b, true)
	file.DesignExits[0].State = "prepared"
	b.addGoal(file)
	acceptanceBuild(t, b, false)
	file.DesignExits[0].State = "committed"
	b.addGoal(file)
	publications := b.publications()
	again := b.review("--dispositions", template)
	if again.Outcome != intentUnchanged || b.closes != 1 || b.publications() != publications || !bytes.Equal(page, mustRead(t, b.design)) {
		t.Fatalf("repeat duplicated acceptance: %+v", again)
	}
}

func TestDesignReviewAcceptanceRechecksCurrentGoal(t *testing.T) {
	t.Parallel()
	for _, handover := range []bool{false, true} {
		t.Run(fmt.Sprintf("handover=%t", handover), func(t *testing.T) {
			t.Parallel()
			b, _, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			template := b.decide(b.review(), nil)
			expected := mustRead(t, b.design)
			held := true
			b.work = intentWorkOwners{units: func(stateroot.Layout) *launch.UnitRunner {
				return &launch.UnitRunner{Manager: &launch.Manager{}, ReviewPolicy: func() (string, error) {
					if held {
						return "person", nil
					}
					return "auto", nil
				}}
			}}
			first := b.review("--dispositions", template)
			if first.Outcome != intentFailed || !strings.Contains(first.Summary, "prepared") || len(b.goalFile(bedGoal).DesignExits) != 0 || !bytes.Equal(expected, mustRead(t, b.design)) {
				t.Fatalf("held acceptance: %+v", first)
			}
			file := b.goalFile(bedGoal)
			file.Revision++
			if handover {
				file.Claimed.Lineage = "successor-session"
			}
			b.addGoal(file)
			held = false
			if handover {
				wrongOwner := b.review("--dispositions", template)
				if wrongOwner.Outcome != intentFailed || len(b.goalFile(bedGoal).DesignExits) != 0 || !bytes.Equal(expected, mustRead(t, b.design)) {
					t.Fatalf("old holder published acceptance: %+v", wrongOwner)
				}
				b.lineage = file.Claimed.Lineage
			}
			resumed := b.review("--dispositions", template)
			if resumed.Outcome != intentConfirmed || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
				t.Fatalf("current holder could not resume: %+v", resumed)
			}
			if exit := b.goalFile(bedGoal).DesignExits[0]; exit.Revision != file.Revision {
				t.Fatalf("acceptance recorded stale revision: %+v", exit)
			}
			if again := b.review("--dispositions", template); again.Outcome != intentUnchanged || b.closes != 1 {
				t.Fatalf("acceptance replay: %+v", again)
			}
		})
	}
}

func TestDesignReviewAcceptanceReleasesCompletedEntry(t *testing.T) {
	t.Parallel()
	b, dir, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	template := b.decide(b.review(), nil)
	if result := b.review("--dispositions", template); result.Outcome != intentConfirmed {
		t.Fatalf("acceptance: %+v", result)
	}
	entryPath := filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader", "chain.json")
	var entry designReviewEntry
	if err := json.Unmarshal(mustRead(t, entryPath), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Exit != nil {
		t.Fatal("completed acceptance still occupies the prepared entry")
	}
	page, publications, fresh := mustRead(t, b.design), b.publications(), b.fresh
	if err := os.Remove(filepath.Join(dir, "return.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(template); err != nil {
		t.Fatal(err)
	}
	if plain := b.review(); plain.Outcome != intentUnchanged || !strings.Contains(plain.Summary, "accepted") || b.closes != 1 || b.publications() != publications || b.fresh != fresh || !bytes.Equal(page, mustRead(t, b.design)) {
		t.Fatalf("plain review lost the committed acceptance: %+v", plain)
	}
	canonical, err := filepath.EvalSymlinks(b.design)
	if err != nil {
		t.Fatal(err)
	}
	b.writeJob(map[string]any{"jobId": "rev2", "role": "design-critic", "status": "running", "round": 1, "goalId": bedGoal, "design": canonical})
	if later := b.review(); later.Outcome != intentInProgress || !strings.Contains(later.Summary, "running") || b.fresh != fresh || b.publications() != publications {
		t.Fatalf("closed acceptance blocked the later chain: %+v", later)
	}
	// A lost cleanup write leaves the committed exit in the prepared slot.
	exit := b.goalFile(bedGoal).DesignExits[0]
	exit.State = "prepared"
	entry.Exit = &exit
	b.writeJSON(entryPath, entry)
	if later := b.review(); later.Outcome != intentInProgress || !strings.Contains(later.Summary, "running") || b.publications() != publications {
		t.Fatalf("stale completed entry blocked the later chain: %+v", later)
	}
	entry = designReviewEntry{}
	if err := json.Unmarshal(mustRead(t, entryPath), &entry); err != nil || entry.Exit != nil {
		t.Fatalf("stale completed entry was not reconciled: %+v %v", entry, err)
	}
}

func TestDesignReviewAcceptanceReplaysPersistenceFailures(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"prepared", "before commit", "after commit", "lost commit response", "after projection", "lost close response", "concurrent edit"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			b, dir, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			expected := mustRead(t, b.design)
			template := b.decide(b.review(), nil)
			owners := b.intentBed.owners()
			endpoint := owners.dependencies.endpoint
			observer := &designAcceptanceRepository{Repository: b.repo}
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				e, err := endpoint(root)
				e.Repository = observer
				return e, err
			}
			owners.delivery = b.owners
			var saved string
			observer.capture = func() error {
				if boundary == "prepared" {
					return errors.New("ledger unavailable after prepare")
				}
				return nil
			}
			observer.before = func() error {
				if strings.Contains(string(mustRead(t, b.design)), "Status: accepted") || b.job("rev1")["chainClosed"] == true {
					t.Fatal("acceptance or closure preceded goal commit")
				}
				if boundary == "before commit" {
					return errors.New("publication storage unavailable")
				}
				return nil
			}
			observer.after = func() (goal.CASOutcome, error) {
				if b.job("rev1")["chainClosed"] == true {
					t.Fatal("closure preceded page projection")
				}
				if boundary == "after commit" {
					saved = b.design + ".saved"
					if err := os.Rename(b.design, saved); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(b.design, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if boundary == "concurrent edit" {
					b.writeFile(b.design, string(expected)+"Concurrent Decision.\n")
				}
				if boundary == "lost commit response" {
					return goal.CASUnknown, errors.New("connection lost after server accepted the commit")
				}
				return goal.CASLanded, nil
			}
			close := b.owners.closeOwner
			b.owners.closeOwner = func(root string, args []string) intentProcessResult {
				page := mustRead(t, b.design)
				file := b.goalFile(bedGoal)
				if !strings.Contains(string(page), "- Status: accepted\n") || len(file.DesignExits) != 1 || file.DesignExits[0].Page != string(page) {
					t.Fatal("the close owner ran before the committed page was projected")
				}
				if boundary == "after projection" {
					return intentProcessResult{code: 1, stderr: []byte("close storage unavailable\n")}
				}
				result := close(root, args)
				if boundary == "lost close response" {
					return intentProcessResult{code: 1, stderr: []byte("close response lost\n")}
				}
				return result
			}
			code, first := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template)
			if boundary != "lost close response" && (code == 0 || first.Outcome == intentConfirmed) {
				t.Fatalf("fault reported acceptance: %+v", first)
			}
			if boundary == "prepared" || boundary == "before commit" {
				if len(b.goalFile(bedGoal).DesignExits) != 0 || !bytes.Equal(expected, mustRead(t, b.design)) {
					t.Fatal("failed preparation exposed acceptance")
				}
				acceptanceBuild(t, b, false)
			} else if boundary == "after projection" || boundary == "lost close response" {
				acceptanceBuild(t, b, true)
			} else if boundary == "lost commit response" {
				if !bytes.Equal(expected, mustRead(t, b.design)) {
					t.Fatal("uncertain commit projected acceptance")
				}
				acceptanceBuild(t, b, false)
			} else if boundary == "concurrent edit" {
				if !strings.Contains(string(mustRead(t, b.design)), "Concurrent Decision.") {
					t.Fatal("concurrent page edit was overwritten")
				}
				acceptanceBuild(t, b, false)
			}
			observer.before, observer.after, observer.capture = nil, nil, nil
			b.owners.closeOwner = close
			if saved != "" {
				if err := os.Remove(b.design); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(saved, b.design); err != nil {
					t.Fatal(err)
				}
			}
			if boundary == "concurrent edit" {
				b.writeFile(b.design, string(expected))
			}
			// Committed acceptance resumes even if advisory evidence is lost.
			if boundary != "prepared" && boundary != "before commit" {
				if err := os.Remove(filepath.Join(dir, "return.md")); err != nil {
					t.Fatal(err)
				}
			}
			code, resumed := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", template)
			if code != 0 || resumed.Outcome != intentConfirmed && resumed.Outcome != intentUnchanged || len(b.goalFile(bedGoal).DesignExits) != 1 || b.job("rev1")["chainClosed"] != true {
				t.Fatalf("same publication did not rejoin: %d %+v", code, resumed)
			}
			acceptanceBuild(t, b, true)
		})
	}
}
