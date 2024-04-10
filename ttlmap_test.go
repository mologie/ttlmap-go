package ttlmap

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLMap_Delete(t *testing.T) {
	m := New[string, string](time.Millisecond)
	m.Store("foo", "bar")
	bar, ok := m.Load("foo")
	if !ok {
		t.Errorf("Expected to load 'foo'")
	}
	if bar != "bar" {
		t.Errorf("Expected 'bar', got %s", bar)
	}
	m.Delete("foo")
	m.Delete("baz")
	_, ok = m.Load("bar")
	if ok {
		t.Errorf("Expected not to load 'bar'")
	}
}

func TestTTLMap_LoadAndDelete(t *testing.T) {
	m := New[string, string](time.Millisecond)
	m.Store("foo", "bar")
	bar, ok := m.LoadAndDelete("foo")
	if !ok {
		t.Errorf("Expected to load and delete 'foo'")
	}
	if bar != "bar" {
		t.Errorf("Expected 'bar', got %s", bar)
	}
	_, ok = m.LoadAndDelete("foo")
	if ok {
		t.Errorf("Expected not to load and delete 'foo'")
	}
}

func TestTTLMap_LoadOrStore(t *testing.T) {
	m := New[string, string](time.Millisecond)
	m.Store("foo", "bar")
	bar, loaded := m.LoadOrStore("foo", "baz")
	if !loaded {
		t.Errorf("Expected 'foo' to be loaded")
	}
	if bar != "bar" {
		t.Errorf("Expected 'bar', got %s", bar)
	}
	m.Delete("foo")
	bar, loaded = m.LoadOrStore("foo", "baz")
	if loaded {
		t.Errorf("Expected 'foo' not to be loaded")
	}
	if bar != "baz" {
		t.Errorf("Expected 'baz', got %s", bar)
	}
}

func TestTTLMap_StoreAndExpire(t *testing.T) {
	env := NewDeterministicEnv()
	defer env.Restore()

	cleanupDone := make(chan struct{})
	cleanupCounter := 0
	cleanupCloseAfter := 5
	m := New[int, int](time.Millisecond, WithExpirationCallback(func(key, value int) {
		cleanupCounter++
		if cleanupCounter == cleanupCloseAfter {
			close(cleanupDone)
		} else if cleanupCounter > cleanupCloseAfter {
			t.Fatalf("Expected only %d expirations, got %d", cleanupCloseAfter, cleanupCounter)
		}
	}))

	// write n values, then read them back
	// clock stays constant, hence cleanup should not yet happen
	for i := 0; i < 5; i++ {
		m.Store(i, i+100)
	}
	for i := 0; i < 5; i++ {
		v, ok := m.Load(i)
		if !ok {
			t.Errorf("Expected to load %d", i)
		}
		if v != i+100 {
			t.Errorf("Expected %d, got %d", i+100, v)
		}
	}
	if ngo := env.goroutines.Load(); ngo != 0 {
		t.Errorf("Expected no new cleanup routines, got %d", ngo)
	}

	// advance the clock and ensure that the next write expires existing entries
	env.AdvanceClock(100 * time.Millisecond)
	m.Store(-1, 123)
	if ngo := env.goroutines.Load(); ngo != 1 {
		t.Errorf("Expected initial cleanup routine to start, got %d", ngo)
	}
	cleanupFailed := time.After(5 * time.Second)
	select {
	case <-cleanupFailed:
		t.Fatalf("Expected initial cleanup to finish within 5 seconds")
	case <-cleanupDone:
	}
	if cleanupCloseAfter != cleanupCounter {
		t.Errorf("Expected %d expirations, got %d", cleanupCloseAfter, cleanupCounter)
	}
	for i := 0; i < 5; i++ {
		_, ok := m.Load(i)
		if ok {
			t.Errorf("Expected not to load %d", i)
		}
	}
	count := 0
	m.Range(func(key, value int) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("Expected one entry (last write) to remain, got %d", count)
	}

	// now clean the last entry too
	cleanupDone = make(chan struct{})
	cleanupCounter = 0
	cleanupCloseAfter = 1
	env.AdvanceClock(100 * time.Millisecond)
	m.hit()
	if ngo := env.goroutines.Load(); ngo != 2 {
		t.Errorf("Expected final cleanup routine to start, got %d", ngo)
	}
	cleanupFailed = time.After(5 * time.Second)
	select {
	case <-cleanupFailed:
		t.Fatalf("Expected final cleanup to finish within 5 seconds")
	case <-cleanupDone:
	}
	if cleanupCounter != 1 {
		t.Errorf("Expected 1 expiration, got %d", cleanupCounter)
	}
	if _, ok := m.Load(-1); ok {
		t.Errorf("Expected not to load final entry")
	}
}

func TestConcurrentCleanup(t *testing.T) {
	// This test is rather for Go's data race detector than for correctness.
	// It should also hit the alternative branch of CleanNow's atomic swap.
	m := New[int, int](10 * time.Millisecond)
	for i := 0; i < 1000; i++ {
		m.Store(i, i)
		wrapGo(m.CleanNow)
	}
}

type DeterministicEnv struct {
	mutex      sync.Mutex
	now        time.Time
	goroutines atomic.Int32
	bakNow     func() time.Time
	bakGo      func(func())
}

func NewDeterministicEnv() *DeterministicEnv {
	env := &DeterministicEnv{now: startTime}
	env.bakNow = wrapNow
	env.bakGo = wrapGo
	wrapNow = func() time.Time {
		env.mutex.Lock()
		now := env.now
		env.mutex.Unlock()
		return now
	}
	wrapGo = func(f func()) {
		env.goroutines.Add(1)
		go f()
	}
	return env
}

func (d *DeterministicEnv) Restore() {
	wrapNow = d.bakNow
	wrapGo = d.bakGo
}

func (d *DeterministicEnv) AdvanceClock(duration time.Duration) {
	d.mutex.Lock()
	d.now = d.now.Add(duration)
	d.mutex.Unlock()
}
