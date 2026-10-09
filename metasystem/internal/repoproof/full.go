// Package repoproof runs this repository's package suite and writes the
// landing report after all native output, so the lane can classify failures.
package repoproof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type Command func([]string, io.Writer, io.Writer) error

func Execute(argv []string, stdout, stderr io.Writer) error {
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdout, command.Stderr = stdout, stderr
	return command.Run()
}

func Run(stdout, stderr io.Writer, getenv func(string) string, command Command) int {
	return run(stdout, stderr, getenv, command, "testing.json")
}

func notRun(stdout, stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
	return 1
}

func run(stdout, stderr io.Writer, getenv func(string) string, command Command, contractPath string) int {
	notRun := func(err error) int {
		return notRun(stdout, stderr, err)
	}
	// The fresh proof worktree has no computer-local settings. Query the
	// registered lane, then ask the public settings verb for just its VM fact.
	var status bytes.Buffer
	if err := command([]string{"metasystem", "landing", "status", "--json"}, &status, stderr); err != nil {
		return notRun(err)
	}
	var lane struct {
		Data struct {
			Root string `json:"root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(status.Bytes(), &lane); err != nil || lane.Data.Root == "" {
		return notRun(fmt.Errorf("the landing lane's checkout could not be read"))
	}
	var settings bytes.Buffer
	if err := command([]string{"metasystem", "settings", "show", "host.proof-vm", "--repo", lane.Data.Root, "--json"}, &settings, stderr); err != nil {
		return notRun(err)
	}
	var fact struct {
		Data struct {
			Settings []struct{ Key, Value string } `json:"settings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(settings.Bytes(), &fact); err != nil || len(fact.Data.Settings) != 1 || fact.Data.Settings[0].Key != "host.proof-vm" {
		return notRun(fmt.Errorf("the host.proof-vm setting could not be read"))
	}
	if vm := fact.Data.Settings[0].Value; vm != "" {
		return runVM(stdout, stderr, getenv, command, vm)
	}
	return runHost(stdout, stderr, getenv, command, contractPath)
}

// RunHost proves the extracted candidate without consulting the host lane again.
func RunHost(stdout, stderr io.Writer, getenv func(string) string, command Command) int {
	return runHost(stdout, stderr, getenv, command, "testing.json")
}

func runHost(stdout, stderr io.Writer, getenv func(string) string, command Command, contractPath string) int {
	failed := map[string][]string{}
	notRun := func(err error) int { return notRun(stdout, stderr, err) }
	packages := "./..."
	if only := getenv("LANDING_ONLY"); only != "" {
		packages = only
		if relative, ok := strings.CutPrefix(only, "metasystem/"); ok {
			packages = "./" + relative
		}
	}
	if err := packageSuite(stdout, stderr, command, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", packages}, failed); err != nil {
		return notRun(err)
	}
	if getenv("LANDING_ONLY") == "" {
		batchFailed := map[string][]string{}
		if err := packageSuite(stdout, stderr, command, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", "-tags", "batchtest", "./cmd/metasystem/", "./internal/landing/..."}, batchFailed); err != nil {
			return notRun(err)
		}
		if len(batchFailed) > 0 {
			failed["go-batchtest"] = nil
			for _, tests := range batchFailed {
				failed["go-batchtest"] = append(failed["go-batchtest"], tests...)
			}
			sort.Strings(failed["go-batchtest"])
		}
		if err := command([]string{"go", "run", "./cmd/devgate", "static"}, stdout, stderr); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() == 126 || exit.ExitCode() == 127 {
				return notRun(err)
			}
			failed["fast-static-build"] = nil
		}
		contract, err := testpolicy.Load(contractPath)
		if err != nil {
			return notRun(err)
		}
		reporter := getenv("METASYSTEM_FULL_REPORTER")
		if reporter == "" {
			reporter = "proof/.full-reporter"
		}
		for _, group := range contract.Groups {
			if group.Adapter != "section" {
				continue
			}
			var report bytes.Buffer
			err := command([]string{reporter, "--section", group.ID}, io.MultiWriter(stdout, &report), stderr)
			var result struct {
				Data struct {
					Groups []struct {
						ID, Status       string
						NativeLaunched   bool
						NativeExitStatus *int
					}
				}
			}
			if decodeErr := json.Unmarshal(report.Bytes(), &result); decodeErr != nil {
				return notRun(fmt.Errorf("section %s report: %w", group.ID, decodeErr))
			}
			found := false
			hasFailure := false
			for _, leg := range result.Data.Groups {
				found = found || leg.ID == group.ID
				if !leg.NativeLaunched || leg.NativeExitStatus == nil || (leg.Status != "passed" && leg.Status != "failed") {
					return notRun(fmt.Errorf("section %s did not complete: %s", leg.ID, leg.Status))
				}
				if leg.Status == "failed" || *leg.NativeExitStatus != 0 {
					failed[leg.ID] = nil
					hasFailure = true
				}
			}
			if !found || (err != nil && !hasFailure) {
				return notRun(fmt.Errorf("section %s has no completed verdict: %v", group.ID, err))
			}
		}
	}
	units := make([]string, 0, len(failed))
	for unit := range failed {
		units = append(units, unit)
	}
	sort.Strings(units)
	for _, unit := range units {
		fmt.Fprintf(stdout, "LANDING-FAILED\t%s\t%s\n", unit, strings.Join(failed[unit], " "))
	}
	fmt.Fprintf(stdout, "LANDING-CHECKED\t%d\n", len(units))
	if len(units) > 0 {
		return 1
	}
	return 0
}

