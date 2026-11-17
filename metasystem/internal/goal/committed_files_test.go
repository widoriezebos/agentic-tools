package goal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func memoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCommittedFilesMemoKeysAndReads(t *testing.T) {
	t.Parallel()
	var memo committedTreeMemo
	root := memoRoot(t)
	tip := strings.Repeat("a", 40)
	calls := 0
	run := func(input []byte, args ...string) (string, error) {
		calls++
		if args[0] != "--no-replace-objects" {
			t.Fatalf("replacement enabled: %v", args)
		}
		switch args[1] {
		case "ls-tree":
			return "plans/goals/backlog.md\n", nil
		case "cat-file":
			if !strings.HasSuffix(string(input), ":./plans/goals/backlog.md\n") {
				t.Fatalf("input %q", input)
			}
			return "blob-id blob 4\nroot\n", nil
		}
		t.Fatalf("unexpected Git: %v", args)
		return "", nil
	}
	read := func(root, revision string, prefixes ...string) {
		t.Helper()
		files, err := memo.files(context.Background(), root, revision, prefixes, run)
		if err != nil || string(files[goalsPrefix+"backlog.md"]) != "root" {
			t.Fatalf("files=%q err=%v", files, err)
		}
	}
	read(root, tip, goalsPrefix, recordsGoalsPrefix)
	read(root, tip, recordsGoalsPrefix, goalsPrefix, goalsPrefix)
	if calls != 2 {
		t.Fatalf("same prefix set started %d Git calls, want one listing and one batch", calls)
	}
	read(root, tip, goalsPrefix+"backlog.md")
	read(memoRoot(t), tip, goalsPrefix, recordsGoalsPrefix)
	if calls != 6 {
		t.Fatalf("different prefix/repository shared reads: %d", calls)
	}
	for _, ref := range []string{"HEAD", "refs/heads/main", tip[:12], strings.Repeat("z", 40)} {
		read(root, ref, goalsPrefix)
		read(root, ref, goalsPrefix)
	}
	if calls != 22 {
		t.Fatalf("mutable/non-ID requests cached: %d", calls)
	}
	read(root, strings.Repeat("b", 64), goalsPrefix)
	read(root, strings.Repeat("b", 64), goalsPrefix)
	if calls != 24 {
		t.Fatalf("SHA-256 not cached: %d", calls)
	}
}

func TestCommittedFilesMemoFailureAndBound(t *testing.T) {
	t.Parallel()
	var memo committedTreeMemo
	ctx := context.Background()
	key := committedTreeKey{commit: "first"}
	boom := errors.New("unavailable object")
	for i := 0; i < 2; i++ {
		if _, err := memo.read(ctx, key, func() (map[string][]byte, error) { return nil, boom }); !errors.Is(err, boom) {
			t.Fatal(err)
		}
	}
	calls := 0
	load := func() (map[string][]byte, error) { calls++; return map[string][]byte{"p": []byte("content")}, nil }
	for i := 0; i < committedTreeLimit; i++ {
		if _, err := memo.read(ctx, committedTreeKey{commit: fmt.Sprint(i)}, load); err != nil {
			t.Fatal(err)
		}
	}
	// Touch the oldest entry, then overflow: entry 1 must leave, entry 0 stays.
	memo.read(ctx, committedTreeKey{commit: "0"}, load)
	memo.read(ctx, key, load)
	memo.read(ctx, committedTreeKey{commit: "0"}, load)
	if memo.entries[committedTreeKey{commit: "1"}] != nil || len(memo.entries) != committedTreeLimit || calls != committedTreeLimit+1 || memo.hits != 2 || memo.misses != uint64(committedTreeLimit+3) {
		t.Fatalf("bound/retry/LRU: entries=%d calls=%d hits=%d misses=%d", len(memo.entries), calls, memo.hits, memo.misses)
	}
	memo.read(ctx, committedTreeKey{commit: "1"}, load)
	if calls != committedTreeLimit+2 {
		t.Fatal("evicted entry did not reload")
	}
}

// A Done call proves that the waiter has joined the in-flight read, without
// sleeps or depending on which goroutine the scheduler happens to run first.
type joinedReadContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *joinedReadContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}

func receiveMemoRead(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("memo read did not return")
		return nil
	}
}

