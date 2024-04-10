package ttlmap

import (
	"sync"
	"sync/atomic"
	"time"
)

// Reference point for nanosecond deltas from Go's monotonic clock in atomic variables.
var startTime = time.Now()

// These variables are replaced for deterministic testing.
var wrapNow = time.Now
var wrapGo = func(f func()) { go f() }

func nowNanos() int64 {
	return wrapNow().Sub(startTime).Nanoseconds()
}

type Option[K, V any] func(*TTLMap[K, V])

func New[K comparable, V any](timeout time.Duration, options ...Option[K, V]) *TTLMap[K, V] {
	m := &TTLMap[K, V]{timeout: timeout}
	m.lastRun.Store(nowNanos()) // to avoid cleanup on first write
	for _, opt := range options {
		opt(m)
	}
	return m
}

func WithExpirationCallback[K comparable, V any](f func(key K, value V)) Option[K, V] {
	return func(m *TTLMap[K, V]) {
		m.expired = f
	}
}

// TTLMap provides a thread-safe map which eventually deletes its entries after timeout, as long as
// there are occasional writes to the map. Its semantics are otherwise identical to sync.Map.
// A TTLMap must not be copied after first use.
// When writes are very seldom, then Reaper may be used for periodic cleanup.
type TTLMap[K, V any] struct {
	timeout  time.Duration
	expired  func(key K, value V)
	m        sync.Map
	cleaning atomic.Bool
	lastRun  atomic.Int64 // unix nanos
}

type entry[T any] struct {
	inner   T
	created time.Time
}

func (m *TTLMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

func (m *TTLMap[K, V]) Load(key K) (value V, ok bool) {
	var raw any
	if raw, ok = m.m.Load(key); ok {
		value = raw.(*entry[V]).inner
	}
	return
}

func (m *TTLMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	var raw any
	if raw, loaded = m.m.LoadAndDelete(key); loaded {
		value = raw.(*entry[V]).inner
	}
	return
}

func (m *TTLMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	var raw any
	if raw, loaded = m.m.LoadOrStore(key, &entry[V]{inner: value, created: wrapNow()}); loaded {
		actual = raw.(*entry[V]).inner
	} else {
		actual = value
		m.hit()
	}
	return
}

func (m *TTLMap[K, V]) range_(f func(key K, value *entry[V]) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(*entry[V]))
	})
}

func (m *TTLMap[K, V]) Range(f func(key K, value V) bool) {
	m.range_(func(key K, value *entry[V]) bool {
		return f(key, value.inner)
	})
}

func (m *TTLMap[K, V]) Store(key K, value V) {
	m.m.Store(key, &entry[V]{inner: value, created: wrapNow()})
	m.hit()
}

func (m *TTLMap[K, V]) hit() {
	lastRun := m.lastRun.Load()
	nextRun := lastRun + int64(m.timeout/2)
	if nextRun < nowNanos() && m.lastRun.CompareAndSwap(lastRun, nextRun) {
		wrapGo(m.CleanNow)
	}
}

// CleanNow synchronously cleans up most expired entries. Entries that are inserted and expire
// while CleanNow is running may be skipped. If a concurrent CleanNow is already running, CleanNow
// returns immediately without performing any work.
func (m *TTLMap[K, V]) CleanNow() {
	if !m.cleaning.CompareAndSwap(false, true) {
		return
	}
	expireBefore := wrapNow().Add(-m.timeout)
	defer m.cleaning.Store(false)
	m.range_(func(key K, value *entry[V]) bool {
		if value.created.Before(expireBefore) {
			_, deleted := m.m.LoadAndDelete(key)
			if m.expired != nil && deleted {
				m.expired(key, value.inner)
			}
		}
		return true
	})
}
