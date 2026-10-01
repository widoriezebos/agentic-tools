package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// deadcodeModule is pinned like staticcheck: the frozen version keeps every
// checkout judging by the same reachability rules, and a run that cannot
// start fails the gate loudly rather than skipping silently. It rides the
// module cache the same way staticcheck does; it is installed for the host
// and run per platform, since go run would build it for the analysed GOOS.
const deadcodeModule = "golang.org/x/tools/cmd/deadcode@v0.50.0"

// deadCodePlatforms are the platforms the engine builds for. A function is
// dead only when no program and no test reaches it on any of them: a helper
// only linux files call is alive although a darwin run cannot see its caller.
var deadCodePlatforms = []string{"darwin", "linux"}

// deadCodeAllowed are functions nothing reaches that stay on purpose, keyed
// file.go#Name (a method is Type.Method) with the reason each stays. An entry
// needs a reason a reader can check; a function merely not wired yet is not
// one.
var deadCodeAllowed = map[string]string{
	"internal/cadence/tick.go#cadenceRequest":               simpleLaneValidateGone,
	"internal/cadence/tick.go#claimCadenceAuthority":        simpleLaneValidateGone,
	"internal/cadence/tick.go#cadenceRunCommand":            simpleLaneValidateGone,
	"internal/cadence/validate.go#cadenceResultPath":        simpleLaneValidateGone,
	"internal/cadence/validate.go#RunOutcome":               simpleLaneValidateGone,
	"internal/cadence/validate.go#ValidateLane.Seams":       simpleLaneValidateGone,
	"internal/cadence/validate.go#planValidation":           simpleLaneValidateGone,
	"internal/cadence/validate.go#launchValidation":         simpleLaneValidateGone,
	"internal/gaterun/cadence.go#GoalCadenceLedger.request": simpleLaneValidateGone,
	"internal/gaterun/cadence.go#GoalCadenceLedger.Claim":   simpleLaneValidateGone,
	"internal/gaterun/cadence.go#GoalCadenceLedger.Publish": simpleLaneValidateGone,
	"internal/landing/batchowner/land.go#FetchBatchOrigin":  simpleLaneValidateGone,
	"internal/gaterun/weight.go#WeightDischargeAt":          simpleLaneValidateGone,
}

// simpleLaneValidateGone is why the validate path stays until the simple
// lane's integration deletes it.
const simpleLaneValidateGone = "landing validate went in the simple lane (unit A); its last callers are in internal/cadence/validate.go, unit B's file, and the integrator deletes these with it"

type deadCodeFinding struct {
	line string
	key  string
}

// parseDeadCode reads deadcode's default report, one
// "file:line:col: unreachable func: Name" line per function.
func parseDeadCode(output string) []deadCodeFinding {
	var findings []deadCodeFinding
	for _, line := range strings.Split(output, "\n") {
		location, name, ok := strings.Cut(line, ": unreachable func: ")
		if !ok {
			continue
		}
		file, _, _ := strings.Cut(location, ":")
		findings = append(findings, deadCodeFinding{line: line, key: file + "#" + strings.TrimSpace(name)})
	}
	return findings
}

// deadCode installs deadcode for this host into a scratch directory, runs it
// with the tests as roots once per platform (GOOS steers only the analysis),
// and returns the reds: a run that could not complete, or the functions every
// platform leaves unreached that the allowlist does not keep.
func (g *gateRun) deadCode() []string {
	bin, err := os.MkdirTemp(tempDir(g.env), "metasystem-gate-deadcode.")
	if err != nil {
		return []string{"dead code check could not run: " + err.Error()}
	}
	defer func() { _ = os.RemoveAll(bin) }()
	var install bytes.Buffer
	if g.d.goTool(g.ctx, g.root, g.env.with("GOBIN="+bin).list(), []string{"install", "-trimpath", deadcodeModule}, &install, &install) != nil {
		return []string{"dead code check could not install deadcode (golang.org/x/tools v0.50.0):\n" + strings.TrimRight(install.String(), "\n")}
	}
	var reds []string
	seen := map[string]int{}
	lines := map[string]string{}
	for _, goos := range deadCodePlatforms {
		var out bytes.Buffer
		call := toolCall{dir: g.root, env: g.env.with("GOOS=" + goos).list(), name: filepath.Join(bin, "deadcode"),
			args: []string{"-test", "./..."}, stdout: &out, stderr: &out}
		if g.d.tool(g.ctx, call) != nil {
			reds = append(reds, fmt.Sprintf("dead code check could not run on %s (deadcode golang.org/x/tools v0.50.0):\n%s", goos, strings.TrimRight(out.String(), "\n")))
			continue
		}
		for _, finding := range parseDeadCode(out.String()) {
			seen[finding.key]++
			lines[finding.key] = finding.line
		}
	}
	if len(reds) > 0 {
		return reds
	}
	var dead []string
	for key, count := range seen {
		if count == len(deadCodePlatforms) && deadCodeAllowed[key] == "" {
			dead = append(dead, lines[key])
		}
	}
	if len(dead) == 0 {
		return nil
	}
	slices.Sort(dead)
	return []string{"dead code: deadcode (golang.org/x/tools v0.50.0, tests as roots) finds functions nothing reaches on " +
		strings.Join(deadCodePlatforms, " and ") + "; delete them, or allowlist a deliberate one with its reason in cmd/devgate/deadcode.go:\n" +
		strings.Join(dead, "\n")}
}
