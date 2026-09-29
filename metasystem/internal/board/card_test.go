package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)

func fixtureHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), ".metasystem")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func seatOf(machine string) Seat {
	return Seat{Machine: machine, Installation: "/checkouts/" + machine + "/metasystem"}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// TestBoardNamesAreConfinedAndPrivate (R24, U10a-1): a nickname or goal that
// could leave the board is refused with BOARD_NAME_UNSAFE naming the
// enrollment remedy and nothing is written outside it; an existing 0755 board
// directory is tightened; a symlinked seat directory is refused; a card is
// 0600 in a 0700 directory; the reader lists no unsafe directory.
func TestBoardNamesAreConfinedAndPrivate(t *testing.T) {
	t.Parallel()
	home := fixtureHome(t)
	outside := filepath.Dir(home)
	for _, card := range []Card{
		{Seat: Seat{Machine: "../../../Documents", Installation: "/x"}, Goal: "g", Stage: StageBuild},
		{Seat: seatOf("m1b"), Goal: "../x", Stage: StageBuild},
		{Seat: Seat{Machine: "..", Installation: "/x"}, Goal: "g", Stage: StageBuild},
		{Seat: seatOf("m1b"), Goal: "a/b", Stage: StageBuild},
	} {
		err := WriteAt(home, card)
		if err == nil || !strings.Contains(err.Error(), "BOARD_NAME_UNSAFE") || !strings.Contains(err.Error(), "git config metasystem.goal.machine NAME") {
			t.Fatalf("WriteAt(%q/%q) = %v, want BOARD_NAME_UNSAFE with the enrollment remedy", card.Seat.Machine, card.Goal, err)
		}
	}
	var stray []string
	_ = filepath.Walk(outside, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			stray = append(stray, path)
		}
		return nil
	})
	if len(stray) != 0 {
		t.Fatalf("an unsafe name wrote files: %v", stray)
	}

	// An existing permissive board directory is tightened before the write.
	if err := os.MkdirAll(filepath.Join(home, "host", "board"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, "host"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, "host", "board"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteAt(home, Card{Seat: seatOf("m1b"), Goal: "goal-x", Stage: StageClaimedIdle, Writer: Writer{At: t0}}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"host", "host/board", "host/board/m1b"} {
		if got := mode(t, filepath.Join(home, dir)); got != 0o700 {
			t.Errorf("%s mode %o, want 0700", dir, got)
		}
	}
	if got := mode(t, filepath.Join(home, "host", "board", "m1b", "goal-x.json")); got != 0o600 {
		t.Errorf("card mode %o, want 0600", got)
	}

	// A seat directory that is a symlink is refused, and nothing is written
	// through it.
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(home, "host", "board", "m1c")); err != nil {
		t.Fatal(err)
	}
	err := WriteAt(home, Card{Seat: seatOf("m1c"), Goal: "goal-y", Stage: StageBuild, Writer: Writer{At: t0}})
	if err == nil || !strings.Contains(err.Error(), "BOARD_PATH_NOT_A_DIRECTORY") {
		t.Fatalf("write through a symlinked seat directory = %v, want BOARD_PATH_NOT_A_DIRECTORY", err)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("the write followed the symlink: %v", entries)
	}

	// The reader lists no directory whose name fails the rule.
	if err := os.Mkdir(filepath.Join(home, "host", "board", "bad name"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, unreadable := Read(home, []Seat{seatOf("m1b")}, aliveProber{}, t0, 20*time.Minute)
	for _, u := range unreadable {
		if strings.Contains(u.Path, "bad name") {
			t.Fatalf("the reader listed an unsafe directory: %+v", u)
		}
	}
}

// TestCardCarriesTheClaimsHistory: a stage change closes the previous stage
// into Stages; the same stage keeps Since and a repeated event advances
// nothing; a card after a terminal one starts a new history.
func TestCardCarriesTheClaimsHistory(t *testing.T) {
	t.Parallel()
	home := fixtureHome(t)
	seat := seatOf("m1b")
	write := func(card Card) Card {
		t.Helper()
		card.Seat, card.Goal = seat, "goal-x"
		if err := WriteAt(home, card); err != nil {
			t.Fatal(err)
		}
		read, ok := readCardFile(filepath.Join(Dir(home), "m1b", "goal-x.json"))
		if !ok {
			t.Fatal("card unreadable after write")
		}
		return read
	}
	write(Card{Stage: StageClaimedIdle, Writer: Writer{At: t0}})
	proof := write(Card{Stage: StageUnitProof, Proof: &Proof{Attempt: "a1", Done: 1, Planned: 189}, Writer: Writer{At: t0.Add(time.Minute)}})
	if len(proof.Stages) != 1 || proof.Stages[0].Stage != StageClaimedIdle || !proof.Stages[0].Until.Equal(t0.Add(time.Minute)) {
		t.Fatalf("stage change did not close the previous stage: %+v", proof.Stages)
	}
	repeat := write(Card{Stage: StageUnitProof, Proof: &Proof{Attempt: "a1", Done: 1, Planned: 189}, Writer: Writer{At: t0.Add(30 * time.Minute)}})
	if !repeat.LastProgressAt.Equal(t0.Add(time.Minute)) || !repeat.Since.Equal(t0.Add(time.Minute)) {
		t.Fatalf("a repeated event moved the card: since %v progress %v", repeat.Since, repeat.LastProgressAt)
	}
	more := write(Card{Stage: StageUnitProof, Proof: &Proof{Attempt: "a1", Done: 2, Planned: 189}, Writer: Writer{At: t0.Add(31 * time.Minute)}})
	if !more.LastProgressAt.Equal(t0.Add(31 * time.Minute)) {
		t.Fatalf("a section more done is progress: %v", more.LastProgressAt)
	}
	write(Card{Stage: StageReleased, Writer: Writer{At: t0.Add(40 * time.Minute)}})
	fresh := write(Card{Stage: StageClaimedIdle, Writer: Writer{At: t0.Add(50 * time.Minute)}})
	if len(fresh.Stages) != 0 || fresh.SchemaVersion != SchemaVersion || fresh.Writer.Pid != int64(os.Getpid()) {
		t.Fatalf("a new claim after a terminal card must start fresh: %+v", fresh)
	}
}

// TestWriteLandsBesideTheRegistry: Write publishes under Home, which the
// registry's fixture variable redirects (testenv sets it for every test
// process), and Self names this process as a card's owner.
func TestWriteLandsBesideTheRegistry(t *testing.T) {
	t.Parallel()
	home, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	override := os.Getenv(registryHomeEnv)
	if override == "" || home != filepath.Join(override, ".metasystem") {
		t.Fatalf("Home() = %q, want the registry's run-scoped home under %q", home, override)
	}
	owner := Self()
	if owner.Pid != int64(os.Getpid()) || owner.PidStartedAt == 0 {
		t.Fatalf("Self() = %+v, want this process with its start", owner)
	}
	card := Card{Seat: seatOf("m1-write"), Goal: "goal-w", Stage: StageBuild, Owner: owner}
	if err := Write(card); err != nil {
		t.Fatal(err)
	}
	stored, ok := readCardFile(filepath.Join(Dir(home), "m1-write", "goal-w.json"))
	if !ok || stored.Owner == nil || *stored.Owner != *owner {
		t.Fatalf("card under Home = %+v, %v", stored, ok)
	}
}
