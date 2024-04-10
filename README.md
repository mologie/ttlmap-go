# ttlmap-go

This is `sync.Map` with generics and automatic expiration of entries.
It's useful for implementing time-bound caches.

Features:

* Guarantees that entries live at least as long as a set timeout
* Thread-safe (using Go's `sync.Map` internally)
* Generics!
* No dependencies
* No close method, can be forgotten and GC'd anytime
* No footguns, API cannot be misused
* Full test coverage

This is yet another TTL map because existing implementations did not quite fit
my need. If you need timeouts per item, then also check out
[mailgun's implementation](https://github.com/mailgun/holster)! It takes a bit
of a different approach, e.g. deletes on access or upon reaching a capacity limit.

## Usage

```go
import "github.com/mologie/ttlmap-go"

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
```

See [cmd/ttlmap-demo/main.go](cmd/ttlmap-demo/main.go) for a more elaborate demo involving `Reaper`,
which cleans up entries during periods without write activity.

## License

This library is provided under the terms of the MIT license.
