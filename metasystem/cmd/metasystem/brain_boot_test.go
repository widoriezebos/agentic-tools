package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

const brainBootTestPacket = "# Fixture brain packet\n\n## The standing instruction\nKeep going.\n"

func useNonFiringBrainBootTimer(t *testing.T) {
	t.Helper()
	originalNow, originalTimer := brainBootNow, newBrainBootTimer
	now := time.Unix(1, 0)
	brainBootNow = func() time.Time { return now }
	newBrainBootTimer = func(time.Duration) brainBootTimer {
		return brainBootTimer{C: make(chan time.Time), Stop: func() bool { return true }}
	}
	t.Cleanup(func() {
		brainBootNow, newBrainBootTimer = originalNow, originalTimer
	})
}

func declaredBrainBootTestRoot(t *testing.T) (string, func(string) string) {
	t.Helper()
	repository := newProofAdmissionRepositoryFixture(t, time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC), false)
	root := repository.root
	identity := func(actualRoot string) string {
		t.Helper()
		if actualRoot != root {
			t.Fatalf("brain ledger root %q, want %q", actualRoot, root)
		}
		return goal.ExistingLedgerIdentityAtEndpoint(goal.Endpoint{Root: root, Repository: repository})
	}
	if err := os.MkdirAll(filepath.Join(root, "records", "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, brain.PacketRelativePath), []byte(brainBootTestPacket), 0o644); err != nil {
		t.Fatal(err)
	}
	record := brain.Record{
		Schema: brain.Schema, Ledger: identity(root), Machine: "mac-cli",
		DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, identity
}

func brainBootTestLayoutReader(root string) func(string) (stateroot.Layout, error) {
	return func(actualRoot string) (stateroot.Layout, error) {
		if actualRoot != root {
			panic(fmt.Sprintf("brain digest root %q, want %q", actualRoot, root))
		}
		return stateroot.Layout{
			GitRoot: root, RepositoryRoot: root, InstallationRoot: root, InstallationRel: ".",
		}, nil
	}
}

func TestBrainBootKeepsPhaseOneWhenOptionalInputChildFails(t *testing.T) {
	useNonFiringBrainBootTimer(t)
	root, identity := declaredBrainBootTestRoot(t)

	original := newBrainBootInputsCommand
	newBrainBootInputsCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 23")
	}
	t.Cleanup(func() { newBrainBootInputsCommand = original })

	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 5000, false, identity)
	if err != nil {
		t.Fatal(err)
	}
	if !output.Declared || !strings.Contains(output.Payload, brainBootTestPacket) || !strings.Contains(output.Payload, "BOOT DEADLINE: asks, held, fleet, digest not read") {
		t.Fatalf("optional-input failure discarded phase one or its diagnostic: %+v", output)
	}
	for _, name := range []string{"asks", "held", "fleet", "digest"} {
		if output.Sections[name] != "skipped" {
			t.Fatalf("section %s = %q, want skipped", name, output.Sections[name])
		}
	}
}

