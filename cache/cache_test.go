package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCorrectness(t *testing.T) {
	ctx := context.Background()

	cache = New(WithTTL(10 * time.Millisecond))

	// Does it render what we expect?
	var buf bytes.Buffer
	Outer("A", "AAA").Render(ctx, &buf)
	equals(t, "AAA", buf.String())

	buf.Reset()
	Outer("B", "BBB").Render(ctx, &buf)
	equals(t, "BBB", buf.String())

	// This will be a cache read
	buf.Reset()
	Outer("A", "AAA").Render(ctx, &buf)
	equals(t, "AAA", buf.String())

	// It is actually caching?

	// This should be slow
	tRender := timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	// This should be fast
	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender < 5*time.Millisecond, "expected fast rendering")

	// Different key, so this should be slow again
	tRender = timeIt(func() { Slow("T").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	// Now fast
	tRender = timeIt(func() { Slow("T").Render(ctx, io.Discard) })
	assert(t, tRender < 5*time.Millisecond, "expected fast rendering")

	// Remove the item
	ctl := cache("")
	ctl.Remove("T")
	tRender = timeIt(func() { Slow("T").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")
}

func TestEviction(t *testing.T) {
	ctx := context.Background()

	cache = New(WithMaxMemory(100), WithTTL(50*time.Millisecond))
	ctl := cache("")

	Outer("A", "AAA").Render(ctx, io.Discard)
	Outer("B", "BBB").Render(ctx, io.Discard)
	Outer("C", "CCC").Render(ctx, io.Discard)
	Outer("D", "DDD").Render(ctx, io.Discard)

	// Only three elements will first into the cache of 100 bytes
	equals(t, 3, ctl.Stats().Items)

	// Wait long enough for everything to expire. Adding a new element
	// will trigger evictList() and purge everything.
	time.Sleep(60 * time.Millisecond)
	Outer("E", "EEE").Render(ctx, io.Discard)

	equals(t, 1, ctl.Stats().Items)
}

func TestDisable(t *testing.T) {
	ctx := context.Background()

	cache = New()
	ctl := cache("")
	ctl.Disable(true)

	// These should all be slow since the cache is disabled
	tRender := timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	// Reenable cache
	ctl.Disable(false)

	// First render will be slow
	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	// Second should be fast
	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender < 5*time.Millisecond, "expected fast rendering")
}

func TestReset(t *testing.T) {
	ctx := context.Background()

	cache = New()
	ctl := cache("")

	tRender := timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")

	// Fast response with cached data
	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender < 5*time.Millisecond, "expected fast rendering")

	assert(t, ctl.Stats().UsedMemory > 0, "expected positive memory usage")

	ctl.Reset()

	assert(t, ctl.Stats().UsedMemory == 0, "expected no memory usage")

	// Slow response following cache reset
	tRender = timeIt(func() { Slow("S").Render(ctx, io.Discard) })
	assert(t, tRender > 150*time.Millisecond, "expected slow rendering")
}

func TestMaxMemory(t *testing.T) {
	ctx := context.Background()

	cache = New(WithMaxMemory(64 * 1024))
	ctl := cache("")

	large := strings.Repeat("A", 50000)
	Outer("1", large).Render(ctx, io.Discard)
	equals(t, 50025, ctl.Stats().UsedMemory)

	Outer("2", large).Render(ctx, io.Discard)
	equals(t, 50025, ctl.Stats().UsedMemory)

	cache = New(WithMaxMemory(110000))
	ctl = cache("")

	Outer("1", large).Render(ctx, io.Discard)
	equals(t, 50025, ctl.Stats().UsedMemory)

	Outer("2", large).Render(ctx, io.Discard)
	equals(t, 2*50025, ctl.Stats().UsedMemory)
}

func TestDefaultMemory(t *testing.T) {
	ctx := context.Background()
	cache = New()
	ctl := cache("")

	large := strings.Repeat("A", 30000000)
	Outer("1", large).Render(ctx, io.Discard)
	equals(t, 30000025, ctl.Stats().UsedMemory)

	Outer("2", large).Render(ctx, io.Discard)
	equals(t, 60000050, ctl.Stats().UsedMemory)

	// This will push over the 64MB limit and evict
	// one ~30MB string.
	small := strings.Repeat("A", 10000000)
	Outer("3", small).Render(ctx, io.Discard)
	equals(t, 40000050, ctl.Stats().UsedMemory)
}

func TestLRUOrder(t *testing.T) {
	ctx := context.Background()

	cache = New(WithMaxMemory(110))

	ctl := cache("")

	equals(t, 0, ctl.Stats().UsedMemory)

	Outer("A", "AAA").Render(ctx, io.Discard)
	equals(t, 1, ctl.Stats().Items)

	Outer("A", "AAA").Render(ctx, io.Discard)
	equals(t, 1, ctl.Stats().Items)

	Outer("B", "BBB").Render(ctx, io.Discard)
	equals(t, 2, ctl.Stats().Items)

	Outer("C", "CCC").Render(ctx, io.Discard)
	equals(t, 3, ctl.Stats().Items)

	// There is only room for 3 elements so this should push one
	// out and leave the cache size unchanged
	Outer("D", "DDD").Render(ctx, io.Discard)
	equals(t, 3, ctl.Stats().Items)

	// Cache is now: D, C, B
	equals(t, "D", peekFront(ctl))
	equals(t, "B", peekBack(ctl))

	Outer("B", "BBB").Render(ctx, io.Discard)

	// Cache is now: B, D, C
	equals(t, "B", peekFront(ctl))
	equals(t, "C", peekBack(ctl))

	Outer("E", "EEE").Render(ctx, io.Discard)

	// Cache is now: E, B, D
	equals(t, "E", peekFront(ctl))
	equals(t, "D", peekBack(ctl))
}

// Test a high-concurrency situation.
//
// To check the efficacy of this test, I disabled the mutexes in the LRU and found that
// it panics... so they're definitely service a purpose!
func TestConcurrency(t *testing.T) {
	// t.Skip()
	ctx := context.Background()

	cache = New(WithMaxMemory(64 * 1024))
	ctl := cache("")

	var wg sync.WaitGroup

	// Let 100 goroutines fight over 10000 cache entries
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 10000; i++ {
				r := rand.Intn(10000)
				key := fmt.Sprintf("Key %d", r)
				val := fmt.Sprintf("Val %d", r)

				var buf bytes.Buffer
				Outer(key, val).Render(ctx, &buf)
				equals(t, val, buf.String())
			}

			wg.Done()

		}()
	}

	wg.Wait()
	reads := 100 * 10000
	equals(t, reads, ctl.Stats().Reads)
	hits := ctl.Stats().Hits

	ratio := float64(hits) / float64(reads)
	// equals(t, 100*10000, ratio)
	// Though it will vary slightly from run to run, the cache can hold
	// about 1647 items, and since we're randomly choosing from 10000
	// entries the hit rate is around 16.7%.
	assert(t, ratio > 0.15 && ratio < 0.175, "expected hit ratio near 0.167. Got %f", ratio)
}

