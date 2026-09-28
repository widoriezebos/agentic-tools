package delegation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// L2813-2936: the final inline-input guard after a successful composition.
// The retired legs reached it by answering the cap differently to the shell
// once composition had run (a METASYSTEM_BIN forwarder around the engine's
// compose-role-packet and config get); composition is now in-process, so
// the test lowers the configured cap between the two steps. A packet that
// composed under the cap and exceeds it at the final check exits 1 with the
// exact brief or message hint, is not confused with the composition-time
// REFUSED-INLINE-INPUT-LIMIT refusal, and the trap removes every
// composition temporary; no record, payload or round is written, because
// both dispatch and follow-up run the guard before their claim.
func TestP4FinalInlineCapGuardFollowsASuccessfulComposition(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ hint, prefix string }{{"brief", ""}, {"message", "follow-"}} {
		s, stderr := internalSession(t, Ports{Clock: &stubClock{now: time.Unix(1790000000, 0)}})
		s.env.RecordOutcome = true
		module, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		for _, relative := range []string{"scripts/agents/role-packets.json", "scripts/agents/roles/verifier.md",
			"skills/verify/SKILL.md", "scripts/agents/schemas/verifier.schema.json"} {
			content, err := os.ReadFile(filepath.Join(module, relative))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(s.root, relative)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.root, relative), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		conf := filepath.Join(s.root, "metasystem.conf")
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=64\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		brief := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(brief, []byte("Working Mode: verify\n\n"+strings.Repeat("Verify this line.\n", 700)), 0o644); err != nil {
			t.Fatal(err)
		}
		job := "packet-final-cap"
		packet, err := s.composePacket(composeRequest{
			role: "verifier", brief: brief, job: job, runtime: "fake", model: "fake-model", toolPolicy: "read-only",
			round: 1, destructiveReach: "MECHANICAL", roundDir: filepath.Join(s.agents, job, "rounds", "1"), capMin: "30", prefix: tc.prefix,
		})
		if err != nil {
			t.Fatalf("%s: composition refused: %v (%s)", tc.hint, err, stderr.String())
		}
		info, err := os.Stat(packet.prompt)
		if err != nil || info.Size() <= 8192 || info.Size() > 65536 {
			t.Fatalf("%s: composed %d bytes (%v), outside 8192 < bytes <= 65536", tc.hint, info.Size(), err)
		}
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=8\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = s.enforceInlineInputLimit(packet.prompt, tc.hint)
		if ExitCode(err) != 1 {
			t.Fatalf("%s: the final guard exited %d, want 1", tc.hint, ExitCode(err))
		}
		want := "inline input exceeds dispatch.max-inline-input-kb; pass a file reference in the " + tc.hint
		if !strings.Contains("\n"+stderr.String(), "\n"+want+"\n") {
			t.Fatalf("%s: the guard lost its exact hint: %q", tc.hint, stderr.String())
		}
		outcome := string(s.outcome)
		if !strings.Contains(outcome, `"outcome":"REFUSED-INTERNAL"`) || strings.Contains(outcome+stderr.String()+s.stdout.String(), "REFUSED-INLINE-INPUT-LIMIT") {
			t.Fatalf("%s: the final guard was confused with the composition refusal: outcome %q", tc.hint, outcome)
		}
		s.cleanupCompositionTemporaries()
		entries, _ := os.ReadDir(s.recordLocks)
		for _, entry := range entries {
			name := strings.TrimPrefix(entry.Name(), "follow-")
			if strings.HasPrefix(name, "composed-packet.") || strings.HasPrefix(name, "composition.") || strings.HasPrefix(name, "composed-staged.") {
				t.Fatalf("%s: the refusal left composition temporary %s", tc.hint, entry.Name())
			}
		}
		for _, path := range []string{filepath.Join(s.jobs, job+".json"), filepath.Join(s.jobs, job+".log"), filepath.Join(s.agents, job)} {
			if _, err := os.Stat(path); err == nil {
				t.Fatalf("%s: the refusal published %s", tc.hint, path)
			}
		}
	}
}

// L2664-2701: the permission envelope as the dispatcher expands it. The
// shipped writable and none presets grant network and the critic preset
// denies it; a repository network floor narrows a preset and refuses a
// value that is neither deny nor allow; an envelope whose roots are not
// arrays fails with the owner's own words on the diagnostics (the delegate
// boundary turns that line into its REFUSED-INTERNAL detail:
// cmd/metasystem TestDelegateInternalRefusalDetailPreservesInternalFailure).
func TestP4PermissionEnvelopePresetsFloorAndRefusals(t *testing.T) {
	t.Parallel()
	s, stderr := internalSession(t, Ports{Clock: &stubClock{now: time.Unix(1790000000, 0)}})
	s.env.RecordOutcome = true
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	presets := filepath.Join(s.root, "scripts", "agents", "permissions")
	if err := os.MkdirAll(presets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, preset := range []string{"workspace", "none", "critic"} {
		content, err := os.ReadFile(filepath.Join(module, "scripts", "agents", "permissions", preset+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(presets, preset+".json"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		want := "allow"
		if preset == "critic" {
			want = "deny"
		}
		if got := fieldOr(filepath.Join(presets, preset+".json"), "network"); got != want {
			t.Fatalf("the shipped %s preset network is %q, want %q", preset, got, want)
		}
	}
	conf := filepath.Join(s.root, "metasystem.conf")
	writeConf := func(body string) {
		if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "expanded.json")
	expand := func(requested string) error {
		stderr.Reset()
		return s.expandPermissions(requested, s.root, false, output)
	}
	writeConf("")
	for preset, want := range map[string]string{"critic": "deny", "none": "allow"} {
		if err := expand(preset); err != nil || fieldOr(output, "network") != want {
			t.Fatalf("%s expanded to network %q (%v, %s), want %s", preset, fieldOr(output, "network"), err, stderr.String(), want)
		}
	}
	writeConf("dispatch.permissions.network=deny\n")
	if err := expand("none"); err != nil || fieldOr(output, "network") != "deny" {
		t.Fatalf("the repository network floor did not narrow the preset: %q (%v)", fieldOr(output, "network"), err)
	}
	writeConf("dispatch.permissions.network=sometimes\n")
	if err := expand("none"); ExitCode(err) != 1 || strings.TrimSpace(stderr.String()) != "dispatch.permissions.network must be deny or allow" ||
		!strings.Contains(string(s.outcome), `"outcome":"REFUSED-INTERNAL"`) {
		t.Fatalf("an invalid network floor: exit %d stderr %q outcome %q", ExitCode(err), stderr.String(), s.outcome)
	}
	s.outcome = nil
	writeConf("")
	invalid := filepath.Join(t.TempDir(), "invalid-permissions.json")
	if err := os.WriteFile(invalid, []byte(`{"readRoots":["."],"writeRoots":"<worktree>","network":"allow","approvals":"deny","tools":"runtime-default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := expand(invalid); ExitCode(err) != 1 || strings.TrimSpace(stderr.String()) != "permission roots must be arrays" || len(s.outcome) != 0 {
		t.Fatalf("an envelope without root arrays: exit %d stderr %q outcome %q", ExitCode(err), stderr.String(), s.outcome)
	}
}
