package board

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRetireMovesTheCardsOfDoneGoals(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"done", "kept", "second-card", "name-taken", "ledger-error", "idempotent", "no-seat-dir", "rename-error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, seat := fixtureHome(t), seatOf("m1e")
			now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
			source := filepath.Join(Dir(home), seat.Machine, "goal-done.json")
			archive := func(at time.Time) string {
				return filepath.Join(Dir(home), RetiredNamespace, seat.Machine, "goal-done."+at.Format("20060102T150405Z")+".done.json")
			}
			read := func(path string) string {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			absent := func(path string) {
				t.Helper()
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s should be absent: %v", path, err)
				}
			}
			write := func(s Seat, id string, count int) string {
				t.Helper()
				if err := WriteAt(home, Card{Seat: s, Goal: id, Stage: StageLanded, Landed: count, Writer: Writer{At: now}}); err != nil {
					t.Fatal(err)
				}
				return read(filepath.Join(Dir(home), s.Machine, id+".json"))
			}
			ended := func(Seat) (map[string]string, error) {
				return map[string]string{"goal-done": ReasonDone, "goal-garbled": ReasonDone}, nil
			}
			seats := []Seat{seat}
			run := func(at time.Time) {
				t.Helper()
				if err := Retire(home, seats, ended, at); err != nil {
					t.Fatal(err)
				}
			}
			if name == "no-seat-dir" {
				run(now)
				absent(filepath.Dir(source))
				if got := Kept(home); len(got) != 0 {
					t.Fatalf("kept absent seat: %v", got)
				}
				return
			}
			if name == "rename-error" {
				if err := os.MkdirAll(filepath.Dir(archive(now)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := moveCard(home, seat.Machine, "goal-done", ReasonDone, now); err == nil {
					t.Fatal("missing source's rename error was discarded")
				}
				absent(archive(now))
				return
			}
			before := write(seat, "goal-done", 1)
			switch name {
			case "kept":
				paths := map[string]string{
					filepath.Join(Dir(home), seat.Machine, "goal-live.json"): write(seat, "goal-live", 0),
					filepath.Join(Dir(home), "m1f", "goal-done.json"):        write(seatOf("m1f"), "goal-done", 0),
				}
				garbled := filepath.Join(Dir(home), seat.Machine, "goal-garbled.json")
				if err := os.WriteFile(garbled, []byte("garbled"), 0600); err != nil {
					t.Fatal(err)
				}
				paths[garbled] = "garbled"
				run(now)
				for path, want := range paths {
					if got := read(path); got != want {
						t.Fatalf("%s changed: %q", path, got)
					}
				}
			case "name-taken":
				if err := os.MkdirAll(filepath.Dir(archive(now)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(archive(now), []byte("sentinel"), 0600); err != nil {
					t.Fatal(err)
				}
				run(now)
				want := "card kept: m1e/goal-done: goal-done.20261021T120000Z.done.json exists or cannot be checked"
				if read(source) != before || read(archive(now)) != "sentinel" || !slices.Equal(Kept(home), []string{want}) {
					t.Fatalf("collision lost bytes or diagnostic: %v", Kept(home))
				}
				run(now.Add(time.Second))
				if read(archive(now.Add(time.Second))) != before {
					t.Fatal("later archive lost card bytes")
				}
			case "ledger-error":
				ui := seatOf("ui")
				uiBytes := write(ui, "goal-done", 0)
				seats = []Seat{ui, seat}
				ended = func(s Seat) (map[string]string, error) {
					if s.Machine == "ui" {
						return nil, errors.New("ledger unreadable\nretry later")
					}
					return map[string]string{"goal-done": ReasonDone}, nil
				}
				run(now)
				if read(filepath.Join(Dir(home), "ui", "goal-done.json")) != uiBytes || !slices.Equal(Kept(home), []string{"card kept: ui/*: ledger unreadable; retry later"}) {
					t.Fatalf("erroring seat: %v", Kept(home))
				}
			default:
				run(now)
			}
			if name != "name-taken" && read(archive(now)) != before {
				t.Fatal("archive changed the card bytes")
			}
			absent(source)
			picture, _ := Read(home, []Seat{seat}, nil, now, time.Hour)
			for _, card := range picture.Cards {
				if card.Goal == "goal-done" {
					t.Fatal("retired card still read")
				}
			}
			for _, card := range picture.Unknown {
				if card.Goal == "goal-done" {
					t.Fatal("retired card still unknown")
				}
			}
			if name == "second-card" {
				second := write(seat, "goal-done", 2)
				run(now.Add(time.Minute))
				if read(archive(now)) != before || read(archive(now.Add(time.Minute))) != second || before == second {
					t.Fatal("distinct cards did not retain their own bytes")
				}
			}
			if name == "idempotent" {
				run(now)
				entries, err := os.ReadDir(filepath.Dir(archive(now)))
				if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(archive(now)) || read(archive(now)) != before {
					t.Fatalf("second pass changed archive: %v %v", entries, err)
				}
			}
			if name != "ledger-error" && (len(Kept(home)) != 0 || strings.TrimSpace(read(filepath.Join(Dir(home), RetiredNamespace, "kept.txt"))) != "") {
				t.Fatalf("unexpected kept lines: %v", Kept(home))
			}
		})
	}
}
