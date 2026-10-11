package rehearsal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

const GreenGoal = "rehearsal-green"

// Harness commits its declarations on main and publishes them before proof.
func (s *Scratch) HarnessSetup(templates, launchSettings string) error {
	write := func(path string, data []byte) error {
		_, err := atomicfile.WriteFile(filepath.Join(s.Module, path), data, 0644, s.Module)
		return err
	}
	for _, template := range []struct{ source, target string }{
		{"value.go", "internal/rehearsalgreen/value.go"},
		{"value_test.go", "internal/rehearsalgreen/value_test.go"},
		{"goal.md.tmpl", "plans/goals/" + GreenGoal + ".md"},
		{"design.md.tmpl", "plans/designs/rehearsal-green.md"},
	} {
		data, err := os.ReadFile(filepath.Join(templates, template.source))
		if err != nil {
			return err
		}
		if template.source == "goal.md.tmpl" {
			data = fixtureGoal(data, time.Now().UTC())
		}
		if err = write(template.target, data); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(filepath.Join(s.Module, "metasystem.conf"))
	if err != nil {
		return err
	}
	var conf strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		key, _, _ := strings.Cut(strings.TrimSpace(line), "=")
		if key != "proof.full" && key != "proof.cheap" && key != "deploy.contract" {
			fmt.Fprintln(&conf, line)
		}
	}
	command := `LANDING_ONLY="${LANDING_ONLY:-fast-static-build metasystem/internal/rehearsalgreen}" sh proof/full.sh --host`
	fmt.Fprintf(&conf, "proof.full=%s\nproof.cheap=%s\ndeploy.contract=\n", command, command)
	if err = write("metasystem.conf", []byte(conf.String())); err != nil {
		return err
	}
	if err = s.fixtureProofContract(); err != nil {
		return err
	}
	if err = write("metasystem.conf.local", []byte(launchSettings)); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "metasystem/metasystem.conf", "metasystem/testing.json", "metasystem/internal/rehearsalgreen", "metasystem/plans/goals/" + GreenGoal + ".md", "metasystem/plans/designs/rehearsal-green.md"},
		{"commit", "-m", "Rehearsal harness for candidate " + s.Candidate},
		{"push", "origin", "main"},
	} {
		if _, err = s.Git(s.Checkout, args...); err != nil {
			return err
		}
	}
	s.Harness, err = s.Git(s.Checkout, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if _, err = s.Git(s.Checkout, "update-ref", goal.AcceptedRef, s.Harness); err != nil {
		return err
	}
	_, err = goal.ReadValidatedTree(s.Module, s.Harness, s.Env)
	return err
}

func fixtureGoal(template []byte, now time.Time) []byte {
	body := []byte(strings.ReplaceAll(string(template), "@TIME@", now.Format(time.RFC3339)))
	return append(body, []byte(fmt.Sprintf("Integrity: sha256=%s\n", goal.IntegrityDigest(body)))...)
}

// Green drives registration and the keeper through the candidate engine.
func (s *Scratch) Green() error {
	seat := filepath.Join(s.Scratch, "seat")
	rel, err := filepath.Rel(s.Checkout, s.Module)
	if err != nil {
		return err
	}
	module := filepath.Join(seat, rel)
	commands := []struct {
		stage, dir string
		args       []string
	}{
		{"", s.Module, []string{"landing", "set", s.Checkout}},
		{"bootstrap", s.Module, []string{"landing", "prove", "--trunk", "--wait"}},
		{"hand-in", module, []string{"work", "land", GreenGoal}},
		{"", s.Module, []string{"landing", "run"}},
		{"gate", s.Module, []string{"landing", "prove", "--gate", "--wait"}},
		{"proof", s.Module, []string{"landing", "prove", "--wait"}},
		{"push", s.Module, []string{"landing", "push"}},
		{"", s.Module, []string{"landing", "status", "--json"}},
	}
	for i, command := range commands {
		if command.stage != "" {
			s.beginStage(command.stage, time.Now())
		}
		switch i {
		case 2:
			if err := s.prepareGreenSeat(seat, module); err != nil {
				return err
			}
			s.Env = append(s.Env, "METASYSTEM_OWNER_LINEAGE=rehearsal")
		case 3:
			s.Env = append(s.Env, "METASYSTEM_OWNER_LINEAGE=landing-agent")
		}
		output, err := s.Verb(command.dir, command.args...)
		if err != nil {
			return err
		}
		switch i {
		case 1:
			bootstrap, found, err := plain.LastResult(s.Module)
			if err != nil || !found || bootstrap.Commit != s.Harness || bootstrap.Result != plain.Green || bootstrap.Scope != "full" {
				return fmt.Errorf("green bootstrap checked %s (%q, %q); expected harness %s in full: %v", bootstrap.Commit, bootstrap.Scope, bootstrap.Result, s.Harness, err)
			}
		case 3:
			var tick struct {
				Data struct {
					Outcome lane.AgentOutcome
					Reasons []string
				}
			}
			if err := json.Unmarshal([]byte(output), &tick); err != nil || tick.Data.Outcome != lane.AgentStarted || len(tick.Data.Reasons) == 0 {
				return fmt.Errorf("green keeper did not start the lane step: %s: %v", output, err)
			}
			if err := s.prepareGreenMerge(); err != nil {
				return err
			}
		case 4:
			gate, found, err := plain.LastGate(s.Module)
			if err != nil || !found || gate.Result != plain.Green && gate.Result != plain.Skipped || gate.Result == plain.Skipped && gate.Reason == "" || gate.Result == plain.Green && (gate.Depth != "impact" || gate.PlanHash == "") {
				return fmt.Errorf("green rehearsal merge gate returned %q at depth %q with reason %q: %v", gate.Result, gate.Depth, gate.Reason, err)
			}
		case 5:
			proof, found, err := plain.LastResult(s.Module)
			if err := CheckGreenProof(proof, found, err); err != nil {
				return err
			}
		case 6:
			push, found, err := plain.LastPush(s.Module)
			origin, gitErr := s.Git(s.Checkout, "ls-remote", "origin", "refs/heads/main")
			if err != nil || gitErr != nil || !found || push.Old != s.Harness || push.Commit == s.Harness || !strings.HasPrefix(origin, push.Commit+"\t") {
				return fmt.Errorf("green rehearsal push did not move scratch main from harness %s to merge %s: %v", s.Harness, push.Commit, err)
			}
		case 7:
			if err := s.checkGreenStatus(output); err != nil {
				return err
			}
		}
	}
	return s.ctx.Err()
}

