package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestProofRunWitnessStateUsesProbeAndFrozenEligibility(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "internal", "tracked")
	if err := os.WriteFile(tracked, []byte("clean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "add", "internal/tracked")
	runGit(t, root, "-c", "user.name=metasystem", "-c", "user.email=metasystem@example.invalid", "commit", "-qm", "initial")
	for name, value := range map[string]string{
		"METASYSTEM_GATE_WITNESS": "", "METASYSTEM_GATE_WITNESS_EXPORT": "",
		"METASYSTEM_COVERAGE_RATCHET_SEED": "0", "METASYSTEM_GATE_FORCE": "0",
		"METASYSTEM_DELIVERY_CONTRACT": "0", "GOFLAGS": "",
	} {
		t.Setenv(name, value)
	}
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("clean state = %q", state)
	}
	if err := os.WriteFile(tracked, []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if state := proofRunWitnessState(root); state != "frozen" {
		t.Fatalf("eligible dirty state = %q", state)
	}
	t.Setenv("METASYSTEM_GATE_FORCE", "1")
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("forced dirty state = %q", state)
	}
	t.Setenv("METASYSTEM_GATE_FORCE", "0")
	t.Setenv("METASYSTEM_GATE_WITNESS", "unusable")
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("unusable witness state = %q", state)
	}

	script := filepath.Join(root, "scripts", "agents", "go-gate.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n[[ \"$1\" == --witness-check-only && \"$METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE\" == ENGINE && \"$METASYSTEM_GATE_WITNESS\" == usable ]]\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GATE_WITNESS", "usable")
	if state := proofRunWitnessState(root); state != "armed" {
		t.Fatalf("usable witness state = %q", state)
	}
	export := filepath.Join(root, "export")
	t.Setenv("METASYSTEM_GATE_WITNESS_EXPORT", export)
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("missing exported witness state = %q", state)
	}
	if err := os.Mkdir(export, 0o700); err != nil {
		t.Fatal(err)
	}
	if state := proofRunWitnessState(root); state != "frozen" {
		t.Fatalf("usable exported witness state = %q", state)
	}
}

func runGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

func TestProofRunLimitsDefaultSilentlyWhenOperationalKnobsAreAbsent(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	limits, err := resolveProofRunLimits(conf)
	if err != nil {
		t.Fatal(err)
	}
	if limits.silence != 30*time.Minute || limits.sectionCap != 45*time.Minute ||
		limits.evidenceTimeout != 60*time.Second || limits.evidenceMax != 512*1024*1024 {
		t.Fatalf("default proof-run limits = %+v", limits)
	}
}

func TestSuiteProgressPrinterSurfacesDeepestLiveSection(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	if err := proofrun.AppendProgressHeader(progress, proofrun.ProgressHeader{LogPaths: []string{"suite.log"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []proofrun.SectionEvent{
		{Suite: "outer", Section: "parent", Event: "start", At: now, Depth: 0},
		{Suite: "inner", Section: "child", Event: "start", At: now, Depth: 1},
	} {
		if err := proofrun.AppendSectionEvent(progress, event); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	stop := startSuiteProgressPrinter(root, time.Hour, &output)
	stop()
	if got := strings.TrimSpace(output.String()); got != "inner:child since 0min" {
		t.Fatalf("progress note = %q", got)
	}
}

func TestSelectedSectionsReadsTwiceConsultedDataFromSelector(t *testing.T) {
	selector := filepath.Join(t.TempDir(), "selector.sh")
	script := `#!/usr/bin/env bash
case "$1" in
  list) printf 'first\tfirst section\nrepeat\trepeated section\n' ;;
  twice) printf 'repeat\n' ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(selector, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	sections, repeated, err := selectedSections(selector, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sections, ",") != "first,repeat" || len(repeated) != 1 || !repeated["repeat"] {
		t.Fatalf("selector data = %v, %v", sections, repeated)
	}
	// A selected run drives one call site, so even a declared-twice section
	// expects a single interval there.
	sections, repeated, err = selectedSections(selector, "repeat", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "repeat" || len(repeated) != 0 {
		t.Fatalf("selected selector data = %v, %v", sections, repeated)
	}
	sections, repeated, err = selectedSections(selector, "first", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "first" || len(repeated) != 0 {
		t.Fatalf("non-repeated selected selector data = %v, %v", sections, repeated)
	}
	sections, repeated, err = selectedSections(selector, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sections, ",") != "first,repeat" || len(repeated) != 0 {
		t.Fatalf("enumerated selector data = %v, %v", sections, repeated)
	}
}

func TestProofRunLimitsRejectOutOfRangeLocalOverride(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("suite.section-cap-min=601\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveProofRunLimits(conf)
	if err == nil || !strings.Contains(err.Error(), "suite.section-cap-min must be an integer from 1 through 600") {
		t.Fatalf("error = %v", err)
	}

	for _, test := range []struct {
		line string
		want string
	}{
		{"suite.progress-silence-min=0\n", "suite.progress-silence-min must be an integer from 1 through 600"},
		{"suite.section-cap-min=601\n", "suite.section-cap-min must be an integer from 1 through 600"},
		{"suite.evidence-copy-timeout-sec=0\n", "suite.evidence-copy-timeout-sec must be an integer from 1 through 600"},
		{"suite.evidence-copy-max-mb=10241\n", "suite.evidence-copy-max-mb must be an integer from 1 through 10240"},
	} {
		if err := os.WriteFile(conf, []byte(test.line), 0o600); err != nil {
			t.Fatal(err)
		}
		problems, err := proofRunConfigProblems(conf)
		if err != nil || len(problems) != 1 || !strings.Contains(problems[0], test.want) {
			t.Fatalf("config validation problems for %q = %v, %v", test.line, problems, err)
		}
	}
}

func TestProofRunLimitsRejectEffectiveLocalAndEnvironmentOverlays(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		envName  string
		wantText string
	}{
		{"suite.section-cap-min", "601", "METASYSTEM_SUITE_SECTION_CAP_MIN", "suite.section-cap-min"},
		{"suite.evidence-copy-timeout-sec", "0", "METASYSTEM_SUITE_EVIDENCE_COPY_TIMEOUT_SEC", "suite.evidence-copy-timeout-sec"},
		{"suite.evidence-copy-max-mb", "10241", "METASYSTEM_SUITE_EVIDENCE_COPY_MAX_MB", "suite.evidence-copy-max-mb"},
	}
	for _, test := range tests {
		t.Run(test.key+" local", func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			if err := os.WriteFile(conf, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf+".local", []byte(test.key+"="+test.value+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := resolveProofRunLimits(conf); err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("effective local error = %v", err)
			}
		})
		t.Run(test.key+" environment", func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			if err := os.WriteFile(conf, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(test.envName, test.value)
			if _, err := resolveProofRunLimits(conf); err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("effective environment error = %v", err)
			}
		})
	}
}
