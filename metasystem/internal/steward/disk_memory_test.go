package steward

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type memoryFloorProcesses struct {
	identity.ListedProcessTable
	reads int
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
