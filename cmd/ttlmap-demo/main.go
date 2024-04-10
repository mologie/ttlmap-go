package main

import (
	"github.com/mologie/ttlmap-go"
	"math/rand/v2"
	"time"
)

func main() {
	demoFromReadme()
	demoReaper()
}

func demoFromReadme() {
	println("DEMO 1: readme example")

	m := ttlmap.New[string, string](1 * time.Second)

	m.Store("a", "foo")
	time.Sleep(2 * time.Second)

	m.Store("b", "bar")
	time.Sleep(2 * time.Second)

	if _, ok := m.Load("a"); ok {
		panic("unreachable")
	} else {
		println("entry a does not exist, because b's write scheduled cleanup")
	}

	if _, ok := m.Load("b"); ok {
		println("entry b still exists despite having expired, because no write happened after it")
	} else {
		panic("unreachable")
	}
}

func demoReaper() {
	println("DEMO 2: reaper example")

	m := ttlmap.New[int, int](1*time.Second, ttlmap.WithExpirationCallback(func(key int, value int) {
		println("expired:", key, value)
	}))

	// OPTIONAL: Start a reaper to clean up expired entries while there is no write activity.
	// In contrast to TTLMap, this one must be closed explicitly to release its resources.
	reaper := ttlmap.NewReaper(m)
	defer reaper.Close()

	for {
		key := rand.N(1000)
		val := rand.N(1000)
		m.Store(key, val)
		d := time.Duration(rand.N(100)) * time.Millisecond
		println("stored:", key, val, "- sleeping for:", d/time.Millisecond, "ms")
		time.Sleep(d)

		if rand.N(1000) < 10 {
			println("sleeping 3s, reaper will continue to run")
			time.Sleep(3 * time.Second)
			count := 0
			m.Range(func(key int, value int) bool {
				count++
				println("range:", key, value)
				return true
			})
			println("count, expecting zero:", count)
			if count != 0 {
				panic("count != 0")
			}
		}
	}
}
