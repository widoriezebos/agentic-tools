package proofrun

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
)

// K9 through the real launcher (critique F-3): a proof the landing lane's
// kernel launched carries its custody record in the environment, and the
// launcher binds the group the suite leads to it. With the execution's own
// process already ended, the record stays live while the suite runs and
// settles once it has ended.
func TestLaunchSuiteBindsTheSuiteGroupToTheLanesCustody(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	record, err := custody.Open(home, custody.KindProve, "batch b1 subject batch", time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	ended := exec.Command("sleep", "120")
	if err := ended.Start(); err != nil {
		t.Fatal(err)
	}
	endedExact, _, err := (identity.KernelProber{}).Probe(int64(ended.Process.Pid))
	_ = ended.Process.Kill()
	_ = ended.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if err := custody.BindChild(home, record.ID, endedExact.Ref()); err != nil {
		t.Fatal(err)
	}
	t.Setenv(custody.EnvHome, home)
	t.Setenv(custody.EnvID, record.ID)

	watching, release := filepath.Join(root, "watching.fifo"), filepath.Join(root, "release.fifo")
	for _, fifo := range []string{watching, release} {
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	watchdog := filepath.Join(root, "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
echo up >`+watching+`
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	progress := filepath.Join(root, "progress.jsonl")
	suite := `printf '{"suite":"fixture","section":"only","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
IFS= read -r _ <"$2"
printf '{"suite":"fixture","section":"only","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	var output, errorsOut bytes.Buffer
	result := make(chan int, 1)
	go func() {
		result <- LaunchSuite(LaunchOptions{
			Suite: "fixture", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"), ProgressPath: progress, LogPath: filepath.Join(root, "logs", "suite.log"),
			TmpPaths: []string{filepath.Join(root, "tmp")}, Banner: "suite-cost suite=fixture witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log",
			ExpectedSections: []string{"only"}, TwiceConsulted: map[string]bool{},
			Silence: time.Minute, SectionCap: time.Minute, EvidenceTimeout: time.Second, EvidenceMax: 1024,
			Poll: 10 * time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
			WatchdogExecutable: watchdog, Command: []string{"bash", "-c", suite, "fixture", progress, release},
			Environment: []string{"PATH=" + os.Getenv("PATH")},
			Output:      &output, ErrorOutput: &errorsOut,
		})
		// A launch that ends early still unblocks the read below.
		if fifo, err := os.OpenFile(watching, os.O_RDWR, 0); err == nil {
			_, _ = fifo.WriteString("ended\n")
			_ = fifo.Close()
		}
	}()
	reader, err := os.Open(watching)
	if err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(reader).ReadString('\n')
	_ = reader.Close()
	if line != "up\n" {
		t.Fatalf("the launch ended before its watchdog ran (%q): %d\n%s%s", line, <-result, output.String(), errorsOut.String())
	}
	state, err := custody.Probe(home, record.ID, custody.Probes{})
	if err != nil || state.State != custody.Live {
		t.Fatalf("custody while the suite runs = %+v %v; want live through the suite's group", state, err)
	}
	writer, err := os.OpenFile(release, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = writer.WriteString("go\n")
	_ = writer.Close()
	if code := <-result; code != 0 {
		t.Fatalf("launch = %d\n%s%s", code, output.String(), errorsOut.String())
	}
	state, err = custody.Probe(home, record.ID, custody.Probes{})
	if err != nil || state.State != custody.Dead {
		t.Fatalf("custody after the suite ended = %+v %v; want settled", state, err)
	}
	stored, _, _ := custody.Read(home, record.ID)
	launch, err := ReadRecord(root, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	bound := map[int64]bool{}
	for _, group := range stored.Groups {
		bound[group.ID] = group.Leader != ""
	}
	if !bound[launch.SuiteProcess.Pid] || !bound[launch.Watchdog.Pid] {
		t.Fatalf("groups = %+v; want the suite's (%d) and the watchdog's (%d), each with its leader", stored.Groups, launch.SuiteProcess.Pid, launch.Watchdog.Pid)
	}
}
