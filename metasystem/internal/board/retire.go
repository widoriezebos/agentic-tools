package board

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// RetiredNamespace is the board directory that keeps retired cards,
// retired/<seat>/<goal>.<retiredAt>.<reason>.json, and kept.txt: the cards
// the last pass left where they were, and why.
const RetiredNamespace = "retired"

// ReasonDone retires a card whose goal its seat's ledger records as done.
const ReasonDone = "done"

// Retire moves, under each seat's lock, the cards of the armed seats whose
// goal ended(seat) names, each by one rename; the archive's name carries the
// time and reason. A card it cannot move stays, with a line in kept.txt, for
// a later pass.
func Retire(home string, seats []Seat, ended func(Seat) (map[string]string, error), now time.Time) error {
	var kept []string
	for _, seat := range seats {
		if SafeName(seat.Machine) {
			kept = append(kept, retireSeat(home, seat, ended, now.UTC())...)
		}
	}
	dir := filepath.Join(Dir(home), RetiredNamespace)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_, err := atomicfile.WriteText(filepath.Join(dir, "kept.txt"), strings.Join(kept, "\n"), home)
	return err
}

// retireSeat reads the seat's ledger and cards under its lock and returns
// a kept line for each card it could not move.
func retireSeat(home string, seat Seat, endedOf func(Seat) (map[string]string, error), now time.Time) []string {
	dir := filepath.Join(Dir(home), seat.Machine)
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return nil
	}
	unlock, err := lockSeat(dir)
	if err != nil {
		return []string{keptLine(seat.Machine, "*", err)}
	}
	defer unlock()
	ended, err := endedOf(seat)
	if err != nil {
		return []string{keptLine(seat.Machine, "*", err)}
	}
	cards, _ := readSeat(dir, seat)
	var kept []string
	for _, card := range cards {
		if reason := ended[card.Goal]; reason != "" {
			if err := moveCard(home, seat.Machine, card.Goal, reason, now); err != nil {
				kept = append(kept, keptLine(seat.Machine, card.Goal, err))
			}
		}
	}
	return kept
}

func keptLine(seat, goal string, err error) string {
	return "card kept: " + seat + "/" + goal + ": " + strings.ReplaceAll(err.Error(), "\n", "; ")
}

// moveCard renames the card to its archive name, never onto a file.
func moveCard(home, machine, goal, reason string, now time.Time) error {
	dir := filepath.Join(Dir(home), RetiredNamespace, machine)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	to := filepath.Join(dir, goal+"."+now.Format("20060102T150405Z")+"."+reason+".json")
	if _, err := os.Lstat(to); !errors.Is(err, fs.ErrNotExist) {
		return errors.New(filepath.Base(to) + " exists or cannot be checked")
	}
	return os.Rename(filepath.Join(Dir(home), machine, goal+".json"), to)
}

// Kept is the last pass's kept lines; none when it kept nothing.
func Kept(home string) []string {
	data, err := os.ReadFile(filepath.Join(Dir(home), RetiredNamespace, "kept.txt"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