func TestBrainBootDeadlineKeepsCompletedSections(t *testing.T) {
	if os.Getenv("GO_WANT_BRAIN_BOOT_DEADLINE_HELPER") == "1" {
		terminated := make(chan os.Signal, 1)
		signal.Notify(terminated, syscall.SIGTERM)
		if err := os.WriteFile(os.Getenv("BRAIN_BOOT_ASKS_PATH"), []byte(`{"status":"complete","lines":[{"text":"kept ask"}]}`+"\n"), 0o600); err != nil {
			os.Exit(97)
		}
		ready := os.NewFile(3, "brain-boot-ready")
		if ready == nil {
			os.Exit(97)
		}
		if _, err := ready.Write([]byte{'x'}); err != nil || ready.Close() != nil {
			os.Exit(97)
		}
		<-terminated
		if err := os.WriteFile(os.Getenv("BRAIN_BOOT_TERM_PATH"), []byte("term"), 0o600); err != nil {
			os.Exit(97)
		}
		os.Exit(0)
	}
	root, identity := declaredBrainBootTestRoot(t)
	originalCommand, originalNow, originalTimer := newBrainBootInputsCommand, brainBootNow, newBrainBootTimer
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = readyRead.Close()
		_ = readyWrite.Close()
	})
	termPath := filepath.Join(t.TempDir(), "term")
	newBrainBootInputsCommand = func(_ string, args ...string) *exec.Cmd {
		dir := ""
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "--dir" {
				dir = args[index+1]
				break
			}
		}
		if dir == "" {
			t.Fatal("boot-inputs command omitted --dir")
		}
		command := exec.Command(os.Args[0], "-test.run=^TestBrainBootDeadlineKeepsCompletedSections$")
		command.Env = append(os.Environ(), "GO_WANT_BRAIN_BOOT_DEADLINE_HELPER=1",
			"BRAIN_BOOT_ASKS_PATH="+filepath.Join(dir, "asks.json"), "BRAIN_BOOT_TERM_PATH="+termPath)
		command.ExtraFiles = []*os.File{readyWrite}
		return command
	}
	brainBootNow = func() time.Time { return time.Unix(1, 0) }
	fired := 0
	var firedDuration time.Duration
	newBrainBootTimer = func(duration time.Duration) brainBootTimer {
		if fired == 0 {
			_ = readyWrite.Close()
			var signal [1]byte
			if _, err := io.ReadFull(readyRead, signal[:]); err != nil {
				t.Fatalf("wait for optional-input section: %v", err)
			}
			fired++
			firedDuration = duration
			ch := make(chan time.Time, 1)
			ch <- brainBootNow()
			return brainBootTimer{C: ch, Stop: func() bool { return false }}
		}
		return brainBootTimer{C: make(chan time.Time), Stop: func() bool { return true }}
	}
	t.Cleanup(func() {
		newBrainBootInputsCommand, brainBootNow, newBrainBootTimer = originalCommand, originalNow, originalTimer
	})

	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 250, false, identity)
	if err != nil {
		t.Fatal(err)
	}
	if fired != 1 || firedDuration != 250*time.Millisecond {
		t.Fatalf("deadline timer fired %d times at %s, want once at 250ms", fired, firedDuration)
	}
	if data, err := os.ReadFile(termPath); err != nil || string(data) != "term" {
		t.Fatalf("deadline did not signal the optional-input process group: data=%q err=%v", data, err)
	}
	if output.Sections["asks"] != "complete" || !strings.Contains(output.Payload, "kept ask") {
		t.Fatalf("completed asks section was discarded after the deadline: %+v", output)
	}
	for _, name := range []string{"held", "fleet", "digest"} {
		if output.Sections[name] != "skipped" {
			t.Fatalf("unfinished section %s = %q, want skipped", name, output.Sections[name])
		}
	}
	if !strings.Contains(output.Payload, "BOOT DEADLINE: held, fleet, digest not read") || strings.Contains(output.Payload, "BOOT DEADLINE: asks") {
		t.Fatalf("deadline line did not name only unfinished sections: %q", output.Payload)
	}
}

func TestBrainBootDeadlineKeepsThreeCompletedSections(t *testing.T) {
	t.Parallel()

	switch os.Getenv("GO_WANT_BRAIN_BOOT_THREE_SECTIONS_HELPER") {
	case "body":
		runBrainBootThreeSectionsBody(t)
	case "sections":
		runBrainBootThreeSectionsChild()
	case "":
		command := exec.Command(commandTestExecutable(t), "-test.run=^TestBrainBootDeadlineKeepsThreeCompletedSections$", "-test.count=1")
		command.Env = brainBootThreeSectionsEnvironment("body")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("isolated three-section deadline witness failed: %v\n%s", err, output)
		}
	default:
		t.Fatalf("unknown three-section deadline helper role %q", os.Getenv("GO_WANT_BRAIN_BOOT_THREE_SECTIONS_HELPER"))
	}
}