func TestCommittedFilesMemoConcurrentCancellation(t *testing.T) {
	t.Parallel()
	for _, cancelProducer := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelProducer), func(t *testing.T) {
			var memo committedTreeMemo
			producerCtx, cancelRead := context.WithCancel(context.Background())
			defer cancelRead()
			waiterCtx, cancelWait := context.WithCancel(context.Background())
			defer cancelWait()
			waiter := &joinedReadContext{Context: waiterCtx, joined: make(chan struct{})}
			started, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			key := committedTreeKey{commit: "same"}
			producerResult, waiterResult := make(chan error, 1), make(chan error, 1)
			go func() {
				_, err := memo.read(producerCtx, key, func() (map[string][]byte, error) {
					calls.Add(1)
					close(started)
					select {
					case <-producerCtx.Done():
						return nil, producerCtx.Err()
					case <-release:
						return map[string][]byte{"p": []byte("original")}, nil
					}
				})
				producerResult <- err
			}()
			<-started
			go func() {
				files, err := memo.read(waiter, key, func() (map[string][]byte, error) { calls.Add(1); return map[string][]byte{"p": []byte("retried")}, nil })
				if err == nil && string(files["p"]) != "retried" {
					err = fmt.Errorf("waiter got %q", files)
				}
				waiterResult <- err
			}()
			<-waiter.joined
			if cancelProducer {
				cancelRead()
				if err := receiveMemoRead(t, producerResult); !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if err := receiveMemoRead(t, waiterResult); err != nil {
					t.Fatal(err)
				}
				if calls.Load() != 2 || memo.misses != 2 || len(memo.entries) != 1 {
					t.Fatal("cancelled producer was retained or waiter did not retry")
				}
			} else {
				cancelWait()
				if err := receiveMemoRead(t, waiterResult); !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				select {
				case <-producerResult:
					t.Fatal("waiter stopped producer")
				default:
				}
				close(release)
				if err := receiveMemoRead(t, producerResult); err != nil {
					t.Fatal(err)
				}
				if calls.Load() != 1 {
					t.Fatal("cancelled waiter started a read")
				}
			}
		})
	}
}

func TestCommittedFilesMemoConcurrentSuccess(t *testing.T) {
	t.Parallel()
	var memo committedTreeMemo
	key := committedTreeKey{commit: "same"}
	started, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 12)
	var calls atomic.Int32
	load := func() (map[string][]byte, error) {
		calls.Add(1)
		close(started)
		<-release
		return map[string][]byte{"p": []byte("content")}, nil
	}
	go func() { _, err := memo.read(context.Background(), key, load); result <- err }()
	<-started
	for i := 0; i < 11; i++ {
		ctx := &joinedReadContext{Context: context.Background(), joined: make(chan struct{})}
		go func() {
			files, err := memo.read(ctx, key, load)
			if err == nil && !reflect.DeepEqual(files, map[string][]byte{"p": []byte("content")}) {
				err = fmt.Errorf("files %q", files)
			}
			result <- err
		}()
		<-ctx.joined
	}
	close(release)
	for i := 0; i < 12; i++ {
		if err := receiveMemoRead(t, result); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 || memo.hits != 11 || memo.misses != 1 {
		t.Fatalf("calls=%d hits=%d misses=%d", calls.Load(), memo.hits, memo.misses)
	}
}

func TestCommittedFilesIncompleteBatchIsNotMemoised(t *testing.T) {
	t.Parallel()
	var memo committedTreeMemo
	root := memoRoot(t)
	calls := 0
	run := func(_ []byte, args ...string) (string, error) {
		calls++
		if args[1] == "ls-tree" {
			return "p\n", nil
		}
		if calls == 2 {
			return "id blob 4\nx", nil
		}
		return "id blob 4\nokay\n", nil
	}
	tip := strings.Repeat("c", 40)
	if _, err := memo.files(context.Background(), root, tip, []string{"p"}, run); err == nil {
		t.Fatal("truncated read succeeded")
	}
	files, err := memo.files(context.Background(), root, tip, []string{"p"}, run)
	if err != nil || string(files["p"]) != "okay" || calls != 4 {
		t.Fatalf("files=%q err=%v calls=%d", files, err, calls)
	}
}
