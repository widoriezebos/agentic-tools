package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func refusalEvent(id string) ReadRefusal {
	return ReadRefusal{
		ID: id, Reason: redundantReadRefusal, Role: "code-critic", CriticRoot: "critic", Round: 1,
		Subject: admissionLiveSubject("implementer", "implementer", "a", "1"), RefusedAt: "2026-09-13T10:00:00Z",
	}
}

func appendUnderRegisterLock(t *testing.T, repo, path string, event ReadRefusal) (bool, error) {
	t.Helper()
	var durable bool
	_, err := withFindingRegisterLock(repo, func() (string, error) {
		var appendErr error
		durable, appendErr = appendReadRefusalLocked(path, repo, event)
		return "", appendErr
	})
	return durable, err
}

func TestReadRefusalAppendUnderLock(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "artifacts", "agents", "critic", "reads-refused.jsonl")
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"event-one", "event-two"} {
		id := id
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := appendUnderRegisterLock(t, repo, path, refusalEvent(id))
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadReadRefusals(path)
	if err != nil || len(loaded) != 2 {
		t.Fatalf("concurrent append = %+v, %v", loaded, err)
	}

	event := refusalEvent("same-event")
	if _, err := appendUnderRegisterLock(t, repo, path, event); err != nil {
		t.Fatal(err)
	}
	durable, err := appendUnderRegisterLock(t, repo, path, event)
	if err != nil || durable {
		t.Fatalf("same-id replay = durable %v, err %v; reading old bytes cannot prove durability", durable, err)
	}
	loaded, err = LoadReadRefusals(path)
	if err != nil || len(loaded) != 3 {
		t.Fatalf("same-id replay appended twice: %+v, %v", loaded, err)
	}
	conflict := event
	conflict.RefusedAt = "2026-09-13T10:00:01Z"
	if _, err := appendUnderRegisterLock(t, repo, path, conflict); err == nil || !strings.Contains(err.Error(), "same-event") {
		t.Fatalf("conflicting same id = %v", err)
	}
	validBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corruptPath := filepath.Join(repo, "artifacts", "agents", "corrupt", "reads-refused.jsonl")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corruptPath, append(validBytes, []byte("not-json\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := appendUnderRegisterLock(t, repo, corruptPath, refusalEvent("event-one")); err == nil || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("same-id replay accepted malformed trailing evidence: %v", err)
	}

	mirror := filepath.Join(repo, "mirror.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mirror, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadReadRefusals(path, mirror)
	if err != nil || len(loaded) != 3 {
		t.Fatalf("local plus mirror = %+v, %v", loaded, err)
	}
}

func TestReadRefusalAppendPublicationOutcomes(t *testing.T) {
	originalWriter := writeReadRefusals
	t.Cleanup(func() { writeReadRefusals = originalWriter })

	t.Run("pre-publication-error", func(t *testing.T) {
		repo := t.TempDir()
		path := filepath.Join(repo, "artifacts", "agents", "critic", "reads-refused.jsonl")
		if _, err := appendUnderRegisterLock(t, repo, path, refusalEvent("old")); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeReadRefusals = func(string, string, string) (bool, error) {
			return false, errors.New("injected pre-publication failure")
		}
		durable, err := appendUnderRegisterLock(t, repo, path, refusalEvent("new"))
		if err == nil || durable {
			t.Fatalf("pre-publication append = durable %v, err %v", durable, err)
		}
		after, readErr := os.ReadFile(path)
		if readErr != nil || string(after) != string(before) {
			t.Fatalf("pre-publication failure changed bytes: %v", readErr)
		}

		cleanRepo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, cleanRepo, "critic", "code-critic", 1, subject, true)
		result, admissionErr := CritiqueReadAdmission(cleanRepo, "code-critic", "candidate", 1, subject)
		assertReadRefusal(t, result, admissionErr, redundantReadRefusal, "critic", 1)
		if result.EventRecorded || result.EventDurable || result.EventID == "" || !strings.Contains(admissionErr.Error(), "pre-publication") {
			t.Fatalf("event failure result = %+v, %v", result, admissionErr)
		}
	})

	t.Run("published-with-durability-doubt", func(t *testing.T) {
		writeReadRefusals = func(path, text, _ string) (bool, error) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return false, err
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				return false, err
			}
			return false, nil
		}
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, true)
		result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		assertReadRefusal(t, result, err, redundantReadRefusal, "critic", 1)
		if !result.EventRecorded || result.EventDurable || result.EventID == "" || !strings.Contains(err.Error(), "durability") {
			t.Fatalf("doubted publication = %+v, %v", result, err)
		}
		path := filepath.Join(repo, "artifacts", "agents", "critic", "reads-refused.jsonl")
		events, loadErr := LoadReadRefusals(path)
		if loadErr != nil || len(events) != 1 || events[0].ID != result.EventID {
			t.Fatalf("published event = %+v, %v", events, loadErr)
		}
	})
}
