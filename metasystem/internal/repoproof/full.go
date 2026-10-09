// Package repoproof runs this repository's package suite and writes the
// landing report after all native output, so the lane can classify failures.
package repoproof

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type HostRunners struct {
	Native      func(proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error)
	Groups      func([]string) ([]proofrun.NamedGroupResult, error)
	Environment func() (string, error)
	Now         func() time.Time
}

type Command func([]string, io.Writer, io.Writer) error

func Execute(argv []string, stdout, stderr io.Writer) error {
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdout, command.Stderr = stdout, stderr
	return command.Run()
}

func Run(stdout, stderr io.Writer, getenv func(string) string, command Command, runners ...HostRunners) int {
	return run(stdout, stderr, getenv, command, "testing.json", runners...)
}

func notRun(stdout, stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
	return 1
}

func run(stdout, stderr io.Writer, getenv func(string) string, command Command, contractPath string, runners ...HostRunners) int {
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
	return runHost(stdout, stderr, getenv, command, contractPath, runners...)
}

// RunHost proves the extracted candidate without consulting the host lane again.
func RunHost(stdout, stderr io.Writer, getenv func(string) string, command Command, runners ...HostRunners) int {
	return runHost(stdout, stderr, getenv, command, "testing.json", runners...)
}

func runHost(stdout, stderr io.Writer, getenv func(string) string, command Command, contractPath string, runners ...HostRunners) int {
	failed := map[string][]string{}
	hooks := HostRunners{}
	if len(runners) > 0 {
		hooks = runners[0]
	}
	if hooks.Now == nil {
		hooks.Now = time.Now
	}
	started := hooks.Now()
	notRun := func(err error) int {
		fmt.Fprintf(stdout, "landing clock total %d\n", hooks.Now().Sub(started).Milliseconds())
		return notRun(stdout, stderr, err)
	}
	root, err := filepath.Abs(filepath.Dir(contractPath))
	if err != nil {
		return notRun(err)
	}
	ctx, environment := context.Background(), os.Environ()
	if hooks.Native == nil {
		hooks.Native = func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
			return proofrun.RunNativeInventory(ctx, r)
		}
	}
	if hooks.Environment == nil {
		hooks.Environment = func() (string, error) { return proofrun.LandingEnvironment(ctx, root, environment) }
	}
	text, err := hooks.Environment()
	if err != nil {
		return notRun(err)
	}
	fmt.Fprintln(stdout, "landing environment "+text)
	contract, err := testpolicy.Load(contractPath)
	if err != nil {
		return notRun(err)
	}
	if hooks.Groups == nil {
		hooks.Groups = func(ids []string) ([]proofrun.NamedGroupResult, error) {
			return proofrun.RunNamedGroups(ctx, root, contract, ids, environment)
		}
	}
	only, scoped := getenv("LANDING_ONLY"), getenv("LANDING_PROOF_SCOPE") == "scoped"
	var groups []string
	selections := []string{"./..."}
	if scoped {
		groups, selections = strings.Fields(getenv("LANDING_PROOF_GROUPS")), strings.Fields(getenv("LANDING_PROOF_PACKAGES"))
	}
	if only != "" {
		groups, selections = nil, nil
		for _, selection := range strings.Fields(only) {
			if slices.ContainsFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == selection }) {
				groups = append(groups, selection)
			} else {
				selections = append(selections, selection)
			}
		}
	}
	full := !scoped && only == ""
	if full {
		staticStart, status := hooks.Now(), "green"
		if err := command([]string{"go", "run", "./cmd/devgate", "static"}, stdout, stderr); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() == 126 || exit.ExitCode() == 127 {
				return notRun(err)
			}
			failed["fast-static-build"] = nil
			status = "red"
		}
		fmt.Fprintf(stdout, "landing group fast-static-build %s %d\n", status, hooks.Now().Sub(staticStart).Milliseconds())
	}
	if len(groups) > 0 {
		results, err := hooks.Groups(groups)
		if err != nil {
			return notRun(err)
		}
		for _, group := range results {
			fmt.Fprintf(stdout, "landing group %s %s %d\n", group.ID, group.Status, group.DurationMS)
			if group.Status != "green" {
				failed[group.ID] = nil
				fmt.Fprintln(stderr, group.Output)
			}
		}
	}
	logRoot, release, err := diskstore.ScratchDir("metasystem-landing-native-")
	if err != nil {
		return notRun(err)
	}
	defer release()
	native := func(packages, tests, tags []string) error {
		type packageShard struct {
			unit  string
			shard int
		}
		reported := map[packageShard]string{}
		result, err := hooks.Native(proofrun.NativeInventoryRequest{Root: root, LogRoot: logRoot, Environment: environment, Packages: packages, Tests: tests, BuildTags: tags, Progress: func(planned int, completed []proofrun.PackageExecution) {
			if completed == nil {
				fmt.Fprintf(stdout, "landing planned %d\n", planned)
			}
			for _, execution := range completed {
				reported[packageShard{packageUnit(execution.Package), execution.Shard}] = execution.Status
				ms := int64(0)
				if execution.ElapsedMS != nil {
					ms = *execution.ElapsedMS
				}
				fmt.Fprintf(stdout, "landing package %s %d %s %d\n", packageUnit(execution.Package), execution.Shard, execution.Status, ms)
			}
		}})
		if err != nil {
			return err
		}
		stdout.Write(result.Output)
		if len(result.Execution) == 0 {
			return fmt.Errorf("the package suite reported no completed packages")
		}
		for _, execution := range result.Execution {
			unit := packageUnit(execution.Package)
			ms := int64(0)
			if execution.ElapsedMS != nil {
				ms = *execution.ElapsedMS
			}
			if status, ok := reported[packageShard{unit, execution.Shard}]; !ok || status != execution.Status {
				fmt.Fprintf(stdout, "landing package %s %d %s %d\n", unit, execution.Shard, execution.Status, ms)
			}
			if slices.Contains(tags, "batchtest") {
				unit = "go-batchtest"
			}
			if execution.Status != "ok" {
				if _, ok := failed[unit]; !ok {
					failed[unit] = nil
				}
			}
		}
		for index, identities := range [][]proofrun.NativeTestIdentity{result.Observed, result.Missing, result.Unexpected} {
			for _, identity := range identities {
				if index != 2 && (identity.Status == "passed" || identity.Status == "skipped") {
					continue
				}
				name := identity.Name
				if strings.HasPrefix(identity.Status, "missing") {
					name += "(did not report)"
				}
				unit := packageUnit(identity.Classname)
				if slices.Contains(tags, "batchtest") {
					unit = "go-batchtest"
				}
				failed[unit] = append(failed[unit], name)
			}
		}
		return nil
	}
	for _, selection := range selections {
		pkg, names, _ := strings.Cut(selection, "=")
		pkg = strings.TrimPrefix(pkg, "metasystem/")
		var tests []string
		if names != "" {
			tests = strings.Split(names, ",")
		}
		if err := native([]string{pkg}, tests, nil); err != nil {
			return notRun(err)
		}
		if only == "" && !full && (pkg == "cmd/metasystem" || strings.HasPrefix(pkg, "internal/landing/")) {
			if err := native([]string{pkg}, tests, []string{"batchtest"}); err != nil {
				return notRun(err)
			}
		}
	}
	if full {
		if err := native([]string{"cmd/metasystem", "internal/landing/..."}, nil, []string{"batchtest"}); err != nil {
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
	fmt.Fprintf(stdout, "landing clock total %d\n", hooks.Now().Sub(started).Milliseconds())
	for _, unit := range units {
		sort.Strings(failed[unit])
		fmt.Fprintf(stdout, "LANDING-FAILED\t%s\t%s\n", unit, strings.Join(slices.Compact(failed[unit]), " "))
	}
	fmt.Fprintf(stdout, "LANDING-CHECKED\t%d\n", len(units))
	if len(units) > 0 {
		return 1
	}
	return 0
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