func (s *Scratch) prepareGreenSeat(seat, module string) error {
	if _, err := s.Git(s.Checkout, "worktree", "add", "-b", "goal/"+GreenGoal, seat, s.Harness); err != nil {
		return err
	}
	for _, name := range []string{"value.go", "value_test.go"} {
		path := filepath.Join(module, "internal/rehearsalgreen", name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err = os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "1", "2")), 0644); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"add", "metasystem/internal/rehearsalgreen"}, {"commit", "-m", "Change the fixture value\n\nGoal-Unit: rehearsal-green/u1\nGoal-Whole: rehearsal-green"}, {"push", "origin", "goal/" + GreenGoal}} {
		if _, err := s.Git(seat, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scratch) prepareGreenMerge() error {
	batch, err := plain.ReadBatch(s.Module)
	if err != nil || batch == nil || batch.Base != s.Harness || len(batch.Members) != 1 || batch.Members[0].Goal != GreenGoal {
		return fmt.Errorf("green rehearsal selection does not contain just the fixture on harness %s: %v", s.Harness, err)
	}
	for _, args := range [][]string{{"checkout", "--detach", "origin/main"}, {"merge", "--no-ff", "--no-edit", batch.Members[0].SHA}} {
		if _, err = s.Git(s.Checkout, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scratch) checkGreenStatus(text string) error {
	var status struct{ Data plain.Status }
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		return err
	}
	for _, entry := range status.Data.Queue {
		if entry.Goal == GreenGoal && entry.State == plain.StateLanded {
			return nil
		}
	}
	return fmt.Errorf("landing status does not say the green goal landed: %s", text)
}

func CheckGreenProof(proof plain.Result, found bool, err error) error {
	if err != nil || !found || proof.Result != plain.Green || proof.Scope != "impact" || !strings.HasPrefix(proof.ScopeReason, "tiers 1;") || proof.CountedFull || proof.PlanHash == "" {
		return fmt.Errorf("green scenario decided depth %q (%s), outcome %q; needs tier-one impact with a plan, no full check: %v", proof.Scope, proof.ScopeReason, proof.Result, err)
	}
	for _, execution := range proof.Executions {
		if execution.Scope == "full" {
			return fmt.Errorf("green scenario decided depth full (%s); expected impact", execution.Reason)
		}
	}
	return nil
}

// RunGreen owns the ten-minute bound, including the bootstrap proof.
func RunGreen(parent context.Context, module string, env []string) (s *Scratch, err error) {
	return runGreen(parent, module, env, Prepare)
}

func runGreen(parent context.Context, module string, env []string, prepare func(context.Context, string, []string) (*Scratch, error)) (s *Scratch, err error) {
	start := time.Now()
	defer func() {
		if s != nil {
			s.beginStage("", time.Now())
			s.WallMS = time.Since(start).Milliseconds()
			s.Breakdown.Total = s.WallMS
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	s, err = prepare(ctx, module, env)
	if err != nil && ctx.Err() == nil {
		first := s
		s, err = prepare(ctx, module, env)
		if first != nil {
			if s == nil {
				s = first
			} else {
				s.Steps = append(first.Steps, s.Steps...)
			}
		}
	}
	if s != nil {
		s.stage, s.stageStart = "setup", start
	}
	if err != nil {
		return s, err
	}
	settings, err := LaunchSettings(s.Module)
	if err == nil {
		err = s.HarnessSetup(filepath.Join(s.Module, "internal/landing/rehearsal/fixtures/green"), settings)
	}
	if err == nil {
		err = s.Green()
	}
	return s, err
}
