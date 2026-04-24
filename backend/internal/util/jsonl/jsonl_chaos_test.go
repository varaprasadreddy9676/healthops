package jsonl

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// marshaledItem is the on-disk shape produced by panickyItem.MarshalJSON when
// it doesn't panic. Load[marshaledItem] consumes records of this shape.
type marshaledItem struct {
	GoroutineID int `json:"goroutineId"`
	Index       int `json:"index"`
}

// panickyItem is a custom type whose MarshalJSON occasionally panics. This
// simulates the worst real-world failure mode for Append: the writer is
// killed (or panics) after the file handle is opened but before any bytes
// have been written. The contract under test is that no torn line ever
// reaches disk in that case.
type panickyItem struct {
	GoroutineID int
	Index       int
	PanicHere   bool
}

func (p panickyItem) MarshalJSON() ([]byte, error) {
	if p.PanicHere {
		panic("simulated mid-marshal crash")
	}
	return json.Marshal(marshaledItem{GoroutineID: p.GoroutineID, Index: p.Index})
}

// TestChaosAppendPanicsLeaveNoTornLines verifies the Append/Load round-trip
// invariant under heavy concurrent load with mid-marshal panics:
//
//   - Either an item is fully present in the file and parses cleanly,
//   - Or it is absent.
//   - Never partial (no "unexpected end of input" from Load).
//
// 50 goroutines * 100 appends each = 5000 attempts; ~10% panic mid-marshal
// (json.Marshal does NOT recover from MarshalerError panics, so they
// propagate up through Append before any bytes are written).
func TestChaosAppendPanicsLeaveNoTornLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	var success, panicked int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g)*7919 + 1))
			for i := 0; i < perGoroutine; i++ {
				func() {
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&panicked, 1)
						}
					}()
					item := panickyItem{
						GoroutineID: g,
						Index:       i,
						PanicHere:   rng.Intn(10) == 0,
					}
					if err := Append(path, item); err != nil {
						t.Errorf("g=%d i=%d: append: %v", g, i, err)
						return
					}
					atomic.AddInt64(&success, 1)
				}()
			}
		}(g)
	}
	wg.Wait()

	t.Logf("appended_ok=%d panicked=%d total_attempts=%d", success, panicked, goroutines*perGoroutine)
	if panicked == 0 {
		t.Fatal("no panics occurred — RNG seed broken, test would not exercise crash path")
	}

	items, err := Load[marshaledItem](path)
	if err != nil {
		t.Fatalf("Load returned error (likely torn line): %v", err)
	}
	if int64(len(items)) != atomic.LoadInt64(&success) {
		t.Fatalf("data loss or torn lines: want %d items (one per successful Append), got %d",
			success, len(items))
	}
}

// TestChaosLoadSkipsTornTrailingLine simulates the most common kill-9
// aftermath: a file containing many valid lines plus one final
// half-written line that lacks a trailing newline and a closing brace.
// Load must skip it without surfacing a JSON error.
func TestChaosLoadSkipsTornTrailingLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	for i := 0; i < 100; i++ {
		if err := Append(path, marshaledItem{GoroutineID: 1, Index: i}); err != nil {
			t.Fatalf("seed append %d: %v", i, err)
		}
	}

	// Append a deliberately malformed trailing fragment: no closing brace,
	// no newline. This is what kill -9 mid-write produces.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open for torn write: %v", err)
	}
	if _, err := f.Write([]byte(`{"goroutineId":99,"index":0,"partial":`)); err != nil {
		t.Fatalf("torn write: %v", err)
	}
	_ = f.Close()

	items, err := Load[marshaledItem](path)
	if err != nil {
		t.Fatalf("Load surfaced error on torn trailing line (must skip silently): %v", err)
	}
	if len(items) != 100 {
		t.Fatalf("torn trailing line corrupted preceding records: want 100 valid items, got %d",
			len(items))
	}
}

// TestChaosConcurrentAppendNoInterleave verifies that POSIX O_APPEND
// atomicity holds for the small (<PIPE_BUF) JSON records HealthOps writes:
// 50 goroutines * 100 concurrent Append calls must result in exactly 5000
// fully-formed lines, with no two writes interleaved within a single line.
func TestChaosConcurrentAppendNoInterleave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if err := Append(path, marshaledItem{GoroutineID: g, Index: i}); err != nil {
					t.Errorf("g=%d i=%d: append: %v", g, i, err)
				}
			}
		}(g)
	}
	wg.Wait()

	items, err := Load[marshaledItem](path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != goroutines*perGoroutine {
		t.Fatalf("interleaving or loss: want %d items, got %d",
			goroutines*perGoroutine, len(items))
	}

	// Every (g, i) pair must be present exactly once — no duplicates, no
	// truncated records that decoded by accident.
	seen := make(map[[2]int]int, goroutines*perGoroutine)
	for _, it := range items {
		seen[[2]int{it.GoroutineID, it.Index}]++
	}
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perGoroutine; i++ {
			n := seen[[2]int{g, i}]
			if n != 1 {
				t.Errorf("item g=%d i=%d: count=%d (want 1)", g, i, n)
			}
		}
	}
}
