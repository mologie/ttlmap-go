package ttlmap

import "time"

// Reaper regularly expunges expired entries from a TTLMap
type Reaper struct {
	ticker *time.Ticker
	stop   chan struct{}
}

// NewReaper creates a new Reaper that will clean the map with a reasonable default interval.
func NewReaper[K comparable, V any](m *TTLMap[K, V]) *Reaper {
	return NewReaperWithInterval(m, m.timeout/2)
}

// NewReaperWithInterval creates a new Reaper that will clean the map with a custom interval.
func NewReaperWithInterval[K comparable, V any](m *TTLMap[K, V], d time.Duration) *Reaper {
	ticker := time.NewTicker(d)
	result := &Reaper{ticker: ticker, stop: make(chan struct{})}
	go result.run(m.CleanNow)
	return result
}

func (b *Reaper) run(expire func()) {
	for {
		select {
		case <-b.ticker.C:
			expire()
		case <-b.stop:
			return
		}
	}
}

func (b *Reaper) Close() {
	b.ticker.Stop()
	close(b.stop)
}