func packageSuite(stdout, stderr io.Writer, command Command, argv []string, failed map[string][]string) error {
	var native bytes.Buffer
	err := command(argv, io.MultiWriter(stdout, &native), stderr)
	completed := 0
	legFailed := false
	decoder := json.NewDecoder(&native)
	for decoder.More() {
		var event struct{ Action, Package, Test string }
		if decodeErr := decoder.Decode(&event); decodeErr != nil {
			return fmt.Errorf("the package suite's report could not be read: %w", decodeErr)
		}
		// Landing's units are paths from the checkout root, also used by its
		// Git readers; Go reports import paths from the module instead.
		event.Package = packageUnit(event.Package)
		if event.Action == "fail" && event.Package != "" {
			legFailed = true
			if event.Test != "" {
				failed[event.Package] = append(failed[event.Package], event.Test)
			} else if _, exists := failed[event.Package]; !exists {
				failed[event.Package] = nil
			}
		}
		if event.Package != "" && event.Test == "" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
			completed++
		}
	}
	if completed == 0 {
		return fmt.Errorf("the package suite reported no completed packages: %v", err)
	}
	if err != nil && !legFailed {
		return err
	}
	return nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func runVM(stdout, stderr io.Writer, getenv func(string) string, command Command, vm string) int {
	commit := getenv("LANDING_COMMIT")
	if commit == "" {
		commit = "HEAD"
	}
	var resolved bytes.Buffer
	if err := command([]string{"git", "rev-parse", "--verify", commit + "^{commit}"}, &resolved, stderr); err != nil {
		return notRun(stdout, stderr, err)
	}
	commit = strings.TrimSpace(resolved.String())
	if commit == "" || strings.ContainsAny(commit, "/\\\n\r") {
		return notRun(stdout, stderr, fmt.Errorf("the proof commit could not be resolved"))
	}
	var top bytes.Buffer
	if err := command([]string{"git", "rev-parse", "--show-toplevel"}, &top, stderr); err != nil {
		return notRun(stdout, stderr, err)
	}
	root := strings.TrimSpace(top.String())
	if root == "" {
		return notRun(stdout, stderr, fmt.Errorf("the proof checkout root could not be resolved"))
	}
	temp, err := os.MkdirTemp(diskstore.ProcessTempRoot(), "metasystem-proof-bundle-")
	if err != nil {
		return notRun(stdout, stderr, err)
	}
	defer os.Remove(temp)
	bundle := filepath.Join(temp, "candidate.bundle")
	defer os.Remove(bundle)
	ref := "refs/metasystem/proof/" + filepath.Base(temp)
	if err := command([]string{"git", "-C", root, "update-ref", ref, commit}, io.Discard, stderr); err != nil {
		return notRun(stdout, stderr, err)
	}
	defer command([]string{"git", "-C", root, "update-ref", "-d", ref, commit}, io.Discard, stderr)
	if err := command([]string{"git", "-C", root, "bundle", "create", bundle, ref}, io.Discard, stderr); err != nil {
		return notRun(stdout, stderr, err)
	}
	dir := "/tmp/metasystem-proof/" + commit
	remoteBundle := dir + ".bundle"
	// Checkout failures are environment failures, before any test can report a code red.
	remote := "if ! (mkdir -p /tmp/metasystem-proof && cat > " + shellQuote(remoteBundle) +
		" && { [ -d " + shellQuote(dir+"/.git") + " ] || git clone --no-checkout " + shellQuote(remoteBundle) + " " + shellQuote(dir) +
		"; } && git -C " + shellQuote(dir) + " fetch --no-tags " + shellQuote(remoteBundle) + " " + shellQuote(ref) +
		" && git -C " + shellQuote(dir) + " checkout --force --detach " + shellQuote(commit) +
		" && test -f " + shellQuote(dir+"/metasystem/proof/full.sh") +
		"); then printf 'LANDING-NOT-RUN\\tenvironment\\n'; exit 1; fi; cd " + shellQuote(dir+"/metasystem") +
		" || { printf 'LANDING-NOT-RUN\\tenvironment\\n'; exit 1; }; LANDING_COMMIT=" + shellQuote(commit) + " LANDING_ONLY=" + shellQuote(getenv("LANDING_ONLY")) + " sh proof/full.sh --host"
	pipeline := "cat " + shellQuote(bundle) + " | limactl shell " + shellQuote(vm) + " -- bash -c " + shellQuote(remote) + `; statuses=("${PIPESTATUS[@]}"); if [ "${statuses[0]}" -ne 0 ]; then exit 2; fi; exit "${statuses[1]}"`
	var report bytes.Buffer
	err = command([]string{"bash", "-c", pipeline}, io.MultiWriter(stdout, &report), stderr)
	// A failed transfer cannot be certified by a report from the remote process.
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return notRun(stdout, stderr, err)
	}
	lines := strings.Split(strings.TrimSpace(report.String()), "\n")
	last := lines[len(lines)-1]
	if last == "LANDING-NOT-RUN\tenvironment" {
		return 1
	}
	if !strings.HasPrefix(last, "LANDING-CHECKED\t") {
		return notRun(stdout, stderr, fmt.Errorf("the VM proof produced no landing report: %v", err))
	}
	if err != nil {
		return 1
	}
	return 0
}
