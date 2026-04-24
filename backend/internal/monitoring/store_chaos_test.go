package monitoring

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChaosFileStoreSurvivesPanicMidMutator simulates abrupt termination by
// panicking inside Update mutators while many goroutines write concurrently.
// After every iteration it constructs a fresh FileStore from the same path and
// asserts the on-disk state.json (a) parses, (b) contains exactly the number
// of checks corresponding to mutators that returned cleanly, and (c) leaves no
// .tmp scratch files behind.
//
// We can't kill -9 our own goroutine, but a panic mid-mutator is the closest
// in-process equivalent to "process died with the lock held": it triggers all
// deferred unlocks without ever reaching writeLocked, so the on-disk file must
// remain at its last committed value.
func TestChaosFileStoreSurvivesPanicMidMutator(t *testing.T) {
	const iterations = 100
	const goroutinesPerIteration = 20
	const panicProbability = 3 // 1-in-N

	start := time.Now()
	var totalSuccess, totalPanics int64

	for iter := 0; iter < iterations; iter++ {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")

		store, err := NewFileStore(path, nil)
		if err != nil {
			t.Fatalf("iter %d: new store: %v", iter, err)
		}

		var wg sync.WaitGroup
		var success, panicked int64
		rngSeed := int64(iter)*1000 + 1

		for g := 0; g < goroutinesPerIteration; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicked, 1)
					}
				}()

				rng := rand.New(rand.NewSource(rngSeed + int64(g)))
				err := store.Update(func(s *State) error {
					s.Checks = append(s.Checks, CheckConfig{
						ID:     fmt.Sprintf("chaos-%d-%d", iter, g),
						Name:   "Chaos",
						Type:   "api",
						Target: "https://example.com",
					})
					if rng.Intn(panicProbability) == 0 {
						panic(fmt.Sprintf("simulated crash mid-mutator iter=%d g=%d", iter, g))
					}
					return nil
				})
				if err == nil {
					atomic.AddInt64(&success, 1)
				}
			}(g)
		}
		wg.Wait()

		atomic.AddInt64(&totalSuccess, success)
		atomic.AddInt64(&totalPanics, panicked)

		// On-disk file must be valid JSON and contain exactly success checks.
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("iter %d: read state: %v", iter, err)
		}
		var disk State
		if err := json.Unmarshal(raw, &disk); err != nil {
			t.Fatalf("iter %d: state.json corrupted (unparseable JSON): %v\n--- file content ---\n%s",
				iter, err, raw)
		}
		if int64(len(disk.Checks)) != success {
			t.Fatalf("iter %d: torn-write or lost data: want %d checks on disk, got %d (panics=%d)",
				iter, success, len(disk.Checks), panicked)
		}

		// Reload via a brand-new store and compare — exercises the cold-start path.
		store2, err := NewFileStore(path, nil)
		if err != nil {
			t.Fatalf("iter %d: cannot reload store: %v", iter, err)
		}
		reloaded := store2.Snapshot()
		if int64(len(reloaded.Checks)) != success {
			t.Fatalf("iter %d: reload count mismatch: want %d, got %d",
				iter, success, len(reloaded.Checks))
		}

		// No .tmp scratch files left behind by aborted writes.
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("iter %d: read dir: %v", iter, err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tmp") {
				t.Fatalf("iter %d: leftover scratch file %q (atomic-rename invariant violated)",
					iter, e.Name())
			}
		}
	}

	elapsed := time.Since(start)
	t.Logf("CHAOS SUMMARY: iterations=%d goroutines/iter=%d total_successes=%d total_panics=%d wall_time=%s",
		iterations, goroutinesPerIteration, totalSuccess, totalPanics, elapsed)
	if totalPanics == 0 {
		t.Fatal("test did not actually exercise panic paths — RNG seed broken")
	}
}

// TestChaosConcurrentUpdatesNoTornJSON is a tighter loop that hammers Update
// with high contention and re-parses the on-disk file mid-flight. Even with
// 200 concurrent appends, every observed snapshot of state.json must be
// parseable JSON — the atomic temp+rename should make every observer see
// either the previous or the next complete file, never a half-written one.
func TestChaosConcurrentUpdatesNoTornJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store, err := NewFileStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}

	const writers = 50
	const writes = 20
	stop := make(chan struct{})

	// Reader: continuously parses state.json. Any parse failure is fatal.
	var observations int64
	var torn int64
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var s State
			if err := json.Unmarshal(raw, &s); err != nil {
				atomic.AddInt64(&torn, 1)
				t.Errorf("torn write observed: %v\nbytes=%d head=%q", err, len(raw), string(raw[:min(80, len(raw))]))
			}
			atomic.AddInt64(&observations, 1)
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				_ = store.Update(func(s *State) error {
					s.Checks = append(s.Checks, CheckConfig{
						ID:     fmt.Sprintf("c-%d-%d", w, i),
						Name:   "n",
						Type:   "api",
						Target: "https://example.com",
					})
					return nil
				})
			}
		}(w)
	}
	wg.Wait()
	close(stop)
	// brief drain
	time.Sleep(10 * time.Millisecond)

	t.Logf("observations=%d torn=%d", observations, torn)
	if torn > 0 {
		t.Fatalf("%d torn JSON observations during %d concurrent writes (atomic-rename broken)",
			torn, writers*writes)
	}
}