func runBrainBootThreeSectionsChild() {
	terminated := make(chan os.Signal, 1)
	signal.Notify(terminated, syscall.SIGTERM)
	sections := []struct {
		path    string
		payload string
	}{
		{os.Getenv("BRAIN_BOOT_ASKS_PATH"), `{"status":"complete","lines":[{"text":"kept ask"}]}` + "\n"},
		{os.Getenv("BRAIN_BOOT_HELD_PATH"), `{"status":"complete","lines":[{"text":"kept held goal"}]}` + "\n"},
		{os.Getenv("BRAIN_BOOT_FLEET_PATH"), `{"status":"complete","lines":[{"text":"kept fleet goal"}]}` + "\n"},
	}
	for _, section := range sections {
		if err := os.WriteFile(section.path, []byte(section.payload), 0o600); err != nil {
			os.Exit(97)
		}
	}
	ready := os.NewFile(3, "brain-boot-three-sections-ready")
	if ready == nil {
		os.Exit(97)
	}
	if _, err := ready.Write([]byte{'x'}); err != nil || ready.Close() != nil {
		os.Exit(97)
	}
	<-terminated
	if err := os.WriteFile(os.Getenv("BRAIN_BOOT_TERM_PATH"), []byte("term"), 0o600); err != nil {
		os.Exit(97)
	}
	os.Exit(0)
}

func runBrainBootThreeSectionsBody(t *testing.T) {
	t.Helper()

	root, identity := declaredBrainBootTestRoot(t)
	originalCommand, originalNow, originalTimer := newBrainBootInputsCommand, brainBootNow, newBrainBootTimer
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = readyRead.Close()
		_ = readyWrite.Close()
	})
	termPath := filepath.Join(t.TempDir(), "term")
	newBrainBootInputsCommand = func(_ string, args ...string) *exec.Cmd {
		dir := ""
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "--dir" {
				dir = args[index+1]
				break
			}
		}
		if dir == "" {
			t.Fatal("boot-inputs command omitted --dir")
		}
		command := exec.Command(commandTestExecutable(t), "-test.run=^TestBrainBootDeadlineKeepsThreeCompletedSections$", "-test.count=1")
		command.Env = brainBootThreeSectionsEnvironment("sections",
			"BRAIN_BOOT_ASKS_PATH="+filepath.Join(dir, "asks.json"),
			"BRAIN_BOOT_HELD_PATH="+filepath.Join(dir, "held.json"),
			"BRAIN_BOOT_FLEET_PATH="+filepath.Join(dir, "fleet.json"),
			"BRAIN_BOOT_TERM_PATH="+termPath)
		command.ExtraFiles = []*os.File{readyWrite}
		return command
	}
	brainBootNow = func() time.Time { return time.Unix(1, 0) }
	fired := 0
	var firedDuration time.Duration
	newBrainBootTimer = func(duration time.Duration) brainBootTimer {
		if fired == 0 {
			_ = readyWrite.Close()
			var ready [1]byte
			if _, err := io.ReadFull(readyRead, ready[:]); err != nil {
				t.Fatalf("wait for three optional-input sections: %v", err)
			}
			fired++
			firedDuration = duration
			ch := make(chan time.Time, 1)
			ch <- brainBootNow()
			return brainBootTimer{C: ch, Stop: func() bool { return false }}
		}
		return brainBootTimer{C: make(chan time.Time), Stop: func() bool { return true }}
	}
	t.Cleanup(func() {
		newBrainBootInputsCommand, brainBootNow, newBrainBootTimer = originalCommand, originalNow, originalTimer
	})

	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 250, false, identity)
	if err != nil {
		t.Fatal(err)
	}
	if fired != 1 || firedDuration != 250*time.Millisecond {
		t.Fatalf("deadline timer fired %d times at %s, want once at 250ms", fired, firedDuration)
	}
	if data, err := os.ReadFile(termPath); err != nil || string(data) != "term" {
		t.Fatalf("deadline did not synchronously terminate the optional-input process group: data=%q err=%v", data, err)
	}
	for _, name := range []string{"asks", "held", "fleet"} {
		if output.Sections[name] != "complete" {
			t.Fatalf("published section %s = %q, want complete", name, output.Sections[name])
		}
	}
	if output.Sections["digest"] != "skipped" {
		t.Fatalf("unpublished digest section = %q, want skipped", output.Sections["digest"])
	}
	for _, retained := range []string{"kept ask", "kept held goal", "kept fleet goal"} {
		if !strings.Contains(output.Payload, retained) {
			t.Fatalf("deadline discarded %q from payload: %q", retained, output.Payload)
		}
	}
	if !strings.Contains(output.Payload, "BOOT DEADLINE: digest not read") ||
		strings.Contains(output.Payload, "BOOT DEADLINE: asks") ||
		strings.Contains(output.Payload, "BOOT DEADLINE: held") ||
		strings.Contains(output.Payload, "BOOT DEADLINE: fleet") {
		t.Fatalf("deadline line did not name only the unpublished digest: %q", output.Payload)
	}
}

