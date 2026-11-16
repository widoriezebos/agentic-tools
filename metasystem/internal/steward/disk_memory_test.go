package steward

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type memoryFloorProcesses struct {
	identity.ListedProcessTable
	reads int
}

// The production defaults are inspected without running their host scans.
func TestMachinePassRoots(t *testing.T) {
	t.Parallel()
	memoryRoot := "/tmp/mst-" + strconv.Itoa(os.Geteuid())
	for _, home := range []string{
		"/tmp/metasystem-test-registry-1/home",
		"/private/tmp/metasystem-test-registry-1/home",
		memoryRoot + "/metasystem-test-registry-1/home",
		"/fallback/metasystem-test-registry-1/home",
	} {
		t.Run(home, func(t *testing.T) {
			t.Parallel()
			inside := filepath.Join(home, "tmp")
			evidence := []string{filepath.Join(home, "evidence"), memoryRoot + "/evidence"}
			temps, volumes := machinePassRoots(home, "/checkout/metasystem", nil, evidence)
			if len(temps) != 0 || len(volumes) != 0 {
				t.Fatalf("test defaults: temp roots %v, volumes %v", temps, volumes)
			}
			explicit := []string{inside, memoryRoot, "/outside/tmp"}
			temps, volumes = machinePassRoots(home, "/checkout/metasystem", explicit, append(evidence, "/outside/evidence"))
			if !slices.Equal(temps, explicit) || !slices.Equal(volumes, []string{"/outside/tmp", "/outside/evidence"}) {
				t.Fatalf("explicit roots: temp roots %v, volumes %v", temps, volumes)
			}
		})
	}
	t.Run("real home", func(t *testing.T) {
		t.Parallel()
		hostTemp, _ := diskstore.HostTempRoot()
		wantTemps := nonEmpty(hostTemp, "/tmp")
		evidence := []string{"/evidence", "/other/metasystem-test-registry-1"}
		temps, volumes := machinePassRoots("/user/.metasystem", "/checkout/metasystem", nil, evidence)
		wantVolumes := []string{"/user/.metasystem", "/checkout/metasystem"}
		for _, root := range append(wantTemps, evidence...) {
			if !diskstore.OnTestMemoryRoot(root) {
				wantVolumes = append(wantVolumes, root)
			}
		}
		if !slices.Equal(temps, wantTemps) || !slices.Equal(volumes, wantVolumes) {
			t.Fatalf("real defaults: temp roots %v, volumes %v; want %v, %v", temps, volumes, wantTemps, wantVolumes)
		}
	})
}

func TestMachinePassInTestRegistryHome(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"defaults", "fallback", "explicit temp", "outside temp", "injected volume", "injected empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newStaleBed(t)
			if name == "fallback" {
				bed.home = filepath.Join(bed.root, "fallback", "metasystem-test-registry-1", "home")
				bed.registry = filepath.Join(bed.home, "armed-checkouts.jsonl")
			}
			if !diskstore.InTestRegistryHome(bed.home) {
				t.Fatalf("fixture is outside its registry home: %s", bed.home)
			}
			evidence := filepath.Join(bed.root, "evidence")
			if err := os.MkdirAll(evidence, 0o700); err != nil {
				t.Fatal(err)
			}
			conf := "metasystem.template=true\ndisk.floor-gib=1048576\nevidence.root=" + evidence + "\n"
			if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte(conf), 0o600); err != nil {
				t.Fatal(err)
			}
			stray := filepath.Join(bed.tmp, "metasystem-fixture-stray")
			if err := os.WriteFile(stray, []byte("stray"), 0o600); err != nil {
				t.Fatal(err)
			}
			pass := bed.pass(diskstore.ModeReport, []string{bed.inst}, false)
			pass.TempRoots, pass.Volumes = nil, nil
			processes := &memoryFloorProcesses{}
			pass.Processes = processes
			var wantVolume string
			switch name {
			case "explicit temp":
				pass.TempRoots = []string{bed.tmp}
			case "outside temp":
				// This read-only directory is outside test storage even when
				// the test's temporary files live on the memory volume.
				outside, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				pass.TempRoots = []string{outside, bed.tmp}
				wantVolume = outside
			case "injected volume":
				pass.Volumes = []string{bed.tmp}
				wantVolume = bed.tmp
			case "injected empty":
				pass.Volumes = []string{}
			}
			own, err := diskSettingsFor(bed.inst)
			if err != nil {
				t.Fatal(err)
			}
			report, _, err := machinePass(context.Background(), bed.home, bed.inst, own, nil, pass)
			if err != nil {
				t.Fatal(err)
			}
			wantFloor := wantVolume != ""
			if active := report.Floor != nil && report.Floor.Active; active != wantFloor {
				t.Fatalf("floor active = %v, want %v", active, wantFloor)
			}
			if (processes.reads > 0) != wantFloor || (report.Census != nil) != wantFloor {
				t.Errorf("census reads = %d, report = %+v; want census = %v", processes.reads, report.Census, wantFloor)
			}
			if wantFloor {
				if len(report.Volumes) != 1 || report.Volumes[0].Path != wantVolume || !report.Volumes[0].BelowFloor || report.Volumes[0].FloorBytes != 1048576<<30 {
					t.Errorf("floor volumes = %+v; want only %s at a petabyte floor", report.Volumes, wantVolume)
				}
			} else if len(report.Volumes) != 0 {
				t.Errorf("test volumes were measured: %+v", report.Volumes)
			}
			wantStray := name == "explicit temp" || name == "outside temp"
			if wantStray {
				if len(report.Strays) != 1 || report.Strays[0].Path != stray {
					t.Errorf("strays = %+v; want only %s", report.Strays, stray)
				}
			} else if len(report.Strays) != 0 || len(report.Foreign) != 0 {
				t.Errorf("implicit temp root was scanned: strays %+v, foreign %+v", report.Strays, report.Foreign)
			}
			if len(report.Classes) < 4 {
				t.Errorf("machine classes did not run: %+v", report.Classes)
			}
		})
	}
}

