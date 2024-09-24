// Package cache implements an in-memory [templ] component cache. This may offer performance
// improvements for applications with slow or deeply-nested components. To use,
// create an instance of the cache and wrap the desired component:
//
//	var cache = New()
//
//	templ MyPage() {
//		@cache("my_key") {
//			@ExpensiveComponent()
//		}
//	}
//
// # Details
//
// The rendered component will be cached and associated with the given key. The key should
// be unique for the wrapped component. Any string can be used, so consider deriving
// the key from parameters the component depends on. For example:
//
//	templ CheckoutPage(user_id int) {
//		@cache(fmt.Sprintf("item_list-%d", user_id)) {
//			@ItemList(user_id)
//		}
//	}
//
// The cache defaults to 64MB of storage and a 5 minute time-to-live (TTL) for items. Once the
// storage limit is reached, the least recently used items will be deleted. When a cached item
// expires, it will be re-rendered when next needed. The storage and TTL are configurable when
// the cache is created by including the [WithTTL] or [WithMaxMemory] options. The TTL is also
// settable at the component level in the template as an override:
//
//	// Set memory and default TTL
//	var cache = New(WithMaxMemory(512000), WithTTL(5*time.Minute))
//
//	templ Homepage() {
//		@cache("menu") {
//			This will be cached for 5 minutes.
//		}
//
//		@cache("stock-quote", WithTTL(30*time.Second)) {
//			This is rerendered every 30 seconds.
//		}
//	}
//
// The cache has functions for use outside of a template to access stats, reset, disable, etc.
// To use these functions, first obtain a component instance with any key:
//
//	cacheCtl := cache("")              // any key works
//	cacheCtl.Remove("key_to_remove")   // manually remove an item from the cache
//
// Cache instances (created with [New]) are independent. They don't share any memory and may
// have different settings.
package cache

import (
	"bytes"
	"context"
	"io"
	"math"
	"time"

	"github.com/a-h/templ"
)

const defaultTTL = time.Duration(5 * time.Minute)
const defaultMem = 64 * 1024 * 1024

// Component is the cache component for use in templates.
type Component struct {
	ttl         time.Duration
	key         string
	initialized bool
	lru         *lru
}

type Option func(c *Component)

// ComponentBuilder creates CacheComponents for use in templates.
//
// See the package documentation for usage examples.
type ComponentBuilder func(key string, opts ...Option) Component

// New creates a cache and returns a builder function
// that can be used in templates. It accepts zero or more functional
// options (WithTTL(), WithMaxMemory()).
func New(opts ...Option) ComponentBuilder {
	base := Component{
		ttl: defaultTTL,
		lru: newLRU(defaultMem),
	}

	for _, opt := range opts {
		opt(&base)
	}
	base.initialized = true

	return func(key string, opts ...Option) Component {
		dupe := base
		dupe.key = key

		for _, opt := range opts {
			opt(&dupe)
		}

		return dupe
	}
}

// WithTTL sets the default expiration duration for the cache,
// or the expiration for an individual component.
func WithTTL(d time.Duration) Option {
	return func(c *Component) {
		c.ttl = d
	}
}

// WithMaxMemory sets the maximum memory (in bytes) used for the cache.
// Note that this will be ignored when set on individual components. If
// the size is 0 then there is no memory limit.
func WithMaxMemory(maxBytes int) Option {
	return func(c *Component) {
		// This can't be changed after initialization
		if c.initialized {
			return
		}

		if maxBytes == 0 {
			maxBytes = math.MaxInt
		}

		c.lru = newLRU(maxBytes)
	}
}

type Stats struct {
	MaxMemory  int // maximum configured memory
	UsedMemory int // memory used by cached items (including expired but not deleted items)
	Items      int // cached item count (including expired but not deleted items)
	Reads      int // total cache reads
	Hits       int // total cache hits
}

// Stats returns basic cache statistics. These will be reset with Reset().
func (c Component) Stats() Stats {
	l := c.lru

	return Stats{
		MaxMemory:  l.maxMem,
		UsedMemory: l.mem,
		Items:      l.list.Len(),
		Reads:      l.reads,
		Hits:       l.hits,
	}
}

// Remove removes/invalidates the cached data for associated with key, if it exists.
func (c Component) Remove(key string) {
	c.lru.deleteKey(key)
}

// Disable will turn off (or back on) caching. This also has the effect of wiping the cache.
func (c *Component) Disable(disable bool) {
	if disable {
		c.lru.reset()
	}

	c.lru.disabled = disable
}

// Reset erases the cache and resets statistics.
func (c *Component) Reset() {
	c.lru.reset()
}

// Render will render child components, using cached data and caching results as needed.
func (c Component) Render(ctx context.Context, w io.Writer) error {
	if cc, isCached := c.lru.get(c.key); isCached {
		_, err := w.Write(cc)
		return err
	}

	// Get children.
	children := templ.GetChildren(ctx)
	ctx = templ.ClearChildren(ctx)
	if children == nil {
		return nil
	}

	// Render children to a buffer.
	var buf bytes.Buffer
	err := children.Render(ctx, &buf)
	if err != nil {
		return err
	}

	// Cache the result.
	c.lru.put(c.key, buf.Bytes(), c.ttl)

	// Write the result to the output.
	_, err = w.Write(buf.Bytes())

	return err
}