func TestLRUTTL(t *testing.T) {
	ctx := context.Background()

	cache = New(WithTTL(200 * time.Millisecond))

	ctl := cache("")
	equals(t, 0, ctl.Stats().UsedMemory)

	Outer("A", "AAA").Render(ctx, io.Discard)
	OuterTTL("B", "BBB", 300*time.Millisecond).Render(ctx, io.Discard)

	time.Sleep(150 * time.Millisecond)

	var buf bytes.Buffer
	Outer("A", "A-updated").Render(ctx, &buf)
	equals(t, "AAA", buf.String())

	buf.Reset()
	Outer("B", "B-updated").Render(ctx, &buf)
	equals(t, "BBB", buf.String())

	time.Sleep(60 * time.Millisecond)

	buf.Reset()
	Outer("A", "A-updated").Render(ctx, &buf)
	equals(t, "A-updated", buf.String())

	buf.Reset()
	Outer("B", "B-updated").Render(ctx, &buf)
	equals(t, "BBB", buf.String())

	time.Sleep(100 * time.Millisecond)

	buf.Reset()
	Outer("B", "B-updated").Render(ctx, &buf)
	equals(t, "B-updated", buf.String())
}

func timeIt(f func()) time.Duration {
	start := time.Now()

	f()

	return time.Since(start)
}

func peekFront(c Component) string {
	return c.lru.list.Front().Value.(*entry).key
}

func peekBack(c Component) string {
	return c.lru.list.Back().Value.(*entry).key
}

/*
 *  Testing helpers, courtesy of https://github.com/benbjohnson/testing
 */

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
