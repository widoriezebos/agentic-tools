package proofrun

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The coverage delta checks coverage only for the Go packages a landing names
// or touches: packages given explicitly, those touched since a base, or those
// with staged Go files. Matching authenticated full-gate coverage is reused;
// otherwise the check runs as one admitted affected-package proof whose worker
// measures each package against its ratchet floor. Every package below its
// floor and every failed package is reported before the check refuses.

// CoverageDeltaOptions is one coverage delta invocation. The seams are the
// process and repository effects; production wiring lives in the engine verb.
type CoverageDeltaOptions struct {
	Root          string // installation root: module, registry and ratchets
	InvocationDir string // resolves a relative --ratchet
	Base          string
	Staged        bool
	Packages      []string
	Ratchet       string
	Engine        string // the engine the proof relaunch and its authentication run
	GOOS          string
	Relaunched    bool // this process is the admitted proof's suite command

	// Git runs git in Root and returns its standard output.
	Git func(args ...string) (string, error)
	// Reuse reports matching retained full-gate coverage for the packages,
	// returning the lines to print; found false means none matches.
	Reuse func(ratchet string, packages []string) (lines []string, found bool, err error)
	// WorkerAuthorized reports whether this process is an authenticated worker
	// of a live admitted proof.
	WorkerAuthorized func() bool
	// Launch admits and runs this check as a proof for the packages and returns
	// the proof's exit status.
	Launch func(ratchet string, packages []string) int
	// GoTest runs `go test -cover` for one package in Root.
	GoTest func(pkg string) (output string, exitCode int)

	Out, Err io.Writer
}

var (
	coverageFloorPattern    = regexp.MustCompile(`^[0-9]+([.][0-9]+)?$`)
	coverageMeasuredPattern = regexp.MustCompile(`coverage: ([0-9][0-9.]*)% of statements`)
)

// CoverageDeltaUsage is the one-line usage of the coverage delta.
const CoverageDeltaUsage = "usage: metasystem internal proof-run coverage-delta [--base REF | --staged | PACKAGE ...] [--ratchet PATH] [--root INSTALLATION]"

