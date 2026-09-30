package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These owner tests carry the assertions the retired `job brief-mode` and
// `job compose-role-packet` command tests made, against the owner functions
// the delegate lifecycle now calls in process.

// briefAdmissionOutcome is one admission: the admitted mode or the refusal.
func briefAdmissionOutcome(t *testing.T, repo briefAuthorityFixture, installRoot, body string, authority, requireMode bool) (string, error) {
	t.Helper()
	brief := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	baseTree, diskRoot := "", ""
	if authority {
		baseTree, diskRoot = repo.root, repo.root
	}
	admission, err := readBriefAdmissionAtRootWithFacts(brief, installRoot, baseTree, diskRoot, requireMode, repo.facts)
	return admission.Mode, err
}

func isSilentBriefRefusal(err error) bool {
	var op *OpError
	return errors.As(err, &op) && op.Code == 1 && op.Message == "" && op.Reason == ""
}

func TestBriefAdmissionAtRootModesBoundsAndAuthority(t *testing.T) {
	t.Parallel()
	repo := newBriefAuthorityRepo(t)
	repo.facts.prefix = "metasystem"
	absent := filepath.Join(repo.root, "absent")

	// Authority only: a complete bounds pair is admitted without a mode; a
	// partial pair is refused by its bounds code.
	if _, err := briefAdmissionOutcome(t, repo, repo.root, "Boundary: []\nCeiling: 1", true, false); err != nil {
		t.Fatalf("authority-only valid pair = %v", err)
	}
	if _, err := briefAdmissionOutcome(t, repo, repo.root, "Boundary: []", true, false); err == nil || !strings.Contains(err.Error(), "the brief's Ceiling header") {
		t.Fatalf("authority-only partial pair = %v", err)
	}

	for _, tc := range []struct {
		name, body, installRoot string
		authority               bool
		wantMode, wantErr       string
		silent                  bool
	}{
		{name: "partial-pair", body: "Working Mode: implement\nBoundary: []", installRoot: repo.root, wantErr: "the brief's"},
		{name: "mode", body: "Working Mode: implement\nBoundary: []\nCeiling: 1", installRoot: repo.root, wantMode: "implement"},
		{name: "empty-mode", body: "Working Mode:", installRoot: repo.root, silent: true},
		{name: "authority-before-mode", body: "Boundary: []\nCeiling: 1\nRead metasystem/internal/u1b-missing", installRoot: repo.root, authority: true, wantErr: "u1b-missing"},
		{name: "authority-before-mode-required", body: "Boundary: []\nCeiling: 1", installRoot: repo.root, authority: true, silent: true},
		{name: "boundary-without-installation-prefix", body: "Working Mode: implement\nBoundary: [\"internal/dispatch/brief.go\"]\nCeiling: 1", installRoot: repo.root, wantErr: "the brief's"},
		{name: "boundary-with-installation-prefix", body: "Working Mode: implement\nBoundary: [\"metasystem/internal/dispatch/brief.go\"]\nCeiling: 1", installRoot: repo.root, wantMode: "implement"},
		{name: "headerless-needs-no-prefix", body: "Working Mode: implement", installRoot: absent, wantMode: "implement"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := briefAdmissionOutcome(t, repo, tc.installRoot, tc.body, tc.authority, true)
			switch {
			case tc.wantMode != "":
				if err != nil || mode != tc.wantMode {
					t.Fatalf("mode = %q, err = %v; want %q", mode, err, tc.wantMode)
				}
			case tc.silent:
				if !isSilentBriefRefusal(err) {
					t.Fatalf("err = %#v, want the silent exit-1 refusal", err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
			}
		})
	}

	brief := filepath.Join(t.TempDir(), "mode-only.md")
	if err := os.WriteFile(brief, []byte("Working Mode: implement\nBoundary: []"), 0o600); err != nil {
		t.Fatal(err)
	}
	if mode, err := BriefModeOnly(brief); err != nil || mode != "implement" {
		t.Fatalf("mode-only = %q, %v", mode, err)
	}

	// Admission rereads no authority bytes: the brief is read once per entry.
	source, err := os.ReadFile("brief.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), "os.ReadFile(briefPath)") != 2 || !strings.Contains(string(source), "validateBriefAuthority(admitted, bounds") {
		t.Fatal("brief admission can reread authority bytes")
	}
}

// Every header problem a brief carries is reported before the installation
// prefix is looked up; only a valid pair reaches the prefix lookup.
func TestBriefAdmissionAtRootReportsHeadersBeforePrefixLookup(t *testing.T) {
	t.Parallel()
	repo := newBriefAuthorityRepo(t)
	unresolvable := filepath.Join(t.TempDir(), "elsewhere")
	for _, tc := range []struct {
		name, body, want string
		silent           bool
	}{
		{"mode-before-prefix", "Boundary: []\nCeiling: 1", "", true},
		{"partial-before-prefix", "Working Mode: implement\nBoundary: []", "the brief's Ceiling header is needed with a Boundary header", false},
		{"malformed-before-prefix", "Working Mode: implement\nBoundary: bad\nCeiling: 1", "the brief's Boundary header must be a JSON array of paths", false},
		{"malformed-ceiling-before-prefix", "Working Mode: implement\nBoundary: []\nCeiling: -1", "the brief's Ceiling header must be a whole number", false},
		{"invalid-member-before-prefix", "Working Mode: implement\nBoundary: [\"/abs\"]\nCeiling: 1", "the brief's Boundary header names a path or pattern that is not valid: \"/abs\"", false},
		{"valid-pair-uses-prefix", "Working Mode: implement\nBoundary: []\nCeiling: 1", "brief admission cannot resolve installation prefix", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := briefAdmissionOutcome(t, repo, unresolvable, tc.body, false, true)
			if tc.silent {
				if !isSilentBriefRefusal(err) {
					t.Fatalf("err = %#v, want the silent exit-1 refusal", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "authority admission") {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

// A first-round tier-3 composition carries the maximal independent-critique
// obligations.
func TestComposeRolePacketTierThreeFirstRoundObligations(t *testing.T) {
	t.Parallel()
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Build the focused change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	params := ComposeRolePacketParams{
		Root: root, Role: "implementer", Brief: brief, JobID: "compose-tier-3", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-write", Round: 1, DestructiveReach: HazardMechanical, GoalTier: 3,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
	}
	if _, err := ComposeRolePacket(params); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(params.CompositionOutput)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(stored, &record); err != nil {
		t.Fatal(err)
	}
	obligations, ok := record["configurationObligations"].(map[string]any)
	if !ok || obligations["independentCritiqueRequired"] != true ||
		obligations["independentCritiqueEffortTier"] != "maximal" ||
		obligations["independentCritiqueReasoningEffort"] != "xhigh" {
		t.Fatalf("tier-3 obligations = %#v", record["configurationObligations"])
	}
}
