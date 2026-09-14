package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stagedRepo carries the role and permissions files staging digests.
func stagedRepo(t *testing.T) string {
	root := gitRepoWithCurrentGoal(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/agents/roles/steward-continuation.md", "# Role: steward-continuation\ncontract\n")
	write("scripts/agents/roles/steward-continuation.requirements.json", `{"required":[]}`)
	write("scripts/agents/schemas/steward-continuation.schema.json", `{"type":"object"}`)
	write("scripts/agents/permissions/workspace.json", `{"write":["workspace"]}`)
	top, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	idPath := RepoIdentityPath(top)
	if err := os.MkdirAll(filepath.Dir(idPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(idPath, InstallIdentity{RepoIdentity: top, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-08-20T15:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStagingBindsTheBytesThatWillRun(t *testing.T) {
	root := stagedRepo(t)
	it, err := StageIntent(root, "st-1", "fix-it", "job-9", "fake", "fixture", "worker provably dead")
	if err != nil {
		t.Fatal(err)
	}
	if it.Role != "steward-continuation" || it.Permissions != "workspace" {
		t.Fatalf("the role and preset are fixed, never chosen: %+v", it)
	}
	if err := VerifyStagedDigests(root, it); err != nil {
		t.Fatalf("unchanged bytes must verify: %v", err)
	}
	brief, err := os.ReadFile(BriefPath(root, "st-1"))
	if err != nil || !strings.Contains(string(brief), `"fix-it"`) {
		t.Fatalf("the brief names the goal: %q %v", brief, err)
	}
}

func TestDriftBetweenMintAndLaunchRefusesByField(t *testing.T) {
	root := stagedRepo(t)
	it, err := StageIntent(root, "st-2", "fix-it", "job-9", "fake", "fixture", "dead")
	if err != nil {
		t.Fatal(err)
	}
	rolePath := filepath.Join(root, "scripts", "agents", "roles", "steward-continuation.md")
	if err := os.WriteFile(rolePath, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStagedDigests(root, it); err == nil || !strings.Contains(err.Error(), "role contract drifted") {
		t.Fatalf("role drift must refuse by field: %v", err)
	}
}

func TestHandoffStagesTheIntentWithThePredecessor(t *testing.T) {
	root := stagedRepo(t)
	const nonce = "1000000000000001"
	fixture := writeStagedHandoffFixture(t, root, nonce)
	it, err := StageHandoffIntent(root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", fixture.binding)
	if err != nil {
		t.Fatal(err)
	}
	if it.Reason != seatHandoffReason || it.Handoff == nil || it.Handoff.StateDigest != fixture.binding.StateDigest {
		t.Fatalf("handoff intent did not bind the state: %+v", it)
	}
	if it.Handoff.Predecessor.Pid != 4242 || it.Handoff.PredecessorTag != "tag-1" ||
		it.Handoff.PredecessorJob != "job-9" || it.Handoff.Session != fixture.binding.Session ||
		it.Handoff.Runtime != "claude" || it.Handoff.RecordedAt != fixture.binding.RecordedAt {
		t.Fatalf("handoff intent lost predecessor or session facts: %+v", it.Handoff)
	}
	if err := VerifyStagedDigests(root, it); err != nil {
		t.Fatalf("unchanged handoff staging must verify: %v", err)
	}
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 || live[0].Handoff == nil || live[0].Handoff.Predecessor != fixture.binding.Predecessor ||
		!live[0].Handoff.RecordedAt.Equal(fixture.binding.RecordedAt) {
		t.Fatalf("persisted intent lost its handoff binding: %+v, %v", live, err)
	}
	brief, err := os.ReadFile(BriefPath(root, nonce))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{nonce, "fix-it", fixture.binding.StateDigest, "metasystem context verify --root", "bounded views", "goal ledger and the job records"} {
		if !strings.Contains(string(brief), required) {
			t.Fatalf("handoff brief omits %q:\n%s", required, brief)
		}
	}
}

func TestVerifyStagedDigestsRejectsAHandoffWithoutBinding(t *testing.T) {
	root := stagedRepo(t)
	it, err := StageIntent(root, "missing-binding", "fix-it", "job-9", "fake", "fixture", seatHandoffReason)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyStagedDigests(root, it); err == nil || !strings.Contains(err.Error(), "carries no handoff binding") {
		t.Fatalf("seatHandoff without its binding must be malformed: %v", err)
	}
}

func TestVerifyStagedDigestsRefusesADriftedStateFile(t *testing.T) {
	t.Run("valid JSON state content", func(t *testing.T) {
		root := stagedRepo(t)
		const nonce = "2000000000000000"
		fixture := writeStagedHandoffFixture(t, root, nonce)
		it, err := StageHandoffIntent(root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", fixture.binding)
		if err != nil {
			t.Fatal(err)
		}
		rewriteState(t, fixture, func(object map[string]any) {
			nextStep := object["nextStep"].(map[string]any)
			nextStep["text"] = "continue the amended B1 proof"
		})
		data, err := os.ReadFile(fixture.binding.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		want := "handoff state digest mismatch expected=" + fixture.binding.StateDigest + " found=" + testDigest(data)
		if err := VerifyStagedDigests(root, it); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("valid JSON state tampering must refuse with %q: %v", want, err)
		}
	})

	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture stagedHandoffFixture)
		want   string
	}{
		{
			name: "state",
			mutate: func(t *testing.T, fixture stagedHandoffFixture) {
				handle, err := os.OpenFile(fixture.binding.StatePath, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := handle.WriteString("x"); err != nil {
					handle.Close()
					t.Fatal(err)
				}
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			},
			want: "handoff state drifted since the authorization was minted",
		},
		{
			name: "manifest",
			mutate: func(t *testing.T, fixture stagedHandoffFixture) {
				handle, err := os.OpenFile(fixture.manifest, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := handle.WriteString("x"); err != nil {
					handle.Close()
					t.Fatal(err)
				}
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			},
			want: "handoff state drifted since the authorization was minted",
		},
		{
			name: "required staged copy",
			mutate: func(t *testing.T, fixture stagedHandoffFixture) {
				handle, err := os.OpenFile(fixture.stagedSource, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := handle.WriteString("x"); err != nil {
					handle.Close()
					t.Fatal(err)
				}
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			},
			want: "handoff state drifted since the authorization was minted",
		},
		{
			name: "live source changes are outside the immutable capture",
			mutate: func(t *testing.T, fixture stagedHandoffFixture) {
				if err := os.WriteFile(fixture.liveSource, []byte("{\"status\":\"done\"}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := stagedRepo(t)
			nonce := "200000000000000" + string(rune('1'+i))
			fixture := writeStagedHandoffFixture(t, root, nonce)
			it, err := StageHandoffIntent(root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", fixture.binding)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, fixture)
			err = VerifyStagedDigests(root, it)
			if test.want == "" {
				if err != nil {
					t.Fatalf("live source drift must not alter the staged snapshot: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("tampering must refuse with %q: %v", test.want, err)
			}
		})
	}
}