func brainBootThreeSectionsEnvironment(role string, values ...string) []string {
	replaced := map[string]bool{
		"GO_WANT_BRAIN_BOOT_THREE_SECTIONS_HELPER": true,
		"BRAIN_BOOT_ASKS_PATH":                     true,
		"BRAIN_BOOT_HELD_PATH":                     true,
		"BRAIN_BOOT_FLEET_PATH":                    true,
		"BRAIN_BOOT_TERM_PATH":                     true,
	}
	environment := make([]string, 0, len(os.Environ())+len(values)+1)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !replaced[name] {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "GO_WANT_BRAIN_BOOT_THREE_SECTIONS_HELPER="+role)
	return append(environment, values...)
}

func TestBrainBootDeliveryHelper(t *testing.T) {
	if os.Getenv("GO_WANT_BRAIN_BOOT_DELIVERY_HELPER") != "1" {
		t.Skip("optional-input child only")
	}
	if os.Getenv("BRAIN_BOOT_DELIVERY_DIGEST") == "1" {
		root, dir := os.Getenv("BRAIN_BOOT_DELIVERY_ROOT"), os.Getenv("BRAIN_BOOT_DELIVERY_DIR")
		if err := writeBrainBootSection(dir, "digest", readBrainDigestWithLayoutReader(root, brainBootTestLayoutReader(root))); err != nil {
			t.Fatal(err)
		}
	}
}

func useBrainBootDeliveryChild(t *testing.T, root string, digest bool) {
	t.Helper()
	original := newBrainBootInputsCommand
	newBrainBootInputsCommand = func(_ string, args ...string) *exec.Cmd {
		dir := ""
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "--dir" {
				dir = args[index+1]
				break
			}
		}
		if dir == "" {
			t.Fatal("boot-inputs command omitted --dir")
		}
		command := exec.Command(os.Args[0], "-test.run=^TestBrainBootDeliveryHelper$", "-test.count=1")
		command.Env = append(os.Environ(), "GO_WANT_BRAIN_BOOT_DELIVERY_HELPER=1",
			"BRAIN_BOOT_DELIVERY_ROOT="+root, "BRAIN_BOOT_DELIVERY_DIR="+dir)
		if digest {
			command.Env = append(command.Env, "BRAIN_BOOT_DELIVERY_DIGEST=1")
		}
		return command
	}
	t.Cleanup(func() { newBrainBootInputsCommand = original })
}

