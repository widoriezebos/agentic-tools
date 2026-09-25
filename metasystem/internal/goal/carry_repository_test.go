package goal

import (
	"fmt"
	"testing"
)

func declareCarryHistory(t *testing.T, endpoint Endpoint, revision, key, value, commit string) {
	t.Helper()
	f, ok := endpoint.Repository.(*fakeGoalRepository)
	if !ok {
		t.Fatal("carry history fixture needs a fake repository")
	}
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if f.historyQueries == nil {
		f.historyQueries = make(map[fakeHistoryQuery]*fakeHistoryReply)
		f.historyT = t
		t.Cleanup(func() {
			f.store.mu.Lock()
			defer f.store.mu.Unlock()
			for query, reply := range f.historyQueries {
				if reply.used == 0 {
					t.Errorf("unused carry history query: revision=%q key=%q value=%q", query.revision, query.key, query.value)
				}
			}
		})
	}
	query := fakeHistoryQuery{revision, key, value}
	if existing, found := f.historyQueries[query]; found {
		if existing.commit != commit {
			t.Fatalf("conflicting carry history reply for %+v: %q and %q", query, existing.commit, commit)
		}
		return
	}
	f.historyQueries[query] = &fakeHistoryReply{commit: commit}
}

func declareCarryAt(t *testing.T, endpoint Endpoint, codeTip, word, anchor, carried string) {
	t.Helper()
	declareCarryHistory(t, endpoint, codeTip, "Goal-Transaction", word, anchor)
	if anchor != "" {
		declareCarryHistory(t, endpoint, anchor+".."+codeTip, "Carry", word, carried)
	}
}

func fakeCodeCarryCommit(t *testing.T, endpoint Endpoint, word string) string {
	t.Helper()
	f, ok := endpoint.Repository.(*fakeGoalRepository)
	if !ok {
		t.Fatal("code push fixture needs a fake repository")
	}
	parent, err := f.Capture(word)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := f.Build(word+"-code", parent, []Change{{Path: fmt.Sprintf("carried-%s.txt", word), Content: []byte("carried\n")}}, "carried change")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.Publish(parent, commit); err != nil || outcome != CASLanded {
		t.Fatalf("code push: %v %v", outcome, err)
	}
	return commit
}

func declareWordHistory(t *testing.T, endpoint Endpoint, word, carried string) {
	t.Helper()
	f := endpoint.Repository.(*fakeGoalRepository)
	f.store.mu.Lock()
	codeTip := f.store.canonical
	anchor := ""
	for commit, fact := range f.store.commits {
		if fact.trailer == word {
			anchor = commit
			break
		}
	}
	f.store.mu.Unlock()
	if anchor == "" {
		t.Fatalf("carry word %s has no transaction commit in the fake history", word)
	}
	declareCarryAt(t, endpoint, codeTip, word, anchor, carried)
}