func (p *memoryFloorProcesses) Pids() ([]int64, error) {
	p.reads++
	return nil, nil
}

func TestMachinePassExcludesTestMemoryVolumes(t *testing.T) {
	t.Parallel()
	memoryRoot := "/tmp/mst-" + strconv.Itoa(os.Geteuid())
	// No mount or marker is needed: the floor exclusion is purely lexical.
	if err := os.MkdirAll(memoryRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory", "private spelling", "forced memory", "outside", "injected memory", "injected empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parent := memoryRoot
			if name == "private spelling" {
				if runtime.GOOS != "darwin" {
					t.Skip("/private/tmp is the macOS spelling")
				}
				parent = "/private" + memoryRoot
			}
			root, err := os.MkdirTemp(parent, "floor-test-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(root); err != nil {
					t.Error(err)
				}
			})
			bed := staleBed{root: root, inst: filepath.Join(root, "repo", "metasystem"), home: filepath.Join(root, "home"), tmp: filepath.Join(root, "tmp")}
			bed.registry = filepath.Join(bed.home, "armed-checkouts.jsonl")
			evidenceRoot := filepath.Join(root, "evidence")
			for _, dir := range []string{filepath.Join(root, "repo", ".git"), bed.inst, bed.home, bed.tmp, evidenceRoot} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			// A petabyte floor exceeds both the fixture volume and the host disk.
			conf := "metasystem.template=true\ndisk.floor-gib=1048576\nevidence.root=" + evidenceRoot + "\n"
			if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte(conf), 0o600); err != nil {
				t.Fatal(err)
			}
			pass := bed.pass(diskstore.ModeReport, []string{bed.inst}, false)
			pass.Volumes = nil
			processes := &memoryFloorProcesses{}
			pass.Processes = processes
			switch name {
			case "forced memory":
				pass.FloorGiB = 1048576
			case "outside":
				// The package directory is on disk regardless of TMPDIR. Report mode
				// only reads temp roots; its lock, cursors and report stay under bed.home.
				outside, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				pass.TempRoots = append(pass.TempRoots, outside)
			case "injected memory":
				pass.Volumes = []string{bed.home}
			case "injected empty":
				pass.Volumes = []string{}
			}
			own, err := diskSettingsFor(bed.inst)
			if err != nil {
				t.Fatal(err)
			}
			report, _, err := machinePass(context.Background(), bed.home, bed.inst, own, nil, pass)
			if err != nil {
				t.Fatal(err)
			}
			wantFloor := name == "outside" || name == "injected memory"
			if active := report.Floor != nil && report.Floor.Active; active != wantFloor {
				t.Errorf("floor active = %v, want %v; report %+v", active, wantFloor, report)
			}
			if (processes.reads > 0) != wantFloor {
				t.Errorf("census reads = %d, want called = %v", processes.reads, wantFloor)
			}
			if !wantFloor && (len(report.Volumes) != 0 || report.Census != nil) {
				t.Errorf("excluded volumes were measured or took a census: %+v", report)
			}
			if wantFloor && (len(report.Volumes) != 1 || !report.Volumes[0].BelowFloor) {
				t.Errorf("retained volume did not reach the floor: %+v", report.Volumes)
			}
		})
	}
}