func TestBrainReadOnlyBootDefersStatusUntilStartDelivered(t *testing.T) {
	useNonFiringBrainBootTimer(t)
	root, identity := declaredBrainBootTestRoot(t)
	useBrainBootDeliveryChild(t, root, false)
	statusPath := brain.StatusPath(root)

	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 50, true, identity)
	if err != nil {
		t.Fatal(err)
	}
	if !output.Declared || len(output.DeclarationSHA256) != 64 {
		t.Fatalf("read-only output omitted declaration authorization: %+v", output)
	}
	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Fatalf("read-only boot wrote status before publication: %v", err)
	}
	if status := runBrainStartDeliveredWithReaders([]string{
		"--root", root, "--repo", root, "--declaration-sha256", output.DeclarationSHA256,
	}, identity, brainBootTestLayoutReader(root)); status != 0 {
		t.Fatalf("start-delivered status = %d", status)
	}
	if _, err := brain.ReadStatus(root); err != nil {
		t.Fatalf("delivery acknowledgment did not write status: %v", err)
	}
}

func TestBrainStartDeliveredRefusesChangedDeclaration(t *testing.T) {
	useNonFiringBrainBootTimer(t)
	root, identity := declaredBrainBootTestRoot(t)
	useBrainBootDeliveryChild(t, root, false)
	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 50, true, identity)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(brain.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	var record brain.Record
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record.DeclaredBy = "another human"
	changed, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(changed, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := runBrainStartDeliveredWithReaders([]string{
		"--root", root, "--repo", root, "--declaration-sha256", output.DeclarationSHA256,
	}, identity, brainBootTestLayoutReader(root)); status != 1 {
		t.Fatalf("changed declaration status = %d, want 1", status)
	}
	if _, err := os.Stat(brain.StatusPath(root)); !os.IsNotExist(err) {
		t.Fatalf("changed declaration wrote status: %v", err)
	}
}

func TestBrainStartDeliveredAdvancesOnlyTheEmittedBrainDigest(t *testing.T) {
	useNonFiringBrainBootTimer(t)
	root, identity := declaredBrainBootTestRoot(t)
	useBrainBootDeliveryChild(t, root, true)
	if err := os.MkdirAll(filepath.Join(root, "records"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "records", "narrator-digest.log"), []byte("delivered digest line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := composeBrainBootModeWithIdentity(root, root, minimumBrainContextBytes, 50, true, identity)
	if err != nil {
		t.Fatal(err)
	}
	resolveLayout := brainBootTestLayoutReader(root)
	pending, err := narratordigest.PendingWithLayoutReader(root, resolveLayout, "brain")
	if err != nil || pending.Cursor == 0 || pending.PrefixSHA256 == "" {
		t.Fatalf("cannot prepare emitted digest coordinates: %+v %v", pending, err)
	}
	if !strings.Contains(output.Payload, "delivered digest line") || !output.DigestEmitted ||
		output.DigestCursor != pending.Cursor || output.DigestPrefixSHA256 != pending.PrefixSHA256 {
		t.Fatalf("composed output did not emit the pending digest: output=%+v pending=%+v", output, pending)
	}
	if status := runBrainStartDeliveredWithReaders([]string{
		"--root", root, "--repo", root,
		"--declaration-sha256", output.DeclarationSHA256,
		"--digest-cursor", fmt.Sprint(output.DigestCursor),
		"--digest-prefix-sha256", output.DigestPrefixSHA256,
	}, identity, resolveLayout); status != 0 {
		t.Fatalf("start-delivered status = %d", status)
	}
	brainCursorPath := narratordigest.CursorPathWithLayoutReader(root, resolveLayout, "brain")
	brainCursor, err := os.ReadFile(brainCursorPath)
	if err != nil {
		t.Fatalf("brain digest cursor was not advanced: %v", err)
	}
	var cursor struct {
		Cursor       int64  `json:"cursor"`
		PrefixSHA256 string `json:"prefixSha256"`
	}
	if err := json.Unmarshal(brainCursor, &cursor); err != nil || cursor.Cursor != pending.Cursor || cursor.PrefixSHA256 != pending.PrefixSHA256 {
		t.Fatalf("brain digest cursor = %+v, want %+v: %v", cursor, pending, err)
	}
	if _, err := os.Stat(narratordigest.CursorPathWithLayoutReader(root, resolveLayout)); !os.IsNotExist(err) {
		t.Fatalf("start delivery changed the human digest cursor: %v", err)
	}
}
