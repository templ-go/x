package cache

import (
	"container/list"
	"sync"
	"time"
)

// lru implements a cache with Least Recently Used eviction
// policy. Expiration per item is tracked as well. Both expiration and
// eviction are handled lazily on read or write. For example, an expired
// key will be deleted when it is read, or if _evictExpired is called
// due to the memory limit being reached.
//
// All cache operations are under mutex protection.
type lru struct {
	lock               sync.RWMutex
	maxMem             int
	mem                int
	cache              map[string]*list.Element
	list               *list.List
	earliestExpiration time.Time
	disabled           bool

	// stats
	reads int
	hits  int
}

type entry struct {
	key        string
	value      []byte
	expiration time.Time
}

// size calculates the total storage for item, including 24 bytes for
// the expiration time.Time.
func (e *entry) size() int {
	return len(e.key) + len(e.value) + 24
}

func newLRU(maxMem int) *lru {
	return &lru{
		maxMem:             maxMem,
		mem:                0,
		cache:              make(map[string]*list.Element),
		list:               list.New(),
		earliestExpiration: time.Now().Add(24 * time.Hour),
	}
}

func (c *lru) reset() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.mem = 0
	c.list = list.New()
	c.cache = make(map[string]*list.Element)
}

func (c *lru) get(key string) ([]byte, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.disabled {
		return nil, false
	}

	c.reads++

	if elem, ok := c.cache[key]; ok {
		e := elem.Value.(*entry)
		if time.Now().Before(e.expiration) {
			c.hits++
			c.list.MoveToFront(elem)
			return e.value, true
		}
		c._deleteKey(e.key)
	}

	return nil, false
}

func (c *lru) put(key string, value []byte, ttl time.Duration) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.disabled {
		return
	}

	expiration := time.Now().Add(ttl)

	// Make sure the key is gone. Updating is possible but complicates size tracking.
	c._deleteKey(key)

	newEntry := &entry{key: key, value: value, expiration: expiration}
	elem := c.list.PushFront(newEntry)
	c.cache[key] = elem
	c.mem += newEntry.size()

	if expiration.Before(c.earliestExpiration) {
		c.earliestExpiration = expiration
	}

	// Bring cache size within max size
	if c.mem > c.maxMem {
		c._evictExpired()

		for c.mem > c.maxMem && c.list.Len() > 1 {
			oldest := c.list.Back()
			if oldest != nil {
				c._deleteKey(oldest.Value.(*entry).key)
			}
		}
	}
}

func (c *lru) deleteKey(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c._deleteKey(key)
}

// _deleteKey should only be called with the lock held.
func (c *lru) _deleteKey(key string) {
	elem := c.cache[key]
	if elem == nil {
		return
	}

	c.list.Remove(elem)
	e := elem.Value.(*entry)
	c.mem -= e.size()
	delete(c.cache, e.key)
}

// _evictExpired should only be called with the lock held.
func (c *lru) _evictExpired() {
	now := time.Now()
	if now.Before(c.earliestExpiration) {
		return
	}

	c.earliestExpiration = now.Add(24 * time.Hour)

	var next *list.Element
	for elem := c.list.Back(); elem != nil; elem = next {
		next = elem.Prev()

		e := elem.Value.(*entry)
		if now.After(e.expiration) {
			c._deleteKey(e.key)
		} else if e.expiration.Before(c.earliestExpiration) {
			c.earliestExpiration = e.expiration
		}
	}
}