// CoverageDelta runs the check and returns its exit status: 0 passed or
// skipped, 1 refused or failed, 2 usage.
func CoverageDelta(o CoverageDeltaOptions) int {
	selections := 0
	if o.Base != "" {
		selections++
	}
	if o.Staged {
		selections++
	}
	if len(o.Packages) > 0 {
		selections++
	}
	if selections > 1 {
		fmt.Fprintln(o.Err, "coverage delta: --base, --staged, and package arguments are mutually exclusive")
		fmt.Fprintln(o.Err, CoverageDeltaUsage)
		return 2
	}
	if selections == 0 {
		fmt.Fprintln(o.Err, CoverageDeltaUsage)
		return 2
	}
	registry := filepath.Join(o.Root, "scripts", "agents", "coverage-ratchet.json")
	// Staged coverage is a law of roots that carry the canonical registry.
	// Fixture and adopted roots without that registry have no local floor.
	if o.Staged {
		if _, err := os.Lstat(registry); err != nil {
			fmt.Fprintln(o.Out, "coverage delta: no ratchet registry at this root; skipped")
			return 0
		}
	}
	packages := o.Packages
	if o.Base != "" || o.Staged {
		args := []string{"diff", "--name-only", "--relative", o.Base, "--", "*.go"}
		failure := "coverage delta: could not derive packages from base " + o.Base
		if o.Staged {
			args = []string{"diff", "--cached", "--name-only", "--relative", "--", "*.go"}
			failure = "coverage delta: could not derive packages from the staged diff"
		}
		output, err := o.Git(args...)
		if err != nil {
			fmt.Fprintln(o.Err, failure)
			return 1
		}
		packages = nil
		for _, file := range strings.Split(output, "\n") {
			if file != "" {
				packages = append(packages, path.Dir(file))
			}
		}
	}
	if o.Staged && len(packages) == 0 {
		fmt.Fprintln(o.Out, "coverage delta: no Go files staged; skipped")
		return 0
	}
	if info, err := os.Stat(o.Engine); err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		fmt.Fprintf(o.Err, "coverage delta: engine is unavailable at %s\n", o.Engine)
		return 1
	}
	ratchet := o.Ratchet
	switch {
	case ratchet == "":
		ratchet = registry
		if o.GOOS == "linux" {
			ratchet = filepath.Join(o.Root, "scripts", "agents", "coverage-ratchet-linux.json")
		}
	case !filepath.IsAbs(ratchet):
		ratchet = filepath.Join(o.InvocationDir, ratchet)
	}
	if info, err := os.Stat(ratchet); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(o.Err, "coverage delta: ratchet file is unavailable: %s\n", ratchet)
		return 1
	}
	floors, err := readCoverageFloors(ratchet)
	if err != nil || len(floors) == 0 {
		fmt.Fprintf(o.Err, "coverage delta: ratchet floors are unreadable: %s\n", ratchet)
		return 1
	}
	module := goModuleName(filepath.Join(o.Root, "go.mod"))
	if module == "" {
		fmt.Fprintln(o.Err, "coverage delta: go.mod does not name a module")
		return 1
	}
	normalized, code := normalizeCoveragePackages(packages, module, o.Err)
	if code != 0 {
		return code
	}
	if len(normalized) == 0 {
		fmt.Fprintln(o.Out, "coverage delta: no Go packages selected")
		return 0
	}

	lines, found, err := o.Reuse(ratchet, normalized)
	if err != nil {
		for _, line := range lines {
			fmt.Fprintln(o.Err, line)
		}
		fmt.Fprintln(o.Err, err)
		fmt.Fprintln(o.Err, "coverage delta: retained coverage authority was unreadable")
		return 1
	}
	if found {
		for _, line := range lines {
			fmt.Fprintln(o.Out, line)
		}
		fmt.Fprintln(o.Out, "coverage delta: reused authenticated full-gate measurements; no coverage test launched")
		return 0
	}

	// A cache miss is one admitted affected-package proof. A process already
	// inside that proof is authenticated by the proof owner and measures.
	worker := o.WorkerAuthorized()
	if o.Relaunched && !worker {
		fmt.Fprintln(o.Err, "coverage delta: relaunched child is not an authorized proof worker")
		return 1
	}
	if !worker {
		return o.Launch(ratchet, normalized)
	}

	var below, failures []string
	for _, pkg := range normalized {
		display, testPackage := pkg, pkg
		if pkg != "." {
			display, testPackage = "./"+pkg, "./"+pkg
		}
		floor, registered := floors[pkg]
		if !registered {
			fmt.Fprintf(o.Out, "coverage delta: %s: no floor registered\n", display)
			continue
		}
		if !coverageFloorPattern.MatchString(floor) {
			fmt.Fprintf(o.Err, "coverage delta: invalid floor for %s: %s\n", display, floor)
			return 1
		}
		floorValue, _ := strconv.ParseFloat(floor, 64)
		floorDisplay := strconv.FormatFloat(floorValue, 'f', 1, 64)
		output, exitCode := o.GoTest(testPackage)
		measured := ""
		for _, match := range coverageMeasuredPattern.FindAllStringSubmatch(output, -1) {
			measured = match[1]
		}
		if exitCode != 0 {
			failures = append(failures, fmt.Sprintf("%s (go test exited %d)", display, exitCode))
			fmt.Fprint(o.Err, strings.TrimSuffix(output, "\n")+"\n")
		}
		if measured == "" {
			if exitCode == 0 {
				failures = append(failures, display+" (no coverage result)")
			}
			continue
		}
		measuredValue, _ := strconv.ParseFloat(measured, 64)
		if measuredValue < floorValue {
			below = append(below, fmt.Sprintf("%s: measured %s%%, floor %s%%", display, measured, floorDisplay))
		} else {
			fmt.Fprintf(o.Out, "coverage delta: %s: %s%% (floor %s%%)\n", display, measured, floorDisplay)
		}
	}
	if len(below) > 0 {
		fmt.Fprintln(o.Err, "coverage delta: packages below floor:")
		for _, finding := range below {
			fmt.Fprintln(o.Err, "  "+finding)
		}
	}
	if len(failures) > 0 {
		fmt.Fprintln(o.Err, "coverage delta: package test failures:")
		for _, failure := range failures {
			fmt.Fprintln(o.Err, "  "+failure)
		}
	}
	if len(below) > 0 || len(failures) > 0 {
		return 1
	}
	fmt.Fprintf(o.Out, "coverage delta: passed (%d package(s) considered)\n", len(normalized))
	return 0
}

// readCoverageFloors returns each ratchet floor as the number's JSON text.
func readCoverageFloors(ratchet string) (map[string]string, error) {
	data, err := os.ReadFile(ratchet)
	if err != nil {
		return nil, err
	}
	var document struct {
		Floors map[string]json.RawMessage `json:"floors"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	floors := make(map[string]string, len(document.Floors))
	for pkg, raw := range document.Floors {
		text := string(raw)
		var quoted string
		if json.Unmarshal(raw, &quoted) == nil {
			text = quoted
		}
		floors[pkg] = text
	}
	return floors, nil
}

func goModuleName(goMod string) string {
	file, err := os.Open(goMod)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

// normalizeCoveragePackages maps Go's package spellings to the ratchet's
// relative keys, dropping duplicates and refusing patterns.
func normalizeCoveragePackages(packages []string, module string, errOut io.Writer) ([]string, int) {
	var normalized []string
	seen := map[string]bool{}
	for _, pkg := range packages {
		pkg = strings.TrimPrefix(pkg, module+"/")
		for strings.HasPrefix(pkg, "./") {
			pkg = strings.TrimPrefix(pkg, "./")
		}
		if pkg != "." {
			pkg = strings.TrimSuffix(pkg, "/")
		}
		if pkg == "" || strings.Contains(pkg, "...") {
			fmt.Fprintf(errOut, "coverage delta: expected one concrete package, got: %s\n", pkg)
			return nil, 2
		}
		if !seen[pkg] {
			seen[pkg] = true
			normalized = append(normalized, pkg)
		}
	}
	return normalized, 0
}
