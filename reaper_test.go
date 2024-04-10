package ttlmap

import (
	"testing"
	"time"
)

func TestNewReaperWithInterval(t *testing.T) {
	env := NewDeterministicEnv()
	defer env.Restore()
	const d = 100 * time.Millisecond

	m := New[int, int](d)
	reaper := NewReaper(m)

	m.Store(1, 1)
	time.Sleep(d)
	if _, ok := m.Load(1); !ok {
		t.Error("Expected entry 1 to still exist as clock stands still")
	}

	env.AdvanceClock(d + 1)
	time.Sleep(d) // for reaper goroutine to wake up
	if _, ok := m.Load(1); ok {
		t.Error("Expected entry 1 to expire despite lack of write activity")
	}

	reaper.Close()
	time.Sleep(d) // for reaper goroutine to exit
	m.Store(2, 2)
	time.Sleep(d) // for reaper goroutine to act if it did not exit
	if _, ok := m.Load(2); !ok {
		t.Error("Expected entry 2 to still exist after reaper stopped")
	}
}
