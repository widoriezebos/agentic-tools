package stopreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func reportFixture(t *testing.T, root, session, attempt string) (string, []byte) {
	t.Helper()
	key := SessionKey("claude", session)
	id := key + "-" + attempt
	identity := Identity{Installation: root, Runtime: "claude", Session: session, SessionKey: key, Attempt: attempt, MainId: "main-1", ObservedAt: "2026-09-14T12:00:00Z"}
	encoded, _ := json.Marshal(identity)
	data := []byte("# Task: test; Stop blocked\n\n<!-- metasystem-stop-report-v1 " + string(encoded) + " -->\n")
	dir, err := ReportDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return id, data
}

func storageRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func bindFixture(t *testing.T, root, session, attempt string) (Reservation, string, []byte) {
	t.Helper()
	id, data := reportFixture(t, root, session, attempt)
	reservation, err := ReserveShortestAlias(root, id)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if err := PublishAlias(reservation, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	return reservation, id, data
}

func TestStopAliasReservesShortestFreePrefix(t *testing.T) {
	root := storageRoot(t)
	attempt := "15fd2d40fae99afd9756ffd62c6903a7"
	id := SessionKey("claude", "one") + "-" + attempt
	first, err := ReserveShortestAlias(root, id)
	if err != nil || first.Alias != "1" {
		t.Fatalf("first reservation = %+v, %v", first, err)
	}
	secondID := SessionKey("claude", "two") + "-" + attempt
	second, err := ReserveShortestAlias(root, secondID)
	if err != nil || second.Alias != "15" {
		t.Fatalf("second reservation = %+v, %v", second, err)
	}
	thirdID := SessionKey("claude", "three") + "-" + attempt
	third, err := ReserveShortestAlias(root, thirdID)
	if err != nil || third.Alias != "15f" {
		t.Fatalf("third reservation = %+v, %v", third, err)
	}
	for _, test := range []struct {
		alias string
		bytes int
	}{{"1", 36}, {"15", 37}, {"15f", 38}} {
		command := "metasystem report stop-status --id " + test.alias
		if len(command) != test.bytes {
			t.Fatalf("command %q has %d bytes, want %d", command, len(command), test.bytes)
		}
	}

	fullAttempt := strings.Repeat("a", 32)
	for length := 1; length < 32; length++ {
		occupied := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", "aliases", fullAttempt[:length]+".json")
		if err := os.WriteFile(occupied, []byte("occupied\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fullID := SessionKey("claude", "full") + "-" + fullAttempt
	full, err := ReserveShortestAlias(root, fullID)
	if err != nil || len(full.Alias) != 32 || len("metasystem report stop-status --id "+full.Alias) != 67 {
		t.Fatalf("full reservation = %+v, %v", full, err)
	}
	if _, err := ReserveShortestAlias(root, SessionKey("claude", "exhausted")+"-"+fullAttempt); err == nil {
		t.Fatal("all occupied prefixes were reused")
	}
}

func TestStopAliasIsExactAndNeverReassigned(t *testing.T) {
	root := storageRoot(t)
	attempts := []string{"1" + strings.Repeat("a", 31), "15" + strings.Repeat("b", 30)}
	reservations := make([]Reservation, 2)
	ids := make([]string, 2)
	reports := make([][]byte, 2)
	for index := range attempts {
		ids[index], reports[index] = reportFixture(t, root, fmt.Sprintf("concurrent-%d", index), attempts[index])
	}
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for index := range attempts {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			reservation, err := ReserveShortestAlias(root, ids[index])
			if err == nil {
				digest := sha256.Sum256(reports[index])
				err = PublishAlias(reservation, hex.EncodeToString(digest[:]))
			}
			reservations[index] = reservation
			errors <- err
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if reservations[0].Alias == reservations[1].Alias {
		t.Fatalf("concurrent reports shared alias %q", reservations[0].Alias)
	}
	for _, reservation := range reservations {
		_, _, resolved, err := Read(root, reservation.Alias)
		if err != nil || resolved.ID != reservation.ReportID {
			t.Fatalf("alias %s resolved to %+v: %v", reservation.Alias, resolved, err)
		}
	}

	first := reservations[0]
	if err := os.Remove(filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", first.ReportID+".md")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Read(root, first.Alias); err == nil {
		t.Fatal("expired alias read a report")
	}
	newID := SessionKey("claude", "later") + "-" + attempts[0]
	newReservation, err := ReserveShortestAlias(root, newID)
	if err != nil {
		t.Fatal(err)
	}
	if newReservation.Alias == first.Alias {
		t.Fatalf("expired alias %q was reassigned", first.Alias)
	}
	if _, err := ReserveShortestAlias(root, SessionKey("claude", "orphan")+"-f"+strings.Repeat("0", 31)); err != nil {
		t.Fatal(err)
	}
	if reused, err := ReserveShortestAlias(root, SessionKey("claude", "after-orphan")+"-f"+strings.Repeat("1", 31)); err != nil || reused.Alias == "f" {
		t.Fatalf("orphan reservation was reused: %+v, %v", reused, err)
	}
}

func TestStopAliasRetainsFullIdentityVerification(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-installation")
	if _, _, _, err := Read(missing, "a"); err == nil {
		t.Fatal("missing alias unexpectedly resolved")
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatalf("read-only lookup created storage: %v", err)
	}

	t.Run("installation spelling may contain an ancestor symlink", func(t *testing.T) {
		physicalParent := storageRoot(t)
		physicalRoot := filepath.Join(physicalParent, "installation")
		if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		linkedParent := filepath.Join(t.TempDir(), "linked-parent")
		if err := os.Symlink(physicalParent, linkedParent); err != nil {
			t.Fatal(err)
		}
		linkedRoot := filepath.Join(linkedParent, "installation")
		reservation, id, data := bindFixture(t, linkedRoot, "linked-installation", "9"+strings.Repeat("a", 31))
		got, identity, resolution, err := Read(linkedRoot, reservation.Alias)
		if err != nil || string(got) != string(data) || identity.Installation != linkedRoot || resolution.ID != id {
			t.Fatalf("ancestor-symlink installation read = identity %+v resolution %+v err %v", identity, resolution, err)
		}
	})

	t.Run("directories redirected below the installation fail closed", func(t *testing.T) {
		root := storageRoot(t)
		reportParent := filepath.Join(root, "artifacts", "agents", "supervision")
		if err := os.MkdirAll(reportParent, 0o755); err != nil {
			t.Fatal(err)
		}
		outsideReports := t.TempDir()
		if err := os.Symlink(outsideReports, filepath.Join(reportParent, "stop-verdicts")); err != nil {
			t.Fatal(err)
		}
		if _, err := ReportDir(root); err == nil {
			t.Fatal("report directory redirected outside the installation was accepted")
		}
		entries, err := os.ReadDir(outsideReports)
		if err != nil || len(entries) != 0 {
			t.Fatalf("redirected report directory received state: entries=%v err=%v", entries, err)
		}

		root = storageRoot(t)
		reportDir, err := ReportDir(root)
		if err != nil {
			t.Fatal(err)
		}
		outsideAliases := t.TempDir()
		if err := os.Symlink(outsideAliases, filepath.Join(reportDir, "aliases")); err != nil {
			t.Fatal(err)
		}
		id, _ := reportFixture(t, root, "redirected-alias", "8"+strings.Repeat("b", 31))
		if _, err := ReserveShortestAlias(root, id); err == nil {
			t.Fatal("alias directory redirected outside the installation was accepted")
		}
		entries, err = os.ReadDir(outsideAliases)
		if err != nil || len(entries) != 0 {
			t.Fatalf("redirected alias directory received state: entries=%v err=%v", entries, err)
		}
	})

	root := storageRoot(t)
	reservation, id, data := bindFixture(t, root, "binding", "a"+strings.Repeat("b", 31))
	got, identity, resolution, err := Read(root, reservation.Alias)
	if err != nil || string(got) != string(data) || identity.Session != "binding" || resolution.ID != id {
		t.Fatalf("alias read = identity %+v resolution %+v err %v", identity, resolution, err)
	}
	legacy, _, legacyResolution, err := Read(root, id)
	if err != nil || string(legacy) != string(data) || legacyResolution.Alias != "" {
		t.Fatalf("legacy full-ID read failed: %+v %v", legacyResolution, err)
	}

	aliasPath := reservation.Path
	original, _ := os.ReadFile(aliasPath)
	badBindings := []string{
		strings.Replace(string(original), `"alias":"`+reservation.Alias+`"`, `"alias":"f"`, 1),
		strings.Replace(string(original), `"reportId":"`, `"reportId":"0`, 1),
		strings.Replace(string(original), `"sha256":"`, `"sha256":"0`, 1),
		`{"schemaVersion":1,"schemaVersion":1,"state":"published","alias":"` + reservation.Alias + `","reportId":"` + id + `","sha256":"` + strings.Repeat("0", 64) + `"}`,
	}
	for index, binding := range badBindings {
		if err := os.WriteFile(aliasPath, []byte(binding), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Read(root, reservation.Alias); err == nil {
			t.Fatalf("damaged binding %d was accepted", index)
		}
	}
	if err := os.WriteFile(aliasPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", id+".md"), append(data, 'x'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Read(root, reservation.Alias); err == nil {
		t.Fatal("modified report passed alias digest verification")
	}

	t.Run("alias and report symlinks fail closed", func(t *testing.T) {
		root := storageRoot(t)
		reservation, id, data := bindFixture(t, root, "symlinks", "c"+strings.Repeat("d", 31))
		outsideAlias := filepath.Join(t.TempDir(), "alias.json")
		binding, _ := os.ReadFile(reservation.Path)
		if err := os.WriteFile(outsideAlias, binding, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(reservation.Path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideAlias, reservation.Path); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Read(root, reservation.Alias); err == nil {
			t.Fatal("symlinked alias was accepted")
		}
		if err := os.Remove(reservation.Path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reservation.Path, binding, 0o600); err != nil {
			t.Fatal(err)
		}
		reportPath := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", id+".md")
		outsideReport := filepath.Join(t.TempDir(), "report.md")
		if err := os.WriteFile(outsideReport, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(reportPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideReport, reportPath); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Read(root, reservation.Alias); err == nil {
			t.Fatal("symlinked report was accepted")
		}
	})

	t.Run("installations keep identical aliases isolated", func(t *testing.T) {
		left := storageRoot(t)
		right := storageRoot(t)
		leftReservation, leftID, _ := bindFixture(t, left, "left", "e"+strings.Repeat("0", 31))
		rightReservation, rightID, _ := bindFixture(t, right, "right", "e"+strings.Repeat("1", 31))
		if leftReservation.Alias != "e" || rightReservation.Alias != "e" {
			t.Fatalf("installation-local shortest aliases = %q, %q", leftReservation.Alias, rightReservation.Alias)
		}
		_, _, leftResolution, leftErr := Read(left, "e")
		_, _, rightResolution, rightErr := Read(right, "e")
		if leftErr != nil || rightErr != nil || leftResolution.ID != leftID || rightResolution.ID != rightID || leftResolution.ID == rightResolution.ID {
			t.Fatalf("installation-local resolution: left=%+v/%v right=%+v/%v", leftResolution, leftErr, rightResolution, rightErr)
		}
	})
}
